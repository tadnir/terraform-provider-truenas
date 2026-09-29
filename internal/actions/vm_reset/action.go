// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_reset

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

// Action hard-resets a VM (vm.reset). Requires TrueNAS 27.0+ (the method does
// not exist on earlier releases).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	VMID types.Int64 `tfsdk:"vm_id"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_reset"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Hard-resets a VM (vm.reset) — the equivalent of a power-cycle. Requires TrueNAS 27.0 or newer.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to reset (truenas_vm.<name>.id).",
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
	// vm.reset was added in TrueNAS 27.0; fail clearly on older releases rather
	// than with a raw "method not found".
	if ok, err := a.client.VersionAtLeast(ctx, 27, 0); err == nil && !ok {
		resp.Diagnostics.AddError("vm_reset requires TrueNAS 27.0+",
			"The vm.reset API does not exist on this TrueNAS release. Use truenas_vm_restart instead.")
		return
	}
	if _, err := a.client.Call(ctx, "vm.reset", cfg.VMID.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("VM reset failed", err.Error())
		return
	}
}
