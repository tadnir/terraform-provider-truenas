// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_set_quota

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

// Action sets user/group/dataset quotas on a dataset (pool.dataset.set_quota).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type quotaModel struct {
	QuotaType  types.String `tfsdk:"quota_type"`
	ID         types.String `tfsdk:"id"`
	QuotaValue types.Int64  `tfsdk:"quota_value"`
}

type model struct {
	Dataset types.String `tfsdk:"dataset"`
	Quotas  types.List   `tfsdk:"quotas"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_set_quota"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets user, group, or dataset quotas on a dataset (pool.dataset.set_quota). A quota_value of 0 removes that quota.",
		Attributes: map[string]schema.Attribute{
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the dataset, e.g. \"tank/data\".",
			},
			"quotas": schema.ListNestedAttribute{
				Required:    true,
				Description: "Quota entries to apply.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"quota_type": schema.StringAttribute{
							Required:    true,
							Description: "DATASET, USER, USEROBJ, GROUP, or GROUPOBJ.",
						},
						"id": schema.StringAttribute{
							Required:    true,
							Description: "UID/username or GID/group name the quota applies to (ignored for DATASET; use \"\").",
						},
						"quota_value": schema.Int64Attribute{
							Required:    true,
							Description: "Quota in bytes (or object count for *OBJ types). 0 removes the quota.",
						},
					},
				},
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
	var quotas []quotaModel
	resp.Diagnostics.Append(cfg.Quotas.ElementsAs(ctx, &quotas, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out := make([]map[string]any, 0, len(quotas))
	for _, q := range quotas {
		out = append(out, map[string]any{
			"quota_type":  q.QuotaType.ValueString(),
			"id":          q.ID.ValueString(),
			"quota_value": q.QuotaValue.ValueInt64(),
		})
	}
	if _, err := a.client.Call(ctx, "pool.dataset.set_quota", cfg.Dataset.ValueString(), out); err != nil {
		resp.Diagnostics.AddError("Dataset set_quota failed", err.Error())
		return
	}
}
