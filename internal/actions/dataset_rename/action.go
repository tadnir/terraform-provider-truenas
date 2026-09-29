// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_rename

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

// Action renames a dataset (pool.dataset.rename).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Dataset   types.String `tfsdk:"dataset"`
	NewName   types.String `tfsdk:"new_name"`
	Recursive types.Bool   `tfsdk:"recursive"`
	Force     types.Bool   `tfsdk:"force"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_rename"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Renames a dataset (pool.dataset.rename). A dataset managed by truenas_dataset should not be renamed out from under Terraform; use this for unmanaged datasets or plan to re-import.",
		Attributes: map[string]schema.Attribute{
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the dataset to rename, e.g. \"tank/old\".",
			},
			"new_name": schema.StringAttribute{
				Required:    true,
				Description: "New full dataset path, e.g. \"tank/new\".",
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Rename descendant datasets too.",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Force the rename, unmounting file systems that need unmounting. TrueNAS refuses a rename without this (it performs no safety checks on the rename), so set it to true unless you have already quiesced the dataset.",
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
	opts := map[string]any{"new_name": cfg.NewName.ValueString()}
	if !cfg.Recursive.IsNull() && !cfg.Recursive.IsUnknown() {
		opts["recursive"] = cfg.Recursive.ValueBool()
	}
	if !cfg.Force.IsNull() && !cfg.Force.IsUnknown() {
		opts["force"] = cfg.Force.ValueBool()
	}
	if _, err := a.client.Call(ctx, "pool.dataset.rename", cfg.Dataset.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Dataset rename failed", err.Error())
		return
	}
}
