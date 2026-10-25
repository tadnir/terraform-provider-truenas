// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_file_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFilesystemFile_basic writes a file into a dataset fixture with
// content_base64 and checks it server-side, rewrites it with new content
// and mode, rewrites it again from content_base64_wo (bumping
// content_wo_version), imports it by path, and finally removes only the
// file resource and checks the documented Delete semantic: the file stays.
func TestAccFilesystemFile_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-fsfile-ds"))
	path := "/mnt/" + datasetName + "/sub/test.txt"
	first := "hello\n"
	second := "hello again, a little longer\n"
	third := "written from a write-only value\n"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("content_base64 = %q", b64(first)), ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "id", path),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0644"),
					testAccCheckFileStat(path, "0644", len(first)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path, fmt.Sprintf("content_base64 = %q", b64(second)), `mode = "0600"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0600"),
					testAccCheckFileStat(path, "0600", len(second)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, path,
					fmt.Sprintf("content_base64_wo = %q\n  content_wo_version = 1", b64(third)), `mode = "0600"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_filesystem_file.test", "content_base64"),
					resource.TestCheckNoResourceAttr("truenas_filesystem_file.test", "content_base64_wo"),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "content_wo_version", "1"),
					resource.TestCheckResourceAttr("truenas_filesystem_file.test", "mode", "0600"),
					testAccCheckFileStat(path, "0600", len(third)),
				),
			},
			{
				ResourceName:            "truenas_filesystem_file.test",
				ImportState:             true,
				ImportStateId:           path,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"content_wo_version"},
			},
			{
				Config: acctest.ProviderConfig() + testAccFilesystemFileConfig(datasetName, "", "", ""),
				Check:  testAccCheckFileStat(path, "0600", len(third)),
			},
		},
	})
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
