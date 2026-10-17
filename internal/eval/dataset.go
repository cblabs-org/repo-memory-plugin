/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/eval/dataset.go
 * Description: Defines JSON dataset and task schemas and loader routines for
 *              evaluation benchmarks B1 through B5.
 *
 * Exported Types / Functions:
 * - Dataset:       Benchmark evaluation dataset containing a list of tasks.
 * - Task:          Single evaluation task question or request/target pair.
 * - LoadDataset(): Reads and unmarshals a benchmark Dataset from a JSON file.
 */

package eval

import (
	"encoding/json"
	"fmt"
	"os"
)

// Dataset represents the JSON format for benchmarks B1 through B5.
type Dataset struct {
	ID        string `json:"id"`        // Unique identifier for the dataset.
	Benchmark string `json:"benchmark"` // Benchmark family identifier (e.g., "B1", "B2").
	Tasks     []Task `json:"tasks"`     // Ordered list of benchmark tasks to evaluate.
}

// Task represents an individual benchmark question or input/output task item.
type Task struct {
	ID              string `json:"id"`                         // Unique task identifier within the dataset.
	Question        string `json:"question,omitempty"`         // Retrieval query or prompt question for B2+ benchmarks.
	OriginalRequest string `json:"original_request,omitempty"` // Original user request text for B1 benchmarks.
	TargetOutput    string `json:"target_output,omitempty"`    // Expected target output text for B1 benchmarks.
}

// LoadDataset reads and unmarshals a benchmark Dataset from the JSON file at path.
func LoadDataset(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading dataset: %w", err)
	}

	var dataset Dataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return nil, fmt.Errorf("unmarshaling dataset: %w", err)
	}

	return &dataset, nil
}
