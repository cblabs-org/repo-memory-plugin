/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/eval/runner.go
 * Description: Executes benchmark evaluation suites against the SQLite FTS5 index
 *              and records retrieval results to the evaluation store.
 *
 * Exported Types / Functions:
 * - Runner:        Executes benchmark tasks against an index and persists results.
 *   - NewRunner(): Constructs a Runner backed by the given Store and Index.
 *   - RunB2():     Executes the B2 episode search benchmark over a Dataset.
 */

// Package eval provides benchmark dataset loaders, runners, result stores, and
// work-order replay harnesses for repository memory evaluations.
package eval

import (
	"context"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

// Runner executes evaluation benchmark tasks against an index and records results.
type Runner struct {
	store *Store       // Destination store for JSONL evaluation results.
	idx   *index.Index // SQLite FTS index queried during benchmark runs.
}

// NewRunner constructs a Runner backed by store and idx.
func NewRunner(store *Store, idx *index.Index) *Runner {
	return &Runner{
		store: store,
		idx:   idx,
	}
}

// RunB2 executes the B2 episode retrieval benchmark across all tasks in dataset
// and appends each task's matched episode IDs to the evaluation store.
func (r *Runner) RunB2(ctx context.Context, dataset *Dataset) error {
	// --- Step 1: Validate that the dataset targets the B2 benchmark ---
	if dataset.Benchmark != "B2" {
		return fmt.Errorf("expected B2 benchmark dataset, got %s", dataset.Benchmark)
	}

	// --- Step 2: Query the FTS episode index for each task and record matches ---
	for _, task := range dataset.Tasks {
		// B2 baseline: query FTS index with the question
		results, err := r.idx.SearchEpisodes(ctx, task.Question)
		if err != nil {
			return fmt.Errorf("searching for task %s: %w", task.ID, err)
		}

		var matches []string
		for _, res := range results {
			matches = append(matches, res.EpisodeID)
		}

		// Record top retrieved results (let's say all returned by search, which is usually limited or we can take top N)
		// Usually SearchEpisodes returns top 10 or so depending on its implementation.

		err = r.store.Record(Result{
			Benchmark: "B2",
			DatasetID: dataset.ID,
			TaskID:    task.ID,
			Query:     task.Question,
			Matches:   matches,
		})
		if err != nil {
			return fmt.Errorf("recording result for task %s: %w", task.ID, err)
		}
	}

	return nil
}
