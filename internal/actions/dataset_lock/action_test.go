// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_lock

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
)

func TestMetadata(t *testing.T) {
	a := &Action{}
	resp := &action.MetadataResponse{}
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "truenas"}, resp)
	if resp.TypeName != "truenas_dataset_lock" {
		t.Errorf("TypeName = %q, want truenas_dataset_lock", resp.TypeName)
	}
}

func TestSchema_RequiredAttr(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	attr, ok := resp.Schema.Attributes["dataset"]
	if !ok || !attr.IsRequired() {
		t.Error("'dataset' should be required")
	}
}
