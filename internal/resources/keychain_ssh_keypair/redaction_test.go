// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUnmarshal_RedactedAttributes is the #57 regression for the keypair: a
// session without the KEYCHAIN_CREDENTIAL write role gets attributes back as
// "********". Unmarshaling must not fail; it flags Redacted.
func TestUnmarshal_RedactedAttributes(t *testing.T) {
	var api keychainSSHKeyPairAPI
	if err := json.Unmarshal([]byte(`{"id":2,"name":"kp","type":"SSH_KEY_PAIR","attributes":"********"}`), &api); err != nil {
		t.Fatalf("unmarshal of redacted attributes failed (the #57 crash): %v", err)
	}
	if !api.Attributes.Redacted {
		t.Error("Attributes.Redacted = false, want true")
	}
}

// TestUnmarshal_NormalAttributes verifies a normal object still parses.
func TestUnmarshal_NormalAttributes(t *testing.T) {
	var api keychainSSHKeyPairAPI
	if err := json.Unmarshal([]byte(`{"id":2,"name":"kp","type":"SSH_KEY_PAIR","attributes":{"private_key":"PEM","public_key":"ssh-ed25519 AAA"}}`), &api); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if api.Attributes.Redacted {
		t.Error("Redacted = true for a normal object, want false")
	}
	if api.Attributes.PrivateKey == nil || *api.Attributes.PrivateKey != "PEM" {
		t.Errorf("attributes not parsed: %+v", api.Attributes)
	}
}

// TestResponseToModel_RedactedErrors verifies a redacted response yields an
// actionable diagnostic naming the role.
func TestResponseToModel_RedactedErrors(t *testing.T) {
	api := &keychainSSHKeyPairAPI{Attributes: keychainSSHKeyPairAttributesAPI{Redacted: true}}
	diags := responseToModel(api, &KeychainSSHKeyPairModel{})
	if !diags.HasError() {
		t.Fatal("responseToModel(redacted) returned no error, want a diagnostic")
	}
	if !strings.Contains(diags[0].Detail(), "KEYCHAIN_CREDENTIAL_WRITE") {
		t.Errorf("diagnostic should name the write role; got %q", diags[0].Detail())
	}
	if d := responseToDataSourceModel(api, &KeychainSSHKeyPairDataSourceModel{}); !d.HasError() {
		t.Error("responseToDataSourceModel(redacted) returned no error, want a diagnostic")
	}
}
