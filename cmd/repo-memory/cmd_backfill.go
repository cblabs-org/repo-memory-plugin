package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
)

var backfillCmd = &cobra.Command{
	Use:   "backfill-captures",
	Short: "Re-queue all completed or dead capture jobs for reprocessing",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := config.DataDir()
		reg, err := config.LoadRegistry(dataDir)
		if err != nil {
			return err
		}

		ctx := context.Background()
		totalRequeued := 0

		for _, entry := range reg {
			store, err := jobs.Open(filepath.Join(dataDir, entry.RepoID, "jobs.sqlite"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open jobs for %s: %v\n", entry.RepoID, err)
				continue
			}

			count, err := store.RequeueKind(ctx, jobs.KindCapture)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to requeue for %s: %v\n", entry.RepoID, err)
			} else {
				totalRequeued += count
				fmt.Printf("Requeued %d captures for %s\n", count, entry.RepoID)
			}

			// Also requeue dead/completed triage jobs
			tCount, err := store.RequeueKind(ctx, jobs.KindTriage)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to requeue triage for %s: %v\n", entry.RepoID, err)
			} else {
				totalRequeued += tCount
				fmt.Printf("Requeued %d triage jobs for %s\n", tCount, entry.RepoID)
			}
			store.Close()
		}

		fmt.Printf("Total jobs requeued: %d\n", totalRequeued)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}
