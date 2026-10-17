/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/writer.go
 * Description: Filesystem persistence for rendered Markdown dossiers, retired
 *              entry history files (<scope>.history.md), and .memory/ATLAS.md,
 *              including orphan dossier cleanup.
 *
 * Exported Types / Functions:
 * - Writer:         Writes rendered dossier Markdown and ATLAS.md files under a base directory.
 *   - NewWriter():  Creates a Writer rooted at baseDir.
 *   - WriteDossier(): Writes or removes a single dossier's active and history Markdown files.
 *   - WriteAtlas(): Writes the repository-wide ATLAS.md file.
 *   - WriteAll():   Writes all active/history dossiers, prunes stale .md files, and updates ATLAS.md.
 */

package dossiers

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Writer persists rendered dossier Markdown files and ATLAS.md under a base
// memory directory (typically <repoRoot>/.memory).
type Writer struct {
	baseDir string // Root memory directory containing ATLAS.md and the dossiers/ subdirectory.
}

// NewWriter returns a new Writer rooted at baseDir.
func NewWriter(baseDir string) *Writer {
	return &Writer{
		baseDir: baseDir,
	}
}

// WriteDossier renders and writes the active (<scope>.md) and optional historical
// (<scope>.history.md) Markdown files for d. If d has no active entries, any
// pre-existing active <scope>.md file is removed.
func (w *Writer) WriteDossier(d *Dossier) error {
	// --- Step 1: Resolve target file paths for active and history Markdown ---
	dossiersDir := filepath.Join(w.baseDir, "dossiers")
	scopeRel := filepath.FromSlash(d.Scope)
	scopePath := filepath.Join(dossiersDir, scopeRel+".md")
	historyPath := filepath.Join(dossiersDir, scopeRel+".history.md")

	// --- Step 2: Handle empty active entries by removing stale active file and writing history ---
	if len(d.Entries) == 0 {
		if err := os.Remove(scopePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if len(d.History) > 0 {
			if err := os.MkdirAll(filepath.Dir(historyPath), 0755); err != nil {
				return err
			}
			renderedHistory := RenderHistory(d)
			if err := os.WriteFile(historyPath, []byte(renderedHistory), 0644); err != nil {
				return err
			}
		}
		return nil
	}

	// --- Step 3: Render and write the active dossier file and optional history file ---
	if err := os.MkdirAll(filepath.Dir(scopePath), 0755); err != nil {
		return err
	}

	rendered := RenderDossier(d)
	if err := os.WriteFile(scopePath, []byte(rendered), 0644); err != nil {
		return err
	}

	if len(d.History) > 0 {
		renderedHistory := RenderHistory(d)
		if err := os.WriteFile(historyPath, []byte(renderedHistory), 0644); err != nil {
			return err
		}
	}

	return nil
}

// WriteAtlas renders and writes the repository-wide ATLAS.md file in w.baseDir.
func (w *Writer) WriteAtlas(atlas *Atlas) error {
	rendered := RenderAtlas(atlas)
	atlasPath := filepath.Join(w.baseDir, "ATLAS.md")
	return os.WriteFile(atlasPath, []byte(rendered), 0644)
}

// WriteAll renders all non-empty dossiers in the map, removes any orphaned
// .md files and empty subdirectories under dossiers/, and writes ATLAS.md.
func (w *Writer) WriteAll(dossiers map[string]*Dossier) error {
	// --- Step 1: Write all non-empty dossiers and build the Atlas summary ---
	atlas := &Atlas{}
	dossiersDir := filepath.Join(w.baseDir, "dossiers")
	expectedFiles := make(map[string]bool)

	for _, d := range dossiers {
		if len(d.Entries) == 0 && len(d.History) == 0 {
			continue
		}
		if err := w.WriteDossier(d); err != nil {
			return err
		}

		scopeRel := filepath.FromSlash(d.Scope)
		if len(d.History) > 0 {
			expectedFiles[filepath.Clean(filepath.Join(dossiersDir, scopeRel+".history.md"))] = true
		}
		if len(d.Entries) == 0 {
			continue
		}
		expectedFiles[filepath.Clean(filepath.Join(dossiersDir, scopeRel+".md"))] = true

		var lastUpdated time.Time
		for _, e := range d.Entries {
			if e.UpdatedAt.After(lastUpdated) {
				lastUpdated = e.UpdatedAt
			}
		}

		atlas.Dossiers = append(atlas.Dossiers, AtlasEntry{
			Scope:       d.Scope,
			Kind:        d.Kind,
			Title:       d.Title,
			EntryCount:  len(d.Entries),
			LastUpdated: lastUpdated,
		})
	}

	// --- Step 2: Prune orphaned .md files and empty directories under dossiers/ ---
	if info, err := os.Stat(dossiersDir); err == nil && info.IsDir() {
		var dirs []string
		_ = filepath.WalkDir(dossiersDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			cleanPath := filepath.Clean(path)
			if entry.IsDir() {
				if cleanPath != filepath.Clean(dossiersDir) {
					dirs = append(dirs, cleanPath)
				}
				return nil
			}
			if strings.HasSuffix(entry.Name(), ".md") && !expectedFiles[cleanPath] {
				_ = os.Remove(cleanPath)
			}
			return nil
		})
		for i := len(dirs) - 1; i >= 0; i-- {
			entries, readErr := os.ReadDir(dirs[i])
			if readErr == nil && len(entries) == 0 {
				_ = os.Remove(dirs[i])
			}
		}
	}

	// --- Step 3: Write the updated ATLAS.md index ---
	return w.WriteAtlas(atlas)
}
