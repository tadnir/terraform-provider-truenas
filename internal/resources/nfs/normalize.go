// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// cidrType is a string type whose values compare by network address, so a
// config value like "192.168.100.10/24" is treated as equal to the
// "192.168.100.0/24" TrueNAS stores and returns. Without this, the host-bits
// form produces "Provider produced inconsistent result after apply" (#13).
type cidrType struct{ basetypes.StringType }

func (t cidrType) Equal(o attr.Type) bool {
	other, ok := o.(cidrType)
	return ok && t.StringType.Equal(other.StringType)
}

func (t cidrType) String() string { return "nfs.cidrType" }

func (t cidrType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return cidrValue{StringValue: in}, nil
}

func (t cidrType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	av, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	sv, ok := av.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", av)
	}
	return cidrValue{StringValue: sv}, nil
}

func (t cidrType) ValueType(context.Context) attr.Value { return cidrValue{} }

// cidrValue is a string value that is semantically equal to another when both
// name the same network (host bits ignored).
type cidrValue struct{ basetypes.StringValue }

func (v cidrValue) Type(context.Context) attr.Type { return cidrType{} }

func (v cidrValue) Equal(o attr.Value) bool {
	other, ok := o.(cidrValue)
	return ok && v.StringValue.Equal(other.StringValue)
}

func (v cidrValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	other, ok := newValuable.(cidrValue)
	if !ok {
		return false, nil
	}
	if v.IsNull() || v.IsUnknown() || other.IsNull() || other.IsUnknown() {
		return false, nil
	}
	return normalizeCIDR(v.ValueString()) == normalizeCIDR(other.ValueString()), nil
}

// normalizeCIDR returns the network-address form of a CIDR, or the input
// unchanged if it does not parse as one.
func normalizeCIDR(s string) string {
	if _, ipnet, err := net.ParseCIDR(s); err == nil {
		return ipnet.String()
	}
	return s
}
