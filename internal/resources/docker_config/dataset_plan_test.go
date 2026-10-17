// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDatasetNeedsReplan guards the #46 fix: the computed dataset is invalidated
// exactly when the pool changes or is unknown, and preserved when the pool is
// unchanged (so an imported Docker instance reconfigured with a different/no
// pool accepts TrueNAS's dataset instead of failing with an inconsistent result,
// while a stable config does not churn).
func TestDatasetNeedsReplan(t *testing.T) {
	cases := []struct {
		name        string
		plan, state types.String
		wantReplan  bool
	}{
		{"pool unchanged -> keep dataset", types.StringValue("tank"), types.StringValue("tank"), false},
		{"pool changed -> replan dataset", types.StringValue("fast"), types.StringValue("tank"), true},
		{"pool unknown -> replan dataset", types.StringUnknown(), types.StringValue("tank"), true},
		{"pool cleared (null vs set) -> replan", types.StringNull(), types.StringValue("tank"), true},
		{"both null -> keep", types.StringNull(), types.StringNull(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := datasetNeedsReplan(tc.plan, tc.state); got != tc.wantReplan {
				t.Errorf("datasetNeedsReplan(plan=%v, state=%v) = %v, want %v", tc.plan, tc.state, got, tc.wantReplan)
			}
		})
	}
}
