// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_clone_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSnapshotClone_basic snapshots a fixture dataset, clones the
// snapshot with a property set at clone time, imports the clone by name
// (which must recover `snapshot` from the clone's origin), and verifies the
// clone is destroyed. Terraform destroys the clone before the snapshot it
// depends on, so no deferred destroy is needed.
func TestAccSnapshotClone_basic(t *testing.T) {
	source := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-clone-src"))
	snap := acctest.RandName("tf-acc-clone-snap")
	clone := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-clone"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloneDestroyed(clone),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "source" {
  name = %q
}

resource "truenas_snapshot" "golden" {
  dataset = truenas_dataset.source.name
  name    = %q
}

resource "truenas_snapshot_clone" "test" {
  snapshot = truenas_snapshot.golden.id
  dataset  = %q

  dataset_properties = {
    compression = "lz4"
  }
}
`, source, snap, clone),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snapshot_clone.test", "id", clone),
					resource.TestCheckResourceAttr("truenas_snapshot_clone.test", "dataset", clone),
					resource.TestCheckResourceAttr("truenas_snapshot_clone.test", "snapshot", source+"@"+snap),
					resource.TestCheckResourceAttr("truenas_snapshot_clone.test", "type", "FILESYSTEM"),
					resource.TestCheckResourceAttrSet("truenas_snapshot_clone.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_snapshot_clone.test", "mountpoint"),
				),
			},
			{
				ResourceName:      "truenas_snapshot_clone.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Set at clone time and not read back.
				ImportStateVerifyIgnore: []string{"dataset_properties"},
			},
		},
	})
}

type datasetSummary struct {
	ID string `json:"id"`
}

func testAccCheckCloneDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// CallRead, not Call: the shared client sits idle through the
		// test's Terraform steps and Call does not re-dial a dropped
		// connection.
		raw, err := acctest.Client().CallRead(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking clone %s: %v", name, err)
		}
		var results []datasetSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("clone %s still exists", name)
		}
		return nil
	}
}
