package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
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

		dbPath := filepath.Join(config.DataDir(), repoID, "views", "index", "episodes.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			// fallback check inside .memory/views/index? No, layout.go puts it in DataDir()/repoID/views/index ?
			// Wait, let's verify where it actually is in layout.go.
			// RepoMemorySubdirectories has "views/index", which is in repoRoot/.memory/
			dbPath = filepath.Join(repoRoot, ".memory", "views", "index", "episodes.db")
		}

		idx, err := index.NewReadOnlyIndex(dbPath)
		if err != nil {
			return fmt.Errorf("opening index: %w", err)
		}
		defer idx.Close()

		ctx := context.Background()
		results, err := idx.SearchEpisodes(ctx, query)
		if err != nil {
			return fmt.Errorf("searching episodes: %w", err)
		}

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No episodes found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EPISODE ID\tDATE\tBRANCH\tSNIPPET\tFILES TOUCHED")
		for _, r := range results {
			snippet := strings.ReplaceAll(r.Snippet, "\n", " ")
			if len(snippet) > 80 {
				snippet = snippet[:77] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.EpisodeID, r.Date.Format("2006-01-02"), r.SessionID, snippet, r.FilesTouched)
		}
		w.Flush()

		return nil
	},
}
