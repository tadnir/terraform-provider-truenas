// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_restart

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
)

func TestMetadata(t *testing.T) {
	a := &Action{}
	resp := &action.MetadataResponse{}
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "truenas"}, resp)
	if resp.TypeName != "truenas_vm_restart" {
		t.Errorf("TypeName = %q, want truenas_vm_restart", resp.TypeName)
	}
}

func TestSchema_RequiredAttr(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	attr, ok := resp.Schema.Attributes["vm_id"]
	if !ok || !attr.IsRequired() {
		t.Error("'vm_id' should be required")
	}
}
