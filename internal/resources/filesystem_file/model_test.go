// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestContent(t *testing.T) {
	plan := &FilesystemFileModel{ContentBase64: types.StringValue("aGk=")}
	if b, err := content(plan, &FilesystemFileModel{}); err != nil || string(b) != "hi" {
		t.Errorf("content_base64: got %q, %v", b, err)
	}

	plan = &FilesystemFileModel{ContentBase64: types.StringNull()}
	cfg := &FilesystemFileModel{ContentBase64WO: types.StringValue("d28=")}
	if b, err := content(plan, cfg); err != nil || string(b) != "wo" {
		t.Errorf("content_base64_wo: got %q, %v", b, err)
	}

	if _, err := content(&FilesystemFileModel{ContentBase64: types.StringValue("not base64!")}, &FilesystemFileModel{}); err == nil {
		t.Error("invalid base64 should fail")
	}
	if _, err := content(&FilesystemFileModel{ContentBase64: types.StringNull()}, &FilesystemFileModel{ContentBase64WO: types.StringNull()}); err == nil {
		t.Error("no content should fail")
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]int{"0644": 0o644, "644": 0o644, "0755": 0o755, "600": 0o600} {
		if got, err := parseMode(in); err != nil || got != want {
			t.Errorf("parseMode(%q) = %o, %v; want %o", in, got, err, want)
		}
	}
}

func TestModeFromStat(t *testing.T) {
	const regularFile = 0o100000
	cases := []struct {
		current types.String
		stMode  int64
		want    string
	}{
		{types.StringValue("644"), regularFile | 0o644, "644"},
		{types.StringValue("0644"), regularFile | 0o644, "0644"},
		{types.StringValue("0644"), regularFile | 0o600, "0600"},
		{types.StringNull(), regularFile | 0o755, "0755"},
	}
	for _, c := range cases {
		if got := modeFromStat(c.current, c.stMode); got.ValueString() != c.want {
			t.Errorf("modeFromStat(%s, %o) = %s, want %s", c.current, c.stMode, got.ValueString(), c.want)
		}
	}
}

func TestSchema(t *testing.T) {
	r := NewResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("schema: %v", diags)
	}
	wo := resp.Schema.Attributes["content_base64_wo"]
	if !wo.IsWriteOnly() || !wo.IsSensitive() {
		t.Error("content_base64_wo should be write-only and sensitive")
	}
	if !resp.Schema.Attributes["path"].IsRequired() {
		t.Error("path should be required")
	}
}

func TestPathRegexp(t *testing.T) {
	for p, ok := range map[string]bool{
		"/mnt/tank/a.txt":      true,
		"/mnt/tank/x/seed.iso": true,
		"/mnt/tank":            false,
		"/mnt/tank/":           false,
		"/etc/passwd":          false,
		"/mnt/tank/dir/":       false,
	} {
		if pathRegexp.MatchString(p) != ok {
			t.Errorf("pathRegexp(%q) = %v, want %v", p, !ok, ok)
		}
	}
}
