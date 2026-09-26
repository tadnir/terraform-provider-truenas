// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *DatasetResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	prior := resourceSchemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &prior,
			StateUpgrader: upgradeStateV0,
		},
	}
}

// resourceSchemaV0 is the truenas_dataset schema as it was at version 0,
// frozen here so that state written by it can still be decoded after the
// live schema moves on. Only the attribute types and the
// Required/Optional/Computed flags matter for decoding, so descriptions,
// validators and plan modifiers are left out. The difference from version
// 1 is that special_small_block_size and copies were numbers.
func resourceSchemaV0() schema.Schema {
	optComputedString := schema.StringAttribute{Optional: true, Computed: true}
	optComputedInt64 := schema.Int64Attribute{Optional: true, Computed: true}
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Computed: true},
			"name":                     schema.StringAttribute{Required: true},
			"type":                     optComputedString,
			"compression":              optComputedString,
			"acltype":                  optComputedString,
			"share_type":               schema.StringAttribute{Optional: true},
			"comments":                 optComputedString,
			"quota":                    optComputedInt64,
			"refquota":                 optComputedInt64,
			"reservation":              optComputedInt64,
			"volsize":                  optComputedInt64,
			"special_small_block_size": optComputedInt64,
			"atime":                    optComputedString,
			"dedup":                    optComputedString,
			"readonly":                 optComputedString,
			"snapdir":                  optComputedString,
			"sync":                     optComputedString,
			"aclmode":                  optComputedString,
			"exec":                     optComputedString,
			"checksum":                 optComputedString,
			"copies":                   optComputedInt64,
			"recordsize":               optComputedString,
			"mountpoint":               schema.StringAttribute{Computed: true},
			"encrypted":                schema.BoolAttribute{Computed: true},
			"pool":                     schema.StringAttribute{Computed: true},
		},
	}
}

// datasetModelV0 is DatasetModel as it was at schema version 0.
type datasetModelV0 struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Type                  types.String `tfsdk:"type"`
	Compression           types.String `tfsdk:"compression"`
	AClType               types.String `tfsdk:"acltype"`
	ShareType             types.String `tfsdk:"share_type"`
	Comments              types.String `tfsdk:"comments"`
	Quota                 types.Int64  `tfsdk:"quota"`
	RefQuota              types.Int64  `tfsdk:"refquota"`
	Reservation           types.Int64  `tfsdk:"reservation"`
	VolSize               types.Int64  `tfsdk:"volsize"`
	SpecialSmallBlockSize types.Int64  `tfsdk:"special_small_block_size"`
	ATime                 types.String `tfsdk:"atime"`
	Dedup                 types.String `tfsdk:"dedup"`
	Readonly              types.String `tfsdk:"readonly"`
	Snapdir               types.String `tfsdk:"snapdir"`
	Sync                  types.String `tfsdk:"sync"`
	AClMode               types.String `tfsdk:"aclmode"`
	Exec                  types.String `tfsdk:"exec"`
	Checksum              types.String `tfsdk:"checksum"`
	Copies                types.Int64  `tfsdk:"copies"`
	RecordSize            types.String `tfsdk:"recordsize"`
	MountPoint            types.String `tfsdk:"mountpoint"`
	Encrypted             types.Bool   `tfsdk:"encrypted"`
	Pool                  types.String `tfsdk:"pool"`
}

// upgradeStateV0 moves state from schema version 0 to 1. The only change
// is the type of special_small_block_size and copies: a number becomes its
// decimal string, and null stays null. At version 0 null meant "not set
// LOCAL", which version 1 records as "INHERIT", but the upgrader does not
// guess that: the refresh that follows every upgrade reads the property's
// source and fills it in (or leaves it null for a dataset type that does
// not carry it).
func upgradeStateV0(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var old datasetModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}

	m := DatasetModel{
		ID:                    old.ID,
		Name:                  old.Name,
		Type:                  old.Type,
		Compression:           old.Compression,
		AClType:               old.AClType,
		ShareType:             old.ShareType,
		Comments:              old.Comments,
		Quota:                 old.Quota,
		RefQuota:              old.RefQuota,
		Reservation:           old.Reservation,
		VolSize:               old.VolSize,
		SpecialSmallBlockSize: int64ToString(old.SpecialSmallBlockSize),
		ATime:                 old.ATime,
		Dedup:                 old.Dedup,
		Readonly:              old.Readonly,
		Snapdir:               old.Snapdir,
		Sync:                  old.Sync,
		AClMode:               old.AClMode,
		Exec:                  old.Exec,
		Checksum:              old.Checksum,
		Copies:                int64ToString(old.Copies),
		RecordSize:            old.RecordSize,
		MountPoint:            old.MountPoint,
		Encrypted:             old.Encrypted,
		Pool:                  old.Pool,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// int64ToString converts a version 0 number to its version 1 decimal
// string, keeping null (and unknown, which state should never hold) as is.
func int64ToString(v types.Int64) types.String {
	switch {
	case v.IsNull():
		return types.StringNull()
	case v.IsUnknown():
		return types.StringUnknown()
	}
	return types.StringValue(strconv.FormatInt(v.ValueInt64(), 10))
}
