package eval

import (
	"context"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

type Runner struct {
	store *Store
	idx   *index.Index
}

func NewRunner(store *Store, idx *index.Index) *Runner {
	return &Runner{
		store: store,
		idx:   idx,
	}
}

func (r *Runner) RunB2(ctx context.Context, dataset *Dataset) error {
	if dataset.Benchmark != "B2" {
		return fmt.Errorf("expected B2 benchmark dataset, got %s", dataset.Benchmark)
	}

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
