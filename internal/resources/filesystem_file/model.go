// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FilesystemFileModel is the truenas_filesystem_file state.
type FilesystemFileModel struct {
	ID               types.String `tfsdk:"id"`
	Path             types.String `tfsdk:"path"`
	ContentBase64    types.String `tfsdk:"content_base64"`
	ContentBase64WO  types.String `tfsdk:"content_base64_wo"`
	ContentWOVersion types.Int64  `tfsdk:"content_wo_version"`
	Mode             types.String `tfsdk:"mode"`
}

// fsStatAPI is the part of filesystem.stat's response this resource reads.
// "mode" is the full stat(2) st_mode, file-type bits included.
type fsStatAPI struct {
	Mode int64  `json:"mode"`
	Type string `json:"type"`
}

// content returns the bytes to write: content_base64 from the plan, or
// content_base64_wo from the configuration (write-only values are only in
// the configuration).
func content(plan, cfg *FilesystemFileModel) ([]byte, error) {
	enc := plan.ContentBase64
	attr := "content_base64"
	if enc.IsNull() {
		enc = cfg.ContentBase64WO
		attr = "content_base64_wo"
	}
	if enc.IsNull() || enc.IsUnknown() {
		return nil, fmt.Errorf("one of content_base64 or content_base64_wo must be set")
	}
	b, err := base64.StdEncoding.DecodeString(enc.ValueString())
	if err != nil {
		return nil, fmt.Errorf("%s is not valid base64: %w", attr, err)
	}
	return b, nil
}

// parseMode turns "0644" or "644" into the integer mode filesystem.put takes.
func parseMode(s string) (int, error) {
	m, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("mode %q: %w", s, err)
	}
	return int(m), nil
}

// modeFromStat keeps the configured spelling of mode ("644" or "0644") when
// it means the same bits as the file has, so a 3-digit mode doesn't plan a
// change after every read.
func modeFromStat(current types.String, stMode int64) types.String {
	bits := stMode & 0o7777
	if !current.IsNull() && !current.IsUnknown() {
		if m, err := parseMode(current.ValueString()); err == nil && int64(m) == bits {
			return current
		}
	}
	return types.StringValue(fmt.Sprintf("%04o", bits))
}
