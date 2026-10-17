/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_pipeline.go
 * Description: CLI subcommand for executing the background knowledge pipeline
 *              synchronously for a given enrolled repository ID.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
	"github.com/spf13/cobra"
)

// pipelineCmd defines the `repo-memory pipeline` subcommand.
var pipelineCmd = &cobra.Command{
	Use:   "pipeline [repoID]",
	Short: "Run the knowledge pipeline synchronously",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve repository root from global registry ---
		repoID := args[0]
		dataDir := config.DataDir()

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

		// --- Step 2: Open SQLite job store and execute synchronous pipeline ---
		store, err := jobs.Open(filepath.Join(dataDir, repoID, "jobs.sqlite"))
		if err != nil {
			return err
		}
		defer store.Close()

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
