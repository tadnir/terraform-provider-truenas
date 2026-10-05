// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package writeonly

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMergeOverlay(t *testing.T) {
	base := map[string]any{"type": "S3", "region": "us-east-1"}

	t.Run("empty overlay returns base unchanged", func(t *testing.T) {
		got, err := MergeOverlay(base, "")
		if err != nil || !reflect.DeepEqual(got, base) {
			t.Fatalf("got %v err %v", got, err)
		}
	})

	t.Run("overlay adds and overrides keys", func(t *testing.T) {
		got, err := MergeOverlay(base, `{"access_key":"AKIA","region":"eu-west-1"}`)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"type": "S3", "region": "eu-west-1", "access_key": "AKIA"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})

	t.Run("invalid overlay JSON errors", func(t *testing.T) {
		if _, err := MergeOverlay(base, "{not json"); err == nil {
			t.Errorf("expected error on invalid overlay")
		}
	})
}

func TestProjectOntoKeys(t *testing.T) {
	// live config as the server returns it: base keys + a secret key the user
	// supplied only via the write-only overlay, plus a server default.
	live := map[string]any{
		"type":        "S3",
		"region":      "eu-west-1",
		"access_key":  "AKIA-SECRET", // present only because it was sent via _wo
		"skip_verify": false,         // server default the user never set
	}

	t.Run("secret and default keys are dropped; base keys reconciled", func(t *testing.T) {
		got := ProjectOntoKeys(`{"type":"S3","region":"us-east-1"}`, live)
		var gm map[string]any
		if err := json.Unmarshal([]byte(got), &gm); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"type": "S3", "region": "eu-west-1"}
		if !reflect.DeepEqual(gm, want) {
			t.Errorf("projection must keep only base keys (reconciled): got %v want %v", gm, want)
		}
		if _, leaked := gm["access_key"]; leaked {
			t.Errorf("secret key leaked into state via projection")
		}
	})

	t.Run("invalid base passes through unchanged", func(t *testing.T) {
		if got := ProjectOntoKeys("{not json", live); got != "{not json" {
			t.Errorf("bad base should pass through, got %q", got)
		}
	})
}
