// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strptr(s string) *string { return &s }

// mkEntry builds one share_acl list-element object. Pass a nil selector to
// leave it unset; whoID as a nil pointer leaves ae_who_id null.
func mkEntry(perm, atype string, sid *string, whoID *smbAclWhoIDAPI, str *string) types.Object {
	sidVal := types.StringNull()
	if sid != nil {
		sidVal = types.StringValue(*sid)
	}
	strVal := types.StringNull()
	if str != nil {
		strVal = types.StringValue(*str)
	}
	idVal := types.ObjectNull(smbAclWhoIDAttrTypes)
	if whoID != nil {
		idVal = types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
			"id_type": types.StringValue(whoID.IDType),
			"id":      types.Int64Value(whoID.ID),
		})
	}
	return types.ObjectValueMust(smbAclEntryAttrTypes, map[string]attr.Value{
		"ae_perm":    types.StringValue(perm),
		"ae_type":    types.StringValue(atype),
		"ae_who_sid": sidVal,
		"ae_who_id":  idVal,
		"ae_who_str": strVal,
	})
}

func mkModel(shareName string, entries ...types.Object) *SMBShareACLModel {
	elems := make([]attr.Value, len(entries))
	for i, e := range entries {
		elems[i] = e
	}
	return &SMBShareACLModel{
		ShareName: types.StringValue(shareName),
		ShareACL:  types.ListValueMust(smbAclEntryObjectType(), elems),
	}
}

func TestSetaclPayload_SIDOnly(t *testing.T) {
	m := mkModel("share1", mkEntry("FULL", "ALLOWED", strptr("S-1-1-0"), nil, nil))
	payload, diags := setaclPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	if payload["share_name"] != "share1" {
		t.Errorf("share_name = %v, want share1", payload["share_name"])
	}
	acl := payload["share_acl"].([]map[string]any)
	if len(acl) != 1 {
		t.Fatalf("len(acl) = %d, want 1", len(acl))
	}
	e := acl[0]
	if e["ae_perm"] != "FULL" || e["ae_type"] != "ALLOWED" || e["ae_who_sid"] != "S-1-1-0" {
		t.Errorf("entry = %+v", e)
	}
	if _, ok := e["ae_who_id"]; ok {
		t.Errorf("ae_who_id should be omitted, got %+v", e)
	}
	if _, ok := e["ae_who_str"]; ok {
		t.Errorf("ae_who_str should be omitted, got %+v", e)
	}
}

func TestSetaclPayload_WhoID(t *testing.T) {
	m := mkModel("share1", mkEntry("CHANGE", "ALLOWED", nil, &smbAclWhoIDAPI{IDType: "GROUP", ID: 1000}, nil))
	payload, diags := setaclPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	acl := payload["share_acl"].([]map[string]any)
	who, ok := acl[0]["ae_who_id"].(map[string]any)
	if !ok {
		t.Fatalf("ae_who_id missing/wrong type: %+v", acl[0])
	}
	if who["id_type"] != "GROUP" || who["id"] != int64(1000) {
		t.Errorf("ae_who_id = %+v", who)
	}
	if _, ok := acl[0]["ae_who_sid"]; ok {
		t.Errorf("ae_who_sid should be omitted")
	}
}

// The headline write-what-you-said case: state specified a SID; the server
// resolved and echoed ae_who_str "everyone@". That must NOT count as drift.
func TestEntriesDrifted_ResolvedStrIsNotDrift(t *testing.T) {
	state := []smbAclEntryModel{{
		AePerm:   types.StringValue("FULL"),
		AeType:   types.StringValue("ALLOWED"),
		AeWhoSID: types.StringValue("S-1-1-0"),
		AeWhoID:  types.ObjectNull(smbAclWhoIDAttrTypes),
		AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{{
		AePerm:   "FULL",
		AeType:   "ALLOWED",
		AeWhoSID: strptr("S-1-1-0"),
		AeWhoStr: strptr("everyone@"), // server-resolved extra
	}}
	if entriesDrifted(state, api) {
		t.Error("server-resolved ae_who_str should not be drift")
	}
}

func TestEntriesDrifted_PermChange(t *testing.T) {
	state := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: types.StringValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes), AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{{AePerm: "CHANGE", AeType: "ALLOWED", AeWhoSID: strptr("S-1-1-0")}}
	if !entriesDrifted(state, api) {
		t.Error("perm change should be drift")
	}
}

func TestEntriesDrifted_PrincipalChange(t *testing.T) {
	state := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: types.StringValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes), AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-32-544")}}
	if !entriesDrifted(state, api) {
		t.Error("SID change should be drift")
	}
}

func TestEntriesDrifted_CountChange(t *testing.T) {
	state := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: types.StringValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes), AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{
		{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-1-0")},
		{AePerm: "READ", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-32-546")},
	}
	if !entriesDrifted(state, api) {
		t.Error("entry count change should be drift")
	}
}

func TestEntriesDrifted_WhoIDMatch(t *testing.T) {
	state := []smbAclEntryModel{{
		AePerm: types.StringValue("CHANGE"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: types.StringNull(),
		AeWhoID: types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
			"id_type": types.StringValue("GROUP"), "id": types.Int64Value(1000),
		}),
		AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{{
		AePerm: "CHANGE", AeType: "ALLOWED",
		AeWhoID:  &smbAclWhoIDAPI{IDType: "GROUP", ID: 1000},
		AeWhoStr: strptr("builtin_users"), // resolved extra, ignored
	}}
	if entriesDrifted(state, api) {
		t.Error("matching who_id with resolved str should not be drift")
	}
}

func TestApiEntriesToList_PrefersSID(t *testing.T) {
	api := []smbAclEntryAPI{{
		AePerm: "FULL", AeType: "ALLOWED",
		AeWhoSID: strptr("S-1-1-0"), AeWhoStr: strptr("everyone@"),
	}}
	list, diags := apiEntriesToList(context.Background(), api)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	var got []smbAclEntryModel
	if d := list.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatalf("ElementsAs: %s", diagText(d))
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].AeWhoSID.ValueString() != "S-1-1-0" {
		t.Errorf("ae_who_sid = %q, want S-1-1-0", got[0].AeWhoSID.ValueString())
	}
	if !got[0].AeWhoStr.IsNull() {
		t.Errorf("ae_who_str should be null when SID present, got %q", got[0].AeWhoStr.ValueString())
	}
}

func TestApiEntriesToList_WhoIDOnly(t *testing.T) {
	api := []smbAclEntryAPI{{
		AePerm: "READ", AeType: "DENIED",
		AeWhoID: &smbAclWhoIDAPI{IDType: "USER", ID: 3001},
	}}
	list, diags := apiEntriesToList(context.Background(), api)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	var got []smbAclEntryModel
	if d := list.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatalf("ElementsAs: %s", diagText(d))
	}
	if got[0].AeWhoID.IsNull() {
		t.Fatal("ae_who_id should be set")
	}
	attrs := got[0].AeWhoID.Attributes()
	if attrs["id_type"].(types.String).ValueString() != "USER" || attrs["id"].(types.Int64).ValueInt64() != 3001 {
		t.Errorf("ae_who_id = %+v", attrs)
	}
}

func TestCountSelectors(t *testing.T) {
	none := smbAclEntryModel{AeWhoSID: types.StringNull(), AeWhoStr: types.StringNull(), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes)}
	if got := countSelectors(none); got != 0 {
		t.Errorf("no selector: got %d, want 0", got)
	}
	sid := none
	sid.AeWhoSID = types.StringValue("S-1-1-0")
	if got := countSelectors(sid); got != 1 {
		t.Errorf("sid only: got %d, want 1", got)
	}
	two := sid
	two.AeWhoID = types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
		"id_type": types.StringValue("GROUP"), "id": types.Int64Value(545),
	})
	if got := countSelectors(two); got != 2 {
		t.Errorf("two selectors: got %d, want 2", got)
	}
	// unknown counts as set (cross-resource reference)
	unk := none
	unk.AeWhoStr = types.StringUnknown()
	if got := countSelectors(unk); got != 1 {
		t.Errorf("unknown selector: got %d, want 1", got)
	}
}

func TestDefaultShareACLPayload(t *testing.T) {
	p := defaultShareACLPayload("share1")
	if p["share_name"] != "share1" {
		t.Errorf("share_name = %v", p["share_name"])
	}
	acl := p["share_acl"].([]map[string]any)
	if len(acl) != 1 || acl[0]["ae_perm"] != "FULL" || acl[0]["ae_type"] != "ALLOWED" || acl[0]["ae_who_sid"] != "S-1-1-0" {
		t.Errorf("default acl = %+v", acl)
	}
}
