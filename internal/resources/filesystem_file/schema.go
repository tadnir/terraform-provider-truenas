// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	pathRegexp = regexp.MustCompile(`^/mnt/[^/]+/.+[^/]$`)
	modeRegexp = regexp.MustCompile(`^0?[0-7]{3}$`)
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a file on a TrueNAS pool: for example a configuration file, a script, or a " +
			"cloud-init seed image for a VM's CD-ROM device. The content comes from one of `content` (text), " +
			"`content_base64` (binary) or `source` (a file on the machine running Terraform), and is written " +
			"with filesystem.put. With none of them, the resource manages a file that already exists on the " +
			"NAS (for example one a Cloud Sync task downloaded) and only fails when it is missing. Content is " +
			"never stored in state: `content` and `content_base64` are write-only, and `sha256` records what " +
			"was written. TrueNAS has no API to delete a single file, so destroying this resource only " +
			"removes it from state and leaves the file in place (it goes when its dataset does).",
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
			"content": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "The file's content as text, written as is (e.g. templatefile(...)). Write-only: " +
					"never stored in state; a change shows as a change of sha256. At most one of content, " +
					"content_base64 and source.",
			},
			"content_base64": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "The file's content, base64-encoded, for binary content built in configuration. " +
					"Write-only, like content. At most one of content, content_base64 and source.",
			},
			"source": schema.StringAttribute{
				Optional: true,
				Description: "Path of a file on the machine running Terraform, streamed to the NAS as is (it " +
					"is not read into memory, so large files work). It is hashed at plan time; a file that " +
					"doesn't exist yet at plan time (made earlier in the same apply) is hashed at apply. The " +
					"path is stored in state, the content is not. At most one of content, content_base64 and " +
					"source.",
			},
			"sha256": schema.StringAttribute{
				Computed: true,
				Description: "Hex SHA-256 of the content written, computed from configuration at plan time; " +
					"a change rewrites the file. Null for a file that already existed on the NAS. TrueNAS can't " +
					"hash a file, so a change on the NAS is detected by its size and modification time: the " +
					"next refresh sets this to \"\", and the next apply writes the file again.",
			},
			"mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "UNIX permission bits in octal, e.g. \"0644\" or \"0755\". When unset, a written " +
					"file gets 0644 and an existing file keeps its mode. Read back from filesystem.stat.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(modeRegexp, "must be 3 or 4 octal digits, e.g. \"0644\" or \"644\""),
				},
			},
		},
	}
}
