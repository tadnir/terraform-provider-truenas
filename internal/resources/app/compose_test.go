// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"
	"testing"
)

func liveRaw(t *testing.T, obj map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal live: %v", err)
	}
	return b
}

func TestReconcileComposeConfig(t *testing.T) {
	live := map[string]any{
		"services": map[string]any{
			"example": map[string]any{
				"image":   "busybox:1.36",
				"command": []any{"sleep", "infinity"},
			},
		},
	}
	raw := liveRaw(t, live)

	t.Run("formatting-only difference is not drift; user YAML preserved", func(t *testing.T) {
		userYAML := "# my app\nservices:\n  example:\n    command: [\"sleep\", \"infinity\"]\n    image: busybox:1.36\n"
		got, equal, err := reconcileComposeConfig(userYAML, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !equal {
			t.Fatalf("expected equal (formatting-only), got drift")
		}
		if got != userYAML {
			t.Errorf("equal must preserve the user's YAML verbatim\n got: %q", got)
		}
	})

	t.Run("image change is drift", func(t *testing.T) {
		userYAML := "services:\n  example:\n    image: busybox:1.35\n    command: [\"sleep\", \"infinity\"]\n"
		_, equal, err := reconcileComposeConfig(userYAML, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if equal {
			t.Errorf("expected drift (image 1.35 vs live 1.36)")
		}
	})

	t.Run("empty prior populates from live (import)", func(t *testing.T) {
		got, equal, err := reconcileComposeConfig("", raw)
		if err != nil || equal || got == "" {
			t.Fatalf("empty prior must populate from live: (%q, %v, %v)", got, equal, err)
		}
		if _, eq, _ := reconcileComposeConfig(got, raw); !eq {
			t.Errorf("populated YAML should round-trip equal to live")
		}
	})

	t.Run("large integer keeps precision (no float64 rounding)", func(t *testing.T) {
		big := map[string]any{"services": map[string]any{
			"db": map[string]any{"image": "x", "shm_size": json.Number("18014398509481985")}, // 2^54+1
		}}
		bigRaw := liveRaw(t, big)
		// Same exact integer in the user's YAML: must compare equal.
		userYAML := "services:\n  db:\n    image: x\n    shm_size: 18014398509481985\n"
		if _, equal, err := reconcileComposeConfig(userYAML, bigRaw); err != nil || !equal {
			t.Errorf("exact large int must be equal, got (equal=%v, err=%v)", equal, err)
		}
		// A different large int differing only in the last digit must be drift.
		userYAML2 := "services:\n  db:\n    image: x\n    shm_size: 18014398509481984\n"
		if _, equal, _ := reconcileComposeConfig(userYAML2, bigRaw); equal {
			t.Errorf("large ints differing by 1 must be detected as drift (float64 would mask this)")
		}
	})

	t.Run("live without services mapping is an error", func(t *testing.T) {
		bad := liveRaw(t, map[string]any{"not_services": 1})
		if _, _, err := reconcileComposeConfig("services:\n  a: {}\n", bad); err == nil {
			t.Errorf("expected error when live has no services mapping")
		}
	})

	t.Run("invalid live JSON is an error", func(t *testing.T) {
		if _, _, err := reconcileComposeConfig("services: {}", json.RawMessage(`not json`)); err == nil {
			t.Errorf("expected error on invalid live JSON")
		}
	})
}
