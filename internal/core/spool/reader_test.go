package spool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReader_NonExistentDir(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "does-not-exist")

	reader := NewReader(spoolDir)
	items, err := reader.ReadEntries()
	if err != nil {
		t.Fatalf("expected nil error for non-existent dir, got %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestReader_FiltersTempAndHiddenFiles(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")
	if err := os.MkdirAll(spoolDir, 0755); err != nil {
		t.Fatal(err)
	}

	writer := NewWriter(spoolDir)
	entry := NewCaptureEntry("antigravity", "conv-real", "/path", "/repo", "main", "h1")
	if _, err := writer.Write(entry); err != nil {
		t.Fatalf("writer.Write failed: %v", err)
	}

	// Create noise files
	_ = os.WriteFile(filepath.Join(spoolDir, ".spool_tmp_12345"), []byte("in progress"), 0644)
	_ = os.WriteFile(filepath.Join(spoolDir, ".hidden.json"), []byte(`{"id":"hidden"}`), 0644)
	_ = os.WriteFile(filepath.Join(spoolDir, "notes.txt"), []byte("random text"), 0644)
	_ = os.WriteFile(filepath.Join(spoolDir, "unfinished.json.tmp"), []byte("partial"), 0644)

	reader := NewReader(spoolDir)
	items, err := reader.ReadEntries()
	if err != nil {
		t.Fatalf("ReadEntries failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected exactly 1 valid item, got %d", len(items))
	}

	if items[0].Entry.ConversationID != "conv-real" {
		t.Errorf("expected conv-real, got %s", items[0].Entry.ConversationID)
	}
}

func TestReader_ChronologicalOrder(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")

	writer := NewWriter(spoolDir)
	now := time.Now().UTC()

	e1 := NewCaptureEntry("antigravity", "conv-1", "/p1", "/repo", "main", "h1")
	e1.Timestamp = now
	e2 := NewCaptureEntry("antigravity", "conv-2", "/p2", "/repo", "main", "h2")
	e2.Timestamp = now.Add(2 * time.Second)
	e3 := NewCaptureEntry("antigravity", "conv-3", "/p3", "/repo", "main", "h3")
	e3.Timestamp = now.Add(4 * time.Second)

	// Write in out-of-order sequence
	if _, err := writer.Write(e2); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(e1); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(e3); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(spoolDir)
	items, err := reader.ReadEntries()
	if err != nil {
		t.Fatalf("ReadEntries failed: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if items[0].Entry.ConversationID != "conv-1" ||
		items[1].Entry.ConversationID != "conv-2" ||
		items[2].Entry.ConversationID != "conv-3" {
		t.Errorf("items not in chronological order: [0]=%s, [1]=%s, [2]=%s",
			items[0].Entry.ConversationID, items[1].Entry.ConversationID, items[2].Entry.ConversationID)
	}
}

func TestReader_Acknowledge(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")

	writer := NewWriter(spoolDir)
	entry := NewCaptureEntry("antigravity", "conv-ack", "/path", "/repo", "main", "h1")
	path, err := writer.Write(entry)
	if err != nil {
		t.Fatal(err)
	}

	reader := NewReader(spoolDir)
	if err := reader.Acknowledge(path); err != nil {
		t.Fatalf("Acknowledge failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file still exists after Acknowledge")
	}

	// Idempotent acknowledge
	if err := reader.Acknowledge(path); err != nil {
		t.Errorf("second acknowledge failed: %v", err)
	}
}

func TestReader_Ingest(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")

	writer := NewWriter(spoolDir)
	now := time.Now().UTC()

	e1 := NewCaptureEntry("antigravity", "c1", "/p1", "/repo", "main", "h1")
	e1.Timestamp = now
	e2 := NewCaptureEntry("antigravity", "c2", "/p2", "/repo", "main", "h2")
	e2.Timestamp = now.Add(time.Second)

	if _, err := writer.Write(e1); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(e2); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(spoolDir)

	// Ingest with error on c2
	failErr := errors.New("database locked")
	processed, err := reader.Ingest(func(item Item) error {
		if item.Entry.ConversationID == "c2" {
			return failErr
		}
		return nil
	})

	if !errors.Is(err, failErr) {
		t.Fatalf("expected failErr, got %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 processed item, got %d", processed)
	}

	// c1 should be acknowledged (gone), c2 should still be in spool
	remaining, err := reader.ReadEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 item remaining, got %d", len(remaining))
	}
	if remaining[0].Entry.ConversationID != "c2" {
		t.Errorf("expected c2 remaining, got %s", remaining[0].Entry.ConversationID)
	}

	// Now succeed on c2
	processed2, err := reader.Ingest(func(item Item) error {
		return nil
	})
	if err != nil {
		t.Fatalf("second ingest failed: %v", err)
	}
	if processed2 != 1 {
		t.Errorf("expected 1 item processed, got %d", processed2)
	}

	final, _ := reader.ReadEntries()
	if len(final) != 0 {
		t.Errorf("expected empty spool, got %d", len(final))
	}
}
