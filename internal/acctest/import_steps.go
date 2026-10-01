// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// ImportReapplyNoop returns the two TestSteps that guard the import lifecycle
// against spurious diffs: import the resource, then re-apply the *identical*
// config and assert the plan is empty.
//
// This catches the class of bug that plain ImportStateVerify misses. Import
// reads state back from the API; attributes the API does not return (write-
// only or create-only inputs) come back null in the imported state. The next
// plan compares the user's config — which sets those inputs — against that
// null state and sees a change. If such an input is wrongly marked
// RequiresReplace, the plan destroys and recreates a resource the user only
// imported (GH #29: dataset encryption inputs; GH #30: vm_device attributes).
// ImportStateVerify alone does not run a plan against the live config, so it
// never sees this; the explicit re-apply step does.
//
// resourceName is the address (e.g. "truenas_dataset.test"). config is the
// exact HCL already applied in the preceding create step (same provider block
// + resource block), so the re-apply is a true no-op when the provider is
// correct. importVerifyIgnore lists attributes the API genuinely cannot round-
// trip on import (e.g. a jsonencoded blob the server re-normalizes) and so
// cannot be state-compared; the empty-plan re-apply still covers them. Pass
// none when every attribute round-trips, and prefer fixing the schema over
// ignoring.
func ImportReapplyNoop(resourceName, config string, importVerifyIgnore ...string) []resource.TestStep {
	return []resource.TestStep{
		{
			ResourceName:            resourceName,
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: importVerifyIgnore,
		},
		{
			Config: config,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}
}
