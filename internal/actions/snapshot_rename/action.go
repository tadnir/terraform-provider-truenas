// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_rename

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ action.Action = &Action{}
var _ action.ActionWithConfigure = &Action{}

// Action renames a snapshot (pool.snapshot.rename).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Snapshot  types.String `tfsdk:"snapshot"`
	NewName   types.String `tfsdk:"new_name"`
	Force     types.Bool   `tfsdk:"force"`
	Recursive types.Bool   `tfsdk:"recursive"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_rename"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Renames a snapshot (pool.snapshot.rename).",
		Attributes: map[string]schema.Attribute{
			"snapshot": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot to rename, e.g. \"tank/data@snap1\".",
			},
			"new_name": schema.StringAttribute{
				Required:    true,
				Description: "New snapshot name (the part after \"@\"), e.g. \"snap1-renamed\". The dataset is taken from `snapshot`; the TrueNAS API is sent the full \"<dataset>@<new_name>\".",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Force the rename. Only used on TrueNAS < 26.0 (pool.snapshot.rename); ignored on 26.0+ where zfs.resource.snapshot.rename is used.",
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Also rename the equivalent snapshot on descendant datasets.",
			},
		},
	}
}

func (a *Action) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	a.client = c
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg model
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Both APIs require new_name as a full "<dataset>@<name>"; build it from the
	// source snapshot's dataset and the user-supplied new name.
	newName := cfg.NewName.ValueString()
	if dataset, _, ok := strings.Cut(cfg.Snapshot.ValueString(), "@"); ok && !strings.Contains(newName, "@") {
		newName = dataset + "@" + newName
	}

	// Version routing: TrueNAS 26.0+ deprecates pool.snapshot.rename in favour
	// of zfs.resource.snapshot.rename (verified live: 27.0 rejects the old one
	// with "Use zfs.resource.snapshot.rename"). The new method takes
	// {current_name, new_name, recursive} and needs no force; the old one takes
	// (id, {new_name, force, recursive}).
	if ok, verr := a.client.VersionAtLeast(ctx, 26, 0); verr == nil && ok {
		opts := map[string]any{
			"current_name": cfg.Snapshot.ValueString(),
			"new_name":     newName,
		}
		if !cfg.Recursive.IsNull() && !cfg.Recursive.IsUnknown() {
			opts["recursive"] = cfg.Recursive.ValueBool()
		}
		if _, err := a.client.Call(ctx, "zfs.resource.snapshot.rename", opts); err != nil {
			resp.Diagnostics.AddError("Snapshot rename failed", err.Error())
		}
		return
	}

	opts := map[string]any{"new_name": newName}
	if !cfg.Force.IsNull() && !cfg.Force.IsUnknown() {
		opts["force"] = cfg.Force.ValueBool()
	}
	if !cfg.Recursive.IsNull() && !cfg.Recursive.IsUnknown() {
		opts["recursive"] = cfg.Recursive.ValueBool()
	}
	if _, err := a.client.Call(ctx, "pool.snapshot.rename", cfg.Snapshot.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Snapshot rename failed", err.Error())
		return
	}
}
