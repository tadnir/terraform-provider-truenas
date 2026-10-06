// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ZvolModel is the Terraform state/plan model for a TrueNAS zvol.
type ZvolModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	VolSize      types.Int64  `tfsdk:"volsize"`
	VolBlockSize types.Int64  `tfsdk:"volblocksize"`
	Compression  types.String `tfsdk:"compression"`
	Sync         types.String `tfsdk:"sync"`
	Dedup        types.String `tfsdk:"dedup"`
	Sparse       types.Bool   `tfsdk:"sparse"`
	Comments     types.String `tfsdk:"comments"`
	Pool         types.String `tfsdk:"pool"`
	Encrypted    types.Bool   `tfsdk:"encrypted"`

	// Source-aware ZFS tuning properties applicable to volumes (coverage audit). Each
	// reads back null when inherited/default rather than set LOCAL, so an
	// inherited value is never carried into state and re-sent. See zfsprops.go.
	Checksum       types.String `tfsdk:"checksum"`
	ReadOnly       types.String `tfsdk:"readonly"`
	Snapdev        types.String `tfsdk:"snapdev"`
	Copies         types.Int64  `tfsdk:"copies"`
	Reservation    types.Int64  `tfsdk:"reservation"`
	RefReservation types.Int64  `tfsdk:"refreservation"`
}

// zvolAPI matches the flat JSON structure returned by pool.dataset.get_instance for zvols.
type zvolAPI struct {
	Name      string `json:"name"`
	Pool      string `json:"pool"`
	Encrypted bool   `json:"encrypted"`

	Compression struct {
		Parsed string `json:"parsed"`
		Source string `json:"source"` // LOCAL, INHERITED, DEFAULT, RECEIVED
	} `json:"compression"`

	// sync/dedup are read source-aware from the "value" field (upper case, e.g.
	// ALWAYS/ON), not the lower-case "parsed" field, so an imported zvol
	// round-trips against an upper-case config. The API key for dedup is
	// "deduplication", not "dedup".
	SyncP  zfsSourced `json:"sync"`
	DedupP zfsSourced `json:"deduplication"`

	VolSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volsize"`

	VolBlockSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volblocksize"`

	// Source-aware ZFS tuning properties (coverage audit); see zfsprops.go.
	ChecksumP  zfsSourced `json:"checksum"`
	ReadOnlyP  zfsSourced `json:"readonly"`
	SnapdevP   zfsSourced `json:"snapdev"`
	CopiesP    zfsSourced `json:"copies"`
	ReservP    zfsSourced `json:"reservation"`
	RefReservP zfsSourced `json:"refreservation"`

	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// apiPayload converts the model to the create/update JSON payload.
// volblocksizeStr converts a byte count to the string enum pool.dataset.create
// accepts for volblocksize. Returns "" for a value that is not a valid size.
func volblocksizeStr(b int64) string {
	switch b {
	case 512:
		return "512"
	case 1024:
		return "1K"
	case 2048:
		return "2K"
	case 4096:
		return "4K"
	case 8192:
		return "8K"
	case 16384:
		return "16K"
	case 32768:
		return "32K"
	case 65536:
		return "64K"
	case 131072:
		return "128K"
	}
	return ""
}

func (m *ZvolModel) apiPayload() map[string]any {
	p := map[string]any{
		"name":    m.Name.ValueString(),
		"type":    "VOLUME",
		"volsize": m.VolSize.ValueInt64(),
	}
	if !m.VolBlockSize.IsNull() && !m.VolBlockSize.IsUnknown() && m.VolBlockSize.ValueInt64() != 0 {
		// pool.dataset.create wants volblocksize as a string enum ("512", "1K",
		// … "128K"), not the raw byte count the schema models it as.
		if s := volblocksizeStr(m.VolBlockSize.ValueInt64()); s != "" {
			p["volblocksize"] = s
		}
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
	}
	if !m.Sync.IsNull() && !m.Sync.IsUnknown() {
		p["sync"] = strings.ToUpper(m.Sync.ValueString())
	}
	if !m.Dedup.IsNull() && !m.Dedup.IsUnknown() {
		p["deduplication"] = strings.ToUpper(m.Dedup.ValueString())
	}
	if !m.Sparse.IsNull() && !m.Sparse.IsUnknown() {
		p["sparse"] = m.Sparse.ValueBool()
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}

	// Source-aware ZFS tuning properties (coverage audit). Only sent when set; an
	// inherited property reads back null so it never reaches the payload.
	putEnum(p, "checksum", m.Checksum)
	putEnum(p, "readonly", m.ReadOnly)
	putEnum(p, "snapdev", m.Snapdev)
	putInt(p, "copies", m.Copies)
	putInt(p, "reservation", m.Reservation)
	putInt(p, "refreservation", m.RefReservation)
	return p
}

// responseToModel populates m from the API response. Sparse is write-only
// (not returned by the API) so the plan/state value is preserved as-is.
func responseToModel(api *zvolAPI, m *ZvolModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Pool = types.StringValue(api.Pool)
	m.Encrypted = types.BoolValue(api.Encrypted)
	m.VolSize = types.Int64Value(api.VolSize.Parsed)
	m.VolBlockSize = types.Int64Value(api.VolBlockSize.Parsed)
	// Compression is source-aware: when it is not set LOCAL on this zvol the
	// value is inherited (or the ZFS default), so report "INHERIT" rather than
	// the resolved value (e.g. "lz4"). This lets compression = "inherit" round-
	// trip instead of failing with an inconsistent result (planned "inherit",
	// applied "lz4"). preserveCase keeps the user's casing of either the real
	// algorithm (local) or the inherit sentinel. (#38)
	if api.Compression.Source == "LOCAL" {
		m.Compression = preserveCase(m.Compression, api.Compression.Parsed)
	} else {
		m.Compression = preserveCase(m.Compression, "INHERIT")
	}
	m.Sync = localString(api.SyncP)
	m.Dedup = localString(api.DedupP)
	m.Comments = types.StringValue(api.UserProperties.Comments.Value)
	// Sparse is write-only (not in API response); preserve plan/state value.

	// Source-aware ZFS tuning properties (coverage audit): recorded only when set LOCAL.
	m.Checksum = localString(api.ChecksumP)
	m.ReadOnly = localString(api.ReadOnlyP)
	m.Snapdev = localString(api.SnapdevP)
	m.Copies = localInt(api.CopiesP)
	m.Reservation = localInt(api.ReservP)
	m.RefReservation = localInt(api.RefReservP)
}

// preserveCase returns current if it matches apiVal case-insensitively
// (preserving the user's chosen casing), or the lower-cased apiVal otherwise.
// Only compression uses it: the API reports compression in lower case ("lz4"),
// which matches the conventional config form.
func preserveCase(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if strings.EqualFold(current.ValueString(), apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}
