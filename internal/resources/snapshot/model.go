// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import "github.com/hashicorp/terraform-plugin-framework/types"

// SnapshotModel is the Terraform state model for truenas_snapshot.
type SnapshotModel struct {
	ID        types.String `tfsdk:"id"` // "dataset@snapname"
	Dataset   types.String `tfsdk:"dataset"`
	Name      types.String `tfsdk:"name"`      // snapshot name only (no "@")
	Recursive types.Bool   `tfsdk:"recursive"` // write-only; not in API response
	// DeferDestroy only affects how the snapshot is destroyed; it is not a
	// property of the snapshot and is never read back.
	DeferDestroy types.Bool `tfsdk:"defer_destroy"`
	// Computed
	Pool      types.String `tfsdk:"pool"`
	CreateTxg types.String `tfsdk:"createtxg"`
}

// SnapshotDatasourceModel is used by the snapshot data source; it omits
// Recursive because the API never returns that field.
type SnapshotDatasourceModel struct {
	ID        types.String `tfsdk:"id"`
	Dataset   types.String `tfsdk:"dataset"`
	Name      types.String `tfsdk:"name"`
	Pool      types.String `tfsdk:"pool"`
	CreateTxg types.String `tfsdk:"createtxg"`
}

// snapshotAPI matches the JSON returned by pool.snapshot.get_instance / pool.snapshot.create.
type snapshotAPI struct {
	ID           string `json:"id"`
	Dataset      string `json:"dataset"`
	Pool         string `json:"pool"`
	SnapshotName string `json:"snapshot_name"`
	CreateTxg    string `json:"createtxg"`
}

// deleteArgs returns the pool.snapshot.delete arguments for a snapshot in
// state. With defer_destroy the options ask for a deferred destroy (zfs
// destroy -d): a snapshot that still has clones or holds is marked for
// destruction instead of the delete failing, and ZFS removes it once the
// last clone or hold goes. Without it the call is exactly what it was
// before the attribute existed.
func deleteArgs(m *SnapshotModel) []any {
	if m.DeferDestroy.ValueBool() {
		return []any{m.ID.ValueString(), map[string]any{"defer": true}}
	}
	return []any{m.ID.ValueString()}
}

// responseToModel copies API response fields into m.
// Recursive is write-only and not present in the response; the caller must
// preserve it from plan/state before calling this function.
func responseToModel(api *snapshotAPI, m *SnapshotModel) {
	m.ID = types.StringValue(api.ID)
	m.Dataset = types.StringValue(api.Dataset)
	m.Name = types.StringValue(api.SnapshotName)
	m.Pool = types.StringValue(api.Pool)
	m.CreateTxg = types.StringValue(api.CreateTxg)
	// m.Recursive is intentionally left unchanged (write-only).
}
