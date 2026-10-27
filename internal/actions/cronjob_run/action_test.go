// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob_run

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMetadata(t *testing.T) {
	a := &Action{}
	resp := &action.MetadataResponse{}
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "truenas"}, resp)
	if resp.TypeName != "truenas_cronjob_run" {
		t.Errorf("TypeName = %q, want truenas_cronjob_run", resp.TypeName)
	}
}

func TestSchema(t *testing.T) {
	a := &Action{}
	resp := &action.SchemaResponse{}
	a.Schema(context.Background(), action.SchemaRequest{}, resp)
	if attr, ok := resp.Schema.Attributes["id"]; !ok || !attr.IsRequired() {
		t.Error("'id' should be required")
	}
	if attr, ok := resp.Schema.Attributes["skip_disabled"]; !ok || !attr.IsOptional() {
		t.Error("'skip_disabled' should be optional")
	}
}

func TestBuildParams(t *testing.T) {
	// id only when skip_disabled is unset
	if got := buildParams(model{ID: types.Int64Value(7), SkipDisabled: types.BoolNull()}); len(got) != 1 || got[0] != int64(7) {
		t.Errorf("buildParams(unset skip_disabled) = %v, want [7]", got)
	}
	// id + skip_disabled when set
	got := buildParams(model{ID: types.Int64Value(7), SkipDisabled: types.BoolValue(true)})
	if len(got) != 2 || got[0] != int64(7) || got[1] != true {
		t.Errorf("buildParams(skip_disabled=true) = %v, want [7 true]", got)
	}
}
