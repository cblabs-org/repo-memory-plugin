package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
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
		
		// Load all claims
		claimsStore, err := store.New(filepath.Join(repoRoot, ".memory", "claims"))
		if err != nil {
			return fmt.Errorf("loading claims store: %w", err)
		}
		
		allClaims, err := claimsStore.ListAll()
		if err != nil {
			return fmt.Errorf("listing claims: %w", err)
		}
		
		var allEntities []entities.Entity
		
		// Execute rebuild
		if err := index.Rebuild(ctx, cacheDir, allClaims, allEntities); err != nil {
			return fmt.Errorf("rebuilding index: %w", err)
		}
		
		fmt.Fprintln(cmd.OutOrStdout(), "Index rebuilt successfully in .memory/.cache/index.db")
		return nil
	},
}
