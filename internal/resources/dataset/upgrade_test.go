// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// upgrade runs one stored state through upgradeFromFork and decodes the
// result into the current model.
func upgrade(t *testing.T, raw string) DatasetModel {
	t.Helper()
	ctx := context.Background()
	var resp resource.UpgradeStateResponse
	upgradeFromFork(ctx, resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(raw)}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics)
	}
	s := resourceSchema()
	val, err := resp.DynamicValue.Unmarshal(s.Type().TerraformType(ctx))
	if err != nil {
		t.Fatal(err)
	}
	var m DatasetModel
	if d := (tfsdk.State{Schema: s, Raw: val}).Get(ctx, &m); d.HasError() {
		t.Fatalf("decode: %v", d)
	}
	return m
}

// State as 1.1.0-terrahome.8 wrote it for a dataset inheriting most
// properties (TerraHome's Tank/Apps/Arcane/Data, abridged only in values).
const forkInheritingState = `{
  "aclmode": "passthrough", "acltype": "nfsv4", "atime": "off",
  "checksum": "INHERIT", "comments": "", "compression": "lz4", "copies": "1",
  "dedup": "INHERIT", "encrypted": false, "exec": "INHERIT",
  "id": "Tank/Apps/Arcane/Data", "mountpoint": "/mnt/Tank/Apps/Arcane/Data",
  "name": "Tank/Apps/Arcane/Data", "pool": "Tank", "quota": 0,
  "readonly": "INHERIT", "recordsize": "INHERIT", "refquota": 0,
  "reservation": 0, "share_type": null, "snapdir": "INHERIT",
  "special_small_block_size": "INHERIT", "sync": "INHERIT",
  "type": "FILESYSTEM", "volsize": 0
}`

func TestUpgradeFromForkInheriting(t *testing.T) {
	m := upgrade(t, forkInheritingState)
	if m.ACLMode.ValueString() != "PASSTHROUGH" || m.ATime.ValueString() != "OFF" {
		t.Errorf("enums not upper-cased: aclmode %v, atime %v", m.ACLMode, m.ATime)
	}
	for name, v := range map[string]interface{ IsNull() bool }{
		"checksum": m.Checksum, "dedup": m.Dedup, "exec": m.Exec, "readonly": m.ReadOnly,
		"recordsize": m.RecordSize, "snapdir": m.Snapdir, "special_small_block_size": m.SpecialSmallBlockSize,
		"sync": m.Sync, "refreservation": m.RefReservation, "encryption_generate_key": m.EncryptionGenerateKey,
	} {
		if !v.IsNull() {
			t.Errorf("%s = %v, want null", name, v)
		}
	}
	if m.Copies.ValueInt64() != 1 {
		t.Errorf("copies = %v, want 1", m.Copies)
	}
	if m.MountPoint.ValueString() != "/mnt/Tank/Apps/Arcane/Data" {
		t.Errorf("mountpoint = %v", m.MountPoint)
	}
}

// Tank/Secrets: local sizes, and the fork's encrypted = true.
func TestUpgradeFromForkEncryptedRoot(t *testing.T) {
	m := upgrade(t, `{"name": "Tank/Secrets", "id": "Tank/Secrets", "encrypted": true,
	  "recordsize": "128K", "special_small_block_size": "131072", "copies": "1", "exec": "off"}`)
	if m.SpecialSmallBlockSize.ValueInt64() != 131072 || m.RecordSize.ValueString() != "128K" {
		t.Errorf("sizes: ssbs %v, recordsize %v", m.SpecialSmallBlockSize, m.RecordSize)
	}
	// The create-only inputs stay null: state cannot tell an encryption root
	// from a child that inherits encryption, and null does not force a
	// replacement (see replaceUnlessImported).
	if !m.Encryption.ValueBool() || !m.InheritEncryption.IsNull() || !m.EncryptionGenerateKey.IsNull() {
		t.Errorf("encryption inputs: encryption %v, inherit %v, generate_key %v",
			m.Encryption, m.InheritEncryption, m.EncryptionGenerateKey)
	}
}

// A size of 0 is a real local value ("keep off the special vdev").
func TestUpgradeFromForkZeroSize(t *testing.T) {
	m := upgrade(t, `{"name": "Tank/x", "special_small_block_size": "0"}`)
	if m.SpecialSmallBlockSize.IsNull() || m.SpecialSmallBlockSize.ValueInt64() != 0 {
		t.Errorf("ssbs = %v, want 0", m.SpecialSmallBlockSize)
	}
}

// State upstream itself wrote is also version 0 and must come through as is.
func TestUpgradeUpstreamStateUnchanged(t *testing.T) {
	m := upgrade(t, `{"name": "tank/x", "id": "tank/x", "copies": 2, "special_small_block_size": 16384,
	  "atime": "OFF", "encryption": true, "inherit_encryption": null, "encryption_generate_key": null,
	  "encrypted": true, "xattr": "SA"}`)
	if m.Copies.ValueInt64() != 2 || m.SpecialSmallBlockSize.ValueInt64() != 16384 || m.ATime.ValueString() != "OFF" {
		t.Errorf("values changed: copies %v, ssbs %v, atime %v", m.Copies, m.SpecialSmallBlockSize, m.ATime)
	}
	if !m.InheritEncryption.IsNull() || !m.EncryptionGenerateKey.IsNull() {
		t.Errorf("encryption inputs invented for upstream state: %v %v", m.InheritEncryption, m.EncryptionGenerateKey)
	}
	if m.XAttr.ValueString() != "SA" {
		t.Errorf("xattr = %v", m.XAttr)
	}
}
