// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestReplaceIfStorePrivatekeyChanged drives the actual store_privatekey plan
// modifier and guards the #49 regression (PR #59): a null prior state (an
// upgrade from before the attribute existed) planning the default true must NOT
// force replacement, while a real flip still must. A revert to an unconditional
// RequiresReplace() fails the first case.
func TestReplaceIfStorePrivatekeyChanged(t *testing.T) {
	// Non-null State/Plan raw so the modifier doesn't short-circuit as a
	// create (State.Raw null) or destroy (Plan.Raw null).
	nonNull := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})

	cases := []struct {
		name        string
		state, plan types.Bool
		wantReplace bool
	}{
		{"upgrade null -> default true: no replace", types.BoolNull(), types.BoolValue(true), false},
		{"unchanged true -> true: no replace", types.BoolValue(true), types.BoolValue(true), false},
		{"flip true -> false: replace", types.BoolValue(true), types.BoolValue(false), true},
		{"null -> false: replace", types.BoolNull(), types.BoolValue(false), true},
		{"flip false -> true: replace", types.BoolValue(false), types.BoolValue(true), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.BoolRequest{
				StateValue: tc.state,
				PlanValue:  tc.plan,
				State:      tfsdk.State{Raw: nonNull},
				Plan:       tfsdk.Plan{Raw: nonNull},
			}
			resp := &planmodifier.BoolResponse{PlanValue: tc.plan}
			replaceIfStorePrivatekeyChanged().PlanModifyBool(context.Background(), req, resp)
			if resp.RequiresReplace != tc.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.wantReplace)
			}
		})
	}
}
