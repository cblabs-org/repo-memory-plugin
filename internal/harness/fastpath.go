/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/fastpath.go
 * Description: Implements fast-path validation checks to determine whether hook
 *              execution should proceed for a workspace.
 *
 * Exported Types / Functions:
 * - FastPathValidation(): Evaluates kill-switch, machine config, registry enrollment, and repo config checks.
 */

package harness

import (
	"os"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

// FastPathValidation checks if hook execution should proceed.
// It executes four fast checks in order:
// 1. REPO_MEMORY_DISABLED=1 environment variable
// 2. Machine config enabled (<data-dir>/config.yaml)
// 3. Workspace enrolled in registry (<data-dir>/registry.json)
// 4. Repo config enabled (<repo-root>/.memory/config.yaml)
//
// If any check fails (disabled or not enrolled), it returns false, nil immediately.
// If all checks pass, it returns true, nil.
func FastPathValidation(workspacePath, dataDir string) (bool, error) {
	// --- Step 1: Check REPO_MEMORY_DISABLED=1 environment variable ---
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
		return false, nil
	}

	if dataDir == "" {
		dataDir = config.DataDir()
	}

	// --- Step 2: Check machine config enablement ---
	machCfg, err := config.LoadMachineConfig(dataDir)
	if err != nil {
		return false, err
	}
	if !machCfg.Enabled {
		return false, nil
	}

	// --- Step 3: Resolve workspace path and verify registry enrollment ---
	if workspacePath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return false, nil
		}
		workspacePath = wd
	}

	entry, found, err := config.FindRegistryEntry(dataDir, workspacePath)
	if err != nil || !found || entry == nil {
		return false, nil
	}

	// --- Step 4: Verify repository-level config enablement ---
	repoCfg, err := config.LoadRepoConfig(entry.RootPath)
	if err != nil {
		return false, err
	}
	if !repoCfg.IsEnabled() {
		return false, nil
	}

	return true, nil
}
