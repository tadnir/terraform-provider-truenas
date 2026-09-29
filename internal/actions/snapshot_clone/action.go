// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone

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

// Action clones a snapshot into a new dataset (pool.snapshot.clone).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Snapshot   types.String `tfsdk:"snapshot"`
	DatasetDst types.String `tfsdk:"dataset_dst"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_clone"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Clones a snapshot into a new dataset (pool.snapshot.clone). The clone is a writable dataset sharing the snapshot's blocks until diverged.",
		Attributes: map[string]schema.Attribute{
			"snapshot": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot to clone, e.g. \"tank/data@snap1\".",
			},
			"dataset_dst": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the new dataset to create as the clone, e.g. \"tank/data-clone\".",
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
	if _, err := a.client.Call(ctx, "pool.snapshot.clone", map[string]any{
		"snapshot":    cfg.Snapshot.ValueString(),
		"dataset_dst": cfg.DatasetDst.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Snapshot clone failed", err.Error())
		return
	}
}
