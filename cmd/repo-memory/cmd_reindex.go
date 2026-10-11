package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/spf13/cobra"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild repository index",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
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

		// Execute rebuild from op log
		if err := idx.RebuildFromOpLog(ctx, opsReader); err != nil {
			return fmt.Errorf("rebuilding index from op log: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Index rebuilt successfully in %s\n", dbPath)
		return nil
	},
}
