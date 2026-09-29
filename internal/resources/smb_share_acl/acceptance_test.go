// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBShareACL_basic drives the full truenas_smb_share_acl lifecycle
// against a live TrueNAS: it manages a dataset + SMB share fixture through the
// provider, then sets a two-entry share ACL (everyone@ by SID + builtin_users
// by Unix GID), verifies it server-side via sharing.smb.getacl, updates the
// entries in place (down to a single everyone@ READ entry), imports by
// share_name, then removes ONLY the truenas_smb_share_acl resource from the
// config and asserts the documented Delete semantic - the share ACL is reset
// to the TrueNAS default of a single "everyone@ FULL ALLOWED" entry (an SMB
// share always has a share ACL) - directly server-side before the framework's
// final teardown. builtin_users' GID 545 is a fixed TrueNAS builtin SMB group.
func TestAccSMBShareACL_basic(t *testing.T) {
	dsName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-sacl-ds"))
	shareName := acctest.RandName("smbacl") // "smbacl-XXXXXXXX", 15 chars

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create: two entries.
			{
				Config: acctest.ProviderConfig() + testAccSMBShareACLConfig(dsName, shareName, twoEntries),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "id", shareName),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_name", shareName),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.#", "2"),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.0.ae_perm", "CHANGE"),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.0.ae_who_sid", "S-1-1-0"),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.1.ae_perm", "FULL"),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.1.ae_who_id.id", "545"),
					testAccCheckShareACLLen(shareName, 2),
					testAccCheckShareACLEntry(shareName, 0, "CHANGE", "ALLOWED", "S-1-1-0"),
				),
			},
			// Update in place: single everyone@ READ entry.
			{
				Config: acctest.ProviderConfig() + testAccSMBShareACLConfig(dsName, shareName, oneEntry),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.#", "1"),
					resource.TestCheckResourceAttr("truenas_smb_share_acl.test", "share_acl.0.ae_perm", "READ"),
					testAccCheckShareACLLen(shareName, 1),
					testAccCheckShareACLEntry(shareName, 0, "READ", "ALLOWED", "S-1-1-0"),
				),
			},
			// Import by share_name (single SID-only entry round-trips exactly).
			{
				ResourceName:      "truenas_smb_share_acl.test",
				ImportState:       true,
				ImportStateId:     shareName,
				ImportStateVerify: true,
			},
			// Remove ONLY the ACL resource: Delete resets the share ACL to the
			// TrueNAS default (everyone@ FULL ALLOWED). The dataset + share
			// fixtures remain so the reset can be checked server-side.
			{
				Config: acctest.ProviderConfig() + testAccSMBShareACLConfig(dsName, shareName, noACL),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckShareACLLen(shareName, 1),
					testAccCheckShareACLEntry(shareName, 0, "FULL", "ALLOWED", "S-1-1-0"),
				),
			},
		},
	})
}

type aclVariant int

const (
	twoEntries aclVariant = iota
	oneEntry
	noACL
)

// testAccSMBShareACLConfig returns HCL for the dataset + SMB share fixtures and,
// depending on variant, the truenas_smb_share_acl resource under test.
func testAccSMBShareACLConfig(dsName, shareName string, variant aclVariant) string {
	fixtures := fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_smb_share" "fixture" {
  name    = %q
  path    = truenas_dataset.fixture.mountpoint
  purpose = "DEFAULT_SHARE"
}
`, dsName, shareName)

	switch variant {
	case twoEntries:
		return fixtures + `
resource "truenas_smb_share_acl" "test" {
  share_name = truenas_smb_share.fixture.name
  share_acl = [
    { ae_perm = "CHANGE", ae_type = "ALLOWED", ae_who_sid = "S-1-1-0" },
    { ae_perm = "FULL", ae_type = "ALLOWED", ae_who_id = { id_type = "GROUP", id = 545 } },
  ]
}
`
	case oneEntry:
		return fixtures + `
resource "truenas_smb_share_acl" "test" {
  share_name = truenas_smb_share.fixture.name
  share_acl = [
    { ae_perm = "READ", ae_type = "ALLOWED", ae_who_sid = "S-1-1-0" },
  ]
}
`
	default: // noACL
		return fixtures
	}
}

// shareACLSummary is the subset of sharing.smb.getacl this test parses.
type shareACLSummary struct {
	ShareACL []struct {
		AePerm   string  `json:"ae_perm"`
		AeType   string  `json:"ae_type"`
		AeWhoSID *string `json:"ae_who_sid"`
	} `json:"share_acl"`
}

func getShareACL(shareName string) (*shareACLSummary, error) {
	c := acctest.Client()
	raw, err := c.CallRead(context.Background(), "sharing.smb.getacl", map[string]any{"share_name": shareName})
	if err != nil {
		return nil, fmt.Errorf("sharing.smb.getacl(%s): %w", shareName, err)
	}
	var got shareACLSummary
	if err := json.Unmarshal(raw, &got); err != nil {
		return nil, fmt.Errorf("parse getacl response: %w", err)
	}
	return &got, nil
}

// testAccCheckShareACLLen asserts, straight from sharing.smb.getacl (bypassing
// Terraform state), that the share has wantLen ACL entries.
func testAccCheckShareACLLen(shareName string, wantLen int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := getShareACL(shareName)
		if err != nil {
			return err
		}
		if len(got.ShareACL) != wantLen {
			return fmt.Errorf("share %s: getacl has %d entries, want %d", shareName, len(got.ShareACL), wantLen)
		}
		return nil
	}
}

// testAccCheckShareACLEntry asserts the entry at idx has the given perm, type,
// and SID, read directly from sharing.smb.getacl.
func testAccCheckShareACLEntry(shareName string, idx int, wantPerm, wantType, wantSID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := getShareACL(shareName)
		if err != nil {
			return err
		}
		if idx >= len(got.ShareACL) {
			return fmt.Errorf("share %s: entry %d out of range (%d entries)", shareName, idx, len(got.ShareACL))
		}
		e := got.ShareACL[idx]
		if e.AePerm != wantPerm || e.AeType != wantType {
			return fmt.Errorf("share %s entry %d: got %s/%s, want %s/%s", shareName, idx, e.AePerm, e.AeType, wantPerm, wantType)
		}
		if e.AeWhoSID == nil || *e.AeWhoSID != wantSID {
			return fmt.Errorf("share %s entry %d: ae_who_sid = %v, want %s", shareName, idx, e.AeWhoSID, wantSID)
		}
		return nil
	}
}
