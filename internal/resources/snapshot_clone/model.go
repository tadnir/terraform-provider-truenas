// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SnapshotCloneModel is the Terraform state model for truenas_snapshot_clone.
type SnapshotCloneModel struct {
	ID                types.String `tfsdk:"id"` // the clone's dataset name
	Snapshot          types.String `tfsdk:"snapshot"`
	Dataset           types.String `tfsdk:"dataset"`
	DatasetProperties types.Map    `tfsdk:"dataset_properties"` // write-only; set at clone time
	// Computed
	Type       types.String `tfsdk:"type"`
	Pool       types.String `tfsdk:"pool"`
	MountPoint types.String `tfsdk:"mountpoint"`
}

// cloneAPI is the part of pool.dataset.get_instance this resource reads.
// origin is the snapshot a clone was created from; it is how Read confirms
// the dataset is still a clone and how import recovers `snapshot`. Its
// rawvalue is the snapshot's full name for a clone, and empty (or "-", as
// zfs(8) prints it) for a dataset that is not one.
type cloneAPI struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Pool       string `json:"pool"`
	MountPoint string `json:"mountpoint"`
	Origin     struct {
		RawValue *string `json:"rawvalue"`
	} `json:"origin"`
}

// origin returns the snapshot the dataset was cloned from, or false when it
// is not (or is no longer) a clone.
func (a *cloneAPI) origin() (string, bool) {
	if a.Origin.RawValue == nil {
		return "", false
	}
	v := strings.TrimSpace(*a.Origin.RawValue)
	if v == "" || v == "-" {
		return "", false
	}
	return v, true
}

// clonePayload builds the pool.snapshot.clone argument.
func clonePayload(snapshot, dataset string, props map[string]string) map[string]any {
	p := map[string]any{
		"snapshot":    snapshot,
		"dataset_dst": dataset,
	}
	if len(props) > 0 {
		p["dataset_properties"] = props
	}
	return p
}

// responseToModel copies the API fields into m. Snapshot is set from
// origin only when the dataset is still a clone; the caller decides what a
// missing origin means. DatasetProperties is write-only and left alone.
func responseToModel(api *cloneAPI, m *SnapshotCloneModel) {
	m.ID = types.StringValue(api.Name)
	m.Dataset = types.StringValue(api.Name)
	m.Type = types.StringValue(api.Type)
	m.Pool = types.StringValue(api.Pool)
	m.MountPoint = types.StringValue(api.MountPoint)
	if o, ok := api.origin(); ok {
		m.Snapshot = types.StringValue(o)
	}
}
