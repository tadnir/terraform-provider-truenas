// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a device attached to a TrueNAS VM (disk, NIC, CD-ROM, display, PCI passthrough, raw file, or USB).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric VM device ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"vm": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Description: "ID of the VM this device belongs to.",
			},
			"attributes": schema.StringAttribute{
				Required: true,
				// Marked Sensitive because this opaque JSON blob can carry secrets for some
				// device types (e.g. the DISPLAY device's VNC password). It is over-broad for
				// device types with no secret fields, but there is no per-dtype schema to
				// scope the flag to, so the whole attribute is masked as the pragmatic fix.
				Sensitive:   true,
				Description: "JSON document of device attributes. Must include \"dtype\": DISK, NIC, CDROM, DISPLAY, PCI, RAW, or USB.",
				PlanModifiers: []planmodifier.String{
					keepAttributesIfConfigMatchesState{},
				},
			},
			"attributes_secrets_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only JSON object of secret attributes (e.g. a DISPLAY device's " +
					"password) merged over attributes when sending to TrueNAS. Never stored in state, and " +
					"not read back on refresh. Requires attributes_secrets_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("attributes_secrets_wo_version")),
				},
			},
			"attributes_secrets_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for attributes_secrets_wo. Bump to re-send a changed " +
					"write-only secret overlay (a write-only value is absent from state). Required when " +
					"attributes_secrets_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("attributes_secrets_wo")),
				},
			},
			"order": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Boot/attach order.",
			},
		},
	}
}
