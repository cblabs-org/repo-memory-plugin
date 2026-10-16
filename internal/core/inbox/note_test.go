package inbox_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
)

func TestInboxStoreLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-inbox"
	store := inbox.NewStore(dataDir, repoID)

	// Empty store returns empty slices
	all, err := store.ListAll()
	if err != nil || len(all) != 0 {
		t.Fatalf("expected empty ListAll, got %v, %v", all, err)
	}

	t1 := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC)

	note1 := &inbox.Note{
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-1",
		StepsCovered:   [2]int{1, 15},
		CreatedAt:      t2, // intentionally later timestamp to test sorting
		DedupKey:       "conv-1:15",
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeGotcha,
				Title: "SQLite WAL busy timeout required",
				Why:   "Concurrent readers/writers lock without busy_timeout",
				Do:    "Always set busy_timeout(5000)",
				Scope: "index",
			},
		},
	}

	id1, err := store.Write(note1)
	if err != nil {
		t.Fatalf("store.Write(note1) failed: %v", err)
	}
	if !strings.HasPrefix(id1, "note-") {
		t.Errorf("expected note- ULID prefix, got %s", id1)
	}

	note2 := &inbox.Note{
		Source:         inbox.SourceCheckpoint,
		ConversationID: "conv-1",
		CreatedAt:      t1,
		DedupKey:       "checkpoint:conv-1:8",
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeDecision,
				Title: "Use ULIDs for note filenames",
				Scope: "inbox",
			},
		},
	}
	id2, err := store.Write(note2)
	if err != nil {
		t.Fatalf("store.Write(note2) failed: %v", err)
	}

	// Read back note1
	read1, err := store.Read(id1)
	if err != nil {
		t.Fatalf("store.Read(%s) failed: %v", id1, err)
	}
	if read1.Source != inbox.SourceTurnNote || len(read1.Items) != 1 || read1.Items[0].Title != "SQLite WAL busy timeout required" {
		t.Errorf("unexpected read1: %+v", read1)
	}

	// ListAll should return note2 (t1) before note1 (t2)
	all, err = store.ListAll()
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(all) != 2 || all[0].ID != id2 || all[1].ID != id1 {
		t.Errorf("expected sorted order [%s, %s], got [%s, %s]", id2, id1, all[0].ID, all[1].ID)
	}

	// ListUnconsolidated should return both
	uncons, err := store.ListUnconsolidated()
	if err != nil || len(uncons) != 2 {
		t.Fatalf("expected 2 unconsolidated notes, got %d (%v)", len(uncons), err)
	}

	// MarkConsolidated on id2
	if err := store.MarkConsolidated(id2); err != nil {
		t.Fatalf("MarkConsolidated failed: %v", err)
	}

	uncons, err = store.ListUnconsolidated()
	if err != nil || len(uncons) != 1 || uncons[0].ID != id1 {
		t.Fatalf("expected only %s unconsolidated, got %+v", id1, uncons)
	}

	// HasDedupKey checks
	has, err := store.HasDedupKey("checkpoint:conv-1:8")
	if err != nil || !has {
		t.Errorf("expected HasDedupKey true for checkpoint:conv-1:8, got %v, %v", has, err)
	}
	hasMissing, err := store.HasDedupKey("non-existent-key")
	if err != nil || hasMissing {
		t.Errorf("expected HasDedupKey false for non-existent-key, got %v, %v", hasMissing, err)
	}
}

func TestCheckAnchors(t *testing.T) {
	repoRoot := t.TempDir()
	pkgDir := filepath.Join(repoRoot, "internal", "core")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(pkgDir, "service.go")
	if err := os.WriteFile(filePath, []byte("package core\n"), 0644); err != nil {
		t.Fatal(err)
	}

	anchors := []string{
		"internal/core/service.go",
		"internal/core/service.go#RunService",
		"internal/core/service.go:42",
		"internal/core/missing.go#Symbol",
		"../../etc/passwd",
	}

	checks := inbox.CheckAnchors(repoRoot, anchors)
	if len(checks) != 5 {
		t.Fatalf("expected 5 anchor checks, got %d", len(checks))
	}

	wantExists := []bool{true, true, true, false, false}
	for i, want := range wantExists {
		if checks[i].Exists != want {
			t.Errorf("anchor %q: got Exists=%v, want %v", checks[i].Anchor, checks[i].Exists, want)
		}
		if checks[i].VerifiedAt.IsZero() {
			t.Errorf("anchor %q: expected non-zero VerifiedAt", checks[i].Anchor)
		}
	}
}
