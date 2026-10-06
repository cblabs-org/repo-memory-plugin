package dossiers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestDossiersRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)

	now := time.Now().UTC()
	op1 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      ops.NewEntryID("inv-"),
		DossierScope: "core/test-module",
		Timestamp:    now,
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"kind":       "module",
			"title":      "Test Module",
			"purpose":    "Testing dossiers",
			"entry_type": "invariant",
			"text":       "First invariant text\nSecond line",
			"anchors":    []string{"file.go"},
			"verified":   "2026-10-06 (PR-01)",
		},
	}
	op2 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      ops.NewEntryID("rul-"),
		DossierScope: "core/test-module",
		Timestamp:    now.Add(time.Minute),
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"entry_type": "ruled-out",
			"text":       "Ruled out text",
		},
	}

	batch := ops.NewBatch([]ops.Op{op1, op2})
	if err := opsWriter.Write(batch); err != nil {
		t.Fatalf("failed to write batch: %v", err)
	}

	projector := NewProjector(opsReader)
	ctx := context.Background()
	dossiersMap, err := projector.Project(ctx)
	if err != nil {
		t.Fatalf("failed to project: %v", err)
	}

	d, ok := dossiersMap["core/test-module"]
	if !ok {
		t.Fatalf("expected dossier core/test-module")
	}

	rendered1 := RenderDossier(d)
	if rendered1 == "" {
		t.Fatalf("rendered output is empty")
	}

	writer := NewWriter(tempDir)
	if err := writer.WriteAll(dossiersMap); err != nil {
		t.Fatalf("failed to write dossiers: %v", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(tempDir, "dossiers", "core/test-module.md"))
	if err != nil {
		t.Fatalf("failed to read written dossier: %v", err)
	}

	if string(onDisk) != rendered1 {
		t.Fatalf("written file does not match rendered output")
	}
	
	atlasDisk, err := os.ReadFile(filepath.Join(tempDir, "ATLAS.md"))
	if err != nil {
		t.Fatalf("failed to read written ATLAS: %v", err)
	}
	if len(atlasDisk) == 0 {
		t.Fatalf("ATLAS is empty")
	}

	// hand edits detection
	newOps, err := DetectHandEdits(rendered1, string(onDisk))
	if err != nil {
		t.Fatalf("DetectHandEdits error: %v", err)
	}
	if len(newOps) > 0 {
		t.Fatalf("expected 0 ops from hand edits for identical files, got %d", len(newOps))
	}
	
	// linting
	warnings := LintDossier(d)
	if len(warnings) > 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestProjectorOperations(t *testing.T) {
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)

	now := time.Now().UTC()
	id1 := ops.NewEntryID("inv-")
	id2 := ops.NewEntryID("rul-")
	id3 := ops.NewEntryID("dec-")

	opAdd := ops.Op{Type: ops.OpAdd, EntryID: id1, DossierScope: "core/mod1", Timestamp: now, Source: ops.SourceSeed, Fields: map[string]any{"entry_type": "invariant", "text": "v1"}}
	opAmend := ops.Op{Type: ops.OpAmend, EntryID: id1, DossierScope: "core/mod1", Timestamp: now.Add(time.Second), Source: ops.SourceSeed, Fields: map[string]any{"text": "v2"}}
	opSupersede := ops.Op{Type: ops.OpSupersede, EntryID: id1, DossierScope: "core/mod1", Timestamp: now.Add(2*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"new_entry_id": id2, "text": "v3"}}
	opRetire := ops.Op{Type: ops.OpRetire, EntryID: id2, DossierScope: "core/mod1", Timestamp: now.Add(3*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"reason": "obsolete"}}
	
	opAdd2 := ops.Op{Type: ops.OpAdd, EntryID: id3, DossierScope: "core/mod1", Timestamp: now.Add(4*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"entry_type": "decision", "text": "dec1"}}
	opMove := ops.Op{Type: ops.OpMove, EntryID: id3, DossierScope: "core/mod1", Timestamp: now.Add(5*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"to_dossier": "core/mod2"}}

	batch := ops.NewBatch([]ops.Op{opAdd, opAmend, opSupersede, opRetire, opAdd2, opMove})
	opsWriter.Write(batch)

	dossiersMap, _ := NewProjector(opsReader).Project(context.Background())

	d1 := dossiersMap["core/mod1"]
	d2 := dossiersMap["core/mod2"]

	if len(d1.Entries) != 0 {
		t.Errorf("mod1 should have 0 active entries, got %d", len(d1.Entries))
	}
	if len(d1.History) != 2 {
		t.Errorf("mod1 should have 2 history entries, got %d", len(d1.History))
	}
	if len(d2.Entries) != 1 || d2.Entries[0].ID != id3 {
		t.Errorf("mod2 should have id3 entry")
	}
}

func TestDossiersEmptySections(t *testing.T) {
	d := &Dossier{
		Scope: "core/empty",
		Entries: []Entry{
			{ID: "inv-1", Type: EntryTypeInvariant, Text: "Just invariant"},
		},
	}
	rendered := RenderDossier(d)
	if strings.Contains(rendered, "## Decisions") {
		t.Errorf("empty section 'Decisions' was rendered")
	}
	if !strings.Contains(rendered, "## Invariants") {
		t.Errorf("non-empty section 'Invariants' was not rendered")
	}
}
