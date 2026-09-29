// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// zfsEnumAttr builds an Optional+Computed string attribute for a source-aware
// ZFS enum property (coverage audit): uppercase values, reads back null when inherited/
// default, and cannot be reverted to inherited by removing it from config.
func zfsEnumAttr(desc string, values ...string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		Validators:    []validator.String{stringvalidator.OneOf(values...)},
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS volume (zvol/block device) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Zvol name (used as Terraform ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full zvol path, e.g. tank/myvol.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"volsize": schema.Int64Attribute{
				Required:    true,
				Description: "Volume size in bytes.",
			},
			"volblocksize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Block size in bytes (512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072). Set at create time only.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			"compression": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Compression algorithm. Case-insensitive: lz4, zstd, off, etc.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sync": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Sync setting: standard, always, or disabled.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dedup": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Deduplication: off, on, or verify.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sparse": schema.BoolAttribute{
				Optional:    true,
				Description: "Sparse provisioning (write-only; not returned by API).",
			},
			"comments": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// --- Source-aware ZFS tuning properties applicable to volumes (coverage audit) ---
			// Optional+Computed; read back null when inherited/default. Reverting a
			// locally-set value to inherited cannot be done by removing it from
			// config — change it out of band and refresh.
			"checksum": zfsEnumAttr("Checksum algorithm: ON, OFF, FLETCHER2, FLETCHER4, SHA256, SHA512, SKEIN, EDONR, or BLAKE3. Null inherits.",
				"ON", "OFF", "FLETCHER2", "FLETCHER4", "SHA256", "SHA512", "SKEIN", "EDONR", "BLAKE3"),
			"readonly": zfsEnumAttr("Mount read-only: ON or OFF. Null inherits.", "ON", "OFF"),
			"snapdev": zfsEnumAttr("Visibility of the volume's snapshot device nodes: VISIBLE or HIDDEN. Null inherits.",
				"VISIBLE", "HIDDEN"),
			"copies": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Number of copies of each block (1-3). Null (unset) inherits from the parent.",
				Validators:    []validator.Int64{int64validator.Between(1, 3)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"reservation": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Reserved space in bytes (guaranteed to this volume including snapshots). Null (unset) inherits.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"refreservation": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Referenced reservation in bytes (space guaranteed to the volume, excluding snapshots). Null (unset) inherits.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool containing this zvol.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the zvol is encrypted.",
			},
		},
	}
}
