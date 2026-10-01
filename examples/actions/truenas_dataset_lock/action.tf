# Lock an encrypted dataset (unmounts it and unloads its key).
action "truenas_dataset_lock" "secret" {
  config {
    dataset      = truenas_dataset.secret.name
    force_umount = true
  }
}
