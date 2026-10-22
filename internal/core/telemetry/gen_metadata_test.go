/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package telemetry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
)

func TestReadGenMetadata_MissingFile(t *testing.T) {
	tempDir := t.TempDir()

	meta, err := telemetry.ReadGenMetadata(tempDir)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil metadata for missing file, got %+v", meta)
	}
}

func TestReadGenMetadata_SampleFile(t *testing.T) {
	tempDir := t.TempDir()

	sampleJSON := `{
  "uncached_input_tokens": 1500,
  "cached_input_tokens": 75000,
  "output_tokens": 420,
  "total_tokens": 76920,
  "turns": [
    {
      "turn_id": "turn-01",
      "turn_index": 1,
      "uncached_input_tokens": 1500,
      "cached_input_tokens": 75000,
      "output_tokens": 420,
      "total_tokens": 76920
    }
  ]
}`

	filePath := filepath.Join(tempDir, "gen_metadata.json")
	if err := os.WriteFile(filePath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("writing sample file: %v", err)
	}

	// Test passing directory
	meta, err := telemetry.ReadGenMetadata(tempDir)
	if err != nil {
		t.Fatalf("ReadGenMetadata(dir) failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.UncachedInputTokens != 1500 {
		t.Errorf("UncachedInputTokens: got %d, want 1500", meta.UncachedInputTokens)
	}
	if meta.CachedInputTokens != 75000 {
		t.Errorf("CachedInputTokens: got %d, want 75000", meta.CachedInputTokens)
	}
	if meta.OutputTokens != 420 {
		t.Errorf("OutputTokens: got %d, want 420", meta.OutputTokens)
	}
	if meta.TotalTokens != 76920 {
		t.Errorf("TotalTokens: got %d, want 76920", meta.TotalTokens)
	}
	if len(meta.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(meta.Turns))
	}
	if meta.Turns[0].TurnID != "turn-01" {
		t.Errorf("turn ID: got %s, want turn-01", meta.Turns[0].TurnID)
	}

	// Test passing direct file path
	metaDirect, err := telemetry.ReadGenMetadata(filePath)
	if err != nil {
		t.Fatalf("ReadGenMetadata(filePath) failed: %v", err)
	}
	if metaDirect == nil || metaDirect.UncachedInputTokens != 1500 {
		t.Errorf("unexpected direct metadata: %+v", metaDirect)
	}
}

func TestReadGenMetadata_TurnArrayFormat(t *testing.T) {
	tempDir := t.TempDir()

	sampleJSON := `[
  {
    "turn_id": "t1",
    "uncached_input_tokens": 100,
    "cached_input_tokens": 200,
    "output_tokens": 50
  },
  {
    "turn_id": "t2",
    "uncached_input_tokens": 300,
    "cached_input_tokens": 400,
    "output_tokens": 150
  }
]`

	filePath := filepath.Join(tempDir, "gen_metadata")
	if err := os.WriteFile(filePath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("writing sample file: %v", err)
	}

	meta, err := telemetry.ReadGenMetadata(tempDir)
	if err != nil {
		t.Fatalf("ReadGenMetadata failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.UncachedInputTokens != 400 {
		t.Errorf("aggregated UncachedInputTokens: got %d, want 400", meta.UncachedInputTokens)
	}
	if meta.CachedInputTokens != 600 {
		t.Errorf("aggregated CachedInputTokens: got %d, want 600", meta.CachedInputTokens)
	}
	if meta.OutputTokens != 200 {
		t.Errorf("aggregated OutputTokens: got %d, want 200", meta.OutputTokens)
	}
	if meta.TotalTokens != 1200 {
		t.Errorf("aggregated TotalTokens: got %d, want 1200", meta.TotalTokens)
	}
	if len(meta.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(meta.Turns))
	}
}

func TestReadGenMetadata_CorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "gen_metadata.json")
	if err := os.WriteFile(filePath, []byte("invalid-json{"), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	meta, err := telemetry.ReadGenMetadata(tempDir)
	if err == nil {
		t.Fatalf("expected error for malformed json, got nil (meta: %+v)", meta)
	}
}
