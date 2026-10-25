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

func New() action.Action { return &Action{} }

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

// vmState is the VM's state from vm.status: RUNNING, STOPPED, SUSPENDED, ...
func vmState(ctx context.Context, c *client.Client, id int64) (string, error) {
	raw, err := c.CallRead(ctx, "vm.status", id)
	if err != nil {
		return "", err
	}
	var st struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", fmt.Errorf("parse vm.status: %w", err)
	}
	return st.State, nil
}

// Action starts a VM (vm.start). A VM that is already running is left as it
// is, so the action can be triggered again safely.
type Action struct{ client *client.Client }

type model struct {
	VMID       types.Int64 `tfsdk:"vm_id"`
	Overcommit types.Bool  `tfsdk:"overcommit"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_start"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Starts a VM (vm.start). Does nothing when the VM is already running.",
		Attributes: map[string]schema.Attribute{
			"vm_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM to start (truenas_vm.<name>.id).",
			},
			"overcommit": schema.BoolAttribute{
				Optional:    true,
				Description: "Start even when there isn't enough free memory for every configured VM. Default false.",
			},
		},
	}
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg model
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := cfg.VMID.ValueInt64()
	state, err := vmState(ctx, a.client, id)
	if err != nil {
		resp.Diagnostics.AddError("VM start failed", err.Error())
		return
	}
	if state == "RUNNING" {
		return
	}
	if _, err := a.client.Call(ctx, "vm.start", buildParams(cfg)...); err != nil {
		resp.Diagnostics.AddError("VM start failed", err.Error())
		return
	}
}

func buildParams(m model) []any {
	return []any{m.VMID.ValueInt64(), map[string]any{"overcommit": m.Overcommit.ValueBool()}}
}
