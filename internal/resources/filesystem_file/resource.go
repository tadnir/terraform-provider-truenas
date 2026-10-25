// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &FilesystemFileResource{}
var _ resource.ResourceWithImportState = &FilesystemFileResource{}
var _ resource.ResourceWithIdentity = &FilesystemFileResource{}
var _ resource.ResourceWithConfigValidators = &FilesystemFileResource{}

// FilesystemFileResource implements the truenas_filesystem_file resource.
//
// filesystem.put is a job that takes the file through an input pipe, so the
// content goes over HTTP (client.PutFile). filesystem.stat reads it back; a
// missing path is ENOENT (client.IsNotFound). There is no public method that
// deletes a file, so Delete only forgets it.
type FilesystemFileResource struct{ client *client.Client }

// NewResource returns a new instance of FilesystemFileResource.
func NewResource() resource.Resource { return &FilesystemFileResource{} }

func (r *FilesystemFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_file"
}

func (r *FilesystemFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *FilesystemFileResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

// ConfigValidators requires exactly one of content_base64 / content_base64_wo.
func (r *FilesystemFileResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("content_base64"),
			path.MatchRoot("content_base64_wo"),
		),
	}
}

func (r *FilesystemFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// write puts the planned content at the planned path, then reads the file
// back into plan.
func (r *FilesystemFileResource) write(ctx context.Context, plan, cfg *FilesystemFileModel) error {
	b, err := content(plan, cfg)
	if err != nil {
		return err
	}
	mode, err := parseMode(plan.Mode.ValueString())
	if err != nil {
		return err
	}
	if err := r.client.PutFile(ctx, plan.Path.ValueString(), b, mode); err != nil {
		return fmt.Errorf("filesystem.put: %w", err)
	}
	st, err := r.stat(ctx, plan.Path.ValueString())
	if err != nil {
		return err
	}
	plan.ID = plan.Path
	plan.Mode = modeFromStat(plan.Mode, st.Mode)
	plan.ContentBase64WO = types.StringNull()
	return nil
}

func (r *FilesystemFileResource) stat(ctx context.Context, p string) (*fsStatAPI, error) {
	raw, err := r.client.CallRead(ctx, "filesystem.stat", p)
	if err != nil {
		return nil, err
	}
	var st fsStatAPI
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("parse filesystem.stat response: %w", err)
	}
	return &st, nil
}

func (r *FilesystemFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg FilesystemFileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan, &cfg); err != nil {
		resp.Diagnostics.AddError("Create file failed", err.Error())
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FilesystemFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilesystemFileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	st, err := r.stat(ctx, state.Path.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read file failed", err.Error())
		return
	}
	state.Mode = modeFromStat(state.Mode, st.Mode)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update rewrites the file: every in-place change (content, the write-only
// version, mode) goes through filesystem.put, which also sets the mode.
func (r *FilesystemFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, cfg FilesystemFileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan, &cfg); err != nil {
		resp.Diagnostics.AddError("Update file failed", err.Error())
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only removes the file from state: TrueNAS has no public method that
// deletes a single file.
func (r *FilesystemFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var p types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("path"), &p)...)
	resp.Diagnostics.AddWarning("File left in place",
		fmt.Sprintf("TrueNAS has no API to delete a single file, so %s was only removed from Terraform state. "+
			"It goes when its dataset is destroyed.", p.ValueString()))
}

// ImportState adopts an existing file by path. Its content is not read
// (content_base64 stays null), so the next apply rewrites it from
// configuration.
func (r *FilesystemFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	p, diags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	st, err := r.stat(ctx, p)
	if err != nil {
		resp.Diagnostics.AddError("Import file failed", err.Error())
		return
	}
	state := FilesystemFileModel{
		ID:               types.StringValue(p),
		Path:             types.StringValue(p),
		ContentBase64:    types.StringNull(),
		ContentBase64WO:  types.StringNull(),
		ContentWOVersion: types.Int64Null(),
		Mode:             modeFromStat(types.StringNull(), st.Mode),
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, p)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
