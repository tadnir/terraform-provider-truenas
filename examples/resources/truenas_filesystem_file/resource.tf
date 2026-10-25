# A configuration file from a template. content is write-only: state keeps
# only its sha256.
resource "truenas_filesystem_file" "config" {
  path    = "/mnt/tank/apps/example/config.yaml"
  content = templatefile("${path.module}/config.yaml.tftpl", { port = 8080 })
  mode    = "0640"
}

# Binary content built in configuration.
resource "truenas_filesystem_file" "logo" {
  path           = "/mnt/tank/apps/example/logo.png"
  content_base64 = filebase64("${path.module}/logo.png")
}

# A file on the machine running Terraform, streamed as is (large files work).
resource "truenas_filesystem_file" "seed" {
  path   = "/mnt/tank/vms/example/seed.iso"
  source = "${path.module}/build/seed.iso"
}

# A file that is already on the NAS (here one a Cloud Sync task downloaded):
# no content, so the resource only checks it is there and tracks its mode.
resource "truenas_filesystem_file" "image" {
  path = "/mnt/tank/vms/images/example-disk.raw"
}
