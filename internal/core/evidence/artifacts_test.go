package evidence

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactDir(t *testing.T) {
	dataDir := "/custom/data"
	repoID := "repo-xyz"
	convID := "conv-123"

	got := ArtifactDir(dataDir, repoID, convID)
	expected := filepath.Join(dataDir, repoID, "evidence", "artifacts", convID)
	if got != expected {
		t.Fatalf("ArtifactDir(%q, %q, %q) = %q; want %q", dataDir, repoID, convID, got, expected)
	}
}

func TestCopyArtifacts(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 1. Valid markdown artifacts
	doc1Path := filepath.Join(srcDir, "architecture.md")
	if err := os.WriteFile(doc1Path, []byte("# Architecture\nDetailed plan."), 0644); err != nil {
		t.Fatalf("writing doc1: %v", err)
	}

	doc2Path := filepath.Join(srcDir, "decisions.md")
	if err := os.WriteFile(doc2Path, []byte("# Decisions\nKey points."), 0644); err != nil {
		t.Fatalf("writing doc2: %v", err)
	}

	// 2. Metadata files (*.metadata.json and .metadata.json)
	meta1Path := filepath.Join(srcDir, "architecture.md.metadata.json")
	if err := os.WriteFile(meta1Path, []byte(`{"title":"Architecture"}`), 0644); err != nil {
		t.Fatalf("writing meta1: %v", err)
	}

	globalMetaPath := filepath.Join(srcDir, ".metadata.json")
	if err := os.WriteFile(globalMetaPath, []byte(`{"version":"1"}`), 0644); err != nil {
		t.Fatalf("writing globalMeta: %v", err)
	}

	// 3. System generated files (should be skipped)
	sysDir := filepath.Join(srcDir, ".system_generated", "logs")
	if err := os.MkdirAll(sysDir, 0755); err != nil {
		t.Fatalf("creating sysDir: %v", err)
	}
	sysTranscript := filepath.Join(sysDir, "transcript.jsonl")
	if err := os.WriteFile(sysTranscript, []byte("{\"step\":1}\n"), 0644); err != nil {
		t.Fatalf("writing sysTranscript: %v", err)
	}
	sysMd := filepath.Join(srcDir, ".system_generated", "system-notes.md")
	if err := os.WriteFile(sysMd, []byte("System notes inside .system_generated"), 0644); err != nil {
		t.Fatalf("writing sysMd: %v", err)
	}

	// 4. Non-matching files (should be skipped)
	txtPath := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("plain text"), 0644); err != nil {
		t.Fatalf("writing txt: %v", err)
	}
	pngPath := filepath.Join(srcDir, "diagram.png")
	if err := os.WriteFile(pngPath, []byte("binary data"), 0644); err != nil {
		t.Fatalf("writing png: %v", err)
	}

	// 5. File larger than 1 MB (should be skipped)
	largePath := filepath.Join(srcDir, "huge.md")
	largeData := bytes.Repeat([]byte("a"), 1024*1024+10) // > 1 MB
	if err := os.WriteFile(largePath, largeData, 0644); err != nil {
		t.Fatalf("writing huge.md: %v", err)
	}

	// Execute CopyArtifacts
	copied, err := CopyArtifacts(srcDir, dstDir)
	if err != nil {
		t.Fatalf("CopyArtifacts failed: %v", err)
	}

	if copied != 4 {
		t.Fatalf("expected 4 files copied, got %d", copied)
	}

	// Verify expected files exist in dstDir
	expectedFiles := []string{
		"architecture.md",
		"decisions.md",
		"architecture.md.metadata.json",
		".metadata.json",
	}
	for _, rel := range expectedFiles {
		p := filepath.Join(dstDir, rel)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist in destination", rel)
		}
	}

	// Verify excluded files do NOT exist in dstDir
	excludedFiles := []string{
		"notes.txt",
		"diagram.png",
		"huge.md",
		filepath.Join(".system_generated", "system-notes.md"),
		filepath.Join(".system_generated", "logs", "transcript.jsonl"),
	}
	for _, rel := range excludedFiles {
		p := filepath.Join(dstDir, rel)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("file %s should NOT have been copied to destination", rel)
		}
	}

	// Idempotency: run again, should copy 0 files
	copiedAgain, err := CopyArtifacts(srcDir, dstDir)
	if err != nil {
		t.Fatalf("second CopyArtifacts call failed: %v", err)
	}
	if copiedAgain != 0 {
		t.Fatalf("expected 0 files copied on second run, got %d", copiedAgain)
	}

	// Update one file (change size) and re-run: only updated file should be copied
	if err := os.WriteFile(doc1Path, []byte("# Architecture\nUpdated plan with more details!"), 0644); err != nil {
		t.Fatalf("updating doc1: %v", err)
	}

	copiedUpdated, err := CopyArtifacts(srcDir, dstDir)
	if err != nil {
		t.Fatalf("third CopyArtifacts call failed: %v", err)
	}
	if copiedUpdated != 1 {
		t.Fatalf("expected 1 file copied after update, got %d", copiedUpdated)
	}

	// Check content updated
	dstDoc1, err := os.ReadFile(filepath.Join(dstDir, "architecture.md"))
	if err != nil {
		t.Fatalf("reading updated dst doc1: %v", err)
	}
	if string(dstDoc1) != "# Architecture\nUpdated plan with more details!" {
		t.Fatalf("destination file content was not updated: %q", string(dstDoc1))
	}
}
