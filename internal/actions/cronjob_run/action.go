// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob_run

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

// Action runs a cron job immediately (cronjob.run; a job). It waits for the
// job to finish and fails if the command exits non-zero, mirroring the other
// run-now actions (snapshot_task_run, cloudsync_run). A disabled cron job (one
// that is never scheduled, enabled = false) still runs when triggered unless
// skip_disabled is set — the common case for a job that exists only to be run
// by this action.
type Action struct{ client *client.Client }

func New() action.Action { return &Action{} }

type model struct {
	ID           types.Int64 `tfsdk:"id"`
	SkipDisabled types.Bool  `tfsdk:"skip_disabled"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cronjob_run"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Runs a cron job immediately (cronjob.run). Waits for the job and fails if the " +
			"command exits non-zero.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required:    true,
				Description: "Cron job id (truenas_cronjob.<name>.id).",
			},
			"skip_disabled": schema.BoolAttribute{
				Optional: true,
				Description: "Skip execution when the cron job is disabled (enabled = false). Defaults to " +
					"false, so a disabled job still runs when triggered — the usual case for a job that exists " +
					"only to be run by this action.",
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

// buildParams builds the cronjob.run positional arguments: the id, plus
// skip_disabled only when the caller set it (otherwise the server default,
// false, applies).
func buildParams(m model) []any {
	params := []any{m.ID.ValueInt64()}
	if !m.SkipDisabled.IsNull() && !m.SkipDisabled.IsUnknown() {
		params = append(params, m.SkipDisabled.ValueBool())
	}
	return params
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg model
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: "Running cron job…"})
	}
	if _, err := a.client.CallJob(ctx, "cronjob.run", buildParams(cfg)...); err != nil {
		resp.Diagnostics.AddError("Cron job run failed", err.Error())
		return
	}
}
