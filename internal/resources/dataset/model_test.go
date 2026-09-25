// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package dataset contains unit tests for the truenas_dataset resource model.
package dataset

import (
	"encoding/json"
	"strings"
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
	api.SpecialSmallBlockSize.Parsed = propertyBytes{Set: true, Value: 16384}
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
			api.SpecialSmallBlockSize.Parsed = propertyBytes{Set: true, Value: 16384}
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

// TestPropertyBytesAcceptsBothWireShapes is the regression test for the
// failure that the first live acceptance run hit: every dataset test failed
// at json.Unmarshal because special_small_block_size's "parsed" arrives as a
// JSON string ("0"), while the sibling byte-valued property recordsize
// arrives as a JSON number (1048576) in the very same response. Both forms
// have to decode.
func TestPropertyBytesAcceptsBothWireShapes(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		wantSet bool
		want    int64
	}{
		{"string zero, as probed on 25.10", `{"parsed": "0"}`, true, 0},
		{"string decimal", `{"parsed": "16384"}`, true, 16384},
		{"string with binary suffix", `{"parsed": "16K"}`, true, 16384},
		{"number, as recordsize returns", `{"parsed": 1048576}`, true, 1048576},
		{"null", `{"parsed": null}`, false, 0},
		{"key absent entirely", `{}`, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Parsed propertyBytes `json:"parsed"`
			}
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatalf("unmarshalling %s: %v", tc.json, err)
			}
			if got.Parsed.Set != tc.wantSet {
				t.Fatalf("Set = %v, want %v", got.Parsed.Set, tc.wantSet)
			}
			if got.Parsed.Value != tc.want {
				t.Errorf("Value = %d, want %d", got.Parsed.Value, tc.want)
			}
		})
	}
}

// TestPropertyBytesRejectsNonNumericString makes sure a value that is not a
// byte count surfaces as a decode error rather than a silent zero, which
// would read back as "not set" and hide a real API change.
func TestPropertyBytesRejectsNonNumericString(t *testing.T) {
	var got struct {
		Parsed propertyBytes `json:"parsed"`
	}
	if err := json.Unmarshal([]byte(`{"parsed": "INHERIT"}`), &got); err == nil {
		t.Fatal("expected an error for a non-numeric parsed value, got none")
	}
}

// localStringProperty describes one enum-valued ZFS property that
// truenas_dataset reads source-aware (see localString). The table below is
// what the tests in this section run over, so a new property of this kind
// needs only a row here to be covered.
type localStringProperty struct {
	attr   string // Terraform attribute
	apiKey string // pool.dataset key, in payloads and in get_instance
	raw    string // a ZFS raw value, as get_instance reports it
	set    func(*apiResponse, localProperty)
	get    func(*DatasetModel) types.String
	put    func(*DatasetModel, types.String)
}

var localStringProperties = []localStringProperty{
	{
		attr: "atime", apiKey: "atime", raw: "off",
		set: func(a *apiResponse, p localProperty) { a.ATime = p },
		get: func(m *DatasetModel) types.String { return m.ATime },
		put: func(m *DatasetModel, v types.String) { m.ATime = v },
	},
	{
		attr: "dedup", apiKey: "deduplication", raw: "off",
		set: func(a *apiResponse, p localProperty) { a.Dedup = p },
		get: func(m *DatasetModel) types.String { return m.Dedup },
		put: func(m *DatasetModel, v types.String) { m.Dedup = v },
	},
}

func strPtr(s string) *string { return &s }

// TestDatasetLocalStringPropertiesKeepLocal checks that a property set on
// the dataset itself reaches state, in the casing the configuration used.
func TestDatasetLocalStringPropertiesKeepLocal(t *testing.T) {
	for _, p := range localStringProperties {
		t.Run(p.attr, func(t *testing.T) {
			api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
			p.set(api, localProperty{RawValue: strPtr(p.raw), Source: "LOCAL"})

			m := &DatasetModel{}
			p.put(m, types.StringValue(strings.ToUpper(p.raw)))
			(&DatasetResource{}).responseToModel(api, m)

			if got := p.get(m); got.IsNull() || got.ValueString() != strings.ToUpper(p.raw) {
				t.Fatalf("LOCAL %s must be kept as configured (%q), got %v", p.attr, strings.ToUpper(p.raw), got)
			}
		})
	}
}

// TestDatasetLocalStringPropertiesNullWhenNotLocal is the same regression
// guard as TestDatasetResponseToModelNullsInheritedSpecialSmallBlockSize,
// for every enum property: get_instance reports the effective value of an
// inherited property, and recording it would make the next update write it
// back as a local setting.
func TestDatasetLocalStringPropertiesNullWhenNotLocal(t *testing.T) {
	for _, p := range localStringProperties {
		for _, prop := range []localProperty{
			{RawValue: strPtr(p.raw), Source: "INHERITED"},
			{RawValue: strPtr(p.raw), Source: "DEFAULT"},
			{RawValue: strPtr(p.raw), Source: "RECEIVED"},
			{RawValue: nil, Source: "LOCAL"},
			{},
		} {
			t.Run(p.attr+"/"+prop.Source, func(t *testing.T) {
				api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
				p.set(api, prop)

				m := &DatasetModel{}
				(&DatasetResource{}).responseToModel(api, m)

				if got := p.get(m); !got.IsNull() {
					t.Fatalf("%s with source %q must be null, got %v", p.attr, prop.Source, got)
				}
				if _, ok := m.updateAPIPayload()[p.apiKey]; ok {
					t.Errorf("%s with source %q must not be sent on update", p.attr, prop.Source)
				}
			})
		}
	}
}

// TestDatasetLocalStringPropertiesPayload checks that a configured value is
// sent upper-cased under the API's key, and that an unset one is omitted so
// TrueNAS leaves the property inherited.
func TestDatasetLocalStringPropertiesPayload(t *testing.T) {
	for _, p := range localStringProperties {
		t.Run(p.attr, func(t *testing.T) {
			m := &DatasetModel{Name: types.StringValue("tank/mydata")}
			p.put(m, types.StringValue(p.raw))
			if got := m.apiPayload()[p.apiKey]; got != strings.ToUpper(p.raw) {
				t.Errorf("expected %s=%q in the payload, got %v", p.apiKey, strings.ToUpper(p.raw), got)
			}

			m = &DatasetModel{Name: types.StringValue("tank/mydata")}
			p.put(m, types.StringNull())
			if _, ok := m.apiPayload()[p.apiKey]; ok {
				t.Errorf("null %s must be omitted from the payload", p.attr)
			}
		})
	}
}

// TestLocalPropertyDecodesGetInstance decodes property objects in the shape
// pool.dataset.get_instance returns them (see the probe quoted on
// propertyBytes), including a null rawvalue, which the API model allows.
func TestLocalPropertyDecodesGetInstance(t *testing.T) {
	var got struct {
		Local     localProperty `json:"local"`
		Inherited localProperty `json:"inherited"`
		Null      localProperty `json:"null"`
	}
	in := `{
		"local":     {"parsed": false, "rawvalue": "off", "value": "OFF", "source": "LOCAL", "source_info": null},
		"inherited": {"parsed": true,  "rawvalue": "on",  "value": "ON",  "source": "INHERITED", "source_info": "tank"},
		"null":      {"parsed": null,  "rawvalue": null,  "value": null,  "source": "LOCAL", "source_info": null}
	}`
	if err := json.Unmarshal([]byte(in), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := got.Local.local(); !ok || v != "off" {
		t.Errorf("local: got %q, %v", v, ok)
	}
	if _, ok := got.Inherited.local(); ok {
		t.Error("inherited must not count as local")
	}
	if _, ok := got.Null.local(); ok {
		t.Error("a null rawvalue must not count as local")
	}
}
