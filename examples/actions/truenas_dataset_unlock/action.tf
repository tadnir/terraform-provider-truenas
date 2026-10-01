# Unlock a passphrase-encrypted dataset. The passphrase is used transiently
# and is not stored in state. Use "key" instead for a key-encrypted dataset.
variable "dataset_passphrase" {
  type      = string
  sensitive = true
}

action "truenas_dataset_unlock" "secret" {
  config {
    dataset    = truenas_dataset.secret.name
    passphrase = var.dataset_passphrase

    # recursive          = true  # also unlock descendants sharing the key
    # toggle_attachments = true  # start services/shares that depend on it
  }
}
