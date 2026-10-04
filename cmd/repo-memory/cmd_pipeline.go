package main

import (
	"context"
	"fmt"
	"os"
	"time"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline [repoID]",
	Short: "Run the knowledge pipeline synchronously",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoID := args[0]
		dataDir := config.DataDir()
		
		// Load registry to find repo root
		reg, err := config.LoadRegistry(dataDir)
		if err != nil {
			return err
		}
		
		var repoRoot string
		for _, entry := range reg {
			if entry.RepoID == repoID {
				repoRoot = entry.RootPath
				break
			}
		}
		if repoRoot == "" {
			return fmt.Errorf("repo %s not enrolled", repoID)
		}

		// Seed triage jobs
		epStore := episode.NewStore(dataDir, repoID)
		sessions, err := epStore.List()
		if err != nil {
			return err
		}
		
		store, err := jobs.Open(filepath.Join(dataDir, repoID, "jobs.sqlite"))
		if err != nil {
			return err
		}
		defer store.Close()
		
		for _, sessionID := range sessions {
			job := &jobs.Job{
				Kind:           jobs.KindTriage,
				Priority:       jobs.PriorityNormal,
				IdempotencyKey: "triage-" + sessionID,
				Payload:        sessionID,
			}
			_, err := store.Enqueue(context.Background(), job)
			if err != nil && err.Error() != "job exists" && err.Error() != "UNIQUE constraint failed" {
				fmt.Fprintf(os.Stderr, "failed to enqueue %s: %v\n", sessionID, err)
			}
		}

		fmt.Printf("Seeded %d episodes for triage.\n", len(sessions))

		// Run pipeline
		ticker := tick.NewTicker(
			dataDir,
			24*time.Hour, // large budget for sync run
			tick.WithHousekeeper(antigravity.NewHousekeeper(dataDir, repoID, "", "15m")),
		)
		
		fmt.Println("Running pipeline...")
		
		start := time.Now()
		
		ctx := context.Background()
		err = ticker.ProcessPipeline(ctx, store, repoRoot, repoID)
		
		fmt.Printf("Pipeline run complete in %s\n", time.Since(start))
		
		return err
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
}
