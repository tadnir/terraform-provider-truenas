// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// TestAccDataset_specialSmallBlockSize exercises special_small_block_size
// over a full lifecycle: set on create, changed in place, imported, and
// then reverted to inherited by writing "inherit" (lower-case, to check the
// configured spelling survives the read, which reports INHERIT). The last
// step drops the attribute from the configuration to pin the documented
// behaviour - an Optional+Computed attribute that is removed from config
// keeps its last applied value, so the step must plan empty.
//
// The values are written as HCL numbers, which Terraform converts to the
// attribute's string type.
//
// 16384 and 32768 are both powers of two below the 128K default record
// size, which is what ZFS requires of special_small_blocks.
func TestAccDataset_specialSmallBlockSize(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-ssbs"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetSSBSConfig(name, "16384"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "special_small_block_size", "16384"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccDatasetSSBSConfig(name, "32768"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "special_small_block_size", "32768"),
				),
			},
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: acctest.ProviderConfig() + testAccDatasetSSBSConfig(name, `"inherit"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "special_small_block_size", "inherit"),
				),
			},
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}
`, name),
				PlanOnly: true,
			},
		},
	})
}

// TestAccDataset_specialSmallBlockSizeInherited is the live counterpart to
// TestDatasetResponseToModelInheritsSpecialSmallBlockSize: a dataset that
// never sets the property must read back as INHERIT, not as the effective
// value get_instance reports, and must therefore plan empty on a second
// run.
func TestAccDataset_specialSmallBlockSizeInherited(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-inh"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "inherited ssbs"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "special_small_block_size", "INHERIT"),
				),
			},
			{
				Config:   acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "inherited ssbs"),
				PlanOnly: true,
			},
		},
	})
}

func testAccDatasetSSBSConfig(name, size string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name                     = %q
  special_small_block_size = %s
}
`, name, size)
}

// testAccDatasetLocalProperty runs one source-aware property (see
// localString and friends) through the lifecycle every such property has to
// survive: set on create, changed in place, imported, and then dropped from
// the configuration, which must plan empty because an Optional+Computed
// attribute keeps its last applied value. A second dataset in the same
// configuration never sets the property and must read back with it null,
// which is the live counterpart to the NullWhenNotLocal unit tests.
//
// first and second are HCL literals; firstState and secondState are what
// state must then hold. extra is any further HCL the tested dataset needs
// for the property to be valid, e.g. an acltype; the plain dataset does not
// get it.
func testAccDatasetLocalProperty(t *testing.T, attr, extra, first, firstState, second, secondState string) {
	prefix := "tf-acc-ds-" + strings.ReplaceAll(attr, "_", "-")
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName(prefix))
	plain := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName(prefix+"-inh"))

	config := func(value string) string {
		set := ""
		if value != "" {
			set = fmt.Sprintf("  %s = %s\n", attr, value)
		}
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
%s%s}

resource "truenas_dataset" "plain" {
  name = %q
}
`, name, extra, set, plain)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckDatasetDestroyed(name),
			testAccCheckDatasetDestroyed(plain),
		),
		Steps: []resource.TestStep{
			{
				Config: config(first),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", attr, firstState),
					resource.TestCheckNoResourceAttr("truenas_dataset.plain", attr),
				),
			},
			{
				Config: config(second),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", attr, secondState),
					resource.TestCheckNoResourceAttr("truenas_dataset.plain", attr),
				),
			},
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   config(""),
				PlanOnly: true,
			},
		},
	})
}

// TestAccDataset_atime: see testAccDatasetLocalProperty.
func TestAccDataset_atime(t *testing.T) {
	testAccDatasetLocalProperty(t, "atime", "", `"off"`, "off", `"on"`, "on")
}

// TestAccDataset_dedup: see testAccDatasetLocalProperty.
// The test dataset holds no data, so turning deduplication on builds no
// dedup table.
func TestAccDataset_dedup(t *testing.T) {
	testAccDatasetLocalProperty(t, "dedup", "", `"off"`, "off", `"on"`, "on")
}

// TestAccDataset_readonly: see testAccDatasetLocalProperty.
func TestAccDataset_readonly(t *testing.T) {
	testAccDatasetLocalProperty(t, "readonly", "", `"on"`, "on", `"off"`, "off")
}

// TestAccDataset_snapdir: see testAccDatasetLocalProperty.
func TestAccDataset_snapdir(t *testing.T) {
	testAccDatasetLocalProperty(t, "snapdir", "", `"visible"`, "visible", `"hidden"`, "hidden")
}

func testAccDatasetConfig(name, compression, comments string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = %q
  comments    = %q
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
		// CallRead, not Call: acctest.Client() is a process-wide singleton
		// connected once, and it sits idle for the whole of a test's
		// Terraform steps. With more than one acceptance test in this
		// package that idle stretch is long enough for the connection to
		// drop, and Call does not re-dial - a dead connection fails the
		// destroy check with "not connected" even though the dataset really
		// is gone. CallRead retries transient failures and reconnects
		// between attempts, which is what acctest.RestoreCall already
		// documents for the same reason. pool.dataset.query is a read, so
		// retrying it is safe.
		raw, err := c.CallRead(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
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
