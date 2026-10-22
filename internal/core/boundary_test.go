/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package core_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var forbiddenAdapters = []string{
	"internal/harness/antigravity",
	"internal/harness/claudecode",
	"internal/harness/fake",
}

func findModuleRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func TestCoreImportBoundaries(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	root, err := findModuleRoot(wd)
	if err != nil {
		t.Fatalf("failed to locate module root (go.mod): %v", err)
	}

	coreDir := filepath.Join(root, "internal", "core")
	fset := token.NewFileSet()
	filesScanned := 0

	err = filepath.Walk(coreDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if filepath.Base(path) == "boundary_test.go" {
			return nil
		}

		filesScanned++
		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("failed to parse %s: %v", path, err)
			return nil
		}

		relPath, _ := filepath.Rel(root, path)

		isTestFile := strings.HasSuffix(path, "_test.go")

		for _, imp := range node.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				importPath = imp.Path.Value
			}

			for _, forbidden := range forbiddenAdapters {
				if strings.Contains(importPath, forbidden) {
					// Tests under internal/core are permitted to import internal/harness/fake (SPEC §18.6 Rule 6).
					// Production code in internal/core must never import any adapter (including fake).
					// No code in internal/core (test or production) may import antigravity or claudecode.
					if isTestFile && forbidden == "internal/harness/fake" {
						continue
					}
					t.Errorf("boundary violation in %s: imports forbidden adapter %q", relPath, importPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("error walking core directory: %v", err)
	}

	if filesScanned == 0 {
		t.Fatalf("expected to scan .go files under %s, but scanned 0", coreDir)
	}

	t.Logf("Successfully scanned %d Go files in internal/core/... with no boundary violations", filesScanned)
}
