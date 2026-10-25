// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import "testing"

func TestUploadURL(t *testing.T) {
	cases := map[string]string{
		"wss://nas.example.com/api/current":      "https://nas.example.com/_upload",
		"wss://nas.example.com:8443/api/current": "https://nas.example.com:8443/_upload",
		"ws://10.0.0.5/api/current":              "http://10.0.0.5/_upload",
	}
	for in, want := range cases {
		got, err := uploadURL(in)
		if err != nil || got != want {
			t.Errorf("uploadURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
