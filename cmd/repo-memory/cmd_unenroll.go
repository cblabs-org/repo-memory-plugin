package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var unenrollPurgeFlag bool

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
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	dataDir := config.DataDir()

	// Find the entry before removal to obtain RepoID if purge is requested
	entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot)

	removedIDs, err := config.RemoveFromRegistry(dataDir, repoRoot)
	if err != nil {
		return fmt.Errorf("removing from registry: %w", err)
	}

	// Remove .agents/plugins.json workspace-plugin entry before removing .enrolled marker
	if err := RemoveWorkspacePluginConfigured(repoRoot); err != nil {
		return fmt.Errorf("removing workspace plugin configuration: %w", err)
	}

	// Remove .memory/.enrolled marker
	if err := config.RemoveEnrolledMarker(repoRoot); err != nil {
		return fmt.Errorf("removing enrolled marker: %w", err)
	}

	if purge {
		var idsToPurge []string
		idsToPurge = append(idsToPurge, removedIDs...)
		if entry != nil && entry.RepoID != "" {
			idsToPurge = append(idsToPurge, entry.RepoID)
		}
		// If no IDs in registry, try generating RepoID from git
		if len(idsToPurge) == 0 {
			if id, err := gitops.GenerateRepoID(repoRoot); err == nil && id != "" {
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
