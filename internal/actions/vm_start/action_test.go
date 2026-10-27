// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_start

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
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

func TestShouldStart(t *testing.T) {
	cases := map[string]bool{"RUNNING": false, "STOPPED": true, "": true, "SUSPENDED": true}
	for state, want := range cases {
		if got := shouldStart(state); got != want {
			t.Errorf("shouldStart(%q) = %v, want %v", state, got, want)
		}
	}
}
