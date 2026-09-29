// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestNormalizeCIDR(t *testing.T) {
	cases := map[string]string{
		"192.168.100.10/24": "192.168.100.0/24",
		"192.168.100.0/24":  "192.168.100.0/24",
		"10.0.0.0/8":        "10.0.0.0/8",
		"127.0.0.1/32":      "127.0.0.1/32",
		"2001:db8::5/64":    "2001:db8::/64",
		"not-a-cidr":        "not-a-cidr", // passthrough
	}
	for in, want := range cases {
		if got := normalizeCIDR(in); got != want {
			t.Errorf("normalizeCIDR(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCIDRSemanticEquals(t *testing.T) {
	ctx := context.Background()
	a := cidrValue{StringValue: basetypes.NewStringValue("192.168.100.10/24")}
	b := cidrValue{StringValue: basetypes.NewStringValue("192.168.100.0/24")}
	eq, diags := a.StringSemanticEquals(ctx, b)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if !eq {
		t.Error("host-bits CIDR should be semantically equal to its network address")
	}

	c := cidrValue{StringValue: basetypes.NewStringValue("10.0.0.0/8")}
	eq, _ = a.StringSemanticEquals(ctx, c)
	if eq {
		t.Error("different networks should not be semantically equal")
	}
}
