// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// runUpgradeV0 runs the version 0 upgrader registered on the resource
// against old, the way the framework does: state decoded with the prior
// schema, the result written with the current one.
func runUpgradeV0(t *testing.T, old datasetModelV0) DatasetModel {
	t.Helper()
	ctx := context.Background()

	upgraders := (&DatasetResource{}).UpgradeState(ctx)
	up, ok := upgraders[0]
	if !ok || up.PriorSchema == nil {
		t.Fatal("no version 0 upgrader with a prior schema is registered")
	}

	prior := tfsdk.State{
		Schema: *up.PriorSchema,
		Raw:    tftypes.NewValue(up.PriorSchema.Type().TerraformType(ctx), nil),
	}
	if diags := prior.Set(ctx, &old); diags.HasError() {
		t.Fatalf("building version 0 state: %v", diags)
	}

	current := resourceSchema()
	resp := &resource.UpgradeStateResponse{State: tfsdk.State{
		Schema: current,
		Raw:    tftypes.NewValue(current.Type().TerraformType(ctx), nil),
	}}
	up.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &prior}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade: %v", resp.Diagnostics)
	}

	var got DatasetModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading upgraded state: %v", diags)
	}
	return got
}

func v0Base() datasetModelV0 {
	return datasetModelV0{
		ID:                    types.StringValue("tank/mydata"),
		Name:                  types.StringValue("tank/mydata"),
		Type:                  types.StringValue("filesystem"),
		Compression:           types.StringValue("lz4"),
		AClType:               types.StringValue("posix"),
		ShareType:             types.StringNull(),
		Comments:              types.StringValue("hello"),
		Quota:                 types.Int64Value(0),
		RefQuota:              types.Int64Value(0),
		Reservation:           types.Int64Value(0),
		VolSize:               types.Int64Value(0),
		SpecialSmallBlockSize: types.Int64Null(),
		ATime:                 types.StringValue("off"),
		Dedup:                 types.StringNull(),
		Readonly:              types.StringNull(),
		Snapdir:               types.StringNull(),
		Sync:                  types.StringValue("always"),
		AClMode:               types.StringNull(),
		Exec:                  types.StringNull(),
		Checksum:              types.StringNull(),
		Copies:                types.Int64Null(),
		RecordSize:            types.StringValue("1M"),
		MountPoint:            types.StringValue("/mnt/tank/mydata"),
		Encrypted:             types.BoolValue(false),
		Pool:                  types.StringValue("tank"),
	}
}

func TestDatasetSchemaVersion(t *testing.T) {
	if v := resourceSchema().Version; v != 1 {
		t.Fatalf("schema version = %d, want 1", v)
	}
}

// TestDatasetUpgradeStateV0ConvertsNumbers: the two attributes that changed
// type become their decimal strings, 0 included, and every other attribute
// is carried over unchanged.
func TestDatasetUpgradeStateV0ConvertsNumbers(t *testing.T) {
	old := v0Base()
	old.SpecialSmallBlockSize = types.Int64Value(0)
	old.Copies = types.Int64Value(2)

	got := runUpgradeV0(t, old)

	if !got.SpecialSmallBlockSize.Equal(types.StringValue("0")) {
		t.Errorf("special_small_block_size = %v, want \"0\"", got.SpecialSmallBlockSize)
	}
	if !got.Copies.Equal(types.StringValue("2")) {
		t.Errorf("copies = %v, want \"2\"", got.Copies)
	}
	for name, c := range map[string]struct{ got, want any }{
		"id":          {got.ID, old.ID},
		"name":        {got.Name, old.Name},
		"type":        {got.Type, old.Type},
		"compression": {got.Compression, old.Compression},
		"acltype":     {got.AClType, old.AClType},
		"share_type":  {got.ShareType, old.ShareType},
		"comments":    {got.Comments, old.Comments},
		"quota":       {got.Quota, old.Quota},
		"volsize":     {got.VolSize, old.VolSize},
		"atime":       {got.ATime, old.ATime},
		"dedup":       {got.Dedup, old.Dedup},
		"sync":        {got.Sync, old.Sync},
		"recordsize":  {got.RecordSize, old.RecordSize},
		"mountpoint":  {got.MountPoint, old.MountPoint},
		"encrypted":   {got.Encrypted, old.Encrypted},
		"pool":        {got.Pool, old.Pool},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", name, c.got, c.want)
		}
	}
}

// TestDatasetUpgradeStateV0KeepsNull: at version 0 null meant "not set
// LOCAL". The upgrader leaves it null rather than guessing "INHERIT"; the
// refresh after the upgrade reads the source and fills it in.
func TestDatasetUpgradeStateV0KeepsNull(t *testing.T) {
	got := runUpgradeV0(t, v0Base())

	if !got.SpecialSmallBlockSize.IsNull() {
		t.Errorf("special_small_block_size = %v, want null", got.SpecialSmallBlockSize)
	}
	if !got.Copies.IsNull() {
		t.Errorf("copies = %v, want null", got.Copies)
	}
	if !got.Dedup.IsNull() {
		t.Errorf("dedup = %v, want null", got.Dedup)
	}
}

// TestDatasetIntegerValidators pins what the two string-typed integer
// attributes accept. HCL numbers arrive already converted to strings.
func TestDatasetIntegerValidators(t *testing.T) {
	ctx := context.Background()
	check := func(vs []validator.String, v string) bool {
		for _, val := range vs {
			resp := &validator.StringResponse{}
			val.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(v)}, resp)
			if resp.Diagnostics.HasError() {
				return false
			}
		}
		return true
	}
	for _, tc := range []struct {
		name string
		vs   []validator.String
		in   string
		ok   bool
	}{
		{"ssbs 0", specialSmallBlockSizeValidators, "0", true},
		{"ssbs 16384", specialSmallBlockSizeValidators, "16384", true},
		{"ssbs INHERIT", specialSmallBlockSizeValidators, "INHERIT", true},
		{"ssbs inherit", specialSmallBlockSizeValidators, "inherit", true},
		{"ssbs negative", specialSmallBlockSizeValidators, "-1", false},
		{"ssbs suffix", specialSmallBlockSizeValidators, "16K", false},
		{"ssbs empty", specialSmallBlockSizeValidators, "", false},
		{"ssbs inherited", specialSmallBlockSizeValidators, "INHERITED", false},
		{"copies 1", copiesValidators, "1", true},
		{"copies 3", copiesValidators, "3", true},
		{"copies INHERIT", copiesValidators, "INHERIT", true},
		{"copies Inherit", copiesValidators, "Inherit", true},
		{"copies 0", copiesValidators, "0", false},
		{"copies 4", copiesValidators, "4", false},
		{"copies 12", copiesValidators, "12", false},
	} {
		if got := check(tc.vs, tc.in); got != tc.ok {
			t.Errorf("%s: %q accepted = %v, want %v", tc.name, tc.in, got, tc.ok)
		}
	}
}
