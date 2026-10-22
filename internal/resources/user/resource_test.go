// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUserSchema verifies key schema attributes.
func TestUserSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// uid must be Int64Attribute, Optional, Computed, with RequiresReplace.
	uidAttr, ok := s.Attributes["uid"]
	if !ok {
		t.Fatal("schema missing 'uid' attribute")
	}
	uidInt64, ok := uidAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'uid' is %T, want schema.Int64Attribute", uidAttr)
	}
	if !uidInt64.IsOptional() {
		t.Error("'uid' should be Optional")
	}
	if !uidInt64.IsComputed() {
		t.Error("'uid' should be Computed")
	}
	if len(uidInt64.PlanModifiers) == 0 {
		t.Error("'uid' should have plan modifiers (RequiresReplace)")
	}

	// username must be Required with RequiresReplace.
	usernameAttr, ok := s.Attributes["username"]
	if !ok {
		t.Fatal("schema missing 'username' attribute")
	}
	usernameStr, ok := usernameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'username' is %T, want schema.StringAttribute", usernameAttr)
	}
	if !usernameStr.IsRequired() {
		t.Error("'username' should be Required")
	}
	if len(usernameStr.PlanModifiers) == 0 {
		t.Error("'username' should have plan modifiers (RequiresReplace)")
	}

	// password must be Optional, Sensitive, NOT Computed.
	passwordAttr, ok := s.Attributes["password"]
	if !ok {
		t.Fatal("schema missing 'password' attribute")
	}
	passwordStr, ok := passwordAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' is %T, want schema.StringAttribute", passwordAttr)
	}
	if !passwordStr.IsOptional() {
		t.Error("'password' should be Optional")
	}
	if passwordStr.IsComputed() {
		t.Error("'password' must NOT be Computed (write-only)")
	}
	if !passwordStr.IsSensitive() {
		t.Error("'password' should be Sensitive")
	}
	if !passwordStr.IsWriteOnly() {
		t.Error("'password' should be WriteOnly")
	}

	// builtin, immutable, local must be Computed-only.
	for _, field := range []string{"builtin", "immutable", "local"} {
		a, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		b, ok := a.(schema.BoolAttribute)
		if !ok {
			t.Errorf("%q is %T, want schema.BoolAttribute", field, a)
			continue
		}
		if !b.IsComputed() {
			t.Errorf("%q should be Computed", field)
		}
		if b.IsOptional() || b.IsRequired() {
			t.Errorf("%q should be Computed-only (not Optional/Required)", field)
		}
	}

	// List attributes.
	for _, listField := range []string{"sudo_commands", "sudo_commands_nopasswd", "groups"} {
		attr, ok := s.Attributes[listField]
		if !ok {
			t.Fatalf("schema missing %q attribute", listField)
		}
		if _, ok := attr.(schema.ListAttribute); !ok {
			t.Errorf("%q is %T, want schema.ListAttribute", listField, attr)
		}
	}
}

// TestUserCreatePayload verifies that createPayload includes uid and username.
func TestUserCreatePayload(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		UID:                  types.Int64Value(1234),
		Username:             types.StringValue("testuser"),
		FullName:             types.StringValue("Test User"),
		Email:                types.StringValue("test@example.com"),
		Home:                 types.StringValue("/home/testuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(true),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Password:             types.StringValue("secret123"),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	// uid and username must be present in create payload.
	if v, ok := payload["uid"]; !ok {
		t.Error("createPayload missing 'uid'")
	} else if v != int64(1234) {
		t.Errorf("payload[uid] = %v, want 1234", v)
	}
	if v, ok := payload["username"]; !ok {
		t.Error("createPayload missing 'username'")
	} else if v != "testuser" {
		t.Errorf("payload[username] = %v, want testuser", v)
	}

	// password must be included when set.
	if v, ok := payload["password"]; !ok {
		t.Error("createPayload missing 'password' when it is set")
	} else if v != "secret123" {
		t.Errorf("payload[password] = %v, want secret123", v)
	}

	// builtin, immutable, local must NOT be in payload.
	for _, forbidden := range []string{"builtin", "immutable", "local", "id"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("createPayload should not contain %q", forbidden)
		}
	}
}

// TestUserCreatePayload_HomeModeAndWebshare verifies that createPayload
// includes home_mode and webshare when set.
func TestUserCreatePayload_HomeModeAndWebshare(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		UID:                  types.Int64Value(1234),
		Username:             types.StringValue("testuser"),
		FullName:             types.StringValue("Test User"),
		Email:                types.StringValue("test@example.com"),
		Home:                 types.StringValue("/home/testuser"),
		HomeMode:             types.StringValue("750"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(true),
		Webshare:             types.BoolValue(true),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Password:             types.StringValue("secret123"),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["home_mode"]; !ok {
		t.Error("createPayload missing 'home_mode' when it is set")
	} else if v != "750" {
		t.Errorf("payload[home_mode] = %v, want 750", v)
	}

	if v, ok := payload["webshare"]; !ok {
		t.Error("createPayload missing 'webshare' when it is set")
	} else if v != true {
		t.Errorf("payload[webshare] = %v, want true", v)
	}
}

// TestUserUpdatePayload verifies that updatePayload omits uid and username.
func TestUserUpdatePayload(t *testing.T) {
	ctx := context.Background()

	// Diff-only: only changed fields are sent. prior is the refreshed state;
	// plan changes full_name, locked and sshpubkey and leaves the rest alone.
	prior := UserModel{
		UID:                  types.Int64Value(1234),
		Username:             types.StringValue("testuser"),
		FullName:             types.StringValue("Old Name"),
		Email:                types.StringValue("old@example.com"),
		Home:                 types.StringValue("/mnt/tank/home"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue("oldkey"),
		SudoCommands:         types.ListNull(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
		Groups:               types.ListNull(types.Int64Type),
		Group:                types.Int64Value(1234),
		Password:             types.StringNull(),
	}
	m := prior
	m.FullName = types.StringValue("Updated Name")
	m.Locked = types.BoolValue(true)
	m.SSHPubKey = types.StringValue("") // clear the key

	payload, diags := m.updatePayload(ctx, prior)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	// Changed fields are present (with their payload transforms).
	if payload["full_name"] != "Updated Name" {
		t.Errorf("full_name = %v, want Updated Name", payload["full_name"])
	}
	if payload["locked"] != true {
		t.Errorf("locked = %v, want true", payload["locked"])
	}
	if v, ok := payload["sshpubkey"]; !ok || v != nil {
		t.Errorf("sshpubkey = %v (present=%v), want nil (empty string clears)", v, ok)
	}
	// Unchanged fields are omitted.
	for _, k := range []string{"email", "home", "shell", "password_disabled", "smb", "ssh_password_enabled", "sudo_commands", "groups", "group"} {
		if _, ok := payload[k]; ok {
			t.Errorf("unchanged field %q must be omitted, got %v", k, payload[k])
		}
	}
	// uid, username and (null) password are never in an update payload.
	for _, k := range []string{"uid", "username", "password"} {
		if _, ok := payload[k]; ok {
			t.Errorf("update payload must not contain %q", k)
		}
	}
}

// TestUserUpdatePayload_ReservedAdmin is the #43 regression, grounded in a live
// probe: the reserved administrator rejects both its non-/mnt `home` and its
// immutable `group` when re-sent unchanged. Diff-only must omit BOTH (and every
// other unchanged field) while still sending a genuine change.
func TestUserUpdatePayload_ReservedAdmin(t *testing.T) {
	ctx := context.Background()
	prior := UserModel{
		Username:         types.StringValue("truenas_admin"),
		Home:             types.StringValue("/home/truenas_admin"),
		Group:            types.Int64Value(950),
		Shell:            types.StringValue("/usr/bin/zsh"),
		Locked:           types.BoolValue(false),
		PasswordDisabled: types.BoolValue(false),
	}
	plan := prior
	plan.FullName = types.StringValue("Local Administrator (edited)")

	p, diags := plan.updatePayload(ctx, prior)
	if diags.HasError() {
		t.Fatalf("updatePayload: %v", diags)
	}
	if p["full_name"] != "Local Administrator (edited)" {
		t.Errorf("full_name = %v, want the edited name", p["full_name"])
	}
	for _, k := range []string{"home", "group", "shell", "locked", "password_disabled"} {
		if _, ok := p[k]; ok {
			t.Errorf("unchanged %q must be omitted (TrueNAS rejects unchanged home/group on the reserved admin), got %v", k, p[k])
		}
	}
}

// TestUserUpdatePayload_UnknownGroupsNotWiped is the v1.5.13 regression: an
// unmanaged (unknown) groups attribute must NOT be sent on update. basePayload
// coerces unknown -> [], and sending [] would clear the account's groups
// (including builtin_administrators on an imported admin, breaking its API key).
// An EXPLICITLY configured empty list is a real removal and IS sent.
func TestUserUpdatePayload_UnknownGroupsNotWiped(t *testing.T) {
	ctx := context.Background()
	prior := UserModel{
		Home:   types.StringValue("/home/truenas_admin"),
		Groups: types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(40), types.Int64Value(544)}),
	}

	// Unknown groups (unset Optional+Computed): must be omitted, not wiped.
	plan := prior
	plan.FullName = types.StringValue("Edited")
	plan.Groups = types.ListUnknown(types.Int64Type)
	p, diags := plan.updatePayload(ctx, prior)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if _, ok := p["groups"]; ok {
		t.Errorf("unknown groups must be omitted, got %#v (would wipe groups)", p["groups"])
	}
	if p["full_name"] != "Edited" {
		t.Errorf("the real change (full_name) must still be sent, got %v", p["full_name"])
	}

	// Explicitly configured empty list: a real removal, must be sent.
	plan2 := prior
	plan2.Groups = types.ListValueMust(types.Int64Type, []attr.Value{})
	p2, _ := plan2.updatePayload(ctx, prior)
	if g, ok := p2["groups"].([]int64); !ok || len(g) != 0 {
		t.Errorf("explicit empty groups must be sent as [], got %#v", p2["groups"])
	}
}

// TestUserUpdatePayload_UnchangedOmitsEverything guards the diff map against
// drift: when plan == prior, every mutable field is unchanged and must be
// omitted. A field added to basePayload but missing from updatePayload's map
// would remain here and fail this test.
func TestUserUpdatePayload_UnchangedOmitsEverything(t *testing.T) {
	m := UserModel{
		FullName: types.StringValue("A"), Email: types.StringValue("a@b.c"),
		Home: types.StringValue("/mnt/h"), Shell: types.StringValue("/bin/sh"),
		Locked: types.BoolValue(true), PasswordDisabled: types.BoolValue(true),
		SMB: types.BoolValue(false), Webshare: types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue("k"),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ALL")}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(40)}),
		Group:                types.Int64Value(950),
		Password:             types.StringNull(),
		HomeMode:             types.StringNull(),
	}
	p, diags := m.updatePayload(context.Background(), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(p) != 0 {
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		t.Errorf("unchanged update must omit all fields; these were not covered by the diff map: %v", keys)
	}
}

// TestUserCreatePayload_GroupCreate verifies that createPayload includes
// group_create when set, and omits it (and 'group') when neither is set.
func TestUserCreatePayload_GroupCreate(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("newuser"),
		FullName:             types.StringValue("New User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/newuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Group:                types.Int64Null(),
		GroupCreate:          types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["group_create"]; !ok {
		t.Error("createPayload missing 'group_create' when it is set")
	} else if v != true {
		t.Errorf("payload[group_create] = %v, want true", v)
	}
	if _, ok := payload["group"]; ok {
		t.Error("createPayload should not contain 'group' when Group is null")
	}
}

// TestUserCreatePayload_GroupID verifies that createPayload includes 'group'
// when a primary group id is set, and omits 'group_create' when it is unset.
func TestUserCreatePayload_GroupID(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("newuser"),
		FullName:             types.StringValue("New User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/newuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Group:                types.Int64Value(3000),
		GroupCreate:          types.BoolNull(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["group"]; !ok {
		t.Error("createPayload missing 'group' when Group is set")
	} else if v != int64(3000) {
		t.Errorf("payload[group] = %v, want 3000", v)
	}
	if _, ok := payload["group_create"]; ok {
		t.Error("createPayload should not contain 'group_create' when GroupCreate is null")
	}
}

// TestUserUpdatePayload_GroupIncluded verifies that updatePayload includes
// 'group' when set (primary group is updatable, unlike uid/username), and
// never includes 'group_create' (create-only, write-only).
func TestUserUpdatePayload_GroupIncluded(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("existinguser"),
		FullName:             types.StringValue("Existing User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/existinguser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListNull(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
		Groups:               types.ListNull(types.Int64Type),
		Group:                types.Int64Value(4000),
		GroupCreate:          types.BoolValue(true), // must never leak into update
	}

	payload, diags := m.updatePayload(ctx, UserModel{})
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	if v, ok := payload["group"]; !ok {
		t.Error("updatePayload missing 'group' when Group is set")
	} else if v != int64(4000) {
		t.Errorf("payload[group] = %v, want 4000", v)
	}
	if _, ok := payload["group_create"]; ok {
		t.Error("updatePayload must never contain 'group_create' (create-only, write-only)")
	}
}

// TestResponseToModel_GroupObjectDecode verifies responseToModel decodes a
// "group" field embedded as an object ({"id": N, ...}), matching the live
// user.query/get_instance wire shape.
func TestResponseToModel_GroupObjectDecode(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "grouped",
		FullName: "Grouped User",
		Home:     "/home/grouped",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`{"id": 5, "bsdgrp_gid": 5}`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m.Group.IsNull() {
		t.Fatal("Group should not be null when API returns a group object")
	}
	if m.Group.ValueInt64() != 5 {
		t.Errorf("Group = %v, want 5", m.Group.ValueInt64())
	}
}

// TestResponseToModel_GroupBareInt verifies responseToModel decodes a
// "group" field returned as a bare integer id.
func TestResponseToModel_GroupBareInt(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "grouped",
		FullName: "Grouped User",
		Home:     "/home/grouped",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`7`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m.Group.ValueInt64() != 7 {
		t.Errorf("Group = %v, want 7", m.Group.ValueInt64())
	}
}

// TestResponseToModel_GroupNull verifies responseToModel maps a null/absent
// "group" field to Int64Null rather than erroring or defaulting to 0.
func TestResponseToModel_GroupNull(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "nogroupuser",
		FullName: "No Group User",
		Home:     "/home/nogroupuser",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`null`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if !m.Group.IsNull() {
		t.Errorf("Group = %v, want null", m.Group)
	}
}

// TestResponseToModel verifies responseToModel populates all fields correctly.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()

	email := "user@example.com"
	pubkey := "ssh-rsa AAAAB3NzaC1"
	api := &userAPI{
		ID:                   99,
		UID:                  1001,
		Username:             "jdoe",
		FullName:             "John Doe",
		Email:                &email,
		Home:                 "/home/jdoe",
		Shell:                "/bin/zsh",
		Locked:               false,
		PasswordDisabled:     false,
		SMB:                  true,
		SSHPasswordEnabled:   true,
		SSHPubKey:            &pubkey,
		SudoCommands:         []string{"/usr/bin/ls"},
		SudoCommandsNoPasswd: []string{},
		Groups:               []int64{100, 200},
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 99 {
		t.Errorf("ID = %v, want 99", m.ID.ValueInt64())
	}
	if m.UID.ValueInt64() != 1001 {
		t.Errorf("UID = %v, want 1001", m.UID.ValueInt64())
	}
	if m.Username.ValueString() != "jdoe" {
		t.Errorf("Username = %v, want jdoe", m.Username.ValueString())
	}
	if m.Email.ValueString() != "user@example.com" {
		t.Errorf("Email = %v, want user@example.com", m.Email.ValueString())
	}
	if m.SSHPubKey.ValueString() != "ssh-rsa AAAAB3NzaC1" {
		t.Errorf("SSHPubKey = %v, want ssh-rsa AAAAB3NzaC1", m.SSHPubKey.ValueString())
	}
	if m.Local.ValueBool() != true {
		t.Errorf("Local = %v, want true", m.Local.ValueBool())
	}

	// Password must NOT be set by responseToModel.
	if !m.Password.IsNull() && !m.Password.IsUnknown() && m.Password.ValueString() != "" {
		t.Errorf("Password should not be set by responseToModel, got %q", m.Password.ValueString())
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if len(sudoCmds) != 1 || sudoCmds[0] != "/usr/bin/ls" {
		t.Errorf("SudoCommands = %v, want [/usr/bin/ls]", sudoCmds)
	}

	var groups []int64
	if d := m.Groups.ElementsAs(ctx, &groups, false); d.HasError() {
		t.Fatalf("Groups.ElementsAs failed: %v", d)
	}
	if len(groups) != 2 || groups[0] != 100 || groups[1] != 200 {
		t.Errorf("Groups = %v, want [100 200]", groups)
	}
}

// TestResponseToModel_NilPointers verifies nil email/sshpubkey map to empty strings.
func TestResponseToModel_NilPointers(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:                   1,
		UID:                  1000,
		Username:             "nomail",
		FullName:             "No Mail",
		Email:                nil,
		Home:                 "/home/nomail",
		Shell:                "/bin/sh",
		Locked:               false,
		PasswordDisabled:     true,
		SMB:                  false,
		SSHPasswordEnabled:   false,
		SSHPubKey:            nil,
		SudoCommands:         nil,
		SudoCommandsNoPasswd: nil,
		Groups:               nil,
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.Email.ValueString() != "" {
		t.Errorf("Email with nil API value = %q, want empty string", m.Email.ValueString())
	}
	if m.SSHPubKey.ValueString() != "" {
		t.Errorf("SSHPubKey with nil API value = %q, want empty string", m.SSHPubKey.ValueString())
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if sudoCmds == nil {
		t.Error("SudoCommands must not be nil when API returns nil")
	}

	var groups []int64
	if d := m.Groups.ElementsAs(ctx, &groups, false); d.HasError() {
		t.Fatalf("Groups.ElementsAs failed: %v", d)
	}
	if groups == nil {
		t.Error("Groups must not be nil when API returns nil")
	}
}

// TestUserSchema_PasswordWOVersion verifies the rotation trigger attribute.
func TestUserSchema_PasswordWOVersion(t *testing.T) {
	a, ok := resourceSchema().Attributes["password_wo_version"]
	if !ok {
		t.Fatal("schema missing 'password_wo_version' attribute")
	}
	i, ok := a.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'password_wo_version' is %T, want schema.Int64Attribute", a)
	}
	if !i.IsOptional() {
		t.Error("'password_wo_version' should be Optional")
	}
	if i.IsWriteOnly() {
		t.Error("'password_wo_version' must NOT be WriteOnly")
	}
	if i.IsComputed() {
		t.Error("'password_wo_version' must NOT be Computed")
	}
}

// TestResponseToModel_PasswordWOVersionPreserved verifies the config-only
// rotation trigger survives responseToModel (it has no API counterpart).
func TestResponseToModel_PasswordWOVersionPreserved(t *testing.T) {
	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "rot",
		FullName: "Rotating User",
		Home:     "/home/rot",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`{"id": 5, "bsdgrp_gid": 5}`),
	}
	m := UserModel{PasswordWOVersion: types.Int64Value(5)}
	if diags := responseToModel(context.Background(), api, &m); diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m.PasswordWOVersion.IsNull() || m.PasswordWOVersion.ValueInt64() != 5 {
		t.Errorf("PasswordWOVersion = %v, want 5", m.PasswordWOVersion)
	}
}

// TestUserCreatePayload_UnknownOmitted verifies that createPayload leaves out
// Optional+Computed fields the configuration does not set (unknown in the
// plan), so TrueNAS applies its defaults instead of receiving zero values: an
// unset home was sent as "" and rejected by user.create.
func TestUserCreatePayload_UnknownOmitted(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("newuser"),
		FullName:             types.StringValue("New User"),
		Email:                types.StringUnknown(),
		Home:                 types.StringUnknown(),
		Shell:                types.StringValue("/usr/sbin/nologin"),
		Locked:               types.BoolUnknown(),
		PasswordDisabled:     types.BoolUnknown(),
		SMB:                  types.BoolValue(false),
		Webshare:             types.BoolUnknown(),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringUnknown(),
		SudoCommands:         types.ListUnknown(types.StringType),
		SudoCommandsNoPasswd: types.ListUnknown(types.StringType),
		Groups:               types.ListUnknown(types.Int64Type),
		Group:                types.Int64Value(42),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	for _, key := range []string{"email", "home", "locked", "password_disabled", "webshare", "sshpubkey", "sudo_commands", "sudo_commands_nopasswd", "groups"} {
		if v, ok := payload[key]; ok {
			t.Errorf("createPayload sent unknown %q as %#v, want it omitted", key, v)
		}
	}
	for key, want := range map[string]any{
		"username":             "newuser",
		"full_name":            "New User",
		"shell":                "/usr/sbin/nologin",
		"smb":                  false,
		"ssh_password_enabled": false,
		"group":                int64(42),
	} {
		if got := payload[key]; got != want {
			t.Errorf("payload[%q] = %#v, want %#v", key, got, want)
		}
	}
}
