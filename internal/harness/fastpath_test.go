/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package harness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

func setupEnrolledRepo(t *testing.T) (dataDir, repoRoot string) {
	t.Helper()
	dataDir = t.TempDir()
	repoRoot = t.TempDir()
	repoID := "test-repo-fastpath-id"

	// Ensure repo storage layout
	if err := config.EnsureStorageLayout(dataDir, repoID, repoRoot); err != nil {
		t.Fatalf("EnsureStorageLayout failed: %v", err)
	}

	// Add to registry
	if err := config.AddToRegistry(dataDir, []string{repoRoot}, repoID, repoRoot); err != nil {
		t.Fatalf("AddToRegistry failed: %v", err)
	}

	return dataDir, repoRoot
}

func TestFastPathValidation_AllEnabled(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)

	valid, err := FastPathValidation(repoRoot, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if !valid {
		t.Error("expected FastPathValidation to return true when all enabled and enrolled")
	}
}

func TestFastPathValidation_EnvDisabled(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)

	t.Setenv("REPO_MEMORY_DISABLED", "1")

	valid, err := FastPathValidation(repoRoot, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if valid {
		t.Error("expected FastPathValidation to return false when REPO_MEMORY_DISABLED=1")
	}
}

func TestFastPathValidation_MachineDisabled(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)

	if err := config.SetMachineEnabled(dataDir, false); err != nil {
		t.Fatalf("SetMachineEnabled(false) failed: %v", err)
	}

	valid, err := FastPathValidation(repoRoot, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if valid {
		t.Error("expected FastPathValidation to return false when machine is disabled")
	}
}

func TestFastPathValidation_NotEnrolled(t *testing.T) {
	dataDir, _ := setupEnrolledRepo(t)
	nonEnrolledDir := t.TempDir()

	valid, err := FastPathValidation(nonEnrolledDir, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if valid {
		t.Error("expected FastPathValidation to return false for non-enrolled workspace")
	}
}

func TestFastPathValidation_RepoDisabled(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)

	if err := config.SetRepoEnabled(repoRoot, false); err != nil {
		t.Fatalf("SetRepoEnabled(false) failed: %v", err)
	}

	valid, err := FastPathValidation(repoRoot, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if valid {
		t.Error("expected FastPathValidation to return false when repo is disabled")
	}
}

func TestFastPathValidation_SubdirectoryEnrolled(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)
	subDir := filepath.Join(repoRoot, "src", "pkg", "service")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	valid, err := FastPathValidation(subDir, dataDir)
	if err != nil {
		t.Fatalf("FastPathValidation failed: %v", err)
	}
	if !valid {
		t.Error("expected FastPathValidation to return true for subdirectory of enrolled repo")
	}
}

func TestFastPathValidation_LatencyBudget(t *testing.T) {
	dataDir, repoRoot := setupEnrolledRepo(t)

	// Warm up
	_, _ = FastPathValidation(repoRoot, dataDir)

	// Run multiple iterations and check latency
	iterations := 20
	start := time.Now()
	for i := 0; i < iterations; i++ {
		valid, err := FastPathValidation(repoRoot, dataDir)
		if err != nil || !valid {
			t.Fatalf("iteration %d failed: valid=%v, err=%v", i, valid, err)
		}
	}
	elapsed := time.Since(start)
	avgDuration := elapsed / time.Duration(iterations)

	t.Logf("FastPathValidation average latency: %v (budget < 5ms)", avgDuration)

	// Average latency must be comfortably well below 5ms (typically < 0.5ms)
	if avgDuration > 5*time.Millisecond {
		t.Errorf("average latency %v exceeds 5ms budget", avgDuration)
	}
}
