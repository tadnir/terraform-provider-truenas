// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestEffectiveTransport(t *testing.T) {
	cases := []struct {
		name string
		in   types.String
		want string
	}{
		{"null reads as LOCAL (upgrade case)", types.StringNull(), "LOCAL"},
		{"unknown reads as LOCAL", types.StringUnknown(), "LOCAL"},
		{"explicit LOCAL", types.StringValue("LOCAL"), "LOCAL"},
		{"explicit SSH", types.StringValue("SSH"), "SSH"},
		{"explicit SSH+NETCAT", types.StringValue("SSH+NETCAT"), "SSH+NETCAT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveTransport(tc.in); got != tc.want {
				t.Errorf("effectiveTransport = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReplaceIfTransportChanged drives the actual transport plan modifier and
// guards the upgrade-replace trap: a null prior state (an upgrade from before
// the attribute existed, or a hand-edited state) planning the default "LOCAL"
// must NOT force replacement, while a real transport change still must. A
// revert to an unconditional RequiresReplace() fails the first case.
func TestReplaceIfTransportChanged(t *testing.T) {
	// Non-null State/Plan raw so the modifier doesn't short-circuit as a
	// create (State.Raw null) or destroy (Plan.Raw null).
	nonNull := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	cases := []struct {
		name        string
		state, plan types.String
		wantReplace bool
	}{
		{"upgrade null -> default LOCAL: no replace", types.StringNull(), types.StringValue("LOCAL"), false},
		{"unchanged LOCAL -> LOCAL: no replace", types.StringValue("LOCAL"), types.StringValue("LOCAL"), false},
		{"unchanged SSH -> SSH: no replace", types.StringValue("SSH"), types.StringValue("SSH"), false},
		{"change LOCAL -> SSH: replace", types.StringValue("LOCAL"), types.StringValue("SSH"), true},
		{"null -> SSH: replace", types.StringNull(), types.StringValue("SSH"), true},
		{"change SSH -> SSH+NETCAT: replace", types.StringValue("SSH"), types.StringValue("SSH+NETCAT"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: tc.state,
				PlanValue:  tc.plan,
				State:      tfsdk.State{Raw: nonNull},
				Plan:       tfsdk.Plan{Raw: nonNull},
			}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			replaceIfTransportChanged().PlanModifyString(context.Background(), req, resp)
			if resp.RequiresReplace != tc.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.wantReplace)
			}
		})
	}
}
