// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"bytes"
	"encoding/json"
	"errors"

	yaml "gopkg.in/yaml.v3"
)

// reconcileComposeConfig compares the user's Compose YAML against the live
// Compose document for a custom app. For a custom app, app.config returns the
// Compose document verbatim as an object (no chart defaults or ix_* keys), so
// unlike catalog `values` no projection is needed — the whole document is the
// user's, and additions/removals made out of band are real drift. The
// comparison is semantic (both sides canonicalized to sorted JSON), so
// formatting, comments, key order, and YAML-vs-JSON number spelling are not
// treated as drift (#34).
//
// It returns the string to store in state, whether the two are equivalent, and
// an error. When equivalent, the user's original YAML is preserved (no spurious
// diff). When they differ — real drift, or an initial populate on import where
// the prior string is empty — the live document is returned as YAML so the diff
// is readable. An unparseable user string is treated as "not equivalent" so the
// live value replaces it. A non-error nil is returned only on success; the error
// carries a generic message and never Compose contents (they may hold secrets).
func reconcileComposeConfig(previous string, liveRaw json.RawMessage) (string, bool, error) {
	// Decode with UseNumber so large integers keep exact precision instead of
	// being rounded through float64, which would otherwise conceal or invent
	// drift on big numeric values.
	live, err := decodeJSONExact(liveRaw)
	if err != nil {
		return "", false, errors.New("app.config did not return a valid Compose object; the previous Compose state has been preserved")
	}
	if m, ok := live.(map[string]any); !ok {
		return "", false, errors.New("app.config did not return a Compose object; the previous Compose state has been preserved")
	} else if _, ok := m["services"].(map[string]any); !ok {
		return "", false, errors.New("app.config did not return a Compose object with a services mapping; the previous Compose state has been preserved")
	}

	liveCanon, err := canonicalJSON(live)
	if err != nil {
		return "", false, errors.New("could not encode the Compose configuration returned by app.config; the previous Compose state has been preserved")
	}

	if userVal, perr := decodeYAMLExact(previous); perr == nil {
		if userCanon, cerr := canonicalJSON(userVal); cerr == nil && userCanon == liveCanon {
			return previous, true, nil // equivalent: preserve the user's exact YAML
		}
	}

	// Different (or prior unparseable / empty): emit the live document as YAML.
	// Serialize from a plainly-decoded copy so json.Number values render as
	// numbers, not quoted strings.
	var display any
	if json.Unmarshal([]byte(liveCanon), &display) == nil {
		if out, merr := yaml.Marshal(display); merr == nil {
			return string(out), false, nil
		}
	}
	return liveCanon, false, nil // fall back to canonical JSON (valid YAML)
}

// decodeJSONExact decodes JSON into a generic value, keeping numbers as
// json.Number (no float64 rounding).
func decodeJSONExact(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// decodeYAMLExact decodes exactly one YAML document into a generic value,
// normalized to JSON-compatible types.
func decodeYAMLExact(s string) (any, error) {
	var v any
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("empty document")
	}
	return normalizeYAML(v), nil
}

// canonicalJSON marshals a value to JSON with map keys sorted (encoding/json
// sorts them), giving a stable canonical form. json.Number marshals to its
// exact token, so integer precision is preserved.
func canonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// normalizeYAML converts any map[interface{}]interface{} into
// map[string]interface{} recursively so the value can be JSON-marshaled.
// yaml.v3 usually already yields string-keyed maps; this is defensive.
func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalizeYAML(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[toString(k)] = normalizeYAML(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normalizeYAML(val)
		}
		return t
	default:
		return v
	}
}

func toString(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	b, _ := json.Marshal(k)
	return string(b)
}
