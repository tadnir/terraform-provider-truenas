// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import "encoding/json"

// projectConfigOntoUserShape returns the live app config reduced to only the
// keys (recursively) that the user set in their own `values` document. This is
// how drift is detected without noise (#33): `app.config` returns the fully
// resolved config — the user's sparse input merged with chart defaults plus
// server-managed `ix_*` keys — so comparing it whole against the user's sparse
// input would report every default and every ix_ key as drift. Projecting onto
// the user's key shape keeps only what the user manages: a key the user set
// that changed on the server shows as drift; defaults and ix_* never appear
// because the user never set them.
//
// userValuesJSON is the user's `values` string (from state); liveConfig is the
// decoded app.config object. The result is canonical JSON (sorted keys, like
// Terraform's jsonencode), so when nothing drifted it equals the user's input.
// On any parse error it returns userValuesJSON unchanged (never invent drift).
func projectConfigOntoUserShape(userValuesJSON string, liveConfig map[string]any) string {
	var userShape map[string]any
	if err := json.Unmarshal([]byte(userValuesJSON), &userShape); err != nil {
		return userValuesJSON
	}
	projected := projectMap(liveConfig, userShape)
	out, err := json.Marshal(projected)
	if err != nil {
		return userValuesJSON
	}
	return string(out)
}

// projectMap keeps, from server, only the keys present in userShape, recursing
// into nested objects so a partially-specified object (the user set one of its
// keys) keeps only that key. A key the user set but the server does not return
// is dropped (it cannot drift against a value that isn't there).
func projectMap(server, userShape map[string]any) map[string]any {
	out := map[string]any{}
	for k, uv := range userShape {
		sv, ok := server[k]
		if !ok {
			continue
		}
		if um, isMap := uv.(map[string]any); isMap {
			if sm, isSrvMap := sv.(map[string]any); isSrvMap {
				out[k] = projectMap(sm, um)
				continue
			}
		}
		out[k] = sv
	}
	return out
}
