/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_reindex.go
 * Description: Rebuilds the repository SQLite FTS5/BM25 and anchor index
 *              (<data-dir>/<repo-id>/index.db) from committed operation logs.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/spf13/cobra"
)

// reindexRepoDir holds the optional --repo directory override for reindexCmd.
var reindexRepoDir string

// reindexCmd defines the `repo-memory reindex` subcommand.
var reindexCmd = &cobra.Command{
	Use:   "reindex [repo-dir]",
	Short: "Rebuild repository index",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// --- Step 1: Resolve target repository root and repo ID ---
		targetDir := "."
		if strings.TrimSpace(reindexRepoDir) != "" {
			targetDir = strings.TrimSpace(reindexRepoDir)
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = strings.TrimSpace(args[0])
		}

		repoRoot, err := gitops.FindRepoRoot(targetDir)
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		dataDir := config.DataDir()
		repoID := ""
		if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
			repoID = entry.RepoID
		}
		if repoID == "" {
			var err error
			repoID, err = gitops.GenerateRepoID(repoRoot)
			if err != nil {
				return fmt.Errorf("generating repo ID: %w", err)
			}
		}

		// --- Step 2: Open SQLite index and replay .memory/ops ---
		dbPath := index.IndexPath(dataDir, repoID)
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return fmt.Errorf("creating index dir: %w", err)
		}

		idx, err := index.NewIndex(dbPath)
		if err != nil {
			return fmt.Errorf("opening index: %w", err)
		}
		defer idx.Close()

		opsDir := filepath.Join(repoRoot, ".memory", "ops")
		opsReader := ops.NewReader(opsDir)

		if err := idx.RebuildFromOpLog(ctx, opsReader); err != nil {
			return fmt.Errorf("rebuilding index from op log: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Index rebuilt successfully in %s\n", dbPath)
		return nil
	},
}

func init() {
	reindexCmd.Flags().StringVar(&reindexRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
}
