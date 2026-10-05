package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/spf13/cobra"
)

var (
	uninstallHarness string
	uninstallPurge   bool
)

// RunUninstall removes the generated plugin directory and optionally purges the data directory.
func RunUninstall(pluginDir, dataDir string, purge bool) error {
	if pluginDir == "" {
		return errors.New("plugin directory cannot be empty")
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("removing plugin directory %s: %w", pluginDir, err)
	}

	if purge && dataDir != "" {
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("purging data directory %s: %w", dataDir, err)
		}
	}

	return nil
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall repo-memory plugin and harness configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if uninstallHarness != "antigravity" {
			return fmt.Errorf("unsupported harness %q (only 'antigravity' is supported in Release 1)", uninstallHarness)
		}

		pluginDir, err := ResolveAntigravityPluginDir()
		if err != nil {
			return err
		}

		dataDir := config.DataDir()

		if err := RunUninstall(pluginDir, dataDir, uninstallPurge); err != nil {
			return err
		}

		fmt.Printf("Successfully uninstalled repo-memory daemon plugin from %s\n", pluginDir)
		if uninstallPurge {
			fmt.Printf("Purged data directory at: %s\n", dataDir)
		}

		return nil
	},
}

func init() {
	uninstallCmd.Flags().StringVar(&uninstallHarness, "harness", "antigravity", "Target agent harness (antigravity)")
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "Purge stored repository memory data as well")
}
