// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_unlock

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

// Action unlocks an encrypted dataset (pool.dataset.unlock; a job).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	Dataset           types.String `tfsdk:"dataset"`
	Passphrase        types.String `tfsdk:"passphrase"`
	Key               types.String `tfsdk:"key"`
	Recursive         types.Bool   `tfsdk:"recursive"`
	Force             types.Bool   `tfsdk:"force"`
	ToggleAttachments types.Bool   `tfsdk:"toggle_attachments"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_unlock"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Unlocks an encrypted dataset (pool.dataset.unlock) with its passphrase or hex key. The secret is used transiently and is not stored in state.",
		Attributes: map[string]schema.Attribute{
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Full path of the encrypted dataset to unlock, e.g. \"tank/secret\".",
			},
			"passphrase": schema.StringAttribute{
				Optional:    true,
				Description: "Passphrase for a passphrase-encrypted dataset. Provide this or key.",
			},
			"key": schema.StringAttribute{
				Optional:    true,
				Description: "Hex key for a key-encrypted dataset. Provide this or passphrase.",
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Also unlock descendant datasets sharing the same key.",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Force the unlock.",
			},
			"toggle_attachments": schema.BoolAttribute{
				Optional:    true,
				Description: "Start services/shares that depend on the unlocked dataset.",
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
	entry := map[string]any{"name": cfg.Dataset.ValueString()}
	if !cfg.Passphrase.IsNull() && !cfg.Passphrase.IsUnknown() {
		entry["passphrase"] = cfg.Passphrase.ValueString()
	}
	if !cfg.Key.IsNull() && !cfg.Key.IsUnknown() {
		entry["key"] = cfg.Key.ValueString()
	}
	opts := map[string]any{"datasets": []map[string]any{entry}}
	if !cfg.Recursive.IsNull() && !cfg.Recursive.IsUnknown() {
		opts["recursive"] = cfg.Recursive.ValueBool()
	}
	if !cfg.Force.IsNull() && !cfg.Force.IsUnknown() {
		opts["force"] = cfg.Force.ValueBool()
	}
	if !cfg.ToggleAttachments.IsNull() && !cfg.ToggleAttachments.IsUnknown() {
		opts["toggle_attachments"] = cfg.ToggleAttachments.ValueBool()
	}
	if _, err := a.client.CallJob(ctx, "pool.dataset.unlock", cfg.Dataset.ValueString(), opts); err != nil {
		resp.Diagnostics.AddError("Dataset unlock failed", err.Error())
		return
	}
}
