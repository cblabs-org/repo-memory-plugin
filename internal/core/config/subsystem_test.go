package config

import (
	"testing"
)

func TestAssignSubsystem(t *testing.T) {
	cfg, err := LoadRepoConfig("../../../")
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}

	tests := []struct {
		file      string
		subsystem string
	}{
		{"internal/core/config/config.go", "core-infrastructure"},
		{"internal/core/pipeline/compact/compact.go", "transcript-episode"},
		{"internal/harness/antigravity/hooks.go", "capture-runtime"},
		{"cmd/repo-memory/main.go", "cli"},
		{"cmd/repo-memory/commands/import.go", "cli"},
		{"unknown/file.go", ""},
	}

	for _, tt := range tests {
		got := cfg.AssignSubsystem(tt.file)
		if got != tt.subsystem {
			t.Errorf("AssignSubsystem(%q) = %q; want %q", tt.file, got, tt.subsystem)
		}
	}
}
