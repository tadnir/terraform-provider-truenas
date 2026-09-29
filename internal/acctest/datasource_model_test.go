// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestDataSourceModelMatchesSchema guards against the class of bug in issue #23:
// a data source whose model struct has a tfsdk field the data source schema does
// not declare fails every read with "mismatch between struct and object: Struct
// defines fields not found in object: <field>". This commonly happens when a data
// source reuses a resource model that carries a write-only field (a secret the
// API never returns), which belongs on the resource but not the read-only data
// source. Source scan, no live box.
func TestDataSourceModelMatchesSchema(t *testing.T) {
	root := repoRoot(t)
	dsFiles, err := filepath.Glob(filepath.Join(root, "internal", "resources", "*", "datasource.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	modelVarRe := regexp.MustCompile(`var\s+\w+\s+(\w+Model)\b`)
	schemaAttrRe := regexp.MustCompile(`"([a-z0-9_]+)":\s*d?schema\.`)
	tagRe := regexp.MustCompile(`tfsdk:"([a-z0-9_]+)"`)

	for _, dsFile := range dsFiles {
		dir := filepath.Dir(dsFile)
		res := filepath.Base(dir)
		dsSrc, err := os.ReadFile(dsFile)
		if err != nil {
			t.Fatalf("read %s: %v", dsFile, err)
		}
		mv := modelVarRe.FindStringSubmatch(string(dsSrc))
		if mv == nil {
			continue // no obvious model variable; skip
		}
		modelType := mv[1]

		pkgFiles, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		schemaAttrs := map[string]bool{}
		var modelTags []string
		structRe := regexp.MustCompile(`(?s)type ` + modelType + ` struct \{(.*?)\n\}`)
		for _, f := range pkgFiles {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			s := string(b)
			for _, m := range schemaAttrRe.FindAllStringSubmatch(s, -1) {
				schemaAttrs[m[1]] = true
			}
			if modelTags == nil {
				if sm := structRe.FindStringSubmatch(s); sm != nil {
					for _, m := range tagRe.FindAllStringSubmatch(sm[1], -1) {
						modelTags = append(modelTags, m[1])
					}
				}
			}
		}
		if modelTags == nil {
			continue // struct not found by the heuristic
		}
		var missing []string
		for _, tag := range modelTags {
			if !schemaAttrs[tag] {
				missing = append(missing, tag)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s data source: model %s has field(s) not in the data source schema: %s "+
				"— reads will fail with \"Struct defines fields not found in object\" (issue #23). "+
				"Add them to the schema (Computed) or drop them from the data source model.",
				res, modelType, strings.Join(missing, ", "))
		}
	}
}
