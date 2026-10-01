# A plain dataset under an existing pool.
resource "truenas_dataset" "data" {
  name        = "tank/data"
  compression = "lz4"
  comments    = "managed by terraform"
}

# A passphrase-encrypted dataset. The encryption inputs are create-only:
# changing any of them recreates the dataset. encryption_passphrase is
# write-only — it is used at create time and never stored in state, so supply
# it from a sensitive variable.
variable "dataset_passphrase" {
  type      = string
  sensitive = true
}

resource "truenas_dataset" "secret" {
  name                  = "tank/secret"
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-256-GCM" # ignored on TrueNAS 27.0+ (fixed server-side)
  encryption_passphrase = var.dataset_passphrase

  # Read back after apply:
  #   encrypted  = true
  #   key_format = "PASSPHRASE"
  #   locked     = false
}

# A key-encrypted dataset with a TrueNAS-generated key (key_format = "HEX").
resource "truenas_dataset" "keyed" {
  name                    = "tank/keyed"
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
}
