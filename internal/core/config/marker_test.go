/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnrolledMarker_LifecycleAndFastPath(t *testing.T) {
	tempDir := t.TempDir()

	// Initially not enrolled
	if IsEnrolledFast(tempDir) {
		t.Fatal("expected IsEnrolledFast to return false for fresh directory")
	}

	// Create marker
	if err := CreateEnrolledMarker(tempDir); err != nil {
		t.Fatalf("CreateEnrolledMarker failed: %v", err)
	}

	markerPath := EnrolledMarkerPath(tempDir)
	fi, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("expected marker file to exist: %v", err)
	}
	if fi.Size() != 0 {
		t.Errorf("expected marker file size to be 0 bytes, got %d", fi.Size())
	}

	// Verify IsEnrolledFast returns true on repo root
	if !IsEnrolledFast(tempDir) {
		t.Error("expected IsEnrolledFast to return true for enrolled directory")
	}

	// Verify IsEnrolledFast returns true in a subdirectory
	subDir := filepath.Join(tempDir, "pkg", "core", "test")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	if !IsEnrolledFast(subDir) {
		t.Error("expected IsEnrolledFast to return true for subdirectory of enrolled directory")
	}

	// Benchmark / latency assertion: < 5ms (target: <= 5ms, typically < 0.1ms)
	start := time.Now()
	iterations := 100
	for i := 0; i < iterations; i++ {
		_ = IsEnrolledFast(tempDir)
		_ = IsEnrolledFast(subDir)
	}
	avgDuration := time.Since(start) / time.Duration(iterations*2)
	if avgDuration > 5*time.Millisecond {
		t.Errorf("IsEnrolledFast took %v on average, expected < 5ms", avgDuration)
	}

	// Remove marker
	if err := RemoveEnrolledMarker(tempDir); err != nil {
		t.Fatalf("RemoveEnrolledMarker failed: %v", err)
	}

	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Error("expected marker file to be removed")
	}

	// After removal, should be false
	if IsEnrolledFast(tempDir) {
		t.Error("expected IsEnrolledFast to return false after marker removal")
	}
	if IsEnrolledFast(subDir) {
		t.Error("expected IsEnrolledFast to return false in subdir after marker removal")
	}
}

func TestIsEnrolled_FallbackToRegistry(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()

	// Not enrolled anywhere
	if IsEnrolled(repoDir) {
		t.Error("expected IsEnrolled to return false initially")
	}

	// Register in registry without marker file
	if err := AddToRegistry(dataDir, []string{repoDir}, "test-repo-id", repoDir); err != nil {
		t.Fatalf("AddToRegistry failed: %v", err)
	}

	// Should be true via registry fallback
	if !IsEnrolled(repoDir) {
		t.Error("expected IsEnrolled to return true via registry fallback")
	}

	// Adding marker file keeps it true
	if err := CreateEnrolledMarker(repoDir); err != nil {
		t.Fatalf("CreateEnrolledMarker failed: %v", err)
	}
	if !IsEnrolled(repoDir) {
		t.Error("expected IsEnrolled to return true with marker")
	}
}
