package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Result struct {
	Timestamp time.Time   `json:"timestamp"`
	Benchmark string      `json:"benchmark"`
	DatasetID string      `json:"dataset_id"`
	TaskID    string      `json:"task_id"`
	Query     string      `json:"query,omitempty"`
	Matches   []string    `json:"matches,omitempty"` // Episode IDs or snippets
	Metrics   interface{} `json:"metrics,omitempty"`
}

type Store struct {
	path string
	file *os.File
	enc  *json.Encoder
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating eval result dir: %w", err)
	}

	resultPath := filepath.Join(dir, "results.jsonl")
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening eval result file: %w", err)
	}

	return &Store{
		path: resultPath,
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

func (s *Store) Record(r Result) error {
	r.Timestamp = time.Now()
	if err := s.enc.Encode(r); err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.file.Close()
}
