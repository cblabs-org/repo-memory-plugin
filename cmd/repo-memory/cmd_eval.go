/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_eval.go
 * Description: CLI subcommand for executing evaluation benchmarks (B1-B5)
 *              against a repository's SQLite memory index and dataset files.
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

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/eval"
	"github.com/spf13/cobra"
)

// evalCmd defines the `repo-memory eval` subcommand.
var evalCmd = &cobra.Command{
	Use:   "eval [benchmark]",
	Short: "Run evaluation benchmarks (B1-B5)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Validate benchmark name and load dataset ---
		benchmark := args[0]
		if benchmark != "B2" {
			return fmt.Errorf("currently only B2 benchmark is implemented")
		}

		datasetFile, _ := cmd.Flags().GetString("dataset")
		if datasetFile == "" {
			return fmt.Errorf("--dataset is required")
		}

		dataset, err := eval.LoadDataset(datasetFile)
		if err != nil {
			return fmt.Errorf("loading dataset: %w", err)
		}

		// --- Step 2: Resolve repository root, repo ID, and evaluation store ---
		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		repoID, err := gitops.GenerateRepoID(repoRoot)
		if err != nil {
			return fmt.Errorf("generating repo id: %w", err)
		}

		evalDir := filepath.Join(config.DataDir(), repoID, "eval")
		store, err := eval.NewStore(evalDir)
		if err != nil {
			return fmt.Errorf("initializing result store: %w", err)
		}
		defer store.Close()

		// --- Step 3: Open read-only SQLite index and execute benchmark ---
		dbPath := filepath.Join(repoRoot, ".memory", "views", "index", "episodes.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbPath = filepath.Join(config.DataDir(), repoID, "views", "index", "episodes.db")
		}

		idx, err := index.NewReadOnlyIndex(dbPath)
		if err != nil {
			return fmt.Errorf("opening index: %w", err)
		}
		defer idx.Close()

		runner := eval.NewRunner(store, idx)

		ctx := context.Background()
		fmt.Fprintf(cmd.OutOrStdout(), "Running evaluation benchmark %s from dataset %s...\n", benchmark, dataset.ID)

		if err := runner.RunB2(ctx, dataset); err != nil {
			return fmt.Errorf("running benchmark %s: %w", benchmark, err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Evaluation completed.")
		return nil
	},
}

func init() {
	evalCmd.Flags().StringP("dataset", "d", "", "Path to the dataset JSON file")
}
