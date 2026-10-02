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

var evalCmd = &cobra.Command{
	Use:   "eval [benchmark]",
	Short: "Run evaluation benchmarks (B1-B5)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
