// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages TrueNAS cloud sync credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric cloud sync credentials ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the cloud sync credentials.",
			},
			"provider_config": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON document of provider settings. Must include \"type\" (e.g. S3, B2, GOOGLE_CLOUD_STORAGE, STORJ_IX).",
			},
			"provider_secrets_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Write-only JSON object of provider settings merged over provider_config when " +
					"sending it, for the secret ones (e.g. {\"access_key_id\": ..., \"secret_access_key\": ...}), " +
					"so they stay out of plan and state. Requires Terraform >= 1.11 and " +
					"provider_secrets_wo_version. Sent on every create and update; bump " +
					"provider_secrets_wo_version to send a changed value.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("provider_secrets_wo_version")),
				},
			},
			"provider_secrets_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Any number; changing it makes Terraform update the credential and so resend " +
					"provider_secrets_wo. While set, a drifted provider_config is read back with only the " +
					"keys it already had, so the secret keys never reach state.",
			},
		},
	}
}
