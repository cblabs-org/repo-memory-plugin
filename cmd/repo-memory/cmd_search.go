package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search captured episodes, claims, and memory",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		repoID, err := gitops.GenerateRepoID(repoRoot)
		if err != nil {
			return fmt.Errorf("generating repo id: %w", err)
		}

		// Build a fresh FTS index from the episode store each time.
		// PA imports and captured episodes are both in the same store.
		dataDir := config.DataDir()
		store := episode.NewStore(dataDir, repoID)

		dbPath := filepath.Join(dataDir, repoID, "episodes_fts.db")
		// Remove stale index to rebuild fresh
		os.Remove(dbPath)

		idx, err := index.NewIndex(dbPath)
		if err != nil {
			return fmt.Errorf("creating FTS index: %w", err)
		}
		defer idx.Close()

		ctx := context.Background()
		if err := idx.PopulateFromStore(ctx, store); err != nil {
			return fmt.Errorf("populating FTS index: %w", err)
		}

		results, err := idx.SearchEpisodes(ctx, query)
		if err != nil {
			return fmt.Errorf("searching episodes: %w", err)
		}

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No episodes found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EPISODE ID\tDATE\tSESSION\tSNIPPET\tFILES TOUCHED")
		for _, r := range results {
			snippet := strings.ReplaceAll(r.Snippet, "\n", " ")
			if len(snippet) > 80 {
				snippet = snippet[:77] + "..."
			}
			date := ""
			if !r.Date.IsZero() {
				date = r.Date.Format("2006-01-02")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.EpisodeID, date, r.SessionID, snippet, r.FilesTouched)
		}
		w.Flush()

		return nil
	},
}

