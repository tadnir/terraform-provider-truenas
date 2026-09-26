// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS dataset (filesystem or volume) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Dataset name (used as Terraform ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full dataset path, e.g. tank/mydata.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Dataset type: FILESYSTEM (default) or VOLUME. Case-insensitive.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
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
			"acltype": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ACL type: posix, nfsv4, or off. Case-insensitive.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"share_type": schema.StringAttribute{
				Optional:    true,
				Description: "Optimised share type: UNIX or WINDOWS (write-only, not returned by API).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comments": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description stored as org.freenas:description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"quota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Quota in bytes (0 = unlimited).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"refquota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Referenced quota in bytes (0 = unlimited).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"reservation": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Reserved space in bytes.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"volsize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Volume size in bytes. Required for type=VOLUME.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"special_small_block_size": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Threshold in bytes below which blocks are written to a " +
					"pool's special allocation class vdev (ZFS special_small_blocks). " +
					"0 disables the behaviour. Must be 0 or a power of two no larger " +
					"than the dataset's record size. Omit the attribute to leave the " +
					"property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"atime": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether reading a file updates its access time: on or off. " +
					"Case-insensitive. Filesystem datasets only. Omit the attribute to " +
					"leave the property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dedup": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Deduplication: on, verify, or off. Case-insensitive. Named dedup to match truenas_zvol; the API key is deduplication. Omit the attribute to leave the property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"readonly": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the dataset is read-only: on or off. Case-insensitive. Omit the attribute to leave the property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"snapdir": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Visibility of the .zfs/snapshot directory: hidden (reachable but not listed), visible, or disabled. Case-insensitive. Omit the attribute to leave the property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sync": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Synchronous write behaviour: standard, always, or disabled. Case-insensitive. Omit the attribute to leave the property inherited from the parent dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mountpoint": schema.StringAttribute{
				Computed:    true,
				Description: "Dataset mountpoint path.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the dataset is encrypted.",
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool containing this dataset.",
			},
		},
	}
}
