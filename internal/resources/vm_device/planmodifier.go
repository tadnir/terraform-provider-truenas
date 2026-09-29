// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// keepStateWhenConfigIsSubset keeps the attributes JSON from state when every
// key in the configuration already has the same value there.
//
// State holds whatever the API reported when the device was imported: every
// attribute, defaults included. Configuration usually names only the keys that
// matter. Without this, the first plan after an import would propose rewriting
// the device to the configured subset, although nothing on it would change.
// Keys the configuration does not name are left to the API, which is how Read
// already treats them (see attributesDrifted).
type keepStateWhenConfigIsSubset struct{}

func (keepStateWhenConfigIsSubset) Description(context.Context) string {
	return "Keeps the attributes in state when the configured attributes are a subset of them with equal values."
}

func (m keepStateWhenConfigIsSubset) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepStateWhenConfigIsSubset) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if configIsSubsetOf(req.ConfigValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

// configIsSubsetOf reports whether the JSON object config has only keys whose
// values equal those in the JSON object state. Anything that does not parse
// as an object is not a subset, so the plan falls back to the configuration.
func configIsSubsetOf(config, state string) bool {
	var c, s map[string]any
	if json.Unmarshal([]byte(config), &c) != nil || json.Unmarshal([]byte(state), &s) != nil {
		return false
	}
	return !attributesDrifted(c, s)
}
