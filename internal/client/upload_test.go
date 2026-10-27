// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

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

func TestWriteUpload(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := writeUpload(w, []byte(`{"method":"filesystem.put"}`), strings.NewReader("file bytes")); err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(w.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(&body, params["boundary"])
	for _, want := range []struct{ name, content string }{
		{"data", `{"method":"filesystem.put"}`},
		{"file", "file bytes"},
	} {
		part, err := r.NextPart()
		if err != nil {
			t.Fatalf("part %s: %v", want.name, err)
		}
		b, _ := io.ReadAll(part)
		if part.FormName() != want.name || string(b) != want.content {
			t.Errorf("part %q = %q, want %q = %q", part.FormName(), b, want.name, want.content)
		}
	}
}
