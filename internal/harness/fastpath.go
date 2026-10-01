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
	// 1. Check REPO_MEMORY_DISABLED=1 environment variable
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
		return false, nil
	}

	if dataDir == "" {
		dataDir = config.DataDir()
	}

	// 2. Check machine config
	machCfg, err := config.LoadMachineConfig(dataDir)
	if err != nil {
		return false, err
	}
	if !machCfg.Enabled {
		return false, nil
	}

	// 3. Determine workspace path
	if workspacePath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return false, nil
		}
		workspacePath = wd
	}

	// Check registry
	entry, found, err := config.FindRegistryEntry(dataDir, workspacePath)
	if err != nil || !found || entry == nil {
		return false, nil
	}

	// 4. Check repo config
	repoCfg, err := config.LoadRepoConfig(entry.RootPath)
	if err != nil {
		return false, err
	}
	if !repoCfg.IsEnabled() {
		return false, nil
	}

	return true, nil
}
