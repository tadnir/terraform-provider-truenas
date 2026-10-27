// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// defaultMode is the mode of a written file when mode is unset.
const defaultMode = 0o644

// FilesystemFileModel is the truenas_filesystem_file state.
type FilesystemFileModel struct {
	ID            types.String `tfsdk:"id"`
	Path          types.String `tfsdk:"path"`
	Content       types.String `tfsdk:"content"`
	ContentBase64 types.String `tfsdk:"content_base64"`
	Source        types.String `tfsdk:"source"`
	SHA256        types.String `tfsdk:"sha256"`
	Mode          types.String `tfsdk:"mode"`
}

// fsStatAPI is the part of filesystem.stat's response this resource reads.
// "mode" is the full stat(2) st_mode, file-type bits included.
type fsStatAPI struct {
	Mode  int64   `json:"mode"`
	Type  string  `json:"type"`
	Size  int64   `json:"size"`
	Mtime float64 `json:"mtime"`
}

// written is what the resource remembers, in private state, about the file
// it last wrote: TrueNAS can't hash a file, so a different size or mtime is
// how a change on the NAS shows.
type written struct {
	Size  int64   `json:"size"`
	Mtime float64 `json:"mtime"`
}

const writtenKey = "written"

func (w written) bytes() []byte {
	b, _ := json.Marshal(w)
	return b
}

func (w written) matches(st *fsStatAPI) bool { return w.Size == st.Size && w.Mtime == st.Mtime }

// isSet reports whether v holds a value (known and not null).
func isSet(v types.String) bool { return !v.IsNull() && !v.IsUnknown() }

// existing reports whether cfg names no content at all, i.e. the resource
// manages a file that is already on the NAS.
func existing(cfg *FilesystemFileModel) bool {
	return cfg.Content.IsNull() && cfg.ContentBase64.IsNull() && cfg.Source.IsNull()
}

// inline returns the bytes of content or content_base64 from cfg, and
// whether either was set. Both are write-only, so they are only in the
// configuration.
func inline(cfg *FilesystemFileModel) ([]byte, bool, error) {
	switch {
	case isSet(cfg.Content):
		return []byte(cfg.Content.ValueString()), true, nil
	case isSet(cfg.ContentBase64):
		b, err := base64.StdEncoding.DecodeString(cfg.ContentBase64.ValueString())
		if err != nil {
			return nil, true, fmt.Errorf("content_base64 is not valid base64: %w", err)
		}
		return b, true, nil
	}
	return nil, false, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hashFile returns the SHA-256 of the file at p, reading it in a stream.
func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// plannedSHA256 is sha256 as planned from the configuration: the hash of
// content, content_base64 or source; unknown when that isn't known yet (an
// unknown value, or a source file another resource makes during the apply);
// null for an existing file.
func plannedSHA256(cfg *FilesystemFileModel) (types.String, error) {
	if cfg.Content.IsUnknown() || cfg.ContentBase64.IsUnknown() || cfg.Source.IsUnknown() {
		return types.StringUnknown(), nil
	}
	if b, ok, err := inline(cfg); ok {
		if err != nil {
			return types.StringNull(), err
		}
		return types.StringValue(hashBytes(b)), nil
	}
	if isSet(cfg.Source) {
		sum, err := hashFile(cfg.Source.ValueString())
		if errors.Is(err, fs.ErrNotExist) {
			return types.StringUnknown(), nil
		}
		if err != nil {
			return types.StringNull(), fmt.Errorf("source: %w", err)
		}
		return types.StringValue(sum), nil
	}
	return types.StringNull(), nil
}

// hashingReader hashes what is read through it.
type hashingReader struct {
	r io.Reader
	h interface {
		io.Writer
		Sum([]byte) []byte
	}
}

func (r *hashingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	_, _ = r.h.Write(p[:n])
	return n, err
}

func (r *hashingReader) sum() string { return hex.EncodeToString(r.h.Sum(nil)) }

// openContent returns a reader over the content cfg names, hashing it as it
// is read, and a close function. It must not be called for an existing
// file.
func openContent(cfg *FilesystemFileModel) (*hashingReader, func(), error) {
	if b, ok, err := inline(cfg); ok {
		if err != nil {
			return nil, nil, err
		}
		return &hashingReader{r: bytes.NewReader(b), h: sha256.New()}, func() {}, nil
	}
	f, err := os.Open(cfg.Source.ValueString())
	if err != nil {
		return nil, nil, fmt.Errorf("source: %w", err)
	}
	return &hashingReader{r: f, h: sha256.New()}, func() { _ = f.Close() }, nil
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
	if isSet(current) {
		if m, err := parseMode(current.ValueString()); err == nil && int64(m) == bits {
			return current
		}
	}
	return types.StringValue(fmt.Sprintf("%04o", bits))
}
