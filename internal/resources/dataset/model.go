// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DatasetModel maps to the TrueNAS pool.dataset API fields.
type DatasetModel struct {
	ID types.String `tfsdk:"id"`

	// Required
	Name types.String `tfsdk:"name"`

	// Optional with TrueNAS defaults
	Type        types.String `tfsdk:"type"`
	Compression types.String `tfsdk:"compression"`
	AClType     types.String `tfsdk:"acltype"`
	ShareType   types.String `tfsdk:"share_type"`
	Comments    types.String `tfsdk:"comments"`
	Quota       types.Int64  `tfsdk:"quota"`
	RefQuota    types.Int64  `tfsdk:"refquota"`
	Reservation types.Int64  `tfsdk:"reservation"`
	VolSize     types.Int64  `tfsdk:"volsize"`

	// SpecialSmallBlockSize is null when the property is not set LOCAL on
	// this dataset, i.e. when it is inherited from a parent or left at the
	// ZFS default. See responseToModel.
	SpecialSmallBlockSize types.Int64 `tfsdk:"special_small_block_size"`

	// The ZFS properties below follow the same rule as
	// SpecialSmallBlockSize: each is null in state unless it is set LOCAL
	// on this dataset, so an inherited value is never written back. See
	// localString.
	ATime types.String `tfsdk:"atime"`
	Dedup types.String `tfsdk:"dedup"`

	// Computed
	MountPoint types.String `tfsdk:"mountpoint"`
	Encrypted  types.Bool   `tfsdk:"encrypted"`
	Pool       types.String `tfsdk:"pool"`
}

// apiPayload converts the model to the create/update JSON payload.
func (m *DatasetModel) apiPayload() map[string]any {
	p := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p["type"] = strings.ToUpper(m.Type.ValueString())
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
	}
	if !m.AClType.IsNull() && !m.AClType.IsUnknown() {
		p["acltype"] = strings.ToUpper(m.AClType.ValueString())
	}
	if !m.ShareType.IsNull() && !m.ShareType.IsUnknown() {
		p["share_type"] = strings.ToUpper(m.ShareType.ValueString())
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}
	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		p["quota"] = m.Quota.ValueInt64()
	}
	if !m.RefQuota.IsNull() && !m.RefQuota.IsUnknown() {
		p["refquota"] = m.RefQuota.ValueInt64()
	}
	if !m.Reservation.IsNull() && !m.Reservation.IsUnknown() {
		p["reservation"] = m.Reservation.ValueInt64()
	}
	// volsize only applies to VOLUME datasets; pool.dataset.update rejects
	// "volsize" outright for FILESYSTEM datasets (TrueNAS API error code
	// 22: 'volsize'). VolSize is Computed in the schema and reads back as 0
	// for FILESYSTEM datasets, so a plain null/unknown guard isn't enough -
	// state carries a known-but-zero value into every later plan. Only
	// include it when it's a real, known, non-zero size.
	if !m.VolSize.IsNull() && !m.VolSize.IsUnknown() && m.VolSize.ValueInt64() != 0 {
		p["volsize"] = m.VolSize.ValueInt64()
	}
	// special_small_block_size is deliberately NOT guarded on != 0 the way
	// volsize is: 0 is a meaningful value here (it disables writing small
	// blocks to the special vdev), not a stand-in for "unset". The
	// equivalent protection is in responseToModel, which leaves this null
	// unless pool.dataset.get_instance reports the property's source as
	// LOCAL. An inherited or default value therefore never reaches this
	// payload, so an apply cannot silently convert an inherited property
	// into a local one.
	if !m.SpecialSmallBlockSize.IsNull() && !m.SpecialSmallBlockSize.IsUnknown() {
		p["special_small_block_size"] = m.SpecialSmallBlockSize.ValueInt64()
	}
	putUpper(p, "atime", m.ATime)
	putUpper(p, "deduplication", m.Dedup)
	return p
}

// putUpper sets key to the upper-cased value of v when v is known and not
// null. pool.dataset.create/update take these enum properties upper-case;
// the attributes accept any case, as compression and acltype do.
func putUpper(p map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = strings.ToUpper(v.ValueString())
	}
}

// updateAPIPayload converts the model to the pool.dataset.update JSON
// payload. It starts from apiPayload (the create payload) and strips keys
// that pool.dataset.update rejects as create-only: "name" (the dataset's
// id is passed as the update method's first positional arg, not a payload
// key) and "type" (changing a dataset's type after creation isn't
// supported; TrueNAS returns "[EINVAL] data.type: Extra inputs are not
// permitted" if it's included).
func (m *DatasetModel) updateAPIPayload() map[string]any {
	p := m.apiPayload()
	delete(p, "name")
	delete(p, "type")
	return p
}

// apiResponse matches the flat JSON structure returned by pool.dataset.get_instance.
// Fields are at the root level (no "properties" wrapper). Quota fields use *int64
// because TrueNAS returns JSON null when no limit is set.
type apiResponse struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MountPoint string `json:"mountpoint"`
	Encrypted  bool   `json:"encrypted"`
	Pool       string `json:"pool"`

	Compression struct {
		Parsed string `json:"parsed"` // lowercase: "lz4"
	} `json:"compression"`

	AClType struct {
		Parsed string `json:"parsed"` // lowercase: "posix", "nfsv4", "off"
	} `json:"acltype"`

	Quota struct {
		Parsed *int64 `json:"parsed"` // null when unlimited
	} `json:"quota"`

	RefQuota struct {
		Parsed *int64 `json:"parsed"`
	} `json:"refquota"`

	Reservation struct {
		Parsed *int64 `json:"parsed"`
	} `json:"reservation"`

	VolSize struct {
		Parsed int64 `json:"parsed"` // 0 for FILESYSTEM datasets
	} `json:"volsize"`

	// SpecialSmallBlockSize carries "source" as well as the value, because
	// the value alone cannot distinguish "set to 0 on this dataset" from
	// "inherited". source is LOCAL, INHERITED, DEFAULT or RECEIVED. The
	// whole sub-object is absent for dataset types that do not carry the
	// property, which propertyBytes reports as unset.
	SpecialSmallBlockSize struct {
		Parsed propertyBytes `json:"parsed"`
		Source string        `json:"source"`
	} `json:"special_small_block_size"`

	ATime localProperty `json:"atime"`
	Dedup localProperty `json:"deduplication"`

	// Comments live under user_properties in TrueNAS 24+
	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// localProperty is the part of a pool.dataset.get_instance property object
// that the source-aware attributes need. rawvalue is used rather than parsed
// because parsed is not consistently typed across properties (the API model
// documents it as "string, boolean, integer, etc.", and see propertyBytes),
// while rawvalue is always the property's ZFS string, e.g. "off",
// "standard", "131072".
type localProperty struct {
	RawValue *string `json:"rawvalue"`
	Source   string  `json:"source"`
}

// local returns the raw ZFS value when the property is set on this dataset
// itself, and false when it is inherited, left at its default, received, or
// absent from the response.
func (p localProperty) local() (string, bool) {
	if p.Source != "LOCAL" || p.RawValue == nil {
		return "", false
	}
	return *p.RawValue, true
}

// localString reads an enum-valued property into state: null unless it is
// set LOCAL, and otherwise in the configured casing when that matches (see
// preserveCase). For every property this is used for, ZFS's raw value is
// the lower-case form of the API's enum ("off" for "OFF").
func localString(current types.String, p localProperty) types.String {
	v, ok := p.local()
	if !ok {
		return types.StringNull()
	}
	return preserveCase(current, v)
}

// propertyBytes decodes the "parsed" field of a byte-valued dataset property
// from pool.dataset.get_instance. It exists because that field is not
// consistently typed: within a single response, some byte-valued properties
// parse as a JSON number and others as a JSON string. Probed live against
// TrueNAS SCALE 25.10 (pool.dataset.get_instance on a filesystem dataset,
// 2026-09-22):
//
//	"special_small_block_size": {"parsed": "0",     "rawvalue": "0",
//	                             "source": "INHERITED", "source_info": "Tank",
//	                             "value": "0"}
//	"recordsize":               {"parsed": 1048576, "rawvalue": "1048576",
//	                             "source": "LOCAL",     "source_info": null,
//	                             "value": "1M"}
//
// Decoding special_small_block_size straight into an int64 is what the first
// acceptance run against a live box failed on, so this accepts either form.
//
// A string carrying a ZFS size suffix ("16K") is accepted as well. The probed
// sample is zero, where a plain decimal string and a suffixed one are
// indistinguishable, and guessing wrong fails the read outright rather than
// degrading it - the same reason "value" is not used here, since that field
// is the human-readable form ("1M") rather than a byte count.
type propertyBytes struct {
	Set   bool
	Value int64
}

func (p *propertyBytes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			return nil
		}
		v, err := parseZFSSize(s)
		if err != nil {
			return err
		}
		p.Set, p.Value = true, v
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	p.Set, p.Value = true, v
	return nil
}

// parseZFSSize parses a byte count written either as a plain decimal or with
// a binary ZFS size suffix, e.g. "16384" or "16K". Suffixes are binary
// multiples, as zfs(8) reports them.
func parseZFSSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size value")
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		mult = 1 << 10
	case 'M', 'm':
		mult = 1 << 20
	case 'G', 'g':
		mult = 1 << 30
	case 'T', 't':
		mult = 1 << 40
	case 'P', 'p':
		mult = 1 << 50
	}
	if mult != 1 {
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing size %q: %w", s, err)
	}
	return n * mult, nil
}
