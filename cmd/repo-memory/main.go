/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/main.go
 * Description: Entry point and root Cobra command registration for the
 *              repo-memory CLI, hook runner, background tick daemon, and MCP server.
 *
 * Exported Types / Functions:
 * - Version: Build-time version string injected via -ldflags.
 */

// The repo-memory command provides the CLI, hook runner, background tick daemon, and MCP server for repository memory.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version holds the build-time version string (overridden via -ldflags "-X main.Version=...").
var Version = "dev"

// rootCmd is the top-level Cobra command for the `repo-memory` binary.
var rootCmd = &cobra.Command{
	Use:     "repo-memory",
	Short:   "Repository-level memory for coding agents",
	Version: Version,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// --- Step 1: Register lifecycle, hook, daemon, and MCP subcommands ---
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(enrollCmd)
	rootCmd.AddCommand(unenrollCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(tickCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)

	// --- Step 2: Register query, diagnostic, and maintenance subcommands ---
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(importPACmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(usageCmd)

	// --- Step 3: Register seeding, op-log, consolidation, and projection subcommands ---
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(seedDocsCmd)
	rootCmd.AddCommand(opsCmd)
	rootCmd.AddCommand(consolidateCmd)
	rootCmd.AddCommand(promoteSkillsCmd)
	rootCmd.AddCommand(gardenCmd)
	rootCmd.AddCommand(summaryCmd)
	rootCmd.AddCommand(briefCmd)
	rootCmd.AddCommand(backfillCmd)
	rootCmd.AddCommand(reprojectCmd)
}

