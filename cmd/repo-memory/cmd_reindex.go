package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		cacheDir := filepath.Join(repoRoot, ".memory", ".cache")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return fmt.Errorf("creating cache dir: %w", err)
		}

		dbPath := filepath.Join(cacheDir, "index.db")
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

		fmt.Fprintln(cmd.OutOrStdout(), "Index rebuilt successfully in .memory/.cache/index.db")
		return nil
	},
}
