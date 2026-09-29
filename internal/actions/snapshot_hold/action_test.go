// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot_hold

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
)

func TestMetadata(t *testing.T) {
	a := &Action{}
	resp := &action.MetadataResponse{}
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "truenas"}, resp)
	if resp.TypeName != "truenas_snapshot_hold" {
		t.Errorf("TypeName = %q, want truenas_snapshot_hold", resp.TypeName)
	}
}

func TestSchema_SnapshotRequired(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	attr, ok := resp.Schema.Attributes["snapshot"]
	if !ok || !attr.IsRequired() {
		t.Error("'snapshot' should be required")
	}
}
