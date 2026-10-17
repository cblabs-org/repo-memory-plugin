/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_unenroll.go
 * Description: Removes a repository from repo-memory tracking, cleans up
 *              .agents/plugins.json and .memory/.enrolled, restores Principal Architect
 *              memory_mode to builtin when present, and optionally purges private data.
 *
 * Exported Types / Functions:
 * - RunUnenroll(): Executes repository unenrollment and optional private data purge.
 * - RunLeave():    Alias for RunUnenroll.
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var unenrollPurgeFlag bool // When true, deletes <data-dir>/<repo-id>/ during unenroll.

// unenrollCmd defines the `repo-memory unenroll` (alias `leave`) subcommand.
var unenrollCmd = &cobra.Command{
	Use:     "unenroll [path]",
	Short:   "Remove a repository from memory tracking",
	Aliases: []string{"leave"},
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}
		return RunUnenroll(cmd, targetDir, unenrollPurgeFlag)
	},
}

// leaveCmd is retained as an alias variable for backwards compatibility.
var leaveCmd = unenrollCmd

func init() {
	unenrollCmd.Flags().BoolVar(&unenrollPurgeFlag, "purge", false, "Delete private repository data directory (<data-dir>/<repo-id>/)")
}

// RunUnenroll executes repository unenrolment.
func RunUnenroll(cmd *cobra.Command, targetDir string, purge bool) error {
	// --- Step 1: Resolve repository root and remove managed GEMINI.md block ---
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	dataDir := config.DataDir()

	entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot)
	createdByUs := found && entry != nil && entry.CreatedInstructionFile

	if err := config.RemoveInstructionBlock(repoRoot, createdByUs); err != nil {
		return fmt.Errorf("removing instruction block from %s: %w", config.InstructionFilename, err)
	}

	// --- Step 2: Remove registry entry, .agents/plugins.json entry, and .enrolled marker ---
	removedIDs, err := config.RemoveFromRegistry(dataDir, repoRoot)
	if err != nil {
		return fmt.Errorf("removing from registry: %w", err)
	}

	if err := RemoveWorkspacePluginConfigured(repoRoot); err != nil {
		return fmt.Errorf("removing workspace plugin configuration: %w", err)
	}

	if err := config.RemoveEnrolledMarker(repoRoot); err != nil {
		return fmt.Errorf("removing enrolled marker: %w", err)
	}

	// Restore Principal Architect memory_mode to builtin fallback if PA state.db exists.
	SyncPAMemoryMode(repoRoot, "builtin")

	// --- Step 3: Optionally purge <data-dir>/<repo-id>/ ---
	if purge {
		var idsToPurge []string
		idsToPurge = append(idsToPurge, removedIDs...)
		if entry != nil && entry.RepoID != "" {
			idsToPurge = append(idsToPurge, entry.RepoID)
		}
		if len(idsToPurge) == 0 {
			if id, genErr := gitops.GenerateRepoID(repoRoot); genErr == nil && id != "" {
				idsToPurge = append(idsToPurge, id)
			}
		}

		purgedMap := make(map[string]bool)
		for _, id := range idsToPurge {
			if id == "" || purgedMap[id] {
				continue
			}
			purgedMap[id] = true
			targetDataDir := filepath.Join(dataDir, id)
			if err := os.RemoveAll(targetDataDir); err != nil {
				return fmt.Errorf("purging data dir %s: %w", targetDataDir, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Purged private data directory: %s\n", targetDataDir)
		}
	}

	if len(removedIDs) == 0 && (entry == nil || !found) {
		fmt.Fprintf(cmd.OutOrStdout(), "Repository was not enrolled: %s\n", repoRoot)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Unenrolled repository %s\n", repoRoot)
	}
	return nil
}

// RunLeave is an alias for RunUnenroll.
func RunLeave(cmd *cobra.Command, targetDir string, purge bool) error {
	return RunUnenroll(cmd, targetDir, purge)
}
