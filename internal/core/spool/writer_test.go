package spool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriter_WriteBasic(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")

	writer := NewWriter(spoolDir)
	entry := NewCaptureEntry("antigravity", "conv-test", "/tmp/transcript.jsonl", "/repo", "main", "c0ffee")

	filePath, err := writer.Write(entry)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if !strings.HasPrefix(filePath, spoolDir) {
		t.Errorf("expected file path to start with %s, got %s", spoolDir, filePath)
	}

	// Verify file content on disk
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var parsed Entry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON from file: %v", err)
	}

	if parsed.ConversationID != "conv-test" {
		t.Errorf("expected conv-test, got %s", parsed.ConversationID)
	}
	if parsed.Kind != KindCapture {
		t.Errorf("expected %s, got %s", KindCapture, parsed.Kind)
	}

	// Verify no temporary files were left behind
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		t.Fatalf("failed to read spool dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in spoolDir, found %d", len(entries))
	}

	if strings.HasPrefix(entries[0].Name(), ".spool_tmp") {
		t.Errorf("temporary file was not cleaned up: %s", entries[0].Name())
	}
}

func TestWriter_MultipleEntriesUniqueAndChronological(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "spool")

	writer := NewWriter(spoolDir)

	now := time.Now().UTC()
	e1 := NewCaptureEntry("antigravity", "c1", "/p1", "/r", "main", "h1")
	e1.Timestamp = now
	e2 := NewBriefEntry("antigravity", "c2", "/r", 1, "test brief")
	e2.Timestamp = now.Add(time.Second)

	p1, err := writer.Write(e1)
	if err != nil {
		t.Fatalf("Write e1 failed: %v", err)
	}
	p2, err := writer.Write(e2)
	if err != nil {
		t.Fatalf("Write e2 failed: %v", err)
	}

	if p1 == p2 {
		t.Errorf("expected distinct paths, got same: %s", p1)
	}

	// Lexicographical ordering test
	name1 := filepath.Base(p1)
	name2 := filepath.Base(p2)
	if name1 >= name2 {
		t.Errorf("expected %s < %s chronologically", name1, name2)
	}
}

func TestWriter_AutoCreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	spoolDir := filepath.Join(tempDir, "nested", "deep", "spool")

	entry := NewCaptureEntry("antigravity", "c3", "/p3", "/r", "main", "h3")
	path, err := Write(spoolDir, entry)
	if err != nil {
		t.Fatalf("Write with auto directory creation failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file does not exist: %v", err)
	}
}
