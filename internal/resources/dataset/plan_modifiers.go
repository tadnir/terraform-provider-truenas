// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// replaceIfChangedFromKnown requires replacement only when a create-only bool
// changes from a known prior value. inherit_encryption and
// encryption_generate_key are never read back, so an imported dataset has null
// for them; setting the matching value should record it in place rather than
// force a destroy/recreate. Update already strips the encryption inputs, so
// recording the value alone is safe. (#29)
func replaceIfChangedFromKnown() planmodifier.Bool {
	return boolplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.BoolRequest, resp *boolplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.StateValue.IsNull() && !req.PlanValue.Equal(req.StateValue)
		},
		"Replaced only when changed from a known value (null after import records in place).",
		"Replaced only when changed from a known value (null after import records in place).",
	)
}
