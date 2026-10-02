package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)

func TestImportPACmd(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize git repo so gitops works
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Override HOME/XDG_DATA_HOME for config.DataDir()
	origXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tempDir)
	defer os.Setenv("XDG_DATA_HOME", origXDG)
	
	// Create test files
	relDir := filepath.Join(".agents", "skills", "principal-architect-workspace")
	paDir := filepath.Join(tempDir, relDir)
	os.MkdirAll(paDir, 0755)

	testFiles := map[string]string{
		"PROJECT.md": "# Project",
		"WORKORDER.md": "# Work Order",
		"context_bridge.md": "# Context",
		"ignore_me.md": "ignore",
	}

	for name, content := range testFiles {
		os.WriteFile(filepath.Join(paDir, name), []byte(content), 0644)
	}

	// Change working directory to tempDir
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"import-pa", relDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("import-pa command failed: %v", err)
	}

	// Verify episodes created
	repoID, _ := gitops.GenerateRepoID(tempDir)
	store := episode.NewStore("", repoID)
	
	episodes, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list episodes: %v", err)
	}

	if len(episodes) != 3 {
		t.Fatalf("Expected 3 episodes, got %d", len(episodes))
	}

	for _, sessionID := range episodes {
		ep, err := store.Read(sessionID)
		if err != nil {
			t.Fatalf("Failed to read episode %s: %v", sessionID, err)
		}
		if ep.Header.Harness != "import" {
			t.Errorf("Expected harness 'import', got %s", ep.Header.Harness)
		}
		if len(ep.Steps) != 1 {
			t.Errorf("Expected 1 step, got %d", len(ep.Steps))
		}
		if ep.Steps[0].Kind != "note" {
			t.Errorf("Expected step kind note, got %s", ep.Steps[0].Kind)
		}
	}
}
