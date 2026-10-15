package dossiers

import (
	"os"
	"path/filepath"
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
	scopePath := filepath.Join(dossiersDir, d.Scope+".md")
	if err := os.MkdirAll(filepath.Dir(scopePath), 0755); err != nil {
		return err
	}

	rendered := RenderDossier(d)
	if err := os.WriteFile(scopePath, []byte(rendered), 0644); err != nil {
		return err
	}

	if len(d.History) > 0 {
		historyPath := filepath.Join(dossiersDir, d.Scope+".history.md")
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
	
	for _, d := range dossiers {
		if len(d.Entries) == 0 && len(d.History) == 0 {
			continue
		}
		if err := w.WriteDossier(d); err != nil {
			return err
		}

		var lastUpdated time.Time
		for _, e := range d.Entries {
			if e.UpdatedAt.After(lastUpdated) {
				lastUpdated = e.UpdatedAt
			}
		}
		
		atlas.Dossiers = append(atlas.Dossiers, AtlasEntry{
			Scope:      d.Scope,
			Kind:       d.Kind,
			Title:      d.Title,
			EntryCount: len(d.Entries),
			LastUpdated: lastUpdated,
		})
	}

	return w.WriteAtlas(atlas)
}
