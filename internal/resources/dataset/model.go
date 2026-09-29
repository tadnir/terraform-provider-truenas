// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
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

	// Source-aware ZFS tuning properties (coverage audit). Each reads back null when the
	// property is inherited/default rather than set on this dataset, so an
	// inherited value is never carried into state and re-sent. Enum names match
	// the ZFS property; sync/dedup match truenas_zvol's existing attributes.
	// See zfsprops.go (localString/localInt).
	ACLMode               types.String `tfsdk:"aclmode"`
	ATime                 types.String `tfsdk:"atime"`
	Exec                  types.String `tfsdk:"exec"`
	ReadOnly              types.String `tfsdk:"readonly"`
	Sync                  types.String `tfsdk:"sync"`
	Checksum              types.String `tfsdk:"checksum"`
	Snapdir               types.String `tfsdk:"snapdir"`
	Dedup                 types.String `tfsdk:"dedup"` // API field "deduplication"
	RecordSize            types.String `tfsdk:"recordsize"`
	Copies                types.Int64  `tfsdk:"copies"`
	SpecialSmallBlockSize types.Int64  `tfsdk:"special_small_block_size"`
	RefReservation        types.Int64  `tfsdk:"refreservation"`

	// XAttr is read-only: ZFS extended-attribute storage mode is returned by
	// get_instance but is NOT in the writable create/update API (checked live
	// on 25.10 and 27.0), so it is exposed for reading only, not set.
	XAttr types.String `tfsdk:"xattr"`

	// Encryption (all create-only; changing any of these recreates the dataset).
	// encryption_passphrase / encryption_key are write-only: read from config,
	// never stored in state.
	Encryption            types.Bool   `tfsdk:"encryption"`
	InheritEncryption     types.Bool   `tfsdk:"inherit_encryption"`
	EncryptionAlgorithm   types.String `tfsdk:"encryption_algorithm"`
	EncryptionGenerateKey types.Bool   `tfsdk:"encryption_generate_key"`
	EncryptionPassphrase  types.String `tfsdk:"encryption_passphrase"`
	EncryptionKey         types.String `tfsdk:"encryption_key"`

	// Computed
	MountPoint types.String `tfsdk:"mountpoint"`
	Encrypted  types.Bool   `tfsdk:"encrypted"`
	KeyFormat  types.String `tfsdk:"key_format"`
	Locked     types.Bool   `tfsdk:"locked"`
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

	// Source-aware ZFS tuning properties (coverage audit). Only sent when set in config
	// or carried LOCAL in state; an inherited property reads back null (see
	// responseToModel), so it never reaches the payload and an apply cannot
	// convert an inherited property into a local one.
	putEnum(p, "aclmode", m.ACLMode)
	putEnum(p, "atime", m.ATime)
	putEnum(p, "exec", m.Exec)
	putEnum(p, "readonly", m.ReadOnly)
	putEnum(p, "sync", m.Sync)
	putEnum(p, "checksum", m.Checksum)
	putEnum(p, "snapdir", m.Snapdir)
	putEnum(p, "deduplication", m.Dedup)
	putStr(p, "recordsize", m.RecordSize)
	putInt(p, "copies", m.Copies)
	putInt(p, "special_small_block_size", m.SpecialSmallBlockSize)
	putInt(p, "refreservation", m.RefReservation)

	// Encryption (create-only). The write-only passphrase/key are injected from
	// req.Config by the resource's Create, not from the model (they are null in
	// state); see encryptionOptions.
	if !m.Encryption.IsNull() && !m.Encryption.IsUnknown() {
		p["encryption"] = m.Encryption.ValueBool()
	}
	if !m.InheritEncryption.IsNull() && !m.InheritEncryption.IsUnknown() {
		p["inherit_encryption"] = m.InheritEncryption.ValueBool()
	}
	if eo := m.encryptionOptions(); len(eo) > 0 {
		p["encryption_options"] = eo
	}
	return p
}

// encryptionOptions builds the non-secret encryption_options from the model.
// The write-only passphrase/key are added separately by Create from req.Config.
func (m *DatasetModel) encryptionOptions() map[string]any {
	eo := map[string]any{}
	if !m.EncryptionAlgorithm.IsNull() && !m.EncryptionAlgorithm.IsUnknown() && m.EncryptionAlgorithm.ValueString() != "" {
		eo["algorithm"] = m.EncryptionAlgorithm.ValueString()
	}
	if !m.EncryptionGenerateKey.IsNull() && !m.EncryptionGenerateKey.IsUnknown() {
		eo["generate_key"] = m.EncryptionGenerateKey.ValueBool()
	}
	return eo
}

// updateAPIPayload converts the model to the pool.dataset.update JSON
// payload. It starts from apiPayload (the create payload) and strips keys
// that pool.dataset.update rejects as create-only: "name" (the dataset's
// id is passed as the update method's first positional arg, not a payload
// key), "type" (changing a dataset's type after creation isn't
// supported; TrueNAS returns "[EINVAL] data.type: Extra inputs are not
// permitted" if it's included) and "share_type", which the 25.10 update
// model excludes the same way (PoolDatasetUpdate marks name, type,
// casesensitivity, share_type and the encryption options as Excluded).
// share_type is RequiresReplace, so a change to it never reaches Update;
// without this, any other in-place change to a dataset whose config sets
// share_type would fail.
func (m *DatasetModel) updateAPIPayload() map[string]any {
	p := m.apiPayload()
	delete(p, "name")
	delete(p, "type")
	delete(p, "share_type")
	// acltype forces replacement, so an update can only resend the value the
	// dataset already has. pool.dataset.update is not a no-op for that: an
	// acltype of POSIX or OFF also writes aclmode=discard and
	// aclinherit=discard as LOCAL properties, silently converting them from
	// inherited to local on every update.
	delete(p, "acltype")
	// Encryption is create-only; pool.dataset.update rejects these.
	delete(p, "encryption")
	delete(p, "inherit_encryption")
	delete(p, "encryption_options")
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
	Locked     bool   `json:"locked"`
	Pool       string `json:"pool"`

	EncryptionAlgorithm struct {
		Value *string `json:"value"`
	} `json:"encryption_algorithm"`
	KeyFormat struct {
		Value *string `json:"value"`
	} `json:"key_format"`

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

	// Source-aware ZFS tuning properties (coverage audit). Each carries "source" so an
	// inherited value can be told from a locally-set one; see zfsprops.go.
	ACLModeP    zfsSourced `json:"aclmode"`
	ATimeP      zfsSourced `json:"atime"`
	ExecP       zfsSourced `json:"exec"`
	ReadOnlyP   zfsSourced `json:"readonly"`
	SyncP       zfsSourced `json:"sync"`
	ChecksumP   zfsSourced `json:"checksum"`
	SnapdirP    zfsSourced `json:"snapdir"`
	DedupP      zfsSourced `json:"deduplication"`
	RecordSizeP zfsSourced `json:"recordsize"`
	CopiesP     zfsSourced `json:"copies"`
	SSBSP       zfsSourced `json:"special_small_block_size"`
	RefResP     zfsSourced `json:"refreservation"`
	XAttrP      zfsSourced `json:"xattr"` // read-only

	// Comments live under user_properties in TrueNAS 24+
	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// injectEncryptionSecrets adds the write-only encryption_passphrase /
// encryption_key from config into the create payload's encryption_options.
func injectEncryptionSecrets(payload map[string]any, cfg *DatasetModel) {
	eo, _ := payload["encryption_options"].(map[string]any)
	if eo == nil {
		eo = map[string]any{}
	}
	if !cfg.EncryptionPassphrase.IsNull() && !cfg.EncryptionPassphrase.IsUnknown() && cfg.EncryptionPassphrase.ValueString() != "" {
		eo["passphrase"] = cfg.EncryptionPassphrase.ValueString()
	}
	if !cfg.EncryptionKey.IsNull() && !cfg.EncryptionKey.IsUnknown() && cfg.EncryptionKey.ValueString() != "" {
		eo["key"] = cfg.EncryptionKey.ValueString()
	}
	if len(eo) > 0 {
		payload["encryption_options"] = eo
	}
}
