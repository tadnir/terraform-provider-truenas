// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import "github.com/hashicorp/terraform-plugin-framework/types"

// VMModel is the Terraform state model for truenas_vm.
type VMModel struct {
	ID              types.Int64  `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Memory          types.Int64  `tfsdk:"memory"`     // bytes
	MinMemory       types.Int64  `tfsdk:"min_memory"` // 0 = unset
	VCPUs           types.Int64  `tfsdk:"vcpus"`
	Cores           types.Int64  `tfsdk:"cores"`
	Threads         types.Int64  `tfsdk:"threads"`
	Bootloader      types.String `tfsdk:"bootloader"` // UEFI, UEFI_CSM
	Autostart       types.Bool   `tfsdk:"autostart"`
	Time            types.String `tfsdk:"time"` // LOCAL, UTC
	ShutdownTimeout types.Int64  `tfsdk:"shutdown_timeout"`
	CPUMode         types.String `tfsdk:"cpu_mode"`  // CUSTOM, HOST-MODEL, HOST-PASSTHROUGH
	CPUModel        types.String `tfsdk:"cpu_model"` // "" = unset
	Running         types.Bool   `tfsdk:"running"`

	// Hardware / boot / CPU options (coverage audit). Plain VM settings (not source-
	// aware); each is Optional+Computed and read back directly from vm.query.
	MachineType                types.String `tfsdk:"machine_type"`    // e.g. q35, i440fx
	ArchType                   types.String `tfsdk:"arch_type"`       // CPU architecture
	BootloaderOVMF             types.String `tfsdk:"bootloader_ovmf"` // OVMF firmware image
	CommandLineArgs            types.String `tfsdk:"command_line_args"`
	CPUSet                     types.String `tfsdk:"cpuset"`  // pinned physical CPUs, e.g. "0-3"
	NodeSet                    types.String `tfsdk:"nodeset"` // pinned NUMA nodes
	EnableSecureBoot           types.Bool   `tfsdk:"enable_secure_boot"`
	TrustedPlatformModule      types.Bool   `tfsdk:"trusted_platform_module"`
	PinVCPUs                   types.Bool   `tfsdk:"pin_vcpus"`
	HideFromMSR                types.Bool   `tfsdk:"hide_from_msr"`
	HypervEnlightenments       types.Bool   `tfsdk:"hyperv_enlightenments"`
	EnableCPUTopologyExtension types.Bool   `tfsdk:"enable_cpu_topology_extension"`
	SuspendOnSnapshot          types.Bool   `tfsdk:"suspend_on_snapshot"`

	// Computed only
	Status types.String `tfsdk:"status"` // RUNNING, STOPPED
}

// vmStatusAPI is the JSON wire format for the nested "status" object returned
// by vm.get_instance / vm.query.
type vmStatusAPI struct {
	State string `json:"state"`
}

// vmAPI is the JSON wire format for a TrueNAS VM object.
type vmAPI struct {
	ID              int64       `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Memory          int64       `json:"memory"`
	MinMemory       *int64      `json:"min_memory"`
	VCPUs           int64       `json:"vcpus"`
	Cores           int64       `json:"cores"`
	Threads         int64       `json:"threads"`
	Bootloader      string      `json:"bootloader"`
	Autostart       bool        `json:"autostart"`
	Time            string      `json:"time"`
	ShutdownTimeout int64       `json:"shutdown_timeout"`
	CPUMode         string      `json:"cpu_mode"`
	CPUModel        *string     `json:"cpu_model"`
	Status          vmStatusAPI `json:"status"`

	// Hardware / boot / CPU options (coverage audit).
	MachineType                *string `json:"machine_type"`
	ArchType                   *string `json:"arch_type"`
	BootloaderOVMF             *string `json:"bootloader_ovmf"`
	CommandLineArgs            string  `json:"command_line_args"`
	CPUSet                     *string `json:"cpuset"`
	NodeSet                    *string `json:"nodeset"`
	EnableSecureBoot           bool    `json:"enable_secure_boot"`
	TrustedPlatformModule      bool    `json:"trusted_platform_module"`
	PinVCPUs                   bool    `json:"pin_vcpus"`
	HideFromMSR                bool    `json:"hide_from_msr"`
	HypervEnlightenments       bool    `json:"hyperv_enlightenments"`
	EnableCPUTopologyExtension bool    `json:"enable_cpu_topology_extension"`
	SuspendOnSnapshot          bool    `json:"suspend_on_snapshot"`
}

// strOrNull maps a nullable API string to a Terraform string (nil -> null).
func strOrNull(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// responseToModel maps a vmAPI struct into a VMModel. MinMemory nil maps to 0;
// CPUModel nil maps to "". Running is derived from Status.State == "RUNNING".
func responseToModel(api *vmAPI, m *VMModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(api.Description)
	m.Memory = types.Int64Value(api.Memory)
	if api.MinMemory != nil {
		m.MinMemory = types.Int64Value(*api.MinMemory)
	} else {
		m.MinMemory = types.Int64Value(0)
	}
	m.VCPUs = types.Int64Value(api.VCPUs)
	m.Cores = types.Int64Value(api.Cores)
	m.Threads = types.Int64Value(api.Threads)
	m.Bootloader = types.StringValue(api.Bootloader)
	m.Autostart = types.BoolValue(api.Autostart)
	m.Time = types.StringValue(api.Time)
	m.ShutdownTimeout = types.Int64Value(api.ShutdownTimeout)
	m.CPUMode = types.StringValue(api.CPUMode)
	if api.CPUModel != nil {
		m.CPUModel = types.StringValue(*api.CPUModel)
	} else {
		m.CPUModel = types.StringValue("")
	}
	m.Status = types.StringValue(api.Status.State)
	m.Running = types.BoolValue(api.Status.State == "RUNNING")

	// Hardware / boot / CPU options (coverage audit).
	m.MachineType = strOrNull(api.MachineType)
	m.ArchType = strOrNull(api.ArchType)
	m.BootloaderOVMF = strOrNull(api.BootloaderOVMF)
	m.CommandLineArgs = types.StringValue(api.CommandLineArgs)
	m.CPUSet = strOrNull(api.CPUSet)
	m.NodeSet = strOrNull(api.NodeSet)
	m.EnableSecureBoot = types.BoolValue(api.EnableSecureBoot)
	m.TrustedPlatformModule = types.BoolValue(api.TrustedPlatformModule)
	m.PinVCPUs = types.BoolValue(api.PinVCPUs)
	m.HideFromMSR = types.BoolValue(api.HideFromMSR)
	m.HypervEnlightenments = types.BoolValue(api.HypervEnlightenments)
	m.EnableCPUTopologyExtension = types.BoolValue(api.EnableCPUTopologyExtension)
	m.SuspendOnSnapshot = types.BoolValue(api.SuspendOnSnapshot)
}

// apiPayload builds the map[string]any payload for vm.create / vm.update.
// Optional fields are only included when set (non-null, non-unknown), per the
// TrueNAS API contract (e.g. vcpus: 0 must never be sent explicitly).
// vmCreateOnlyFields are accepted by vm.create but rejected by vm.update
// ("Extra inputs are not permitted"). They are dropped from the update payload
// and marked RequiresReplace in the schema so a change recreates the VM.
var vmCreateOnlyFields = []string{"bootloader_ovmf", "enable_secure_boot"}

// apiUpdatePayload is apiPayload with the create-only fields removed, for
// vm.update (which rejects them).
func (m *VMModel) apiUpdatePayload() map[string]any {
	p := m.apiPayload()
	for _, k := range vmCreateOnlyFields {
		delete(p, k)
	}
	return p
}

func (m *VMModel) apiPayload() map[string]any {
	p := map[string]any{
		"name":   m.Name.ValueString(),
		"memory": m.Memory.ValueInt64(),
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.VCPUs.IsNull() && !m.VCPUs.IsUnknown() {
		p["vcpus"] = m.VCPUs.ValueInt64()
	}
	if !m.Cores.IsNull() && !m.Cores.IsUnknown() {
		p["cores"] = m.Cores.ValueInt64()
	}
	if !m.Threads.IsNull() && !m.Threads.IsUnknown() {
		p["threads"] = m.Threads.ValueInt64()
	}
	if !m.Bootloader.IsNull() && !m.Bootloader.IsUnknown() {
		p["bootloader"] = m.Bootloader.ValueString()
	}
	if !m.Autostart.IsNull() && !m.Autostart.IsUnknown() {
		p["autostart"] = m.Autostart.ValueBool()
	}
	if !m.Time.IsNull() && !m.Time.IsUnknown() {
		p["time"] = m.Time.ValueString()
	}
	if !m.ShutdownTimeout.IsNull() && !m.ShutdownTimeout.IsUnknown() {
		p["shutdown_timeout"] = m.ShutdownTimeout.ValueInt64()
	}
	if !m.CPUMode.IsNull() && !m.CPUMode.IsUnknown() {
		p["cpu_mode"] = m.CPUMode.ValueString()
	}
	if !m.CPUModel.IsNull() && !m.CPUModel.IsUnknown() && m.CPUModel.ValueString() != "" {
		p["cpu_model"] = m.CPUModel.ValueString()
	}
	if !m.MinMemory.IsNull() && !m.MinMemory.IsUnknown() && m.MinMemory.ValueInt64() != 0 {
		p["min_memory"] = m.MinMemory.ValueInt64()
	}

	// Hardware / boot / CPU options (coverage audit). Strings sent when non-empty;
	// bools sent when set (Computed reads the server default back otherwise).
	putVMStr(p, "machine_type", m.MachineType)
	putVMStr(p, "arch_type", m.ArchType)
	putVMStr(p, "bootloader_ovmf", m.BootloaderOVMF)
	putVMStr(p, "command_line_args", m.CommandLineArgs)
	putVMStr(p, "cpuset", m.CPUSet)
	putVMStr(p, "nodeset", m.NodeSet)
	putVMBool(p, "enable_secure_boot", m.EnableSecureBoot)
	putVMBool(p, "trusted_platform_module", m.TrustedPlatformModule)
	putVMBool(p, "pin_vcpus", m.PinVCPUs)
	putVMBool(p, "hide_from_msr", m.HideFromMSR)
	putVMBool(p, "hyperv_enlightenments", m.HypervEnlightenments)
	putVMBool(p, "enable_cpu_topology_extension", m.EnableCPUTopologyExtension)
	putVMBool(p, "suspend_on_snapshot", m.SuspendOnSnapshot)
	return p
}

// putVMStr adds a string field when set and non-empty.
func putVMStr(p map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		p[key] = v.ValueString()
	}
}

// putVMBool adds a bool field when set.
func putVMBool(p map[string]any, key string, v types.Bool) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueBool()
	}
}
