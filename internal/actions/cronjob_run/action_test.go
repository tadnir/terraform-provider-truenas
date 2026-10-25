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
		t.Errorf("TypeName = %q", resp.TypeName)
	}
}

func TestBuildParams(t *testing.T) {
	got := buildParams(model{ID: types.Int64Value(4), SkipDisabled: types.BoolNull()})
	if len(got) != 2 || got[0] != int64(4) || got[1] != false {
		t.Errorf("buildParams = %#v, want [4 false]", got)
	}
	got = buildParams(model{ID: types.Int64Value(4), SkipDisabled: types.BoolValue(true)})
	if got[1] != true {
		t.Errorf("buildParams skip_disabled = %#v, want true", got[1])
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
