// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// keepAttributesIfConfigMatchesState keeps the prior state value for the
// attributes JSON when every key the configuration sets already has its
// configured value in state. After import the state holds every key the API
// reports (defaults included) while a configuration normally sets only a
// subset, so the first plan would otherwise propose rewriting attributes to
// that subset even though nothing on the device changes — mirroring the Read
// path, which already ignores keys the configuration omits (attributesDrifted). (#30)
type keepAttributesIfConfigMatchesState struct{}

func (m keepAttributesIfConfigMatchesState) Description(_ context.Context) string {
	return "keeps the imported attributes when the configuration is a subset that already matches"
}

func (m keepAttributesIfConfigMatchesState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m keepAttributesIfConfigMatchesState) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var stateAttrs, configAttrs map[string]any
	if json.Unmarshal([]byte(req.StateValue.ValueString()), &stateAttrs) != nil {
		return
	}
	if json.Unmarshal([]byte(req.ConfigValue.ValueString()), &configAttrs) != nil {
		return
	}
	// attributesDrifted(configAttrs, stateAttrs) is true when a configured key
	// is missing from, or differs in, state. When it is false every configured
	// key already matches, so keep the (fuller) state value.
	if !attributesDrifted(configAttrs, stateAttrs) {
		resp.PlanValue = req.StateValue
	}
}
