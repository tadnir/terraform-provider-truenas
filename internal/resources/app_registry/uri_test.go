// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPreserveURI guards the #47 fix: TrueNAS canonicalizes a registry URI by
// appending a trailing slash, so the configured value must round-trip when it
// differs only by that slash; a genuinely different URI takes the API value.
func TestPreserveURI(t *testing.T) {
	cases := []struct {
		name    string
		current types.String
		apiVal  string
		want    string
	}{
		{"trailing-slash equivalence kept", types.StringValue("https://ghcr.io"), "https://ghcr.io/", "https://ghcr.io"},
		{"already-slashed kept", types.StringValue("https://ghcr.io/"), "https://ghcr.io/", "https://ghcr.io/"},
		{"real change takes api value", types.StringValue("https://ghcr.io"), "https://registry.example.com/", "https://registry.example.com/"},
		{"null current takes api value", types.StringNull(), "https://index.docker.io/v1/", "https://index.docker.io/v1/"},
		{"unknown current takes api value", types.StringUnknown(), "https://ghcr.io/", "https://ghcr.io/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := preserveURI(tc.current, tc.apiVal).ValueString(); got != tc.want {
				t.Errorf("preserveURI(%v, %q) = %q, want %q", tc.current, tc.apiVal, got, tc.want)
			}
		})
	}
}

// TestResponseToModel_URISlashPreserved proves preserveURI is actually WIRED
// INTO the resource read path (not just defined): with the configured value
// carried in m.URI and the API returning the slash-normalized form, the read
// must keep the configured value — the exact round-trip that failed in #47. The
// datasource read keeps the API value verbatim (computed-only).
func TestResponseToModel_URISlashPreserved(t *testing.T) {
	api := &appRegistryAPI{Name: "ghcr", URI: "https://ghcr.io/"}

	m := &AppRegistryModel{URI: types.StringValue("https://ghcr.io")}
	if d := responseToModel(api, m); d.HasError() {
		t.Fatalf("responseToModel: %v", d)
	}
	if got := m.URI.ValueString(); got != "https://ghcr.io" {
		t.Errorf("resource read URI = %q, want the configured %q (preserveURI not wired in)", got, "https://ghcr.io")
	}

	dm := &AppRegistryDataSourceModel{}
	if d := responseToDataSourceModel(api, dm); d.HasError() {
		t.Fatalf("responseToDataSourceModel: %v", d)
	}
	if got := dm.URI.ValueString(); got != "https://ghcr.io/" {
		t.Errorf("datasource read URI = %q, want the API value %q verbatim", got, "https://ghcr.io/")
	}
}
