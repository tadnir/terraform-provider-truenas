# Start a VM (Terraform 1.14+). Does nothing when it is already running.
action "truenas_vm_start" "example" {
  config {
    vm_id = truenas_vm.example.id
  }
}

# Start a new VM once its last device exists: truenas_vm's `running = true`
# would start it on create, before any device is attached.
resource "truenas_vm_device" "nic" {
  vm = truenas_vm.example.id
  attributes = jsonencode({
    dtype      = "NIC"
    type       = "VIRTIO"
    nic_attach = "br0"
  })

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.truenas_vm_start.example]
    }
  }
}
