// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_TopLevelRequiredFields verifies "name" and "username" are
// Required, matching app.registry.create's own "required" list (probed
// live on both TrueNAS 25.10 and 26.0).
func TestSchema_TopLevelRequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, field := range []string{"name", "username"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", field, attr)
		}
		if !strAttr.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute
// is Computed-only.
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

// TestSchema_DescriptionOptionalNotComputed verifies "description" is
// Optional (nullable) and NOT Computed: app.registry.create's server-side
// default is null (not a computed non-null value), so no UseStateForUnknown
// plan modifier is needed and omitting it never causes drift.
func TestSchema_DescriptionOptionalNotComputed(t *testing.T) {
	s := resourceSchema()

	descAttr, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	descStr, ok := descAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", descAttr)
	}
	if !descStr.IsOptional() {
		t.Error("'description' should be Optional")
	}
	if descStr.IsComputed() {
		t.Error("'description' should NOT be Computed")
	}
	if descStr.IsRequired() {
		t.Error("'description' should not be Required")
	}
}

// TestSchema_URIOptionalComputed verifies "uri" is Optional+Computed: probed
// live, app.registry.create defaults uri server-side to
// "https://index.docker.io/v1/" when omitted, so it needs UseStateForUnknown
// to avoid perpetual diffs.
func TestSchema_URIOptionalComputed(t *testing.T) {
	s := resourceSchema()

	uriAttr, ok := s.Attributes["uri"]
	if !ok {
		t.Fatal("schema missing 'uri' attribute")
	}
	uriStr, ok := uriAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'uri' attribute is %T, want schema.StringAttribute", uriAttr)
	}
	if !uriStr.IsOptional() {
		t.Error("'uri' should be Optional")
	}
	if !uriStr.IsComputed() {
		t.Error("'uri' should be Computed")
	}
}

// TestSchema_PasswordWORequiredSensitiveWriteOnly verifies "password_wo" is
// Required, Sensitive, WriteOnly, and NOT Computed: app.registry.create
// requires a credential (probed live on both releases), the API documents it
// as "masked for security", and no live read-back evidence exists to persist
// it in state (see model.go's doc comment for the full decisive-probe
// writeup on why no entry could be created to observe a read-back value).
// The legacy plain "password" attribute must be gone (renamed to the
// write-only password_wo/password_wo_version pair — #48).
func TestSchema_PasswordWORequiredSensitiveWriteOnly(t *testing.T) {
	s := resourceSchema()

	if _, ok := s.Attributes["password"]; ok {
		t.Error("schema must not expose a plain 'password' attribute; use write-only 'password_wo'")
	}

	pwAttr, ok := s.Attributes["password_wo"]
	if !ok {
		t.Fatal("schema missing 'password_wo' attribute")
	}
	pwStr, ok := pwAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password_wo' attribute is %T, want schema.StringAttribute", pwAttr)
	}
	if !pwStr.IsRequired() {
		t.Error("'password_wo' should be Required")
	}
	if !pwStr.IsSensitive() {
		t.Error("'password_wo' should be Sensitive")
	}
	if !pwStr.IsWriteOnly() {
		t.Error("'password_wo' should be WriteOnly")
	}
	if pwStr.IsComputed() {
		t.Error("'password_wo' should NOT be Computed")
	}
}

// TestSchema_PasswordWOVersionRequiredInt64 verifies the rotation trigger
// "password_wo_version" is a Required Int64 (not write-only, not computed):
// it must be stored in state so a change produces the plan diff that drives
// app.registry.update to re-send a rotated password_wo. (#48)
func TestSchema_PasswordWOVersionRequiredInt64(t *testing.T) {
	s := resourceSchema()

	vAttr, ok := s.Attributes["password_wo_version"]
	if !ok {
		t.Fatal("schema missing 'password_wo_version' attribute")
	}
	vInt, ok := vAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'password_wo_version' attribute is %T, want schema.Int64Attribute", vAttr)
	}
	if !vInt.IsRequired() {
		t.Error("'password_wo_version' should be Required")
	}
	if vInt.IsWriteOnly() {
		t.Error("'password_wo_version' must NOT be WriteOnly (it is the state-stored rotation trigger)")
	}
	if vInt.IsComputed() {
		t.Error("'password_wo_version' should NOT be Computed")
	}
}
