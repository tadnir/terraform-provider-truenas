// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_restart

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

// Action restarts a VM (vm.restart; a job).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	VMID types.Int64 `tfsdk:"vm_id"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_restart"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Restarts a VM (vm.restart) — a graceful stop followed by a start.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to restart (truenas_vm.<name>.id).",
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
	if _, err := a.client.CallJob(ctx, "vm.restart", cfg.VMID.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("VM restart failed", err.Error())
		return
	}
}
