// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	pathRegexp = regexp.MustCompile(`^/mnt/[^/]+/.+[^/]$`)
	modeRegexp = regexp.MustCompile(`^0?[0-7]{3}$`)
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a file on a TrueNAS pool, written with filesystem.put: for example a script, a " +
			"configuration file, or a cloud-init seed image for a VM's CD-ROM device. The content is given " +
			"base64-encoded, so binary files work too. TrueNAS has no API to delete a single file, so " +
			"destroying this resource only removes it from state and leaves the file in place (it goes when " +
			"its dataset does).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The file's path.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required: true,
				Description: "Absolute path of the file, under /mnt/<pool>/. Missing parent directories are " +
					"created. Changing it writes a new file (the old one is left in place).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(pathRegexp, "must be a file path under /mnt/<pool>/"),
				},
			},
			"content_base64": schema.StringAttribute{
				Optional: true,
				Description: "The file's content, base64-encoded (e.g. filebase64(\"seed.iso\") or " +
					"base64encode(templatefile(...))). Stored in state; use content_base64_wo for content that " +
					"must stay out of it. Exactly one of content_base64 or content_base64_wo must be set. " +
					"Changing it rewrites the file.",
			},
			"content_base64_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only alternative to content_base64: read from configuration and never " +
					"stored in state. Requires content_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("content_wo_version")),
				},
			},
			"content_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for content_base64_wo. Change it to rewrite the file with the " +
					"current content_base64_wo (a write-only value is absent from state, so its change can't be " +
					"detected). Required when content_base64_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("content_base64_wo")),
				},
			},
			"mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("0644"),
				Description: "UNIX permission bits in octal, e.g. \"0644\" (the default) or \"0755\". Read back " +
					"from filesystem.stat.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(modeRegexp, "must be 3 or 4 octal digits, e.g. \"0644\" or \"644\""),
				},
			},
		},
	}
}
