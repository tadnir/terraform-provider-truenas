# Run a cloud backup task now (Terraform 1.14+).
# id is the numeric task id — reference the managing resource rather than
# hardcoding it (or discover it with `midclt call cloud_backup.query`).
action "truenas_cloud_backup_run" "example" {
  config {
    id = truenas_cloud_backup.offsite.id
    # A first backup can take hours: start it and return.
    wait = false
  }
}
