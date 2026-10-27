// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// model returns a configuration with every content attribute null except
// the ones set.
func model(set func(m *FilesystemFileModel)) *FilesystemFileModel {
	m := &FilesystemFileModel{
		Content:       types.StringNull(),
		ContentBase64: types.StringNull(),
		Source:        types.StringNull(),
	}
	if set != nil {
		set(m)
	}
	return m
}

func TestPlannedSHA256(t *testing.T) {
	const hiSHA = "8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4" // sha256("hi")
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		cfg     *FilesystemFileModel
		want    types.String
		wantErr bool
	}{
		{"content", model(func(m *FilesystemFileModel) { m.Content = types.StringValue("hi") }), types.StringValue(hiSHA), false},
		{"content_base64", model(func(m *FilesystemFileModel) { m.ContentBase64 = types.StringValue("aGk=") }), types.StringValue(hiSHA), false},
		{"bad base64", model(func(m *FilesystemFileModel) { m.ContentBase64 = types.StringValue("not base64!") }), types.StringNull(), true},
		{"source", model(func(m *FilesystemFileModel) { m.Source = types.StringValue(src) }), types.StringValue(hiSHA), false},
		{"source not made yet", model(func(m *FilesystemFileModel) { m.Source = types.StringValue(src + ".missing") }), types.StringUnknown(), false},
		{"unknown content", model(func(m *FilesystemFileModel) { m.Content = types.StringUnknown() }), types.StringUnknown(), false},
		{"existing file", model(nil), types.StringNull(), false},
	}
	for _, c := range cases {
		got, err := plannedSHA256(c.cfg)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestOpenContentHashes(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("from a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, cfg := range map[string]*FilesystemFileModel{
		"content": model(func(m *FilesystemFileModel) { m.Content = types.StringValue("from a file") }),
		"source":  model(func(m *FilesystemFileModel) { m.Source = types.StringValue(src) }),
	} {
		rd, done, err := openContent(cfg)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b, err := io.ReadAll(rd)
		done()
		if err != nil || string(b) != "from a file" {
			t.Errorf("%s: read %q, %v", name, b, err)
		}
		if want := hashBytes([]byte("from a file")); rd.sum() != want {
			t.Errorf("%s: sum %s, want %s", name, rd.sum(), want)
		}
	}
}

func TestWrittenMatches(t *testing.T) {
	w := written{Size: 3, Mtime: 1700000000.5}
	if !w.matches(&fsStatAPI{Size: 3, Mtime: 1700000000.5}) {
		t.Error("same size and mtime should match")
	}
	if w.matches(&fsStatAPI{Size: 4, Mtime: 1700000000.5}) || w.matches(&fsStatAPI{Size: 3, Mtime: 1700000001}) {
		t.Error("a different size or mtime should not match")
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
	for _, a := range []string{"content", "content_base64"} {
		if attr := resp.Schema.Attributes[a]; !attr.IsWriteOnly() || !attr.IsSensitive() {
			t.Errorf("%s should be write-only and sensitive", a)
		}
	}
	if !resp.Schema.Attributes["sha256"].IsComputed() {
		t.Error("sha256 should be computed")
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
