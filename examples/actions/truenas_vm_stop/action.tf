# Stop a VM (Terraform 1.14+) and wait until it is down. Does nothing when it
# is already stopped.
action "truenas_vm_stop" "example" {
  config {
    vm_id = truenas_vm.example.id
  }
}

# Stop a VM before Terraform removes its devices on destroy (Terraform
# 1.16+ for destroy events).
resource "truenas_vm_device" "disk" {
  vm = truenas_vm.example.id
  attributes = jsonencode({
    dtype = "DISK"
    path  = "/dev/zvol/tank/vms/example"
    type  = "VIRTIO"
  })

  lifecycle {
    action_trigger {
      events  = [before_destroy]
      actions = [action.truenas_vm_stop.example]
    }
  }
}
