package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplayHarness_InjectCondition(t *testing.T) {
	tempRepo := t.TempDir()
	tempDataDir := t.TempDir()

	harness := NewReplayHarness(tempRepo, tempDataDir)

	// Test ConditionNoMemory
	err := harness.InjectCondition(ConditionNoMemory, "", "")
	if err != nil {
		t.Fatalf("unexpected error injecting NoMemory: %v", err)
	}

	// Verify repo config was created and set to false
	cfgFile := filepath.Join(tempRepo, ".memory", "config.yaml")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Fatalf("expected repo config to be created")
	}

	// Read content
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error reading repo config: %v", err)
	}
	if string(data) != "enabled: false\n" {
		t.Errorf("expected enabled: false, got %s", string(data))
	}
}

func TestReplayHarness_RunWorker_DryRun(t *testing.T) {
	// A full dry run of RunWorker isn't entirely practical here without mocking agy,
	// but we can ensure it doesn't crash on setup.
	tempRepo := t.TempDir()
	tempDataDir := t.TempDir()

	harness := NewReplayHarness(tempRepo, tempDataDir)
	
	// We won't actually call RunWorker because it would try to run agy.
	_ = harness
}
