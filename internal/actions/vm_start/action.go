// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_start

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ action.Action = &Action{}
var _ action.ActionWithConfigure = &Action{}

// Action starts a VM (vm.start; a plain call). It first reads vm.status and
// no-ops when the VM is already running, so it is safe to trigger again (e.g.
// an after_create on a truenas_vm_device, which fires for every device).
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	VMID       types.Int64 `tfsdk:"vm_id"`
	Overcommit types.Bool  `tfsdk:"overcommit"`
}

// vmStatus is the subset of vm.status's response this action reads.
type vmStatus struct {
	State string `json:"state"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_start"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Starts a VM (vm.start). No-op when the VM is already running, so it is safe to " +
			"trigger repeatedly — e.g. after each truenas_vm_device is created.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to start (truenas_vm.<name>.id).",
			},
			"overcommit": schema.BoolAttribute{
				Optional:    true,
				Description: "Allow starting even if it would overcommit host memory (vm.start's overcommit option).",
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

// shouldStart reports whether a VM in the given vm.status state needs starting.
// Only a non-running VM is started; "RUNNING" is a no-op.
func shouldStart(state string) bool {
	return state != "RUNNING"
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg model
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := cfg.VMID.ValueInt64()

	raw, err := a.client.CallRead(ctx, "vm.status", id)
	if err != nil {
		resp.Diagnostics.AddError("VM status read failed", err.Error())
		return
	}
	var st vmStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		resp.Diagnostics.AddError("Parse vm.status response", err.Error())
		return
	}
	if !shouldStart(st.State) {
		if resp.SendProgress != nil {
			resp.SendProgress(action.InvokeProgressEvent{Message: "VM already running; nothing to do"})
		}
		return
	}

	params := []any{id}
	if !cfg.Overcommit.IsNull() && !cfg.Overcommit.IsUnknown() {
		params = append(params, map[string]any{"overcommit": cfg.Overcommit.ValueBool()})
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: "Starting VM…"})
	}
	if _, err := a.client.Call(ctx, "vm.start", params...); err != nil {
		resp.Diagnostics.AddError("VM start failed", err.Error())
		return
	}
}
