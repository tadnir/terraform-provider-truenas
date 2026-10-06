// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProjectConfigOntoUserShape(t *testing.T) {
	// live app.config: user's sparse input merged with chart defaults + ix_* keys.
	live := map[string]any{
		"TZ":           "UTC",
		"release_name": "syncthing", // chart default, user never set
		"resources":    map[string]any{"limits": map[string]any{"cpus": "2"}},
		"syncthing": map[string]any{
			"enable_relaying": false,
			"gui_port":        float64(8384), // default the user never set
		},
		"ix_context": map[string]any{"foo": "bar"}, // server-managed
		"ix_volumes": []any{"a", "b"},              // server-managed
	}

	tests := []struct {
		name string
		user string
		want map[string]any
	}{
		{
			name: "top-level key the user set is kept; defaults and ix_* dropped",
			user: `{"TZ":"America/New_York"}`,
			want: map[string]any{"TZ": "UTC"}, // value projected from live (drift: UTC)
		},
		{
			name: "nested partial: only the sub-key the user set is kept",
			user: `{"syncthing":{"enable_relaying":true}}`,
			want: map[string]any{"syncthing": map[string]any{"enable_relaying": false}},
		},
		{
			name: "multiple keys; unrelated defaults and ix_* never appear",
			user: `{"TZ":"x","syncthing":{"enable_relaying":true}}`,
			want: map[string]any{"TZ": "UTC", "syncthing": map[string]any{"enable_relaying": false}},
		},
		{
			name: "a user key absent from live is dropped",
			user: `{"nonexistent":"y"}`,
			want: map[string]any{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := projectConfigOntoUserShape(tc.user, live)
			var gotMap map[string]any
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Fatalf("result not valid JSON: %v (%s)", err, got)
			}
			if !reflect.DeepEqual(gotMap, tc.want) {
				t.Errorf("projection mismatch\n got: %v\nwant: %v", gotMap, tc.want)
			}
		})
	}
}

// When nothing drifted, the projection equals the user's canonical input, so
// the state value is unchanged and no diff is produced.
func TestProjectConfigOntoUserShape_NoDriftIsStable(t *testing.T) {
	live := map[string]any{
		"TZ":           "America/New_York",
		"release_name": "syncthing",
		"ix_context":   map[string]any{"x": "y"},
	}
	// Terraform's jsonencode produces sorted, compact JSON; so does json.Marshal.
	userCanonical := `{"TZ":"America/New_York"}`
	got := projectConfigOntoUserShape(userCanonical, live)
	if got != userCanonical {
		t.Errorf("no-drift projection should equal user input\n got: %s\nwant: %s", got, userCanonical)
	}
}

// A malformed user document is returned unchanged — never invent drift.
func TestProjectConfigOntoUserShape_BadJSONPassesThrough(t *testing.T) {
	bad := `{not json`
	if got := projectConfigOntoUserShape(bad, map[string]any{"a": "b"}); got != bad {
		t.Errorf("bad JSON should pass through unchanged, got %s", got)
	}
}
