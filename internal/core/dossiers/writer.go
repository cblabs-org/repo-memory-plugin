package dossiers

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Writer struct {
	baseDir string
}

func NewWriter(baseDir string) *Writer {
	return &Writer{
		baseDir: baseDir,
	}
}

func (w *Writer) WriteDossier(d *Dossier) error {
	dossiersDir := filepath.Join(w.baseDir, "dossiers")
	scopeRel := filepath.FromSlash(d.Scope)
	scopePath := filepath.Join(dossiersDir, scopeRel+".md")
	historyPath := filepath.Join(dossiersDir, scopeRel+".history.md")

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

func (w *Writer) WriteAtlas(atlas *Atlas) error {
	rendered := RenderAtlas(atlas)
	atlasPath := filepath.Join(w.baseDir, "ATLAS.md")
	return os.WriteFile(atlasPath, []byte(rendered), 0644)
}

func (w *Writer) WriteAll(dossiers map[string]*Dossier) error {
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

	return w.WriteAtlas(atlas)
}
