/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_enable.go
 * Description: CLI subcommand and runner for enabling repository memory either
 *              globally across the machine or for the current Git repository.
 *
 * Exported Types / Functions:
 * - RunEnable(): Enables repository memory globally or for the current repository.
 */

package main

import (
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

// enableRepoFlag indicates whether to enable memory only for the current repository (--repo).
var enableRepoFlag bool

// enableCmd defines the `repo-memory enable` subcommand.
var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable repository memory globally or for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunEnable(cmd, enableRepoFlag)
	},
}

func init() {
	enableCmd.Flags().BoolVar(&enableRepoFlag, "repo", false, "Enable repository memory for the current repository only")
}

// RunEnable enables repository memory globally or for the current repository.
func RunEnable(cmd *cobra.Command, repoOnly bool) error {
	// --- Step 1: Handle per-repository enablement and workspace plugin registration ---
	if repoOnly {
		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repository root: %w", err)
		}
		if err := config.SetRepoEnabled(repoRoot, true); err != nil {
			return fmt.Errorf("enabling repository memory: %w", err)
		}
		if err := EnsureWorkspacePluginConfigured(repoRoot); err != nil {
			return fmt.Errorf("configuring workspace plugin: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Enabled repository memory for %s\n", repoRoot)
		return nil
	}

	// --- Step 2: Handle global machine-wide enablement ---
	dataDir := config.DataDir()
	if err := config.SetMachineEnabled(dataDir, true); err != nil {
		return fmt.Errorf("enabling machine memory: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Enabled repository memory globally")
	return nil
}
