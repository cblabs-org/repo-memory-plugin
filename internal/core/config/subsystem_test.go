package config

import (
	"testing"
)

func TestSubsystemAssignment(t *testing.T) {
	cfg := RepoConfig{
		Subsystems: map[string]SubsystemConfig{
			"core": {
				Globs: []string{"internal/core/*", "internal/core/*/*"},
			},
			"api": {
				Globs: []string{"api/*"},
			},
		},
	}

	tests := []struct {
		file     string
		expected string
	}{
		{"internal/core/foo.go", "core"},
		{"internal/core/config/config.go", "core"},
		{"api/server.go", "api"},
		{"cmd/main.go", ""},
	}

	for _, tt := range tests {
		got := cfg.AssignSubsystem(tt.file)
		if got != tt.expected {
			t.Errorf("AssignSubsystem(%q) = %q, want %q", tt.file, got, tt.expected)
		}
	}
}

func TestUnassignedFiles(t *testing.T) {
	cfg := RepoConfig{
		Subsystems: map[string]SubsystemConfig{
			"core": {
				Globs: []string{"internal/core/*"},
			},
		},
	}

	files := []string{
		"internal/core/foo.go",
		"cmd/main.go",
		"README.md",
	}

	unassigned := cfg.UnassignedFiles(files)
	if len(unassigned) != 2 {
		t.Errorf("expected 2 unassigned files, got %d", len(unassigned))
	}
	if unassigned[0] != "cmd/main.go" || unassigned[1] != "README.md" {
		t.Errorf("unexpected unassigned files: %v", unassigned)
	}
}
