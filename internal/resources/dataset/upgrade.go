// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

// FORK ONLY (tadnir/terraform-provider-truenas, branch terrahome). Not for
// upstream: it converts state written by the fork's own builds up to
// 1.1.0-terrahome.8, which carried a different truenas_dataset schema.
//
// Those builds had the twelve ZFS properties before upstream added them in
// 1.3.0, in another shape: every property a string, "INHERIT" in state
// whenever the property was not set on the dataset itself, lower-case enum
// values, copies and special_small_block_size as decimal strings, and a
// settable encrypted in place of upstream's encryption inputs. That state
// is at schema version 1 (the fork's own upgrade had moved it from 0), and
// upstream's schema cannot decode it ("INHERIT" is not a number). So the
// schema version is 2 here and state at 0 or 1 goes through
// upgradeFromFork. State upstream itself wrote (version 0) passes through
// unchanged in effect.

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithUpgradeState = &DatasetResource{}

// schemaVersion is 2 only so that the fork's older state is upgraded.
const schemaVersion = 2

func (r *DatasetResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {StateUpgrader: upgradeFromFork},
		1: {StateUpgrader: upgradeFromFork},
	}
}

func upgradeFromFork(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.RawState == nil || len(req.RawState.JSON) == 0 {
		resp.Diagnostics.AddError("Cannot upgrade truenas_dataset state", "the stored state has no JSON form")
		return
	}
	upgraded, err := forkStateToCurrent(req.RawState.JSON)
	if err != nil {
		resp.Diagnostics.AddError("Cannot upgrade truenas_dataset state", err.Error())
		return
	}
	typ := resourceSchema().Type().TerraformType(ctx)
	val, err := (&tfprotov6.RawState{JSON: upgraded}).UnmarshalWithOpts(typ, tfprotov6.UnmarshalOpts{
		ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true},
	})
	if err != nil {
		resp.Diagnostics.AddError("Cannot upgrade truenas_dataset state", err.Error())
		return
	}
	dv, err := tfprotov6.NewDynamicValue(typ, val)
	if err != nil {
		resp.Diagnostics.AddError("Cannot upgrade truenas_dataset state", err.Error())
		return
	}
	resp.DynamicValue = &dv
}

// forkEnumProps are the fork's lower-case string properties that upstream
// validates against upper-case values.
var forkEnumProps = []string{"aclmode", "atime", "checksum", "dedup", "exec", "readonly", "snapdir", "sync"}

// forkNumberProps were decimal strings in the fork and are numbers upstream.
var forkNumberProps = []string{"copies", "special_small_block_size"}

// forkStateToCurrent rewrites one dataset's state JSON from the fork's
// schema into the current one. Read refreshes every property from TrueNAS
// straight after, so what matters is that the result decodes.
func forkStateToCurrent(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var st map[string]any
	if err := dec.Decode(&st); err != nil {
		return nil, err
	}

	isInherit := func(v any) bool {
		s, ok := v.(string)
		return ok && strings.EqualFold(s, "INHERIT")
	}

	for _, k := range forkEnumProps {
		if isInherit(st[k]) {
			st[k] = nil
		} else if s, ok := st[k].(string); ok {
			st[k] = strings.ToUpper(s)
		}
	}
	if isInherit(st["recordsize"]) {
		st["recordsize"] = nil
	} else if s, ok := st["recordsize"].(string); ok {
		st["recordsize"] = strings.ToUpper(s)
	}
	for _, k := range forkNumberProps {
		switch v := st[k].(type) {
		case string:
			if isInherit(v) || v == "" {
				st[k] = nil
				continue
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			st[k] = n
		}
	}

	// encrypted was settable in the fork and is read-only upstream, where
	// encryption carries the setting. Read fills in encryption anyway; the
	// create-only inputs (inherit_encryption, encryption_generate_key) stay
	// null, because state cannot tell an encryption root from a dataset that
	// inherits encryption, and a null there no longer forces replacement.
	if enc, ok := st["encrypted"].(bool); ok {
		if _, set := st["encryption"]; !set {
			st["encryption"] = enc
		}
	}

	return json.Marshal(st)
}
