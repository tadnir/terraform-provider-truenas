// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestSchema_TopLevelRequiredFields verifies "name" and "create_type" are
// Required, matching certificate.create's own "required" list.
func TestSchema_TopLevelRequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, field := range []string{"name", "create_type"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		req, ok := attr.(interface{ IsRequired() bool })
		if !ok || !req.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies "id" is Computed-only.
func TestSchema_IDComputedUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestSchema_CreateTypeRequiresReplace verifies "create_type" carries
// RequiresReplace, since certificate.update never accepts it (probed live).
func TestSchema_CreateTypeRequiresReplace(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["create_type"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'create_type' attribute is %T, want schema.StringAttribute", s.Attributes["create_type"])
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("'create_type' should have plan modifiers (RequiresReplace)")
	}
}

// TestSchema_UpdatableFieldsNotRequiresReplace verifies the three fields
// certificate.update actually accepts (name, renew_days,
// add_to_trusted_store — probed live) do NOT carry RequiresReplace, while
// spot-checking that an immutable field (certificate) does.
func TestSchema_UpdatableFieldsNotRequiresReplace(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", s.Attributes["name"])
	}
	if len(nameAttr.PlanModifiers) != 0 {
		t.Error("'name' should be updatable in place (no RequiresReplace)")
	}

	certAttr, ok := s.Attributes["certificate"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'certificate' attribute is %T, want schema.StringAttribute", s.Attributes["certificate"])
	}
	if len(certAttr.PlanModifiers) == 0 {
		t.Error("'certificate' should carry RequiresReplace (certificate.update rejects it, probed live)")
	}
}

// TestSchema_PrivatekeySensitiveNotWriteOnly verifies "privatekey" is
// Sensitive+Computed (read back on refresh) rather than WriteOnly — see
// model.go's certificateAPI doc comment for the live probe evidence
// (certificate.get_instance returns it intact; only the create/update job
// result masks it).
func TestSchema_PrivatekeySensitiveNotWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["privatekey"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'privatekey' attribute is %T, want schema.StringAttribute", s.Attributes["privatekey"])
	}
	if !attr.Sensitive {
		t.Error("'privatekey' should be Sensitive")
	}
	if !attr.Computed {
		t.Error("'privatekey' should be Computed (read back from certificate.get_instance)")
	}
	if attr.WriteOnly {
		t.Error("'privatekey' should NOT be WriteOnly")
	}
}

// TestSchema_PassphraseWriteOnlyByConvention verifies "passphrase" is
// Sensitive and Optional but deliberately NOT Computed, since the API never
// echoes it back (probed live: absent from every create/update/
// get_instance/query response).
func TestSchema_PassphraseWriteOnlyByConvention(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["passphrase"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'passphrase' attribute is %T, want schema.StringAttribute", s.Attributes["passphrase"])
	}
	if !attr.Sensitive {
		t.Error("'passphrase' should be Sensitive")
	}
	if !attr.Optional {
		t.Error("'passphrase' should be Optional")
	}
	if attr.Computed {
		t.Error("'passphrase' should NOT be Computed — the API never returns it")
	}
}

func TestStorePrivatekeyChanged(t *testing.T) {
	ctx := context.Background()
	stateObj := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	cases := []struct {
		name        string
		state, plan types.Bool
		noState     bool
		want        bool
	}{
		{"create", types.BoolNull(), types.BoolValue(false), true, false},
		{"null state to default", types.BoolNull(), types.BoolValue(true), false, false},
		{"null state to false", types.BoolNull(), types.BoolValue(false), false, true},
		{"true to false", types.BoolValue(true), types.BoolValue(false), false, true},
		{"false to true", types.BoolValue(false), types.BoolValue(true), false, true},
		{"unchanged false", types.BoolValue(false), types.BoolValue(false), false, false},
		{"unknown plan", types.BoolValue(true), types.BoolUnknown(), false, false},
	}
	for _, c := range cases {
		req := planmodifier.BoolRequest{StateValue: c.state, PlanValue: c.plan}
		if c.noState {
			req.State.Raw = tftypes.NewValue(tftypes.Object{}, nil)
		} else {
			req.State.Raw = stateObj
		}
		var resp boolplanmodifier.RequiresReplaceIfFuncResponse
		storePrivatekeyChanged(ctx, req, &resp)
		if resp.RequiresReplace != c.want {
			t.Errorf("%s: RequiresReplace = %v, want %v", c.name, resp.RequiresReplace, c.want)
		}
	}
}
