// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_clone

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

// Action clones a VM (vm.clone), including its devices/disks.
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	VMID types.Int64  `tfsdk:"vm_id"`
	Name types.String `tfsdk:"name"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_clone"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Clones a VM (vm.clone), copying its configuration and zvol disks into a new VM.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to clone (truenas_vm.<name>.id).",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Name for the cloned VM. If omitted, TrueNAS derives one from the source.",
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
	params := []any{cfg.VMID.ValueInt64()}
	if !cfg.Name.IsNull() && !cfg.Name.IsUnknown() {
		params = append(params, cfg.Name.ValueString())
	}
	if _, err := a.client.Call(ctx, "vm.clone", params...); err != nil {
		resp.Diagnostics.AddError("VM clone failed", err.Error())
		return
	}
}
