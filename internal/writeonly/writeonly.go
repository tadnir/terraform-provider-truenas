// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package writeonly holds helpers for the write-only "secrets-out-of-state"
// pattern (GH #36/#37): a base JSON attribute the user sets with non-secret
// keys, plus a write-only companion holding secret keys that is merged over the
// base when building the API payload but never stored in state. On read, the
// base is reconciled using only the keys the user already has in it, so secret
// keys the server returns are never absorbed into state.
package writeonly

import "encoding/json"

// MergeOverlay returns base with the overlay JSON object merged over it (overlay
// keys add to or override base keys). overlayJSON is the config value of the
// write-only companion attribute; an empty string leaves base unchanged. The
// merge is shallow — secrets are top-level keys (account/key/api_token/...).
func MergeOverlay(base map[string]any, overlayJSON string) (map[string]any, error) {
	if overlayJSON == "" {
		return base, nil
	}
	var overlay map[string]any
	if err := json.Unmarshal([]byte(overlayJSON), &overlay); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out, nil
}

// ProjectOntoKeys reduces live to only the keys present in baseJSON (recursing
// into nested objects), returned as canonical JSON (sorted keys, like
// Terraform's jsonencode). Keys the user did not put in the base attribute —
// including write-only secret keys the server echoes back — are dropped, so
// they never reach state. On a parse error baseJSON is returned unchanged
// (never invent drift).
func ProjectOntoKeys(baseJSON string, live map[string]any) string {
	var base map[string]any
	if err := json.Unmarshal([]byte(baseJSON), &base); err != nil {
		return baseJSON
	}
	out, err := json.Marshal(projectMap(live, base))
	if err != nil {
		return baseJSON
	}
	return string(out)
}

// projectMap keeps, from live, only the keys present in shape, recursing into
// nested objects.
func projectMap(live, shape map[string]any) map[string]any {
	out := map[string]any{}
	for k, sv := range shape {
		lv, ok := live[k]
		if !ok {
			continue
		}
		if sm, isMap := sv.(map[string]any); isMap {
			if lm, isLiveMap := lv.(map[string]any); isLiveMap {
				out[k] = projectMap(lm, sm)
				continue
			}
		}
		out[k] = lv
	}
	return out
}
