// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Creates a writable ZFS clone of a snapshot (pool.snapshot.clone). " +
			"The clone is a new dataset or zvol that shares blocks with the snapshot " +
			"until either is written to, so it is created instantly and costs no space " +
			"up front. Destroying the resource destroys the clone, not the snapshot. " +
			"The snapshot cannot be destroyed while the clone exists unless it is " +
			"destroyed deferred.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The clone's dataset name (same as dataset).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"snapshot": schema.StringAttribute{
				Required:    true,
				Description: "Full name of the snapshot to clone, e.g. tank/golden@v1.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset": schema.StringAttribute{
				Required: true,
				Description: "Name of the dataset or zvol to create, e.g. tank/vms/test1. " +
					"It must be in the same pool as the snapshot and must not exist yet.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset_properties": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "ZFS properties to set on the clone when it is created, as " +
					"property name to value (e.g. { compression = \"lz4\" }). Write-only: " +
					"not read back, so later changes made outside Terraform are not " +
					"detected. Changing it re-creates the clone.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "FILESYSTEM or VOLUME, following the cloned snapshot's dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool containing the clone.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mountpoint": schema.StringAttribute{
				Computed:    true,
				Description: "The clone's mountpoint (empty for a zvol).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
