// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_lock

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

// Action locks an encrypted dataset (pool.dataset.lock; a job).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Dataset     types.String `tfsdk:"dataset"`
	ForceUmount types.Bool   `tfsdk:"force_umount"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_lock"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Locks a passphrase-encrypted dataset (pool.dataset.lock), making its data inaccessible until unlocked.",
		Attributes: map[string]schema.Attribute{
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the encrypted dataset to lock, e.g. \"tank/secret\".",
			},
			"force_umount": schema.BoolAttribute{
				Optional:    true,
				Description: "Force-unmount the dataset if it is busy.",
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
	if !cfg.ForceUmount.IsNull() && !cfg.ForceUmount.IsUnknown() {
		opts["force_umount"] = cfg.ForceUmount.ValueBool()
	}
	if _, err := a.client.CallJob(ctx, "pool.dataset.lock", cfg.Dataset.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Dataset lock failed", err.Error())
		return
	}
}
