package eval

import (
	"encoding/json"
	"fmt"
	"os"
)

// Dataset represents the format for benchmarks B1-B5.
type Dataset struct {
	ID        string `json:"id"`
	Benchmark string `json:"benchmark"`
	Tasks     []Task `json:"tasks"`
}

type Task struct {
	ID       string `json:"id"`
	Question string `json:"question,omitempty"`
	// For B1, it might contain the original request and target output
	OriginalRequest string `json:"original_request,omitempty"`
	TargetOutput    string `json:"target_output,omitempty"`
}

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
