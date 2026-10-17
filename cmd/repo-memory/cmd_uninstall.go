/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_uninstall.go
 * Description: Implements the `repo-memory uninstall` CLI command, removing the
 *              installed harness plugin bundle and optionally purging stored
 *              repository memory data.
 *
 * Exported Types / Functions:
 * - RunUninstall(): Removes the generated plugin directory and optionally purges the data directory.
 */

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/spf13/cobra"
)

var (
	uninstallHarness string // Target agent harness name (--harness, defaults to "antigravity").
	uninstallPurge   bool   // When true, deletes the global repo-memory data directory during uninstall.
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

// uninstallCmd defines the `repo-memory uninstall` subcommand.
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall repo-memory plugin and harness configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Validate target harness and resolve plugin/data directories ---
		if uninstallHarness != "antigravity" {
			return fmt.Errorf("unsupported harness %q (only 'antigravity' is supported in Release 1)", uninstallHarness)
		}

		pluginDir, err := ResolveAntigravityPluginDir()
		if err != nil {
			return err
		}

		dataDir := config.DataDir()

		// --- Step 2: Remove plugin directory and optionally purge data directory ---
		if err := RunUninstall(pluginDir, dataDir, uninstallPurge); err != nil {
			return err
		}

		// --- Step 3: Print completion summary and enrolled-repo cleanup reminder ---
		fmt.Printf("Successfully uninstalled repo-memory daemon plugin from %s\n", pluginDir)
		if uninstallPurge {
			fmt.Printf("Purged data directory at: %s\n", dataDir)
		}

		fmt.Println("\n⚠ Any enrolled repos still have .agents/plugins.json pointing to the removed plugin.")
		fmt.Println("  Run 'repo-memory unenroll' in each enrolled repo, or remove .agents/plugins.json manually.")

		return nil
	},
}

func init() {
	uninstallCmd.Flags().StringVar(&uninstallHarness, "harness", "antigravity", "Target agent harness (antigravity)")
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "Purge stored repository memory data as well")
}
