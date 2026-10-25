# A small configuration file, kept in state.
resource "truenas_filesystem_file" "config" {
  path           = "/mnt/tank/apps/example/config.yaml"
  content_base64 = base64encode(templatefile("${path.module}/config.yaml.tftpl", { port = 8080 }))
  mode           = "0640"
}

# A file holding a secret: the content is write-only and never stored in
# state. Bump content_wo_version to rewrite it.
resource "truenas_filesystem_file" "seed" {
  path               = "/mnt/tank/vms/example/seed.iso"
  content_base64_wo  = filebase64(var.seed_iso)
  content_wo_version = 1
}
