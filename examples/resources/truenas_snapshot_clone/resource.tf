resource "truenas_dataset" "golden" {
  name = "tank/golden"
}

resource "truenas_snapshot" "golden_v1" {
  dataset = truenas_dataset.golden.name
  name    = "v1"
}

# A throwaway copy of the golden dataset, created instantly and sharing its
# blocks until written to. Reference the snapshot by id so Terraform creates
# it first and destroys the clone before it.
resource "truenas_snapshot_clone" "test1" {
  snapshot = truenas_snapshot.golden_v1.id
  dataset  = "tank/scratch/test1"

  dataset_properties = {
    compression = "lz4"
  }
}
