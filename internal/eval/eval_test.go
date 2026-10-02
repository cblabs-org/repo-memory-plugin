package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestDatasetLoading(t *testing.T) {
	tmpDir := t.TempDir()
	datasetPath := filepath.Join(tmpDir, "b2.json")
	
	dataset := Dataset{
		ID:        "b2-test-1",
		Benchmark: "B2",
		Tasks: []Task{
			{ID: "t1", Question: "how do I configure?"},
		},
	}
	
	data, _ := json.Marshal(dataset)
	os.WriteFile(datasetPath, data, 0644)
	
	loaded, err := LoadDataset(datasetPath)
	if err != nil {
		t.Fatalf("LoadDataset failed: %v", err)
	}
	
	if loaded.Benchmark != "B2" || len(loaded.Tasks) != 1 {
		t.Errorf("Unexpected dataset content")
	}
}

func TestStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	
	err = store.Record(Result{
		Benchmark: "B2",
		DatasetID: "ds1",
		TaskID:    "t1",
	})
	if err != nil {
		t.Errorf("Record failed: %v", err)
	}
	
	store.Close()
	
	data, err := os.ReadFile(filepath.Join(tmpDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("Failed reading results: %v", err)
	}
	
	if len(data) == 0 {
		t.Errorf("Expected results, got empty file")
	}
}

func TestRunnerB2(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "episodes.db")
	
	// Create an index
	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex failed: %v", err)
	}
	defer idx.Close()
	
	// Index a dummy episode
	err = idx.InsertEpisode(context.Background(), index.EpisodeRecord{
		EpisodeID:    "ep1",
		Date:         time.Now(),
		SessionID:    "sess1",
		Content:      "configure the system with config.yaml",
		FilesTouched: "config.yaml",
	})
	if err != nil {
		t.Fatalf("IndexEpisode failed: %v", err)
	}
	
	store, _ := NewStore(tmpDir)
	defer store.Close()
	
	runner := NewRunner(store, idx)
	
	dataset := &Dataset{
		ID:        "b2-test",
		Benchmark: "B2",
		Tasks: []Task{
			{ID: "t1", Question: "configure"},
		},
	}
	
	err = runner.RunB2(context.Background(), dataset)
	if err != nil {
		t.Fatalf("RunB2 failed: %v", err)
	}
	
	// Verify result store has output
	store.Close()
	data, _ := os.ReadFile(filepath.Join(tmpDir, "results.jsonl"))
	var res Result
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}
	
	if len(res.Matches) == 0 || res.Matches[0] != "ep1" {
		t.Errorf("Expected ep1 match, got: %v", res.Matches)
	}
}
