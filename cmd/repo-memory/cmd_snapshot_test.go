/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)

func TestSnapshotCommand(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(tempDir, "data")
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Unenrolled repo test
	origWd, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"snapshot", "conv-test-1"})

	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unenrolled repo, got nil")
	}

	// 2. Enroll repo in registry
	if err := config.AddToRegistry(dataDir, []string{repoDir}, repoID, repoDir); err != nil {
		t.Fatal(err)
	}

	// 3. Setup fake brain directory with transcript and artifacts
	brainBase := filepath.Join(tempDir, "brain")
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", brainBase)

	convID := "conv-snap-test"
	convDir := filepath.Join(brainBase, convID)
	sysLogs := filepath.Join(convDir, ".system_generated", "logs")
	if err := os.MkdirAll(sysLogs, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(sysLogs, "transcript_full.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("{\"step\":0}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(convDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Plan\nTest plan content"), 0644); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(convDir, "plan.md.metadata.json")
	if err := os.WriteFile(metaPath, []byte(`{"title":"Plan"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Run snapshot command
	buf.Reset()
	errBuf.Reset()
	rootCmd.SetArgs([]string{"snapshot", convID})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("snapshot command failed: %v (stderr: %s)", err, errBuf.String())
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "Snapshotted conv-snap-test") {
		t.Errorf("unexpected output: %s", outStr)
	}

	// 5. Verify transcript in queue/
	queuePath := filepath.Join(dataDir, repoID, "queue", convID+".jsonl")
	if _, err := os.Stat(queuePath); err != nil {
		t.Errorf("expected queue transcript file %s to exist: %v", queuePath, err)
	}

	// 6. Verify artifacts in evidence/artifacts/<convID>/
	dstPlan := filepath.Join(dataDir, repoID, "evidence", "artifacts", convID, "plan.md")
	if _, err := os.Stat(dstPlan); err != nil {
		t.Errorf("expected artifact file %s to exist: %v", dstPlan, err)
	}
	dstMeta := filepath.Join(dataDir, repoID, "evidence", "artifacts", convID, "plan.md.metadata.json")
	if _, err := os.Stat(dstMeta); err != nil {
		t.Errorf("expected metadata file %s to exist: %v", dstMeta, err)
	}

	// 7. Verify non-existent conversation returns error
	buf.Reset()
	errBuf.Reset()
	rootCmd.SetArgs([]string{"snapshot", "conv-does-not-exist"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for non-existent conversation, got nil")
	}
}
