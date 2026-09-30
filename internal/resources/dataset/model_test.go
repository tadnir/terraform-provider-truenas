// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package dataset contains unit tests for the truenas_dataset resource model.
package dataset

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys is a regression test for a
// live acceptance failure: pool.dataset.update rejected the update payload
// with "[EINVAL] data.type: Extra inputs are not permitted" because the
// payload (built from the same apiPayload used for create) still included
// "type". The dataset id is passed as pool.dataset.update's first
// positional argument, not as a "name" payload key, so "name" must be
// stripped too.
func TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys(t *testing.T) {
	m := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Compression: types.StringValue("lz4"),
		Comments:    types.StringValue("updated"),
	}

	payload := m.updateAPIPayload()

	if _, ok := payload["type"]; ok {
		t.Errorf("update payload must not include \"type\", got: %v", payload)
	}
	if _, ok := payload["name"]; ok {
		t.Errorf("update payload must not include \"name\", got: %v", payload)
	}

	// Sanity: other writable fields still make it through.
	if payload["compression"] != "LZ4" {
		t.Errorf("expected compression=LZ4 to survive, got %v", payload["compression"])
	}
	if payload["comments"] != "updated" {
		t.Errorf("expected comments=updated to survive, got %v", payload["comments"])
	}
}

// TestDatasetCreateAPIPayloadStillIncludesType verifies the create payload
// (apiPayload) is unaffected by the update-only stripping in
// updateAPIPayload - type and name must still be present for pool.dataset.create.
func TestDatasetCreateAPIPayloadStillIncludesType(t *testing.T) {
	m := &DatasetModel{
		Name: types.StringValue("tank/mydata"),
		Type: types.StringValue("filesystem"),
	}

	payload := m.apiPayload()

	if payload["name"] != "tank/mydata" {
		t.Errorf("expected name=tank/mydata in create payload, got %v", payload["name"])
	}
	if payload["type"] != "FILESYSTEM" {
		t.Errorf("expected type=FILESYSTEM in create payload, got %v", payload["type"])
	}
}

// TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem is a regression test
// for a live acceptance failure: "truenas API error (code 22): 'volsize'".
// After a FILESYSTEM dataset is created and read back, VolSize is a known
// (but zero) value in state - responseToModel sets it from
// api.VolSize.Parsed, which is 0 for FILESYSTEM datasets. Because VolSize
// is Computed with UseStateForUnknown, that known-zero value flows into
// every later plan, so a plain null/unknown guard on apiPayload wasn't
// enough to keep "volsize" out of pool.dataset.update payloads for
// FILESYSTEM datasets. Only a real, known, non-zero size (as set for
// type=VOLUME) should be sent.
func TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem(t *testing.T) {
	m := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Compression: types.StringValue("lz4"),
		VolSize:     types.Int64Value(0), // as read back for FILESYSTEM datasets
	}

	payload := m.updateAPIPayload()

	if _, ok := payload["volsize"]; ok {
		t.Errorf("update payload for FILESYSTEM dataset must not include \"volsize\", got: %v", payload)
	}
}

// TestDatasetAPIPayloadIncludesVolsizeForVolume verifies that a real,
// non-zero volsize (as used by type=VOLUME datasets) still makes it into
// the payload - the fix must not suppress legitimate volsize values.
func TestDatasetAPIPayloadIncludesVolsizeForVolume(t *testing.T) {
	m := &DatasetModel{
		Name:    types.StringValue("tank/myzvol"),
		Type:    types.StringValue("volume"),
		VolSize: types.Int64Value(1073741824),
	}

	payload := m.apiPayload()

	if payload["volsize"] != int64(1073741824) {
		t.Errorf("expected volsize=1073741824 for VOLUME dataset, got %v", payload["volsize"])
	}
}

// TestDatasetUpdatePayloadOmitsShareType: pool.dataset.update rejects
// share_type as create-only, so a dataset that sets it in configuration
// must still be updatable in place (here, a comment change).
func TestDatasetUpdatePayloadOmitsShareType(t *testing.T) {
	m := &DatasetModel{
		Name:      types.StringValue("tank/mydata"),
		ShareType: types.StringValue("smb"),
		Comments:  types.StringValue("changed"),
	}
	if got := m.apiPayload()["share_type"]; got != "SMB" {
		t.Errorf("create payload must carry share_type=SMB, got %v", got)
	}
	if _, ok := m.updateAPIPayload()["share_type"]; ok {
		t.Error("update payload must not carry share_type")
	}
}

// TestDatasetUpdatePayloadOmitsACLType: acltype forces replacement, and
// resending it on update makes TrueNAS set aclmode and aclinherit locally.
func TestDatasetUpdatePayloadOmitsACLType(t *testing.T) {
	m := &DatasetModel{
		Name:    types.StringValue("tank/test"),
		AClType: types.StringValue("posix"),
		ATime:   types.StringValue("OFF"),
	}
	if _, ok := m.updateAPIPayload()["acltype"]; ok {
		t.Error("update payload must not carry acltype")
	}
	if got := m.apiPayload()["acltype"]; got != "POSIX" {
		t.Errorf("create payload acltype = %v, want POSIX", got)
	}
}

// TestDatasetMountpointAndPoolKeepStateOnUpdate: both follow from name,
// which forces replacement, so an in-place update must not plan them as
// unknown. An unknown mountpoint replaced every truenas_filesystem_acl whose
// path was built from it.
func TestDatasetMountpointAndPoolKeepStateOnUpdate(t *testing.T) {
	for _, name := range []string{"mountpoint", "pool"} {
		attr, ok := resourceSchema().Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is not a string attribute", name)
		}
		if len(attr.PlanModifiers) == 0 {
			t.Errorf("%s has no plan modifier; an update would plan it as unknown", name)
		}
	}
}

// TestEncryptionInputsReplaceUnlessImported: inherit_encryption and
// encryption_generate_key are create-only and never read back, so after an
// import state has no value for them. Setting them in configuration then
// must not recreate an encrypted dataset; changing a recorded value must.
func TestEncryptionInputsReplaceUnlessImported(t *testing.T) {
	ctx := context.Background()
	s := resourceSchema()
	objType := s.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for k, ty := range objType.AttributeTypes {
		vals[k] = tftypes.NewValue(ty, nil)
	}
	existing := tftypes.NewValue(objType, vals)
	for _, name := range []string{"inherit_encryption", "encryption_generate_key"} {
		attr := s.Attributes[name].(schema.BoolAttribute)
		for _, tc := range []struct {
			state types.Bool
			want  bool
		}{
			{types.BoolNull(), false},
			{types.BoolValue(false), true},
		} {
			req := planmodifier.BoolRequest{
				State:      tfsdk.State{Schema: s, Raw: existing},
				Plan:       tfsdk.Plan{Schema: s, Raw: existing},
				StateValue: tc.state,
				PlanValue:  types.BoolValue(true),
			}
			resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
			for _, m := range attr.PlanModifiers {
				m.PlanModifyBool(ctx, req, resp)
			}
			if resp.RequiresReplace != tc.want {
				t.Errorf("%s: state %v -> true: RequiresReplace = %v, want %v", name, tc.state, resp.RequiresReplace, tc.want)
			}
		}
	}
}
