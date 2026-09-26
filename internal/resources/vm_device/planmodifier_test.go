// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKeepStateWhenConfigIsSubset(t *testing.T) {
	imported := `{"dtype":"DISPLAY","port":5900,"password":"x","bind":"0.0.0.0","web":true,"resolution":"1024x768"}`
	cases := []struct {
		name, config, state string
		stateNull, keep     bool
	}{
		{name: "subset with equal values keeps state", config: `{"dtype":"DISPLAY","port":5900,"password":"x"}`, state: imported, keep: true},
		{name: "key order and spacing do not matter", config: `{ "port": 5900, "dtype": "DISPLAY" }`, state: imported, keep: true},
		{name: "changed value plans the config", config: `{"dtype":"DISPLAY","port":5901}`, state: imported},
		{name: "key missing from state plans the config", config: `{"dtype":"DISPLAY","wait":true}`, state: imported},
		{name: "no state (create) plans the config", config: `{"dtype":"DISPLAY"}`, stateNull: true},
		{name: "unparsable config plans the config", config: `not json`, state: imported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := types.StringValue(tc.state)
			if tc.stateNull {
				state = types.StringNull()
			}
			req := planmodifier.StringRequest{ConfigValue: types.StringValue(tc.config), StateValue: state, PlanValue: types.StringValue(tc.config)}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
			keepStateWhenConfigIsSubset{}.PlanModifyString(context.Background(), req, resp)
			want := tc.config
			if tc.keep {
				want = tc.state
			}
			if got := resp.PlanValue.ValueString(); got != want {
				t.Errorf("plan = %s, want %s", got, want)
			}
		})
	}
}
