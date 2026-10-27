// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_stop

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

// Action stops a VM (vm.stop; a job). It first reads vm.status and no-ops when
// the VM is already stopped, so it is safe to trigger again — e.g. a
// before_destroy on a truenas_vm_device, which fires for every device.
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	VMID              types.Int64 `tfsdk:"vm_id"`
	Force             types.Bool  `tfsdk:"force"`
	ForceAfterTimeout types.Bool  `tfsdk:"force_after_timeout"`
}

// vmStatus is the subset of vm.status's response this action reads.
type vmStatus struct {
	State string `json:"state"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_stop"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Stops a VM (vm.stop). No-op when the VM is already stopped, so it is safe to " +
			"trigger repeatedly — e.g. before each truenas_vm_device is destroyed.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to stop (truenas_vm.<name>.id).",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Power the VM off immediately instead of requesting a graceful shutdown (vm.stop's force option).",
			},
			"force_after_timeout": schema.BoolAttribute{
				Optional:    true,
				Description: "Force the VM off if it has not shut down gracefully within the timeout (vm.stop's force_after_timeout option).",
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

// shouldStop reports whether a VM in the given vm.status state needs stopping.
// Only a running VM is stopped; anything else (STOPPED, etc.) is a no-op.
func shouldStop(state string) bool {
	return state == "RUNNING"
}

// stopOptions builds the vm.stop options object, including only the flags the
// caller set so the server defaults apply to the rest.
func stopOptions(m model) map[string]any {
	opts := map[string]any{}
	if !m.Force.IsNull() && !m.Force.IsUnknown() {
		opts["force"] = m.Force.ValueBool()
	}
	if !m.ForceAfterTimeout.IsNull() && !m.ForceAfterTimeout.IsUnknown() {
		opts["force_after_timeout"] = m.ForceAfterTimeout.ValueBool()
	}
	return opts
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
	if !shouldStop(st.State) {
		if resp.SendProgress != nil {
			resp.SendProgress(action.InvokeProgressEvent{Message: "VM already stopped; nothing to do"})
		}
		return
	}

	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: "Stopping VM…"})
	}
	if _, err := a.client.CallJob(ctx, "vm.stop", id, stopOptions(cfg)); err != nil {
		resp.Diagnostics.AddError("VM stop failed", err.Error())
		return
	}
}
