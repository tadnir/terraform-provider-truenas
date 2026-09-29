// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SMBShareACLResource{}
var _ resource.ResourceWithImportState = &SMBShareACLResource{}
var _ resource.ResourceWithIdentity = &SMBShareACLResource{}
var _ resource.ResourceWithValidateConfig = &SMBShareACLResource{}

// SMBShareACLResource implements the truenas_smb_share_acl resource: a
// declarative, share_name-keyed wrapper around sharing.smb.setacl/getacl.
type SMBShareACLResource struct{ client *client.Client }

// NewResource returns a new SMBShareACLResource.
func NewResource() resource.Resource { return &SMBShareACLResource{} }

func (r *SMBShareACLResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_share_acl"
}

func (r *SMBShareACLResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

// IdentitySchema backs identity-based import by share_name. This resource is
// not listable: sharing.smb.getacl operates on a single named share and there
// is no "enumerate every share ACL" query to back a list resource (the shares
// themselves are listed by truenas_smb_share).
func (r *SMBShareACLResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *SMBShareACLResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// getacl reads the current share ACL for a share name.
func (r *SMBShareACLResource) getacl(ctx context.Context, shareName string) (*smbGetAclAPI, error) {
	raw, err := r.client.CallRead(ctx, "sharing.smb.getacl", map[string]any{"share_name": shareName})
	if err != nil {
		return nil, err
	}
	var api smbGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parse sharing.smb.getacl response: %w", err)
	}
	return &api, nil
}

// applyACL calls sharing.smb.setacl with the model's entries.
func (r *SMBShareACLResource) applyACL(ctx context.Context, m *SMBShareACLModel) error {
	payload, diags := setaclPayload(ctx, m)
	if diags.HasError() {
		return fmt.Errorf("build sharing.smb.setacl payload: %s", diagText(diags))
	}
	if _, err := r.client.Call(ctx, "sharing.smb.setacl", payload); err != nil {
		return fmt.Errorf("sharing.smb.setacl: %w", err)
	}
	return nil
}

// ValidateConfig enforces at plan time what the server enforces at apply time
// (verified live: sharing.smb.setacl returns EINVAL "You must set one of ...
// ae_who_sid, ae_who_str, or ae_who_id" for an entry with no principal). Each
// entry must set exactly one of the three principal selectors; an unknown
// (interpolated) value counts as set, so cross-resource references validate.
func (r *SMBShareACLResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg SMBShareACLModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ShareACL.IsNull() || cfg.ShareACL.IsUnknown() {
		return
	}
	entries, diags := entriesFromList(ctx, cfg.ShareACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for i, e := range entries {
		set := countSelectors(e)
		if set != 1 {
			resp.Diagnostics.AddAttributeError(
				path.Root("share_acl").AtListIndex(i),
				"Exactly one principal selector required",
				fmt.Sprintf("share_acl entry %d must set exactly one of ae_who_sid, ae_who_id, or "+
					"ae_who_str (got %d). The server rejects any other combination.", i, set),
			)
		}
	}
}

func (r *SMBShareACLResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SMBShareACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyACL(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Create SMB share ACL failed", err.Error())
		return
	}

	// Write-what-you-said: keep the plan's exact entries in state. The server
	// resolves extra principal selectors, but storing those would drift against
	// a config that never set them (see model.go entriesDrifted).
	plan.ID = types.StringValue(idFor(plan.ShareName.ValueString()))
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SMBShareACLResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SMBShareACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.getacl(ctx, state.ShareName.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read SMB share ACL failed", err.Error())
		return
	}

	stateEntries, diags := entriesFromList(ctx, state.ShareACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if entriesDrifted(stateEntries, api.ShareACL) {
		list, d := apiEntriesToList(ctx, api.ShareACL)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ShareACL = list
	}

	state.ID = types.StringValue(idFor(state.ShareName.ValueString()))
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SMBShareACLResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SMBShareACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyACL(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Update SMB share ACL failed", err.Error())
		return
	}

	plan.ID = types.StringValue(idFor(plan.ShareName.ValueString()))
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete resets the share ACL to the TrueNAS default (everyone@ FULL ALLOWED).
// An SMB share always has a share ACL, so there is nothing to "remove"; this is
// the SMB equivalent of truenas_filesystem_acl's strip-to-trivial Delete.
func (r *SMBShareACLResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SMBShareACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	shareName := state.ShareName.ValueString()
	_, err := r.client.Call(ctx, "sharing.smb.setacl", defaultShareACLPayload(shareName))
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete SMB share ACL failed", err.Error())
		return
	}

	resp.Diagnostics.AddWarning(
		"SMB share ACL reset to default",
		fmt.Sprintf(
			"truenas_smb_share_acl for share %q was destroyed by resetting its share ACL to the "+
				"TrueNAS default (a single \"everyone@ FULL ALLOWED\" entry) via sharing.smb.setacl. "+
				"An SMB share always has a share ACL, so it cannot be removed - only reset. The share "+
				"itself and its filesystem ACL are untouched.",
			shareName,
		),
	)
}

func (r *SMBShareACLResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by share name, e.g. `terraform import truenas_smb_share_acl.example myshare`.
	shareName, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.getacl(ctx, shareName)
	if err != nil {
		resp.Diagnostics.AddError("Import SMB share ACL failed", err.Error())
		return
	}

	list, d := apiEntriesToList(ctx, api.ShareACL)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := SMBShareACLModel{
		ID:        types.StringValue(idFor(shareName)),
		ShareName: types.StringValue(shareName),
		ShareACL:  list,
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
