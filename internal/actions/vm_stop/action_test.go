// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_stop

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMetadata(t *testing.T) {
	a := &Action{}
	resp := &action.MetadataResponse{}
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "truenas"}, resp)
	if resp.TypeName != "truenas_vm_stop" {
		t.Errorf("TypeName = %q, want truenas_vm_stop", resp.TypeName)
	}
}

func TestSchema(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	if attr, ok := resp.Schema.Attributes["vm_id"]; !ok || !attr.IsRequired() {
		t.Error("'vm_id' should be required")
	}
	for _, name := range []string{"force", "force_after_timeout"} {
		if attr, ok := resp.Schema.Attributes[name]; !ok || !attr.IsOptional() {
			t.Errorf("'%s' should be optional", name)
		}
	}
}

func TestBuildParams(t *testing.T) {
	cases := []struct {
		m    model
		want map[string]any
	}{
		{model{VMID: types.Int64Value(3), Force: types.BoolNull(), ForceAfterTimeout: types.BoolNull()},
			map[string]any{"force": false, "force_after_timeout": true}},
		{model{VMID: types.Int64Value(3), Force: types.BoolValue(true), ForceAfterTimeout: types.BoolValue(false)},
			map[string]any{"force": true, "force_after_timeout": false}},
	}
	for _, c := range cases {
		got := buildParams(c.m)
		if want := []any{int64(3), c.want}; !reflect.DeepEqual(got, want) {
			t.Errorf("buildParams(%+v) = %#v, want %#v", c.m, got, want)
		}
	}
}
