// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDeleteArgsDeferDestroy pins the pool.snapshot.delete call: the
// deferred option is sent only when defer_destroy is true, and otherwise the
// call is the bare id it always was (null and false alike).
func TestDeleteArgsDeferDestroy(t *testing.T) {
	id := types.StringValue("tank/data@snap")
	cases := []struct {
		name         string
		deferDestroy types.Bool
		want         []any
	}{
		{"true", types.BoolValue(true), []any{"tank/data@snap", map[string]any{"defer": true}}},
		{"false", types.BoolValue(false), []any{"tank/data@snap"}},
		{"null", types.BoolNull(), []any{"tank/data@snap"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deleteArgs(&SnapshotModel{ID: id, DeferDestroy: tc.deferDestroy})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
