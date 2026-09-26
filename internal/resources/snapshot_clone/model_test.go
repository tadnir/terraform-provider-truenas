// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestClonePayload(t *testing.T) {
	got := clonePayload("tank/golden@v1", "tank/vms/a", nil)
	want := map[string]any{"snapshot": "tank/golden@v1", "dataset_dst": "tank/vms/a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("without properties: got %#v, want %#v", got, want)
	}

	got = clonePayload("tank/golden@v1", "tank/vms/a", map[string]string{"compression": "lz4"})
	if !reflect.DeepEqual(got["dataset_properties"], map[string]string{"compression": "lz4"}) {
		t.Errorf("dataset_properties not passed through: %#v", got)
	}
}

// TestCloneOrigin decodes origin in the pool.dataset.get_instance property
// shape and checks what counts as "is a clone".
func TestCloneOrigin(t *testing.T) {
	cases := []struct {
		json   string
		want   string
		isClon bool
	}{
		{`{"origin": {"parsed": "tank/golden@v1", "rawvalue": "tank/golden@v1", "value": "tank/golden@v1", "source": "NONE"}}`, "tank/golden@v1", true},
		{`{"origin": {"parsed": "", "rawvalue": "", "value": null, "source": "NONE"}}`, "", false},
		{`{"origin": {"rawvalue": "-"}}`, "", false},
		{`{"origin": {"rawvalue": null}}`, "", false},
		{`{}`, "", false},
	}
	for _, tc := range cases {
		var api cloneAPI
		if err := json.Unmarshal([]byte(tc.json), &api); err != nil {
			t.Fatalf("%s: %v", tc.json, err)
		}
		got, ok := api.origin()
		if got != tc.want || ok != tc.isClon {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", tc.json, got, ok, tc.want, tc.isClon)
		}
	}
}

// TestResponseToModelKeepsSnapshotWhenPromoted: a promoted clone has no
// origin; state must keep the snapshot it was created from, since changing
// it would plan a replacement that destroys the dataset.
func TestResponseToModelKeepsSnapshotWhenPromoted(t *testing.T) {
	api := &cloneAPI{Name: "tank/vms/a", Type: "FILESYSTEM", Pool: "tank", MountPoint: "/mnt/tank/vms/a"}
	m := &SnapshotCloneModel{Snapshot: types.StringValue("tank/golden@v1")}
	responseToModel(api, m)
	if m.Snapshot.ValueString() != "tank/golden@v1" {
		t.Errorf("snapshot changed to %v", m.Snapshot)
	}
	if notACloneWarning(api).WarningsCount() == 0 {
		t.Error("expected a warning for a dataset with no origin")
	}
}

func TestResponseToModelSetsFromOrigin(t *testing.T) {
	var api cloneAPI
	raw := `{"name": "tank/vms/a", "type": "VOLUME", "pool": "tank", "mountpoint": null,
		"origin": {"rawvalue": "tank/golden@v1"}}`
	if err := json.Unmarshal([]byte(raw), &api); err != nil {
		t.Fatal(err)
	}
	m := &SnapshotCloneModel{}
	responseToModel(&api, m)
	if m.Snapshot.ValueString() != "tank/golden@v1" || m.ID.ValueString() != "tank/vms/a" ||
		m.Type.ValueString() != "VOLUME" || m.MountPoint.ValueString() != "" {
		t.Errorf("unexpected model: %+v", m)
	}
	if notACloneWarning(&api).WarningsCount() != 0 {
		t.Error("a clone must not warn")
	}
}
