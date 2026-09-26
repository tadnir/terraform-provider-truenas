// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SnapshotCloneResource{}
var _ resource.ResourceWithImportState = &SnapshotCloneResource{}
var _ resource.ResourceWithIdentity = &SnapshotCloneResource{}

// SnapshotCloneResource implements truenas_snapshot_clone. A clone has no
// updatable attributes of its own: everything configurable forces
// replacement, and the clone's ZFS properties after creation belong to a
// truenas_dataset or truenas_zvol resource if they are to be managed.
type SnapshotCloneResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &SnapshotCloneResource{} }

func (r *SnapshotCloneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_clone"
}

func (r *SnapshotCloneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SnapshotCloneResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *SnapshotCloneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *SnapshotCloneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SnapshotCloneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	props := map[string]string{}
	if !plan.DatasetProperties.IsNull() && !plan.DatasetProperties.IsUnknown() {
		resp.Diagnostics.Append(plan.DatasetProperties.ElementsAs(ctx, &props, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// pool.snapshot.clone returns true, not the new dataset, so the state
	// comes from reading the clone back.
	_, err := r.client.Call(ctx, "pool.snapshot.clone",
		clonePayload(plan.Snapshot.ValueString(), plan.Dataset.ValueString(), props))
	if err != nil {
		resp.Diagnostics.AddError("Clone snapshot failed", err.Error())
		return
	}

	api, err := r.get(ctx, plan.Dataset.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after clone failed", err.Error())
		return
	}
	responseToModel(api, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SnapshotCloneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SnapshotCloneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read clone failed", err.Error())
		return
	}
	resp.Diagnostics.Append(notACloneWarning(api)...)
	responseToModel(api, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never reached: every configurable attribute forces replacement.
func (r *SnapshotCloneResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported",
		"truenas_snapshot_clone has no in-place updates; all attribute changes force replacement.")
}

func (r *SnapshotCloneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SnapshotCloneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Not recursive: a clone that has grown children or snapshots of its
	// own was put to use outside this resource, and destroying those
	// silently is not this resource's call. The delete fails and says why.
	_, err := r.client.CallJob(ctx, "pool.dataset.delete", state.ID.ValueString(),
		map[string]any{"recursive": false})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete clone failed", err.Error())
	}
}

func (r *SnapshotCloneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by the clone's dataset name. snapshot is recovered from origin.
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import clone failed", err.Error())
		return
	}
	if _, ok := api.origin(); !ok {
		resp.Diagnostics.AddError("Not a clone",
			fmt.Sprintf("%s has no origin snapshot, so it is not a clone; manage it with truenas_dataset or truenas_zvol instead.", id))
		return
	}

	state := SnapshotCloneModel{DatasetProperties: types.MapNull(types.StringType)}
	responseToModel(api, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SnapshotCloneResource) get(ctx context.Context, name string) (*cloneAPI, error) {
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", name)
	if err != nil {
		return nil, err
	}
	var api cloneAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parsing pool.dataset.get_instance response: %w", err)
	}
	return &api, nil
}

// notACloneWarning explains a clone that no longer has an origin, which is
// what `zfs promote` does to it. State keeps the snapshot it was created
// from rather than planning a replacement, because replacing would destroy
// what is now an independent dataset holding its own data.
func notACloneWarning(api *cloneAPI) diag.Diagnostics {
	var diags diag.Diagnostics
	if _, ok := api.origin(); !ok {
		diags.AddWarning("Dataset is no longer a clone",
			fmt.Sprintf("%s has no origin snapshot (was it promoted?). It is left in state as it was; "+
				"remove it from state and manage it as a dataset if that is intended.", api.Name))
	}
	return diags
}
