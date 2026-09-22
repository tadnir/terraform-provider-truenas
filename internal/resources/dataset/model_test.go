// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package dataset contains unit tests for the truenas_dataset resource model.
package dataset

import (
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

// TestDatasetPayloadIncludesZeroSpecialSmallBlockSize pins the difference
// between special_small_block_size and volsize. For volsize a zero is an
// artefact of reading back a FILESYSTEM dataset and must be suppressed; for
// special_small_block_size zero is a real, meaningful setting (it disables
// routing small blocks to the pool's special vdev), so a configured 0 has
// to reach the payload. Guarding this one on != 0 as well would make
// "special_small_block_size = 0" silently unenforceable.
func TestDatasetPayloadIncludesZeroSpecialSmallBlockSize(t *testing.T) {
	m := &DatasetModel{
		Name:                  types.StringValue("tank/mydata"),
		SpecialSmallBlockSize: types.Int64Value(0),
	}

	payload := m.apiPayload()

	v, ok := payload["special_small_block_size"]
	if !ok {
		t.Fatalf("configured special_small_block_size=0 must reach the payload, got: %v", payload)
	}
	if v != int64(0) {
		t.Errorf("expected special_small_block_size=0, got %v", v)
	}
}

// TestDatasetPayloadOmitsUnsetSpecialSmallBlockSize covers the ordinary
// case: a dataset whose configuration says nothing about the property must
// not send the key at all, so TrueNAS leaves it inherited.
func TestDatasetPayloadOmitsUnsetSpecialSmallBlockSize(t *testing.T) {
	m := &DatasetModel{
		Name:                  types.StringValue("tank/mydata"),
		SpecialSmallBlockSize: types.Int64Null(),
	}

	if _, ok := m.apiPayload()["special_small_block_size"]; ok {
		t.Errorf("null special_small_block_size must be omitted from the payload")
	}
}

// TestDatasetResponseToModelKeepsLocalSpecialSmallBlockSize verifies that a
// property genuinely set on this dataset is read into state.
func TestDatasetResponseToModelKeepsLocalSpecialSmallBlockSize(t *testing.T) {
	api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
	parsed := int64(16384)
	api.SpecialSmallBlockSize.Parsed = &parsed
	api.SpecialSmallBlockSize.Source = "LOCAL"

	m := &DatasetModel{}
	(&DatasetResource{}).responseToModel(api, m)

	if m.SpecialSmallBlockSize.IsNull() {
		t.Fatal("a LOCAL special_small_block_size must be recorded in state")
	}
	if got := m.SpecialSmallBlockSize.ValueInt64(); got != 16384 {
		t.Errorf("expected 16384, got %d", got)
	}
}

// TestDatasetResponseToModelNullsInheritedSpecialSmallBlockSize is the
// regression this attribute's design exists for. pool.dataset.get_instance
// reports the *effective* value for an inherited property, so reading the
// value alone would put a number in state for every dataset in the pool.
// Because the attribute is Optional+Computed, that number would then be
// written back by the next pool.dataset.update and convert an inherited
// property into a local one behind the user's back - the same shape of bug
// as the volsize regression above, except that a zero check cannot catch
// it, because 0 is a legitimate value here. "source" is the discriminator.
func TestDatasetResponseToModelNullsInheritedSpecialSmallBlockSize(t *testing.T) {
	for _, source := range []string{"INHERITED", "DEFAULT", "RECEIVED"} {
		t.Run(source, func(t *testing.T) {
			api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
			parsed := int64(16384)
			api.SpecialSmallBlockSize.Parsed = &parsed
			api.SpecialSmallBlockSize.Source = source

			m := &DatasetModel{}
			(&DatasetResource{}).responseToModel(api, m)

			if !m.SpecialSmallBlockSize.IsNull() {
				t.Fatalf("source=%s must leave special_small_block_size null, got %d",
					source, m.SpecialSmallBlockSize.ValueInt64())
			}
			if _, ok := m.updateAPIPayload()["special_small_block_size"]; ok {
				t.Errorf("source=%s must not send special_small_block_size on update", source)
			}
		})
	}
}

// TestDatasetResponseToModelHandlesAbsentSpecialSmallBlockSize covers a
// get_instance response that omits the property entirely, which is why
// Parsed is a pointer rather than a plain int64.
func TestDatasetResponseToModelHandlesAbsentSpecialSmallBlockSize(t *testing.T) {
	api := &apiResponse{Name: "tank/myzvol", Type: "VOLUME"}

	m := &DatasetModel{}
	(&DatasetResource{}).responseToModel(api, m)

	if !m.SpecialSmallBlockSize.IsNull() {
		t.Errorf("an absent special_small_block_size must be null, got %d",
			m.SpecialSmallBlockSize.ValueInt64())
	}
}
