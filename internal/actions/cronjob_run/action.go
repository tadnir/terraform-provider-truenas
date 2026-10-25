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

// Action runs a cron job now (cronjob.run; a job). It waits for the command to
// finish, and fails when the command exits non-zero (the job fails then).
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
		Description: "Runs a cron job immediately (cronjob.run) and waits for its command to finish. " +
			"Fails when the command exits with a non-zero status.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required:    true,
				Description: "Cron job id (truenas_cronjob.<name>.id).",
			},
			"skip_disabled": schema.BoolAttribute{
				Optional:    true,
				Description: "Refuse to run the job when it is disabled. Default false: a disabled job runs too, so a job kept disabled (never scheduled) can serve as a run-on-demand command.",
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
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: "Running cron job…"})
	}
	if _, err := a.client.CallJob(ctx, "cronjob.run", buildParams(cfg)...); err != nil {
		resp.Diagnostics.AddError("Cron job run failed", err.Error())
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: "Cron job finished."})
	}
}

func buildParams(m model) []any {
	return []any{m.ID.ValueInt64(), m.SkipDisabled.ValueBool()}
}
