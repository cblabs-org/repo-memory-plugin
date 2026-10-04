package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestSearchCommand(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize git repo so gitops works
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create memory layout
	memDir := filepath.Join(tempDir, ".memory", "views", "index")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(memDir, "episodes.db")

	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	record := index.EpisodeRecord{
		EpisodeID:    "ep-123",
		SessionID:    "sess-abc",
		Content:      "Added a new feature for searching",
		FilesTouched: "cmd_search.go",
		Outcomes:     "success",
		Date:         time.Now(),
	}
	if err := idx.InsertEpisode(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Change working directory to tempDir so gitops finds the repo root
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"search", "feature"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ep-123") {
		t.Errorf("expected output to contain ep-123, got: %s", output)
	}
	if !strings.Contains(output, "sess-abc") {
		t.Errorf("expected output to contain sess-abc, got: %s", output)
	}
}
