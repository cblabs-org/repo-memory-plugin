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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// loadAdjacentEnvFile loads optional KEY=VALUE defaults from a repo-memory.env file located
// adjacent to the resolved binary executable without overriding existing environment variables.
func loadAdjacentEnvFile() {
	// --- Step 1: Resolve canonical executable directory ---
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	envPath := filepath.Join(filepath.Dir(execPath), "repo-memory.env")

	// --- Step 2: Open and parse KEY=VALUE lines if repo-memory.env is present ---
	f, err := os.Open(envPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		val := strings.Trim(strings.TrimSpace(v), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, os.ExpandEnv(val))
		}
	}
}

func main() {
	loadAdjacentEnvFile()
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

