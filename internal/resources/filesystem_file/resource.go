// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
var _ resource.ResourceWithModifyPlan = &FilesystemFileResource{}

// FilesystemFileResource implements the truenas_filesystem_file resource.
//
// filesystem.put is a job that takes the file through an input pipe, so the
// content goes over HTTP (client.PutFile). filesystem.stat reads it back; a
// missing path is ENOENT (client.IsNotFound). There is no public method that
// reads a file's hash, so a change on the NAS is noticed by size and mtime
// (kept in private state), and none that deletes a file, so Delete only
// forgets it.
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

// ConfigValidators allows at most one of content, content_base64 and source;
// with none, the file must already exist on the NAS.
func (r *FilesystemFileResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.Conflicting(
			path.MatchRoot("content"),
			path.MatchRoot("content_base64"),
			path.MatchRoot("source"),
		),
	}
}

// ModifyPlan plans sha256 from the configuration, so a change of a
// write-only content or of the source file's bytes shows as a change.
func (r *FilesystemFileResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var cfg FilesystemFileModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sum, err := plannedSHA256(&cfg)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("sha256"), "Invalid file content", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("sha256"), sum)...)
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

// privateSetter is the part of a response's private state write uses.
type privateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// write writes the configured content at the planned path (or, for an
// existing file, checks it is there and sets its mode), then reads the file
// back into plan.
func (r *FilesystemFileResource) write(ctx context.Context, plan, cfg *FilesystemFileModel, priv privateSetter) diag.Diagnostics {
	var diags diag.Diagnostics
	p := plan.Path.ValueString()
	plan.ID = plan.Path
	plan.Content = types.StringNull()
	plan.ContentBase64 = types.StringNull()

	if existing(cfg) {
		st, err := r.stat(ctx, p)
		if client.IsNotFound(err) {
			diags.AddError("File not found", fmt.Sprintf("%s does not exist. With none of content, "+
				"content_base64 and source set, truenas_filesystem_file manages a file that is already on the NAS.", p))
			return diags
		}
		if err != nil {
			diags.AddError("Read file failed", err.Error())
			return diags
		}
		if err := r.setMode(ctx, p, plan.Mode, st); err != nil {
			diags.AddError("Set file mode failed", err.Error())
			return diags
		}
		plan.SHA256 = types.StringNull()
		diags.Append(priv.SetKey(ctx, writtenKey, nil)...)
		return append(diags, r.readBack(ctx, plan, nil)...)
	}

	mode := defaultMode
	if isSet(plan.Mode) {
		m, err := parseMode(plan.Mode.ValueString())
		if err != nil {
			diags.AddError("Invalid mode", err.Error())
			return diags
		}
		mode = m
	}
	rd, done, err := openContent(cfg)
	if err != nil {
		diags.AddError("Read file content failed", err.Error())
		return diags
	}
	err = r.client.PutFile(ctx, p, rd, mode)
	done()
	if err != nil {
		diags.AddError("Write file failed", "filesystem.put: "+err.Error())
		return diags
	}
	sum := rd.sum()
	if isSet(plan.SHA256) && plan.SHA256.ValueString() != sum {
		diags.AddError("File content changed during apply",
			fmt.Sprintf("%s was planned with sha256 %s but the content written has %s; the source file "+
				"changed between plan and apply. Plan again.", p, plan.SHA256.ValueString(), sum))
		return diags
	}
	plan.SHA256 = types.StringValue(sum)
	return append(diags, r.readBack(ctx, plan, priv)...)
}

// readBack stats the file into plan and, when priv is set, remembers its
// size and mtime as the ones this resource wrote.
func (r *FilesystemFileResource) readBack(ctx context.Context, plan *FilesystemFileModel, priv privateSetter) diag.Diagnostics {
	var diags diag.Diagnostics
	st, err := r.stat(ctx, plan.Path.ValueString())
	if err != nil {
		diags.AddError("Read file failed", err.Error())
		return diags
	}
	plan.Mode = modeFromStat(plan.Mode, st.Mode)
	if priv != nil {
		diags.Append(priv.SetKey(ctx, writtenKey, written{Size: st.Size, Mtime: st.Mtime}.bytes())...)
	}
	return diags
}

// setMode sets the file's permission bits with filesystem.setperm when mode
// is set and differs from st's.
func (r *FilesystemFileResource) setMode(ctx context.Context, p string, mode types.String, st *fsStatAPI) error {
	if !isSet(mode) {
		return nil
	}
	m, err := parseMode(mode.ValueString())
	if err != nil {
		return err
	}
	if int64(m) == st.Mode&0o7777 {
		return nil
	}
	_, err = r.client.CallFilesystemMutation(ctx, "filesystem.setperm", map[string]any{
		"path":    p,
		"mode":    fmt.Sprintf("%o", m),
		"options": map[string]any{"recursive": false, "traverse": false},
	})
	if err != nil {
		return fmt.Errorf("filesystem.setperm: %w", err)
	}
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
	if st.Type != "" && st.Type != "FILE" {
		return nil, fmt.Errorf("%s is not a file (filesystem.stat type %s)", p, st.Type)
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
	resp.Diagnostics.Append(r.write(ctx, &plan, &cfg, resp.Private)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read stats the file. A file this resource wrote whose size or mtime has
// changed since gets sha256 "", so the next apply writes it again.
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
	if isSet(state.SHA256) {
		raw, diags := req.Private.GetKey(ctx, writtenKey)
		resp.Diagnostics.Append(diags...)
		var w written
		if len(raw) > 0 && json.Unmarshal(raw, &w) == nil && !w.matches(st) {
			state.SHA256 = types.StringValue("")
		}
	}
	state.Mode = modeFromStat(state.Mode, st.Mode)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update writes the file again when its content changed, and otherwise only
// sets its mode, so a mode change doesn't upload a large source again.
func (r *FilesystemFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, cfg, state FilesystemFileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !existing(&cfg) && isSet(plan.SHA256) && plan.SHA256.Equal(state.SHA256) {
		p := plan.Path.ValueString()
		st, err := r.stat(ctx, p)
		if err == nil {
			err = r.setMode(ctx, p, plan.Mode, st)
		}
		if err != nil {
			resp.Diagnostics.AddError("Set file mode failed", err.Error())
			return
		}
		plan.ID = plan.Path
		plan.Content = types.StringNull()
		plan.ContentBase64 = types.StringNull()
		resp.Diagnostics.Append(r.readBack(ctx, &plan, resp.Private)...)
	} else {
		resp.Diagnostics.Append(r.write(ctx, &plan, &cfg, resp.Private)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only removes the file from state: TrueNAS has no public method that
// deletes a single file. A file this resource wrote gets a warning; an
// existing one was never its to delete.
func (r *FilesystemFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilesystemFileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || state.SHA256.IsNull() {
		return
	}
	resp.Diagnostics.AddWarning("File left in place",
		fmt.Sprintf("TrueNAS has no API to delete a single file, so %s was only removed from Terraform state. "+
			"It goes when its dataset is destroyed.", state.Path.ValueString()))
}

// ImportState adopts an existing file by path. Its content is not read
// (sha256 stays null), so a configuration with content writes it again on
// the next apply, and one without manages it as an existing file.
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
		ID:            types.StringValue(p),
		Path:          types.StringValue(p),
		Content:       types.StringNull(),
		ContentBase64: types.StringNull(),
		Source:        types.StringNull(),
		SHA256:        types.StringNull(),
		Mode:          modeFromStat(types.StringNull(), st.Mode),
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, p)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
