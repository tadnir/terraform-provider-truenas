// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestZvolEncryptionPayload(t *testing.T) {
	m := &ZvolModel{
		Name: types.StringValue("fast/v"), VolSize: types.Int64Value(1 << 26),
		Encryption:            types.BoolValue(true),
		InheritEncryption:     types.BoolValue(false),
		EncryptionAlgorithm:   types.StringValue("AES-256-GCM"),
		EncryptionGenerateKey: types.BoolValue(true),
	}
	p := m.apiPayload()
	if p["encryption"] != true {
		t.Errorf("payload should include encryption=true")
	}
	eo, ok := p["encryption_options"].(map[string]any)
	if !ok || eo["algorithm"] != "AES-256-GCM" || eo["generate_key"] != true {
		t.Errorf("encryption_options wrong: %v", p["encryption_options"])
	}
	// write-only passphrase/key come from config via injectEncryptionSecrets
	cfg := &ZvolModel{EncryptionPassphrase: types.StringValue("pw12345678")}
	injectEncryptionSecrets(p, cfg)
	if p["encryption_options"].(map[string]any)["passphrase"] != "pw12345678" {
		t.Errorf("passphrase not injected: %v", p["encryption_options"])
	}
}

func TestZvolInheritEncryptionReconcile(t *testing.T) {
	root := "fast/parent"
	// inherited (root != self) => inherit_encryption true
	api := &zvolAPI{Name: "fast/parent/child", Encrypted: true, EncryptionRoot: &root}
	var m ZvolModel
	responseToModel(api, &m)
	if !m.InheritEncryption.ValueBool() {
		t.Errorf("child whose encryption_root is an ancestor should be inherit_encryption=true")
	}
	// own key (root == self) => false
	self := "fast/own"
	api2 := &zvolAPI{Name: "fast/own", Encrypted: true, EncryptionRoot: &self}
	var m2 ZvolModel
	responseToModel(api2, &m2)
	if m2.InheritEncryption.ValueBool() {
		t.Errorf("zvol owning its key should be inherit_encryption=false")
	}
	// not encrypted => false, and passphrase/key never populated from API
	api3 := &zvolAPI{Name: "fast/plain", Encrypted: false}
	var m3 ZvolModel
	responseToModel(api3, &m3)
	if m3.InheritEncryption.ValueBool() {
		t.Errorf("unencrypted zvol should be inherit_encryption=false")
	}
	if !m3.EncryptionPassphrase.IsNull() && m3.EncryptionPassphrase.ValueString() != "" {
		t.Errorf("passphrase must not be read back from API")
	}
}
