// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccAcmeDnsAuthenticator_basic exercises the full Tier 1 contract for
// a cloudflare-variant ACME DNS authenticator. The cloudflare variant (with
// a syntactically valid but fake api_token) is used rather than shell:
// probed live, acme.dns.authenticator.create does not validate credentials
// against the actual DNS provider for any variant, but shell uniquely
// performs a local filesystem check ("script" must be an existing file
// under a pool mount point) that a value-only test has no easy way to
// satisfy portably.
func TestAccAcmeDnsAuthenticator_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-acme-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAcmeDnsAuthenticatorConfig(name, "tf-acc-dummy-token-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_acme_dns_authenticator.test", "id"),
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "name", name),
				),
			},
			// Update in place: change the token (a full replace under the
			// hood via acme.dns.authenticator.update, confirmed live).
			{
				Config: acctest.ProviderConfig() + testAccAcmeDnsAuthenticatorConfig(name, "tf-acc-dummy-token-2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "name", name),
				),
			},
			{
				ResourceName:            "truenas_acme_dns_authenticator.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
		},
	})
}

func testAccAcmeDnsAuthenticatorConfig(name, token string) string {
	return fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name = %q
  attributes = jsonencode({
    authenticator = "cloudflare"
    api_token     = %q
  })
}
`, name, token)
}

func testAccCheckAcmeDnsAuthenticatorDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "acme.dns.authenticator.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking ACME DNS authenticator %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing acme.dns.authenticator.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("ACME DNS authenticator %s still exists", name)
		}
		return nil
	}
}

// TestAccAcmeDnsAuthenticator_writeOnlySecrets creates a cloudflare
// authenticator whose api_token comes from attributes_secrets_wo, checks
// TrueNAS received it while state holds only attributes, then bumps the
// version to resend a new token.
func TestAccAcmeDnsAuthenticator_writeOnlySecrets(t *testing.T) {
	name := acctest.RandName("tf-acc-acme-dns-wo")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAcmeDnsAuthenticatorDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAcmeDnsAuthenticatorWOConfig(name, "tf-acc-dummy-token-1", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "attributes", `{"authenticator":"cloudflare"}`),
					resource.TestCheckNoResourceAttr("truenas_acme_dns_authenticator.test", "attributes_secrets_wo"),
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "attributes_secrets_wo_version", "1"),
					testAccCheckAcmeDnsAuthenticatorToken(name, "tf-acc-dummy-token-1"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAcmeDnsAuthenticatorWOConfig(name, "tf-acc-dummy-token-2", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_acme_dns_authenticator.test", "attributes", `{"authenticator":"cloudflare"}`),
					testAccCheckAcmeDnsAuthenticatorToken(name, "tf-acc-dummy-token-2"),
				),
			},
		},
	})
}

func testAccAcmeDnsAuthenticatorWOConfig(name, token string, version int) string {
	return fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "test" {
  name                          = %q
  attributes                    = jsonencode({ authenticator = "cloudflare" })
  attributes_secrets_wo         = jsonencode({ api_token = %q })
  attributes_secrets_wo_version = %d
}
`, name, token, version)
}

// testAccCheckAcmeDnsAuthenticatorToken verifies TrueNAS holds the given
// api_token for the named authenticator.
func testAccCheckAcmeDnsAuthenticatorToken(name, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().CallRead(context.Background(), "acme.dns.authenticator.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("querying ACME DNS authenticator %s: %w", name, err)
		}
		var results []struct {
			Attributes map[string]any `json:"attributes"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("parsing acme.dns.authenticator.query: %w", err)
		}
		if len(results) != 1 {
			return fmt.Errorf("want 1 ACME DNS authenticator named %s, got %d", name, len(results))
		}
		if got := results[0].Attributes["api_token"]; got != want {
			return fmt.Errorf("api_token = %v, want %s", got, want)
		}
		return nil
	}
}
