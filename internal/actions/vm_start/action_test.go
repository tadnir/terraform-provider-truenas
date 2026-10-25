// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_start

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
	if resp.TypeName != "truenas_vm_start" {
		t.Errorf("TypeName = %q, want truenas_vm_start", resp.TypeName)
	}
}

func TestSchema(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	if attr, ok := resp.Schema.Attributes["vm_id"]; !ok || !attr.IsRequired() {
		t.Error("'vm_id' should be required")
	}
	if attr, ok := resp.Schema.Attributes["overcommit"]; !ok || !attr.IsOptional() {
		t.Error("'overcommit' should be optional")
	}
}

func TestBuildParams(t *testing.T) {
	got := buildParams(model{VMID: types.Int64Value(3), Overcommit: types.BoolNull()})
	want := []any{int64(3), map[string]any{"overcommit": false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildParams = %#v, want %#v", got, want)
	}
}
