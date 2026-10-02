package extractor

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreMatcher holds simple gitignore rules.
type IgnoreMatcher struct {
	rules []string
}

func NewIgnoreMatcher(root string) *IgnoreMatcher {
	m := &IgnoreMatcher{}
	// Read .gitignore if exists
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err == nil {
		defer f.Close()
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

func (m *IgnoreMatcher) Match(path string, isDir bool) bool {
	// A very basic gitignore matcher for the purpose of the work order.
	base := filepath.Base(path)
	if base == ".git" {
		return true
	}
	for _, rule := range m.rules {
		// remove trailing slash from rule
		rule = strings.TrimSuffix(rule, "/")
		if match, _ := filepath.Match(rule, base); match {
			return true
		}
		// if rule doesn't have slash, also try matching the whole path relative to root?
		// for simplicity, just base matching is often enough for standard ignores in tests.
		if strings.Contains(rule, "/") {
			if match, _ := filepath.Match(rule, path); match {
				return true
			}
		}
	}
	return false
}

// Walk files in root, skipping ignored
func Walk(root string, fn func(path string, d fs.DirEntry) error) error {
	m := NewIgnoreMatcher(root)
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
