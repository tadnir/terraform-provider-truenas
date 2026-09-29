// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_promote

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

// Action promotes a cloned dataset (pool.dataset.promote), reversing the
// clone/origin relationship so the clone no longer depends on its origin snapshot.
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Dataset types.String `tfsdk:"dataset"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_promote"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Promotes a cloned dataset (pool.dataset.promote) so it no longer depends on its origin snapshot.",
		Attributes: map[string]schema.Attribute{
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the cloned dataset to promote, e.g. \"tank/clone\".",
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
	if _, err := a.client.Call(ctx, "pool.dataset.promote", cfg.Dataset.ValueString()); err != nil {
		resp.Diagnostics.AddError("Dataset promote failed", err.Error())
		return
	}
}
