/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/extractor/tree.go
 * Description: Provides gitignore-aware directory traversal for the entity
 *              extractor so ignored build outputs and .git metadata are skipped.
 *
 * Exported Types / Functions:
 * - IgnoreMatcher: Evaluates relative file paths against .gitignore glob patterns.
 *   - NewIgnoreMatcher(): Loads .gitignore rules from a repository root directory.
 *   - Match():            Reports whether a relative path matches any ignore rule.
 * - Walk():        Traverses a repository tree while pruning ignored files and directories.
 */

package extractor

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreMatcher holds simple gitignore rules for pruning directory walks.
type IgnoreMatcher struct {
	rules []string // Trimmed non-comment glob patterns loaded from .gitignore.
}

// NewIgnoreMatcher loads .gitignore patterns from root if a .gitignore file is present.
func NewIgnoreMatcher(root string) *IgnoreMatcher {
	// --- Step 1: Initialize matcher and open .gitignore if present ---
	m := &IgnoreMatcher{}
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err == nil {
		defer f.Close()
		// --- Step 2: Parse non-empty, non-comment pattern lines ---
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				m.rules = append(m.rules, line)
			}
		}
	}
	return m
}

// Match reports whether the relative path (with directory indicator isDir) should be ignored.
func (m *IgnoreMatcher) Match(path string, isDir bool) bool {
	// --- Step 1: Always ignore the .git metadata directory ---
	base := filepath.Base(path)
	if base == ".git" {
		return true
	}

	// --- Step 2: Check basename and relative path against configured glob rules ---
	for _, rule := range m.rules {
		rule = strings.TrimSuffix(rule, "/")
		if match, _ := filepath.Match(rule, base); match {
			return true
		}
		if strings.Contains(rule, "/") {
			if match, _ := filepath.Match(rule, path); match {
				return true
			}
		}
	}
	return false
}

// Walk traverses files under root, skipping paths matched by the repository's .gitignore rules.
func Walk(root string, fn func(path string, d fs.DirEntry) error) error {
	// --- Step 1: Build the gitignore matcher for root ---
	m := NewIgnoreMatcher(root)

	// --- Step 2: Walk the directory tree and prune ignored entries ---
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return fn(path, d)
		}
		if m.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return fn(path, d)
	})
}
