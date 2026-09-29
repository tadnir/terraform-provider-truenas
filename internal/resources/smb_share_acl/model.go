// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// SMBShareACLModel is the Terraform state/plan model for a
// truenas_smb_share_acl resource: a share_name-keyed declarative wrapper
// around the imperative sharing.smb.setacl/getacl methods (both probed live
// as job:false plain calls on TrueNAS 27.0). It mirrors truenas_filesystem_
// acl's "write-what-you-said" philosophy - the plan's exact entries are kept
// in state verbatim; a semantic drift comparison on Read only overwrites them
// when the underlying share ACL genuinely changed - but uses a typed nested
// list rather than an opaque JSON string, since the SMB share ACL entry is a
// small, fixed shape (5 fields, one principal selector) unlike filesystem_acl's
// NFS4/POSIX1E union.
type SMBShareACLModel struct {
	ID        types.String `tfsdk:"id"`         // == share_name
	ShareName types.String `tfsdk:"share_name"` // keyed by share name, not a numeric id
	ShareACL  types.List   `tfsdk:"share_acl"`  // list of smbAclEntryModel
}

// SMBShareACLDataSourceModel is the read-only lookup model for the
// truenas_smb_share_acl datasource (looked up by share_name).
type SMBShareACLDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ShareName types.String `tfsdk:"share_name"`
	ShareACL  types.List   `tfsdk:"share_acl"`
}

// smbAclEntryModel is one Terraform-side share ACL entry. The three ae_who_*
// fields are alternative principal selectors (SID, Unix ID, or name); a given
// entry sets exactly one on the way in. They are Optional (NOT Computed): the
// server resolves the others on write (e.g. ae_who_sid "S-1-1-0" reads back
// with ae_who_str "everyone@"), but storing those server-added values would
// make Terraform show a diff against a config that never set them, so this
// resource keeps only what the user wrote (see entriesDrifted).
type smbAclEntryModel struct {
	AePerm   types.String `tfsdk:"ae_perm"`
	AeType   types.String `tfsdk:"ae_type"`
	AeWhoSID types.String `tfsdk:"ae_who_sid"`
	AeWhoID  types.Object `tfsdk:"ae_who_id"`
	AeWhoStr types.String `tfsdk:"ae_who_str"`
}

// smbAclWhoIDAttrTypes is the attribute-type map for the ae_who_id nested
// object ({id_type: "USER"|"GROUP", id: <uid/gid>}).
var smbAclWhoIDAttrTypes = map[string]attr.Type{
	"id_type": types.StringType,
	"id":      types.Int64Type,
}

// smbAclEntryAttrTypes is the attribute-type map for one share_acl list element.
var smbAclEntryAttrTypes = map[string]attr.Type{
	"ae_perm":    types.StringType,
	"ae_type":    types.StringType,
	"ae_who_sid": types.StringType,
	"ae_who_id":  types.ObjectType{AttrTypes: smbAclWhoIDAttrTypes},
	"ae_who_str": types.StringType,
}

// smbAclEntryObjectType is the element type of the share_acl list.
func smbAclEntryObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: smbAclEntryAttrTypes}
}

// --- wire (API) shapes for sharing.smb.getacl / setacl ---

// smbAclWhoIDAPI is the ae_who_id object on the wire.
type smbAclWhoIDAPI struct {
	IDType string `json:"id_type"`
	ID     int64  `json:"id"`
}

// smbAclEntryAPI is one entry on the wire. The ae_who_* fields are pointers
// with omitempty so absent selectors are omitted from setacl payloads (the
// API rejects an entry with no principal) and distinguished from empty on read.
type smbAclEntryAPI struct {
	AePerm   string          `json:"ae_perm"`
	AeType   string          `json:"ae_type"`
	AeWhoSID *string         `json:"ae_who_sid,omitempty"`
	AeWhoID  *smbAclWhoIDAPI `json:"ae_who_id,omitempty"`
	AeWhoStr *string         `json:"ae_who_str,omitempty"`
}

// smbGetAclAPI is the full sharing.smb.getacl / setacl response shape
// (setacl echoes the resolved ACL, so Create/Update parse its return directly
// instead of a follow-up getacl round trip).
type smbGetAclAPI struct {
	ShareName string           `json:"share_name"`
	ShareACL  []smbAclEntryAPI `json:"share_acl"`
}

// entriesFromList converts the model's share_acl list into a Go slice.
func entriesFromList(ctx context.Context, l types.List) ([]smbAclEntryModel, diag.Diagnostics) {
	var entries []smbAclEntryModel
	if l.IsNull() || l.IsUnknown() {
		return entries, nil
	}
	diags := l.ElementsAs(ctx, &entries, false)
	return entries, diags
}

// setaclPayload builds the sharing.smb.setacl argument from the model's
// entries. Only the principal selector(s) actually set on each entry are sent.
func setaclPayload(ctx context.Context, m *SMBShareACLModel) (map[string]any, diag.Diagnostics) {
	entries, diags := entriesFromList(ctx, m.ShareACL)
	if diags.HasError() {
		return nil, diags
	}

	acl := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{
			"ae_perm": e.AePerm.ValueString(),
			"ae_type": e.AeType.ValueString(),
		}
		if !e.AeWhoSID.IsNull() && !e.AeWhoSID.IsUnknown() {
			entry["ae_who_sid"] = e.AeWhoSID.ValueString()
		}
		if !e.AeWhoStr.IsNull() && !e.AeWhoStr.IsUnknown() {
			entry["ae_who_str"] = e.AeWhoStr.ValueString()
		}
		if !e.AeWhoID.IsNull() && !e.AeWhoID.IsUnknown() {
			var who smbAclWhoIDModel
			d := e.AeWhoID.As(ctx, &who, basetypes.ObjectAsOptions{})
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			entry["ae_who_id"] = map[string]any{
				"id_type": who.IDType.ValueString(),
				"id":      who.ID.ValueInt64(),
			}
		}
		acl = append(acl, entry)
	}

	return map[string]any{
		"share_name": m.ShareName.ValueString(),
		"share_acl":  acl,
	}, diags
}

// smbAclWhoIDModel decodes the ae_who_id object for payload building.
type smbAclWhoIDModel struct {
	IDType types.String `tfsdk:"id_type"`
	ID     types.Int64  `tfsdk:"id"`
}

// defaultShareACLPayload builds the setacl argument used by Delete: it resets
// the share to the server default of a single "everyone@ FULL ALLOWED" entry
// (probed live as sharing.smb.setacl's documented default share_acl), the SMB
// equivalent of filesystem_acl's strip-to-trivial Delete.
func defaultShareACLPayload(shareName string) map[string]any {
	return map[string]any{
		"share_name": shareName,
		"share_acl": []map[string]any{
			{"ae_perm": "FULL", "ae_type": "ALLOWED", "ae_who_sid": "S-1-1-0"},
		},
	}
}

// entriesDrifted reports whether the share ACL returned by the API differs, in
// a way the user would care about, from what is currently stored in state.
// It compares entry count, and per entry the ae_perm, ae_type, and the single
// principal selector the state entry actually specified (SID, Unix ID, or
// name) - deliberately ignoring the other server-resolved selectors so that a
// SID-specified entry does not drift merely because the server echoed a
// resolved ae_who_str.
func entriesDrifted(state []smbAclEntryModel, api []smbAclEntryAPI) bool {
	if len(state) != len(api) {
		return true
	}
	for i, s := range state {
		a := api[i]
		if s.AePerm.ValueString() != a.AePerm || s.AeType.ValueString() != a.AeType {
			return true
		}
		if !principalMatches(s, a) {
			return true
		}
	}
	return false
}

// principalMatches checks the API entry against the one principal selector the
// state entry specified. An entry with no selector set (shouldn't happen for a
// valid config) is treated as matching so it doesn't force perpetual drift.
func principalMatches(s smbAclEntryModel, a smbAclEntryAPI) bool {
	switch {
	case !s.AeWhoSID.IsNull() && !s.AeWhoSID.IsUnknown():
		return a.AeWhoSID != nil && *a.AeWhoSID == s.AeWhoSID.ValueString()
	case !s.AeWhoStr.IsNull() && !s.AeWhoStr.IsUnknown():
		return a.AeWhoStr != nil && *a.AeWhoStr == s.AeWhoStr.ValueString()
	case !s.AeWhoID.IsNull() && !s.AeWhoID.IsUnknown():
		if a.AeWhoID == nil {
			return false
		}
		attrs := s.AeWhoID.Attributes()
		idType, _ := attrs["id_type"].(types.String)
		id, _ := attrs["id"].(types.Int64)
		return idType.ValueString() == a.AeWhoID.IDType && id.ValueInt64() == a.AeWhoID.ID
	default:
		return true
	}
}

// apiEntriesToList converts API entries into a typed share_acl list, choosing a
// single principal selector per entry (SID, then Unix ID, then name) so the
// resulting state matches the one-selector shape a user config would use. Used
// on import, on the datasource, and when Read detects genuine drift.
func apiEntriesToList(ctx context.Context, api []smbAclEntryAPI) (types.List, diag.Diagnostics) {
	entries := make([]smbAclEntryModel, 0, len(api))
	for _, a := range api {
		e := smbAclEntryModel{
			AePerm:   types.StringValue(a.AePerm),
			AeType:   types.StringValue(a.AeType),
			AeWhoSID: types.StringNull(),
			AeWhoID:  types.ObjectNull(smbAclWhoIDAttrTypes),
			AeWhoStr: types.StringNull(),
		}
		switch {
		case a.AeWhoSID != nil:
			e.AeWhoSID = types.StringValue(*a.AeWhoSID)
		case a.AeWhoID != nil:
			obj, d := types.ObjectValue(smbAclWhoIDAttrTypes, map[string]attr.Value{
				"id_type": types.StringValue(a.AeWhoID.IDType),
				"id":      types.Int64Value(a.AeWhoID.ID),
			})
			if d.HasError() {
				return types.ListNull(smbAclEntryObjectType()), d
			}
			e.AeWhoID = obj
		case a.AeWhoStr != nil:
			e.AeWhoStr = types.StringValue(*a.AeWhoStr)
		}
		entries = append(entries, e)
	}
	return types.ListValueFrom(ctx, smbAclEntryObjectType(), entries)
}

// idFor returns the resource id / identity for a share name.
func idFor(shareName string) string { return shareName }

// countSelectors counts how many principal selectors an entry sets. An unknown
// (interpolated) value counts as set so cross-resource references validate.
func countSelectors(e smbAclEntryModel) int {
	n := 0
	if !e.AeWhoSID.IsNull() {
		n++
	}
	if !e.AeWhoStr.IsNull() {
		n++
	}
	if !e.AeWhoID.IsNull() {
		n++
	}
	return n
}

// diagText flattens diagnostics into a single string for wrapping in an error.
func diagText(diags diag.Diagnostics) string {
	if !diags.HasError() {
		return ""
	}
	msg := ""
	for _, d := range diags.Errors() {
		if msg != "" {
			msg += "; "
		}
		msg += d.Summary() + ": " + d.Detail()
	}
	return msg
}

// String is a tiny helper for readable test/debug output.
func (m *SMBShareACLModel) String() string {
	return fmt.Sprintf("smb_share_acl(share_name=%s)", m.ShareName.ValueString())
}
