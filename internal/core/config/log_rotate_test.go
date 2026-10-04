package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRotateLogFile_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// 1. Below threshold: no rotation
	if err := os.WriteFile(logPath, []byte("short content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RotateLogFile(logPath, 100, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Errorf("expected no .1 file when below threshold")
	}

	// 2. First rotation: > 100 bytes
	largeData := make([]byte, 150)
	if err := os.WriteFile(logPath, largeData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := RotateLogFile(logPath, 100, 3); err != nil {
		t.Fatalf("first rotation failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected .1 file after first rotation")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected original log file to be renamed")
	}

	// 3. Second rotation
	if err := os.WriteFile(logPath, largeData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := RotateLogFile(logPath, 100, 3); err != nil {
		t.Fatalf("second rotation failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".2"); err != nil {
		t.Errorf("expected .2 file after second rotation")
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected .1 file after second rotation")
	}

	// 4. Third rotation
	if err := os.WriteFile(logPath, largeData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := RotateLogFile(logPath, 100, 3); err != nil {
		t.Fatalf("third rotation failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".3"); err != nil {
		t.Errorf("expected .3 file after third rotation")
	}
	if _, err := os.Stat(logPath + ".2"); err != nil {
		t.Errorf("expected .2 file after third rotation")
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected .1 file after third rotation")
	}

	// 5. Fourth rotation: .3 is deleted/replaced, keeping max 3 rotated files
	if err := os.WriteFile(logPath, largeData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := RotateLogFile(logPath, 100, 3); err != nil {
		t.Fatalf("fourth rotation failed: %v", err)
	}
	if _, err := os.Stat(logPath + ".4"); !os.IsNotExist(err) {
		t.Errorf("expected no .4 file, maxFiles=3")
	}
	if _, err := os.Stat(logPath + ".3"); err != nil {
		t.Errorf("expected .3 file to exist")
	}
}

func TestCleanProbeLogs(t *testing.T) {
	tempDir := t.TempDir()

	probe1 := filepath.Join(tempDir, "probe_event_1.json")
	probe2 := filepath.Join(tempDir, "probe_event_1_transcript.jsonl")
	probe3 := filepath.Join(tempDir, "custom.probe")
	nonProbe := filepath.Join(tempDir, "registry.json")

	_ = os.WriteFile(probe1, []byte("{}"), 0644)
	_ = os.WriteFile(probe2, []byte("{}"), 0644)
	_ = os.WriteFile(probe3, []byte("{}"), 0644)
	_ = os.WriteFile(nonProbe, []byte("{}"), 0644)

	// Clean all probes (olderThan = 0)
	deleted, err := CleanProbeLogs(tempDir, 0)
	if err != nil {
		t.Fatalf("CleanProbeLogs failed: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted probe files, got %d", deleted)
	}

	if _, err := os.Stat(nonProbe); err != nil {
		t.Errorf("non-probe file was unexpectedly deleted")
	}
	if _, err := os.Stat(probe1); !os.IsNotExist(err) {
		t.Errorf("probe1 was not deleted")
	}

	// Test with olderThan duration
	probeOld := filepath.Join(tempDir, "probe_old.json")
	_ = os.WriteFile(probeOld, []byte("{}"), 0644)
	// Set mtime to 10 days ago
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(probeOld, oldTime, oldTime)

	probeNew := filepath.Join(tempDir, "probe_new.json")
	_ = os.WriteFile(probeNew, []byte("{}"), 0644)

	// Clean older than 7 days
	deleted, err = CleanProbeLogs(tempDir, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CleanProbeLogs failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted file older than 7 days, got %d", deleted)
	}
	if _, err := os.Stat(probeOld); !os.IsNotExist(err) {
		t.Errorf("expected old probe to be deleted")
	}
	if _, err := os.Stat(probeNew); err != nil {
		t.Errorf("expected new probe to be kept")
	}
}
