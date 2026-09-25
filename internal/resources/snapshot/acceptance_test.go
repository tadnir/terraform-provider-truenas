// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSnapshot_basic creates a dataset fixture, snapshots it, imports the
// snapshot by "dataset@name", and verifies destruction. truenas_snapshot has
// no Update method (every attribute forces replacement), so there is no
// separate in-place update step; changing the fixture's name attribute would
// force a full replace, which ImportState (relying on a stable ID) cannot
// observe as an "update" — so this test exercises create -> import -> destroy.
func TestAccSnapshot_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-snap-ds"))
	snapName := acctest.RandName("tf-acc-snap")
	snapID := fmt.Sprintf("%s@%s", datasetName, snapName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnapshotDestroyed(snapID),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSnapshotConfig(datasetName, snapName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snapshot.test", "id", snapID),
					resource.TestCheckResourceAttr("truenas_snapshot.test", "dataset", datasetName),
					resource.TestCheckResourceAttr("truenas_snapshot.test", "name", snapName),
					resource.TestCheckResourceAttrSet("truenas_snapshot.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_snapshot.test", "createtxg"),
				),
			},
			// Import by "dataset@snapname".
			{
				ResourceName:      "truenas_snapshot.test",
				ImportState:       true,
				ImportStateVerify: true,
				// recursive and defer_destroy are write-only and not returned
				// by the API, so they cannot be recovered by import.
				ImportStateVerifyIgnore: []string{"recursive", "defer_destroy"},
			},
		},
	})
}

// TestAccSnapshot_deferDestroy turns defer_destroy on for an existing
// snapshot, which must be an in-place change (createtxg unchanged, so the
// snapshot was not re-created), then destroys it with the deferred option.
// With no clones or holds a deferred destroy removes the snapshot at once,
// which the destroy check confirms.
func TestAccSnapshot_deferDestroy(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-snap-ds"))
	snapName := acctest.RandName("tf-acc-snap-defer")
	snapID := fmt.Sprintf("%s@%s", datasetName, snapName)
	var txg string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnapshotDestroyed(snapID),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSnapshotDeferConfig(datasetName, snapName, false),
				Check: resource.TestCheckResourceAttrWith("truenas_snapshot.test", "createtxg", func(v string) error {
					txg = v
					return nil
				}),
			},
			{
				Config: acctest.ProviderConfig() + testAccSnapshotDeferConfig(datasetName, snapName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snapshot.test", "defer_destroy", "true"),
					resource.TestCheckResourceAttrWith("truenas_snapshot.test", "createtxg", func(v string) error {
						if v != txg {
							return fmt.Errorf("createtxg changed from %s to %s: the snapshot was re-created", txg, v)
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccSnapshotDeferConfig(datasetName, snapName string, deferDestroy bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_snapshot" "test" {
  dataset       = truenas_dataset.fixture.name
  name          = %q
  defer_destroy = %v
}
`, datasetName, snapName, deferDestroy)
}

func testAccSnapshotConfig(datasetName, snapName string, recursive bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_snapshot" "test" {
  dataset   = truenas_dataset.fixture.name
  name      = %q
  recursive = %v
}
`, datasetName, snapName, recursive)
}

// snapshotSummary is the subset of pool.snapshot.query fields this test
// package needs directly (outside of the provider's own resource code).
type snapshotSummary struct {
	ID string `json:"id"`
}

func testAccCheckSnapshotDestroyed(id string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		// CallRead, not Call: acctest.Client() is connected once and sits
		// idle through each test's Terraform steps, and Call does not re-dial
		// a dropped connection. With more than one acceptance test in the
		// package, everything after the first would fail here with "not
		// connected". pool.snapshot.query is a read, so retrying is safe.
		raw, err := c.CallRead(context.Background(), "pool.snapshot.query", [][]any{{"id", "=", id}})
		if err != nil {
			return fmt.Errorf("error checking snapshot %s: %v", id, err)
		}
		var results []snapshotSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.snapshot.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("snapshot %s still exists", id)
		}
		return nil
	}
}
