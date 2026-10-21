// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkInterface_bridge would create a BRIDGE interface (br999)
// with no members and no aliases, verify it, then destroy it.
//
// truenas_network_interface changes are staged and then committed with an
// auto-rollback safety window against the box's live networking stack
// (interface.commit / interface.checkin). A memberless, IP-less BRIDGE is
// inert and never touches the management interface, but the commit/checkin
// still exercises the box's global network apply. So this is gated behind
// TRUENAS_TEST_NET_INTERFACE (not just TF_ACC) — run it only against a
// disposable box you are prepared to lose network access to. The 60s
// auto-rollback is the backstop if a commit ever severs connectivity.
func TestAccNetworkInterface_bridge(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NET_INTERFACE") == "" {
		t.Skip("set TRUENAS_TEST_NET_INTERFACE=1 (on a disposable box) to run the network_interface create/destroy test")
	}
	acctest.PreCheck(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInterfaceDestroyed("br999"),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNetworkInterfaceBridgeConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_interface.test", "name", "br999"),
					resource.TestCheckResourceAttr("truenas_network_interface.test", "type", "BRIDGE"),
					resource.TestCheckResourceAttrSet("truenas_network_interface.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_network_interface.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// testAccCheckInterfaceDestroyed verifies the named interface is gone.
func testAccCheckInterfaceDestroyed(name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "interface.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("querying interface %s: %w", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("parsing interface.query: %w", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("interface %s still exists", name)
		}
		return nil
	}
}

func testAccNetworkInterfaceBridgeConfig() string {
	return `
resource "truenas_network_interface" "test" {
  name = "br999"
  type = "BRIDGE"
}
`
}

// TestAccNetworkInterface_datasourceEnp7s0 is a strictly read-only
// datasource lookup of the box's physical management NIC (enp7s0). It never
// creates, modifies, or imports a resource — only a data source Read — so
// it carries none of the commit/checkin risk of TestAccNetworkInterface_bridge.
// It skips itself if enp7s0 is not present on the target host, so it is
// safe to run against any TF_ACC target.
func TestAccNetworkInterface_datasourceEnp7s0(t *testing.T) {
	// PreCheck validates credentials too — acctest.Client() panics without
	// them, so gate on the full precheck rather than TF_ACC alone.
	acctest.PreCheck(t)

	const ifaceName = "enp7s0"

	raw, err := acctest.Client().Call(context.Background(), "interface.query", [][]any{{"id", "=", ifaceName}})
	if err != nil {
		t.Fatalf("querying interface.query for %s: %v", ifaceName, err)
	}
	var results []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("parsing interface.query response: %v", err)
	}
	if len(results) == 0 {
		t.Skipf("%s not present on target host: skipping read-only datasource check", ifaceName)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_network_interface" "enp7s0" {
  name = "enp7s0"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_network_interface.enp7s0", "name", "enp7s0"),
					resource.TestCheckResourceAttr("data.truenas_network_interface.enp7s0", "type", "PHYSICAL"),
					resource.TestCheckResourceAttrSet("data.truenas_network_interface.enp7s0", "id"),
				),
			},
		},
	})
}

// ifaceAddrRecord is the subset of interface.query used by the physical-manage
// test's safety guard: both the configured aliases and the runtime state
// aliases, so the test can refuse to touch the NIC carrying the provider's own
// connection.
type ifaceAddrRecord struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Aliases     []struct {
		Address string `json:"address"`
	} `json:"aliases"`
	State struct {
		Aliases []struct {
			Address string `json:"address"`
		} `json:"aliases"`
	} `json:"state"`
}

// readIfaceAddrRecord queries the named interface, returning whether it exists
// plus its type, description, and every IP address bound to it (configured +
// runtime).
func readIfaceAddrRecord(t *testing.T, name string) (found bool, rec ifaceAddrRecord, addrs []string) {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "interface.query", [][]any{{"id", "=", name}})
	if err != nil {
		t.Fatalf("interface.query for %s: %v", name, err)
	}
	var results []ifaceAddrRecord
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("parsing interface.query: %v", err)
	}
	if len(results) == 0 {
		return false, ifaceAddrRecord{}, nil
	}
	rec = results[0]
	for _, a := range rec.Aliases {
		addrs = append(addrs, a.Address)
	}
	for _, a := range rec.State.Aliases {
		addrs = append(addrs, a.Address)
	}
	return true, rec, addrs
}

// restorePhysicalDescription sets the interface's description back to orig and
// commits, mirroring the resource's own stage->commit->checkin (60s
// auto-rollback). Registered via t.Cleanup before the test's mutating apply.
func restorePhysicalDescription(t *testing.T, name, orig string) {
	t.Helper()
	ctx := context.Background()
	if _, err := acctest.Client().Call(ctx, "interface.update", name, map[string]any{"description": orig}); err != nil {
		t.Fatalf("RESTORE FAILED for %s: interface.update: %v", name, err)
	}
	if _, err := acctest.Client().Call(ctx, "interface.commit", map[string]any{"checkin_timeout": 60, "rollback": true}); err != nil {
		t.Fatalf("RESTORE FAILED for %s: interface.commit: %v", name, err)
	}
	if _, err := acctest.Client().Call(ctx, "interface.checkin"); err != nil {
		t.Fatalf("RESTORE FAILED for %s: interface.checkin: %v", name, err)
	}
}

// TestAccNetworkInterface_physicalManage proves a PHYSICAL interface can be
// imported and then managed (the #<employee-report> feature): it updates the
// interface's description in place and verifies a clean follow-up plan.
//
// SAFETY. A physical interface is the box's real hardware NIC, so this test:
//   - requires TRUENAS_TEST_NET_INTERFACE=1 (accept the network-apply/commit
//     risk, same gate as the bridge test) AND TRUENAS_PHYSICAL_IFACE=<name>
//     naming the exact safe NIC — it never auto-discovers one, because an
//     auto-chosen physical NIC could be the management interface;
//   - hard-refuses (t.Fatal) if that NIC carries the provider's own endpoint
//     IP, so even a misconfigured env var cannot strand the box;
//   - changes only "description" — never ipv4_dhcp/aliases/mtu, which could
//     affect reachability — and restores the original description in t.Cleanup;
//   - relies on the resource's 60s interface.commit auto-rollback as the final
//     backstop.
func TestAccNetworkInterface_physicalManage(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NET_INTERFACE") == "" {
		t.Skip("set TRUENAS_TEST_NET_INTERFACE=1 (on a recoverable box) to run the physical-interface management test")
	}
	name := os.Getenv("TRUENAS_PHYSICAL_IFACE")
	if name == "" {
		t.Skip("set TRUENAS_PHYSICAL_IFACE=<a safe, non-management physical NIC> to run the physical-interface management test")
	}
	acctest.PreCheck(t)

	found, rec, addrs := readIfaceAddrRecord(t, name)
	if !found {
		t.Skipf("interface %q not present on target host", name)
	}
	if rec.Type != "PHYSICAL" {
		t.Skipf("interface %q is %s, not PHYSICAL", name, rec.Type)
	}

	// Hard guard: never manage the NIC the provider is reaching the box through.
	host := ""
	if u, err := url.Parse(acctest.Endpoint()); err == nil {
		host = u.Hostname()
	}
	for _, a := range addrs {
		if a == host {
			t.Fatalf("refusing to run: %q carries the provider endpoint IP %q (the management interface); point TRUENAS_PHYSICAL_IFACE at a different NIC", name, host)
		}
	}

	t.Cleanup(func() { restorePhysicalDescription(t, name, rec.Description) })

	const newDesc = "tf-acc managed physical (safe to remove)"
	cfg := func(desc string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_network_interface" "p" {
  name        = %q
  type        = "PHYSICAL"
  description = %q
}
`, name, desc)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Import the existing physical interface into state — it cannot
				// be created — and persist it for the apply step below.
				Config:             cfg(newDesc),
				ResourceName:       "truenas_network_interface.p",
				ImportState:        true,
				ImportStateId:      name,
				ImportStatePersist: true,
			},
			{
				// Manage it: the description change applies in place.
				Config: cfg(newDesc),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_interface.p", "type", "PHYSICAL"),
					resource.TestCheckResourceAttr("truenas_network_interface.p", "description", newDesc),
				),
			},
			{
				// No drift.
				Config:   cfg(newDesc),
				PlanOnly: true,
			},
		},
	})
}
