package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
)

func TestLoadAndSaveMark(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-test"
	convID := "conv-1234-5678"

	// Non-existent mark returns default zero mark
	m, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil {
		t.Fatalf("LoadMark failed: %v", err)
	}
	if m.ConversationID != convID || m.StepIndex != 0 || m.NoteCount != 0 {
		t.Errorf("unexpected default mark: %+v", m)
	}
	if m.NoteTitles == nil || len(m.NoteTitles) != 0 {
		t.Errorf("expected empty non-nil NoteTitles slice, got %v", m.NoteTitles)
	}

	now := time.Now().UTC().Truncate(time.Second)
	m.StepIndex = 42
	m.ByteOffset = 10240
	m.StepTimestamp = "2026-10-15T12:00:00Z"
	m.LastNoteAt = now
	m.NoteTitles = []string{"First note", "Second note"}
	m.NoteCount = 2
	m.PendingRequestStep = 40

	if err := state.SaveMark(dataDir, repoID, m); err != nil {
		t.Fatalf("SaveMark failed: %v", err)
	}

	loaded, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil {
		t.Fatalf("LoadMark after SaveMark failed: %v", err)
	}
	if loaded.StepIndex != 42 || loaded.ByteOffset != 10240 || loaded.StepTimestamp != "2026-10-15T12:00:00Z" {
		t.Errorf("loaded mark fields mismatch: %+v", loaded)
	}
	if loaded.NoteCount != 2 || len(loaded.NoteTitles) != 2 || loaded.NoteTitles[0] != "First note" {
		t.Errorf("loaded note metadata mismatch: %+v", loaded)
	}
	if loaded.PendingRequestStep != 40 {
		t.Errorf("expected PendingRequestStep 40, got %d", loaded.PendingRequestStep)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestMarkPathSanitization(t *testing.T) {
	dataDir := "/tmp/data"
	repoID := "repo-1"

	safePath := state.MarkPath(dataDir, repoID, "abc-123_XYZ")
	if !strings.HasSuffix(safePath, filepath.Join("state", "marks", "abc-123_XYZ.json")) {
		t.Errorf("unexpected safe path: %s", safePath)
	}

	traversalPath := state.MarkPath(dataDir, repoID, "../../etc/passwd")
	if strings.Contains(traversalPath, "..") || strings.Contains(traversalPath, "passwd") {
		t.Errorf("expected path traversal ID to be hashed, got: %s", traversalPath)
	}
}

func TestClampIfRewound(t *testing.T) {
	t.Run("zero mark does not clamp", func(t *testing.T) {
		m := &state.Mark{ConversationID: "c1"}
		if m.ClampIfRewound(10, "2026-10-15T12:00:00Z") {
			t.Error("expected false for zero mark")
		}
	})

	t.Run("unchanged step and timestamp does not clamp", func(t *testing.T) {
		m := &state.Mark{
			ConversationID: "c1",
			StepIndex:      10,
			ByteOffset:     500,
			StepTimestamp:  "2026-10-15T12:00:00Z",
		}
		if m.ClampIfRewound(15, "2026-10-15T12:00:00Z") {
			t.Error("expected false when stepCount > StepIndex and timestamp matches")
		}
		if m.StepIndex != 10 || m.ByteOffset != 500 {
			t.Errorf("mark should remain untouched, got %+v", m)
		}
	})

	t.Run("step count shrank clamps mark", func(t *testing.T) {
		m := &state.Mark{
			ConversationID:     "c1",
			StepIndex:          20,
			ByteOffset:         2048,
			StepTimestamp:      "2026-10-15T12:00:00Z",
			PendingRequestStep: 19,
			NoteCount:          3,
		}
		if !m.ClampIfRewound(12, "2026-10-15T12:00:00Z") {
			t.Error("expected true when stepCount <= StepIndex")
		}
		if m.StepIndex != 0 || m.ByteOffset != 0 || m.StepTimestamp != "" || m.PendingRequestStep != 0 {
			t.Errorf("expected mark watermarks reset to zero, got %+v", m)
		}
		if m.NoteCount != 3 {
			t.Errorf("expected NoteCount preserved, got %d", m.NoteCount)
		}
	})

	t.Run("timestamp mismatch at mark step clamps mark", func(t *testing.T) {
		m := &state.Mark{
			ConversationID: "c1",
			StepIndex:      10,
			ByteOffset:     1024,
			StepTimestamp:  "2026-10-15T12:00:00Z",
		}
		if !m.ClampIfRewound(25, "2026-10-15T12:05:00Z") {
			t.Error("expected true when timestamp at mark step changed")
		}
		if m.StepIndex != 0 || m.ByteOffset != 0 || m.StepTimestamp != "" {
			t.Errorf("expected mark watermarks reset, got %+v", m)
		}
	})
}

func TestCountNotesToday(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-notes"
	now := time.Date(2026, 10, 15, 14, 30, 0, 0, time.UTC)
	yesterday := now.Add(-24 * time.Hour)

	// Missing directory returns 0, nil
	c, err := state.CountTurnNotesToday(dataDir, repoID, now)
	if err != nil || c != 0 {
		t.Fatalf("expected 0, nil for missing dir, got %d, %v", c, err)
	}

	notesDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeFixture := func(name, source string, created time.Time) {
		body := `{"id":"` + name + `","source":"` + source + `","created_at":"` + created.Format(time.RFC3339) + `"}`
		if err := os.WriteFile(filepath.Join(notesDir, name+".json"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeFixture("note-1", "turn-note", now)
	writeFixture("note-2", "turn-note", now.Add(-2*time.Hour))
	writeFixture("note-3", "mcp-note", now)
	writeFixture("note-4", "turn-note", yesterday)

	turnCount, err := state.CountTurnNotesToday(dataDir, repoID, now)
	if err != nil {
		t.Fatalf("CountTurnNotesToday failed: %v", err)
	}
	if turnCount != 2 {
		t.Errorf("expected 2 turn-notes today, got %d", turnCount)
	}

	allCount, err := state.CountNotesToday(dataDir, repoID, now)
	if err != nil {
		t.Fatalf("CountNotesToday failed: %v", err)
	}
	if allCount != 3 {
		t.Errorf("expected 3 total notes today, got %d", allCount)
	}
}
