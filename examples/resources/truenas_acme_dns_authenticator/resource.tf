resource "truenas_acme_dns_authenticator" "route53" {
  name = "route53-dns01"

  # "attributes" is a JSON document; its shape depends on the
  # "authenticator" value. See the resource description for all five
  # supported variants (cloudflare, digitalocean, OVH, route53, shell).
  attributes = jsonencode({
    authenticator     = "route53"
    access_key_id     = var.route53_access_key_id
    secret_access_key = var.route53_secret_access_key
  })
}

variable "route53_access_key_id" {
  type      = string
  sensitive = true
}

variable "route53_secret_access_key" {
  type      = string
  sensitive = true
}

# With Terraform >= 1.11 the credentials can stay out of plan and state:
# only the non-secret keys go in "attributes", the rest in the write-only
# "attributes_secrets_wo". Bump the version to send a new token.
resource "truenas_acme_dns_authenticator" "cloudflare" {
  name       = "cloudflare-dns01"
  attributes = jsonencode({ authenticator = "cloudflare" })

  attributes_secrets_wo         = jsonencode({ api_token = var.cloudflare_api_token })
  attributes_secrets_wo_version = 1
}

variable "cloudflare_api_token" {
  type      = string
  sensitive = true
  ephemeral = true
}
