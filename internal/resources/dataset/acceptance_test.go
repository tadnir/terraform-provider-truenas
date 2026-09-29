// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDataset_basic creates a ZFS dataset, checks its computed
// attributes, updates a mutable field (comments), imports it by name, and
// verifies destruction.
func TestAccDataset_basic(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", name),
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "lz4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "initial comment"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "aclmode", "PASSTHROUGH"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "acltype", "nfsv4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "atime", "OFF"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "checksum", "SHA256"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "copies", "2"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "dedup", "ON"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "quota", "2147483648"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "recordsize", "128K"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "sync", "ALWAYS"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "mountpoint"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "id"),
				),
			},
			// Update a mutable field in place (comments); compression left
			// unchanged to keep this a pure in-place-update step.
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "updated comment"),
				),
			},
			// Import by dataset name.
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetConfig(name, compression, comments string) string {
	// Full-surface: every writable ZFS property this resource models is set to a
	// non-default value so the post-apply plan (and ImportStateVerify) prove each
	// one round-trips. share_type is write-only (not read back) and left out.
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = %q
  comments    = %q

  aclmode                  = "PASSTHROUGH"
  acltype                  = "nfsv4"
  atime                    = "OFF"
  exec                     = "OFF"
  checksum                 = "SHA256"
  copies                   = 2
  dedup                    = "ON"
  quota                    = 2147483648
  refquota                 = 1073741824
  reservation              = 10485760
  refreservation           = 10485760
  recordsize               = "128K"
  snapdir                  = "VISIBLE"
  special_small_block_size = 0
  sync                     = "ALWAYS"
}
`, name, compression, comments)
}

// datasetSummary is the subset of pool.dataset.query fields this test
// package needs directly (outside of the provider's own resource code).
type datasetSummary struct {
	ID string `json:"id"`
}

func testAccCheckDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []datasetSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

// TestAccDataset_identityAfterUpdate verifies the resource populates its
// identity after an in-place update (regression for issue #20): a resource that
// declares an identity schema but omits SetIdentity in Update fails every
// update with "no resource identity data after update". Gated to Terraform
// 1.12+, where resource identity exists.
func TestAccDataset_identityAfterUpdate(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-ident"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "identity initial"),
			},
			{
				// In-place update; assert the identity's id matches state id.
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "identity updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValueMatchesState("truenas_dataset.test", tfjsonpath.New("id")),
				},
			},
		},
	})
}

// TestAccDataset_encryptedPassphrase creates a passphrase-encrypted dataset and
// verifies the encryption state reads back (issue #18). encryption_passphrase is
// write-only; inherit_encryption / encryption_generate_key are create-only inputs
// the API does not return, so they are ignored on import.
func TestAccDataset_encryptedPassphrase(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-enc"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "enc" {
  name                  = %q
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-256-GCM"
  encryption_passphrase = "test-passphrase-1234"
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encryption", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encryption_algorithm", "AES-256-GCM"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "key_format", "PASSPHRASE"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "locked", "false"),
				),
			},
			{
				ResourceName:            "truenas_dataset.enc",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"encryption_passphrase", "encryption_key", "inherit_encryption", "encryption_generate_key"},
			},
		},
	})
}

// TestAccDataset_encryptedGeneratedKey creates a key-based encrypted dataset
// with a generated key (issue #18), covering encryption_generate_key.
func TestAccDataset_encryptedGeneratedKey(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-enckey"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "enckey" {
  name                    = %q
  encryption              = true
  inherit_encryption      = false
  encryption_algorithm    = "AES-256-GCM"
  encryption_generate_key = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "key_format", "HEX"),
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "locked", "false"),
				),
			},
		},
	})
}
