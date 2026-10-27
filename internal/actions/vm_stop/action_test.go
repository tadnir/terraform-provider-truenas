// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_stop

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
	for _, n := range []string{"force", "force_after_timeout"} {
		if attr, ok := resp.Schema.Attributes[n]; !ok || !attr.IsOptional() {
			t.Errorf("%q should be optional", n)
		}
	}
}

func TestShouldStop(t *testing.T) {
	cases := map[string]bool{"RUNNING": true, "STOPPED": false, "": false}
	for state, want := range cases {
		if got := shouldStop(state); got != want {
			t.Errorf("shouldStop(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestStopOptions(t *testing.T) {
	// unset -> empty (server defaults apply)
	if got := stopOptions(model{Force: types.BoolNull(), ForceAfterTimeout: types.BoolNull()}); len(got) != 0 {
		t.Errorf("stopOptions(unset) = %v, want empty", got)
	}
	got := stopOptions(model{Force: types.BoolValue(true), ForceAfterTimeout: types.BoolValue(false)})
	if got["force"] != true || got["force_after_timeout"] != false {
		t.Errorf("stopOptions = %v, want force=true force_after_timeout=false", got)
	}
}
