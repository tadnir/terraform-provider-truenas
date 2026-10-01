// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCloudsyncCredentials_basic tests create, update, and import of
// cloud sync credentials. The provider_config credentials are not validated
// against a real STORJ_IX endpoint at create time.
func TestAccCloudsyncCredentials_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-creds")
	renamedName := name + "-renamed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		// CheckDestroy runs against the last-applied config, in which the
		// resource has already been renamed, so it must check for the
		// renamed name rather than the original one.
		CheckDestroy: testAccCheckCloudsyncCredentialsDestroyed(renamedName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsConfig(name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloudsync_credentials.test", "id"),
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "name", name),
				),
			},
			// Update the name in place.
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsConfig(renamedName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "name", renamedName),
				),
			},
			{
				ResourceName:            "truenas_cloudsync_credentials.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"provider_config"},
			},
		},
	})
}

func testAccCloudsyncCredentialsConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name = %q
  provider_config = jsonencode({
    type              = "STORJ_IX"
    access_key_id     = "tfacc"
    secret_access_key = "tfacc"
  })
}
`, name)
}

func testAccCheckCloudsyncCredentialsDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "cloudsync.credentials.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking cloudsync credentials %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing cloudsync.credentials.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("cloudsync credentials %s still exists", name)
		}
		return nil
	}
}

// TestAccCloudsyncCredentials_writeOnlySecrets creates credentials whose
// access keys come from provider_secrets_wo, checks TrueNAS received them
// while state holds only provider_config, then bumps the version to resend a
// new secret.
func TestAccCloudsyncCredentials_writeOnlySecrets(t *testing.T) {
	name := acctest.RandName("tf-acc-creds-wo")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudsyncCredentialsDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsWOConfig(name, "tfacc-secret-1", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_cloudsync_credentials.test", "provider_secrets_wo"),
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "provider_secrets_wo_version", "1"),
					testAccCheckCloudsyncCredentialsSecret(name, "tfacc-secret-1"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsWOConfig(name, "tfacc-secret-2", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "provider_secrets_wo_version", "2"),
					testAccCheckCloudsyncCredentialsSecret(name, "tfacc-secret-2"),
				),
			},
		},
	})
}

func testAccCloudsyncCredentialsWOConfig(name, secret string, version int) string {
	return fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name            = %q
  provider_config = jsonencode({ type = "STORJ_IX" })
  provider_secrets_wo = jsonencode({
    access_key_id     = "tfacc"
    secret_access_key = %q
  })
  provider_secrets_wo_version = %d
}
`, name, secret, version)
}

// testAccCheckCloudsyncCredentialsSecret verifies TrueNAS holds the given
// secret_access_key for the named credentials.
func testAccCheckCloudsyncCredentialsSecret(name, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "cloudsync.credentials.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("querying cloudsync credentials %s: %w", name, err)
		}
		var results []struct {
			Provider map[string]any `json:"provider"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("parsing cloudsync.credentials.query: %w", err)
		}
		if len(results) != 1 {
			return fmt.Errorf("want 1 cloudsync credentials named %s, got %d", name, len(results))
		}
		if got := results[0].Provider["secret_access_key"]; got != want {
			return fmt.Errorf("secret_access_key = %v, want %s", got, want)
		}
		return nil
	}
}
