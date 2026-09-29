// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_hold

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

// Action places a hold on a snapshot (pool.snapshot.hold), preventing deletion.
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Snapshot  types.String `tfsdk:"snapshot"`
	Recursive types.Bool   `tfsdk:"recursive"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot_hold"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Places a hold on a snapshot (pool.snapshot.hold). A held snapshot cannot be deleted until released.",
		Attributes: map[string]schema.Attribute{
			"snapshot": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot to hold, e.g. \"tank/data@snap1\".",
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Also hold the equivalent snapshot on descendant datasets.",
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
	if _, err := a.client.Call(ctx, "pool.snapshot.hold", cfg.Snapshot.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Snapshot hold failed", err.Error())
		return
	}
}
