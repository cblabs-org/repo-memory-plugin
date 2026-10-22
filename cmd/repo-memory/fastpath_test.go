/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

func TestFastPath_NonEnrolledLatency(t *testing.T) {
	nonEnrolledDir := t.TempDir()

	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = nonEnrolledDir

	// Verify PreToolUse fail-open response and latency
	var stdout bytes.Buffer
	hookCmd.SetOut(&stdout)

	start := time.Now()
	err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreToolUse"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != `{"decision":"allow"}` {
		t.Errorf("expected fail-open allowance, got %q", stdout.String())
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("fast path took %v, expected < 5ms", elapsed)
	}

	// Verify other events return {} and are < 5ms
	events := []string{"PreInvocation", "PostToolUse", "Stop"}
	for _, ev := range events {
		stdout.Reset()
		start = time.Now()
		err = hookCmd.RunE(hookCmd, []string{"antigravity", ev})
		elapsed = time.Since(start)

		if err != nil {
			t.Fatalf("hookCmd.RunE(%s) failed: %v", ev, err)
		}
		if strings.TrimSpace(stdout.String()) != "{}" {
			t.Errorf("expected {} for event %s, got %q", ev, stdout.String())
		}
		if elapsed > 5*time.Millisecond {
			t.Errorf("fast path for %s took %v, expected < 5ms", ev, elapsed)
		}
	}
}

func BenchmarkFastPath_NonEnrolled(b *testing.B) {
	nonEnrolledDir := b.TempDir()

	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = nonEnrolledDir

	var stdout bytes.Buffer
	hookCmd.SetOut(&stdout)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		_ = hookCmd.RunE(hookCmd, []string{"antigravity", "PreToolUse"})
	}
}

func TestEnrolledMarker_InitAndLeaveIntegration(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "init commit")

	// Initially unenrolled
	if config.IsEnrolledFast(repoDir) {
		t.Fatal("expected repo to be unenrolled initially")
	}

	// Run Enroll
	var buf bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&buf)
	if err := RunEnroll(cmdEnroll, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Marker must now exist
	if !config.IsEnrolledFast(repoDir) {
		t.Error("expected IsEnrolledFast to return true after RunEnroll")
	}
	markerPath := config.EnrolledMarkerPath(repoDir)
	fi, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("marker file %s not found: %v", markerPath, err)
	}
	if fi.Size() != 0 {
		t.Errorf("expected 0-byte marker, got size %d", fi.Size())
	}

	// Run Unenroll
	buf.Reset()
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&buf)
	if err := RunUnenroll(cmdUnenroll, repoDir, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}

	// Marker must now be removed
	if config.IsEnrolledFast(repoDir) {
		t.Error("expected IsEnrolledFast to return false after RunUnenroll")
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Errorf("expected marker file %s to be deleted", markerPath)
	}
}
