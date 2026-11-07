// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// effectiveTransport is the transport that actually applies: an unset (null)
// or not-yet-known (unknown) value means the documented default, "LOCAL".
// This mirrors transportOrDefault in resource.go, but takes the attribute
// value directly (not the whole model) and treats unknown as the default too,
// which is what the plan modifier needs — the planned value is unknown only
// when it will resolve to the default.
func effectiveTransport(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return "LOCAL"
	}
	return v.ValueString()
}

// replaceIfTransportChanged requires replacement only when the EFFECTIVE
// transport changes. Prior state written before this attribute existed holds
// null, which reads as the default "LOCAL"; once the Default fills it the plan
// shows null -> "LOCAL", and an unconditional RequiresReplace() read that as a
// change and planned a destroy/recreate of every replication task on upgrade.
// Comparing effective values makes null -> "LOCAL" a no-op while a real change
// (e.g. LOCAL -> SSH) still forces replacement, since the TrueNAS API does not
// allow changing a replication task's transport in place. Mirrors
// replaceIfStorePrivatekeyChanged in the certificate resource.
func replaceIfTransportChanged() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = effectiveTransport(req.StateValue) != effectiveTransport(req.PlanValue)
		},
		"Replaced only when the effective transport changes (unset reads as the default LOCAL).",
		"Replaced only when the effective transport changes (unset reads as the default LOCAL).",
	)
}
