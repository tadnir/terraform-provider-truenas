// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_rollback

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ action.Action = &Action{}
var _ action.ActionWithConfigure = &Action{}

// Action rolls a dataset back to a snapshot (pool.snapshot.rollback).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Snapshot          types.String `tfsdk:"snapshot"`
	Recursive         types.Bool   `tfsdk:"recursive"`
	RecursiveClones   types.Bool   `tfsdk:"recursive_clones"`
	RecursiveRollback types.Bool   `tfsdk:"recursive_rollback"`
	Force             types.Bool   `tfsdk:"force"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_rollback"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Rolls a dataset back to one of its snapshots (pool.snapshot.rollback). Destroys any data written after the snapshot.",
		Attributes: map[string]schema.Attribute{
			"snapshot": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot name to roll back to, e.g. \"tank/data@snap1\".",
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Destroy any snapshots and bookmarks more recent than the one specified.",
			},
			"recursive_clones": schema.BoolAttribute{
				Optional:    true,
				Description: "Like recursive, but also destroy any clones of those snapshots.",
			},
			"recursive_rollback": schema.BoolAttribute{
				Optional:    true,
				Description: "Roll back the given snapshot and all intermediate snapshots on all descendant datasets.",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Force unmount of any file systems that need to be unmounted.",
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
	opts := map[string]any{}
	if !cfg.Recursive.IsNull() && !cfg.Recursive.IsUnknown() {
		opts["recursive"] = cfg.Recursive.ValueBool()
	}
	if !cfg.RecursiveClones.IsNull() && !cfg.RecursiveClones.IsUnknown() {
		opts["recursive_clones"] = cfg.RecursiveClones.ValueBool()
	}
	if !cfg.RecursiveRollback.IsNull() && !cfg.RecursiveRollback.IsUnknown() {
		opts["recursive_rollback"] = cfg.RecursiveRollback.ValueBool()
	}
	if !cfg.Force.IsNull() && !cfg.Force.IsUnknown() {
		opts["force"] = cfg.Force.ValueBool()
	}
	if _, err := a.client.Call(ctx, "pool.snapshot.rollback", cfg.Snapshot.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Snapshot rollback failed", err.Error())
		return
	}
}
