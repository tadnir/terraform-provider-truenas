// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFilesystemFile_basic writes a file into a dataset fixture from
// content and checks it server-side, rewrites it from content_base64 with a
// new mode, then from a source file on this machine, imports it by path,
// and finally removes only the file resource and checks the documented
// Delete semantic: the file stays. A second resource manages that file as
// an existing one (no content).
func TestAccFilesystemFile_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-fsfile-ds"))
	path := "/mnt/" + datasetName + "/sub/test.txt"
	first := "hello\n"
	second := "hello again, a little longer\n"
	third := "streamed from a local file, the longest of the three\n"
	src := filepath.Join(t.TempDir(), "third.txt")
	if err := os.WriteFile(src, []byte(third), 0o600); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("content = %q", first), ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "id", path),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0644"),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "sha256", sha(first)),
					resource.TestCheckNoResourceAttr("truenas_filesystem_file.test", "content"),
					testAccCheckFileStat(path, "0644", len(first)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("content_base64 = %q", b64(second)), `mode = "0600"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0600"),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "sha256", sha(second)),
					resource.TestCheckNoResourceAttr("truenas_filesystem_file.test", "content_base64"),
					testAccCheckFileStat(path, "0600", len(second)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("source = %q", src), `mode = "0600"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "sha256", sha(third)),
					testAccCheckFileStat(path, "0600", len(third)),
				),
			},
			{
				// Only the mode changes: set without writing the file again.
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("source = %q", src), `mode = "0640"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0640"),
					testAccCheckFileStat(path, "0640", len(third)),
				),
			},
			{
				ResourceName:            "truenas_filesystem_file.test",
				ImportState:             true,
				ImportStateId:           path,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source", "sha256"},
			},
			{
				// The file stays when its resource goes, and can then be
				// managed as an existing file.
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, "", "", "") +
					testAccFilesystemFileExistingConfig(path),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_filesystem_file.existing", "sha256"),
					resource.TestCheckResourceAttr("truenas_filesystem_file.existing", "mode", "0640"),
					testAccCheckFileStat(path, "0640", len(third)),
				),
			},
		},
	})
}

// TestAccFilesystemFile_existingMissing checks that an existing-file
// resource fails clearly when the file isn't there.
func TestAccFilesystemFile_existingMissing(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-fsfile-ds"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, "", "", "") +
					testAccFilesystemFileExistingConfig("/mnt/"+datasetName+"/missing.txt"),
				ExpectError: regexp.MustCompile(`does not exist`),
			},
		},
	})
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// testAccFilesystemFileExistingConfig manages the file at path as an
// existing file, without content.
func testAccFilesystemFileExistingConfig(path string) string {
	return fmt.Sprintf(`
resource "truenas_filesystem_file" "existing" {
  path = %q

  depends_on = [truenas_dataset.fixture]
}
`, path)
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// testAccFilesystemFileConfig is the dataset fixture, plus the file at path
// with the given content and mode lines unless path is "".
func testAccFilesystemFileConfig(datasetName, path, contentLines, modeLine string) string {
	cfg := fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}
`, datasetName)
	if path == "" {
		return cfg
	}
	return cfg + fmt.Sprintf(`
resource "truenas_filesystem_file" "test" {
  path = %q
  %s
  %s

  depends_on = [truenas_dataset.fixture]
}
`, path, contentLines, modeLine)
}

// testAccCheckFileStat calls filesystem.stat directly and checks the file's
// permission bits and size.
func testAccCheckFileStat(path, wantMode string, wantSize int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "filesystem.stat", path)
		if err != nil {
			return fmt.Errorf("filesystem.stat(%s): %v", path, err)
		}
		var st struct {
			Mode int64 `json:"mode"`
			Size int64 `json:"size"`
		}
		if err := json.Unmarshal(raw, &st); err != nil {
			return fmt.Errorf("parse filesystem.stat response: %v", err)
		}
		if got := fmt.Sprintf("%04o", st.Mode&0o7777); got != wantMode {
			return fmt.Errorf("filesystem.stat(%s) mode = %s, want %s", path, got, wantMode)
		}
		if st.Size != int64(wantSize) {
			return fmt.Errorf("filesystem.stat(%s) size = %d, want %d", path, st.Size, wantSize)
		}
		return nil
	}
}
