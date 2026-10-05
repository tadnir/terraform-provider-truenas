// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &AppResource{}
var _ resource.ResourceWithImportState = &AppResource{}
var _ resource.ResourceWithIdentity = &AppResource{}

// AppResource manages a single TrueNAS app (Docker-based, TrueNAS 24.10+).
type AppResource struct{ client *client.Client }

// NewResource returns a new AppResource.
func NewResource() resource.Resource { return &AppResource{} }

func (r *AppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *AppResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AppResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *AppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// getConfigRaw fetches the live, fully-resolved app configuration via
// app.config as raw JSON, so callers that need exact numeric precision (Compose
// reconciliation, #34) can decode it with json.Number.
func (r *AppResource) getConfigRaw(ctx context.Context, name string) (json.RawMessage, error) {
	return r.client.CallRead(ctx, "app.config", name)
}

// getConfig fetches the live app configuration decoded as a generic object, for
// catalog `values` projection onto the user's key shape (#33).
func (r *AppResource) getConfig(ctx context.Context, name string) (map[string]any, error) {
	raw, err := r.getConfigRaw(ctx, name)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// getInstance fetches a single app by name via app.get_instance (sync call).
func (r *AppResource) getInstance(ctx context.Context, name string) (*appAPI, error) {
	raw, err := r.client.CallRead(ctx, "app.get_instance", name)
	if err != nil {
		return nil, err
	}
	var api appAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// getInstanceSettled reads the app but waits for it to leave the transient
// DEPLOYING state first (up to a timeout), so a read-back right after
// create/update/start does not report running=false while the app is still
// deploying and contradict a planned running=true. (#28)
func (r *AppResource) getInstanceSettled(ctx context.Context, name string) (*appAPI, error) {
	const timeout = 10 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		api, err := r.getInstance(ctx, name)
		if err != nil || api.State != "DEPLOYING" || time.Now().After(deadline) {
			return api, err
		}
		select {
		case <-ctx.Done():
			return api, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CallJob(ctx, "app.create", payload); err != nil {
		resp.Diagnostics.AddError("Failed to create app", err.Error())
		return
	}

	name := plan.Name.ValueString()
	api, err := r.getInstanceSettled(ctx, name)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("App create read-back failed", fmt.Sprintf("app %q was not found after creation (create may have failed silently): %s", name, err.Error()))
			return
		}
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}

	// If the plan explicitly requests the app be stopped, and it is
	// currently running (apps typically start automatically on create),
	// stop it and re-read.
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && !plan.Running.ValueBool() && api.State == "RUNNING" {
		if _, err := r.client.CallJob(ctx, "app.stop", name); err != nil {
			resp.Diagnostics.AddError("Failed to stop app", err.Error())
			return
		}
		api, err = r.getInstanceSettled(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Read-back failed", err.Error())
			return
		}
	}

	// If the plan explicitly requests the app be running, and it is not
	// currently running, start it and re-read.
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool() && api.State != "RUNNING" {
		if _, err := r.client.CallJob(ctx, "app.start", name); err != nil {
			resp.Diagnostics.AddError("Failed to start app", err.Error())
			return
		}
		api, err = r.getInstanceSettled(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Read-back failed", err.Error())
			return
		}
	}

	// values/custom_compose_config_string/catalog_app are never echoed back
	// by the API (write-only). responseToModel does not touch them, so the
	// plan's values are preserved in state as-is.
	responseToModel(api, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use Name (not ID) for the lookup: after ImportState only "name" is
	// populated in state, so relying on ID here would break import.
	// ID and Name always carry the same value for this resource.
	api, err := r.getInstance(ctx, state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read app failed", err.Error())
		return
	}

	// ComposeYAML/CatalogApp are not echoed back by the API; responseToModel
	// preserves them. Values IS reconciled below for drift detection.
	responseToModel(api, &state)

	if state.CustomApp.ValueBool() {
		// Custom app: app.config returns the Compose document verbatim as an
		// object. Reconcile custom_compose_config_string so drift (an edit made
		// in the UI/API) is detected, and the Compose input is populated on
		// import. Read failures are surfaced rather than treated as verified;
		// the message never includes Compose contents (they may hold secrets). (#34)
		raw, err := r.getConfigRaw(ctx, state.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Read app failed",
				"could not read the live app configuration to detect Compose drift: "+err.Error())
			return
		}
		newYAML, equal, rerr := reconcileComposeConfig(state.ComposeYAML.ValueString(), raw)
		if rerr != nil {
			resp.Diagnostics.AddError("Read app failed", rerr.Error())
			return
		}
		if !equal {
			state.ComposeYAML = types.StringValue(newYAML)
		}
	} else if !state.Values.IsNull() && state.Values.ValueString() != "" {
		// Catalog app: reconcile `values`, projected onto the keys the user set,
		// so drift in those keys is detected without chart defaults / ix_* showing
		// as noise (#33). Only when the user manages values (non-empty).
		if cfg, err := r.getConfig(ctx, state.Name.ValueString()); err == nil {
			state.Values = types.StringValue(projectConfigOntoUserShape(state.Values.ValueString(), cfg))
		}
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()

	// version changed -> app.upgrade (must run before app.update / read-back,
	// otherwise the read-back overwrites plan.Version with the old value and
	// Terraform aborts with "Provider produced inconsistent result after
	// apply").
	if needsUpgrade(&plan, &state) {
		if _, err := r.client.CallJob(ctx, "app.upgrade", name, map[string]any{
			"app_version": plan.Version.ValueString(),
		}); err != nil {
			resp.Diagnostics.AddError("Failed to upgrade app", err.Error())
			return
		}
	}

	// values or compose config changed -> app.update
	if !plan.Values.Equal(state.Values) || !plan.ComposeYAML.Equal(state.ComposeYAML) {
		updatePayload, diags := plan.updatePayload()
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(updatePayload) > 0 {
			if _, err := r.client.CallJob(ctx, "app.update", name, updatePayload); err != nil {
				resp.Diagnostics.AddError("Failed to update app", err.Error())
				return
			}
		}
	}

	// running state changed -> app.start / app.stop
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && !plan.Running.Equal(state.Running) {
		if plan.Running.ValueBool() {
			if _, err := r.client.CallJob(ctx, "app.start", name); err != nil {
				resp.Diagnostics.AddError("Failed to start app", err.Error())
				return
			}
		} else {
			if _, err := r.client.CallJob(ctx, "app.stop", name); err != nil {
				resp.Diagnostics.AddError("Failed to stop app", err.Error())
				return
			}
		}
	}

	api, err := r.getInstanceSettled(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "app.delete", state.ID.ValueString(), map[string]any{
		"remove_images":     true,
		"remove_ix_volumes": false,
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete app", err.Error())
		return
	}
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
}
