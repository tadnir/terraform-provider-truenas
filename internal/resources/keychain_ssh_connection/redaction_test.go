// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUnmarshal_RedactedAttributes is the #57 regression: a session without the
// KEYCHAIN_CREDENTIAL write role gets attributes back as the string "********".
// Unmarshaling must not fail; it flags Redacted instead.
func TestUnmarshal_RedactedAttributes(t *testing.T) {
	var api keychainSSHConnectionAPI
	if err := json.Unmarshal([]byte(`{"id":4,"name":"example","type":"SSH_CREDENTIALS","attributes":"********"}`), &api); err != nil {
		t.Fatalf("unmarshal of redacted attributes failed (the #57 crash): %v", err)
	}
	if !api.Attributes.Redacted {
		t.Error("Attributes.Redacted = false, want true for a \"********\" attributes value")
	}
}

// TestUnmarshal_NormalAttributes verifies a normal object still parses into the
// fields and is not flagged redacted.
func TestUnmarshal_NormalAttributes(t *testing.T) {
	var api keychainSSHConnectionAPI
	if err := json.Unmarshal([]byte(`{"id":4,"name":"example","type":"SSH_CREDENTIALS","attributes":{"host":"h","port":22,"username":"root","private_key":2,"remote_host_key":"k","connect_timeout":10}}`), &api); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if api.Attributes.Redacted {
		t.Error("Redacted = true for a normal object, want false")
	}
	if api.Attributes.Host != "h" || api.Attributes.Port != 22 {
		t.Errorf("attributes not parsed: %+v", api.Attributes)
	}
}

// TestResponseToModel_RedactedErrors verifies a redacted response yields an
// actionable diagnostic naming the role, not a silent/zeroed read.
func TestResponseToModel_RedactedErrors(t *testing.T) {
	api := &keychainSSHConnectionAPI{Attributes: keychainSSHConnectionAttributesAPI{Redacted: true}}
	diags := responseToModel(api, &KeychainSSHConnectionModel{})
	if !diags.HasError() {
		t.Fatal("responseToModel(redacted) returned no error, want a diagnostic")
	}
	if !strings.Contains(diags[0].Detail(), "KEYCHAIN_CREDENTIAL_WRITE") {
		t.Errorf("diagnostic should name the write role; got %q", diags[0].Detail())
	}

	dsDiags := responseToDataSourceModel(api, &KeychainSSHConnectionDataSourceModel{})
	if !dsDiags.HasError() {
		t.Error("responseToDataSourceModel(redacted) returned no error, want a diagnostic")
	}
}
