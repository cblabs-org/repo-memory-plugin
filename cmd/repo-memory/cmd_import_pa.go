package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
)

var importPACmd = &cobra.Command{
	Use:   "import-pa [dir]",
	Short: "Import Principal Architect files as episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ".agents/skills/principal-architect-workspace"
		if len(args) > 0 {
			dir = args[0]
		}

		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		repoID, err := gitops.GenerateRepoID(repoRoot)
		if err != nil {
			return fmt.Errorf("generating repo id: %w", err)
		}

		store := episode.NewStore(config.DataDir(), repoID)
		
		targetFiles := map[string]bool{
			"PROJECT.md": true,
			"WORKORDER.md": true,
			"context_bridge.md": true,
		}

		importedCount := 0

		err = filepath.Walk(filepath.Join(repoRoot, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			if !targetFiles[info.Name()] {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(contentBytes)

			date := info.ModTime()

			sessionID := fmt.Sprintf("pa-%s-%s", info.Name(), ulid.Make().String())
			
			epID := episode.NewEpisodeIDWithTime(date)
			
			header := episode.Header{
				EpisodeID: epID,
				Harness:   "import",
				SessionID: sessionID,
				RepoID:    repoID,
				Started:   date,
				Ended:     date,
			}

			step := harness.Step{
				Index: 1,
				Kind:  harness.StepKindNote,
				Text:  content,
				File:  path,
			}

			ep := episode.NewEpisode(header, step)
			if err := store.Write(ep); err != nil {
				return fmt.Errorf("writing episode %s: %w", path, err)
			}
			importedCount++
			
			fmt.Printf("Imported %s as episode %s\n", path, epID)
			return nil
		})
		
		if err != nil {
			return err
		}
		
		fmt.Printf("Imported %d PA files.\n", importedCount)
		return nil
	},
}
