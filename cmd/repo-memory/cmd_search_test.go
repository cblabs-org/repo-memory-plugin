/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"bytes"
		"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	)

func TestSearchCommand(t *testing.T) {
	tempDir := t.TempDir()

	os.Setenv("REPO_MEMORY_DATA_DIR", filepath.Join(tempDir, "data"))
	defer os.Unsetenv("REPO_MEMORY_DATA_DIR")

	// Initialize git repo so gitops works
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create memory layout
	memDir := filepath.Join(tempDir, ".memory", "views", "index")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	repoID, _ := gitops.GenerateRepoID(tempDir)
	dataDir := filepath.Join(tempDir, "data", repoID)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = filepath.Join(dataDir, "episodes_fts.db")

	store := episode.NewStore(filepath.Join(tempDir, "data"), repoID)
	ep := episode.NewEpisode(episode.Header{
		EpisodeID: "ep-123",
		SessionID: "sess-abc",
		Started:   time.Now(),
	})
	
	// manually add step to bypass the empty check and parse correctly
	ep.Steps = []harness.Step{
		{Text: "Added a new feature for searching", File: "cmd_search.go"},
	}
	if _, err := store.WriteEpisode(ep); err != nil {
		t.Fatal(err)
	}

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
