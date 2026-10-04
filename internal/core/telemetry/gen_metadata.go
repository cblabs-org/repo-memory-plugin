package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TurnMetadata captures token metrics for a single IDE turn.
type TurnMetadata struct {
	TurnID              string `json:"turn_id,omitempty"`
	TurnIndex           int    `json:"turn_index,omitempty"`
	UncachedInputTokens int    `json:"uncached_input_tokens,omitempty"`
	CachedInputTokens   int    `json:"cached_input_tokens,omitempty"`
	OutputTokens        int    `json:"output_tokens,omitempty"`
	TotalTokens         int    `json:"total_tokens,omitempty"`
}

// GenMetadata captures per-turn token usage for IDE sessions.
type GenMetadata struct {
	UncachedInputTokens int            `json:"uncached_input_tokens,omitempty"`
	CachedInputTokens   int            `json:"cached_input_tokens,omitempty"`
	OutputTokens        int            `json:"output_tokens,omitempty"`
	TotalTokens         int            `json:"total_tokens,omitempty"`
	Turns               []TurnMetadata `json:"turns,omitempty"`
}

// ReadGenMetadata reads per-turn token usage from the Antigravity gen_metadata file.
// If dataDir is empty, it checks ~/.gemini/antigravity.
// If the file does not exist, it returns nil, nil without error.
//
// Experimental: undocumented Antigravity internal, may change without notice
func ReadGenMetadata(dataDir string) (*GenMetadata, error) {
	targetPath := resolveGenMetadataPath(dataDir)
	if targetPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading gen_metadata file: %w", err)
	}

	meta, err := ParseGenMetadata(data)
	if err != nil {
		return nil, fmt.Errorf("parsing gen_metadata: %w", err)
	}

	return meta, nil
}

// ParseGenMetadata parses the raw JSON bytes of gen_metadata.
func ParseGenMetadata(data []byte) (*GenMetadata, error) {
	// Try parsing as GenMetadata object first
	var meta GenMetadata
	if err := json.Unmarshal(data, &meta); err == nil && (meta.UncachedInputTokens > 0 || meta.CachedInputTokens > 0 || meta.OutputTokens > 0 || len(meta.Turns) > 0) {
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	// Try parsing as a slice of TurnMetadata
	var turns []TurnMetadata
	if err := json.Unmarshal(data, &turns); err == nil && len(turns) > 0 {
		meta = GenMetadata{
			Turns: turns,
		}
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	// Try generic unmarshal to check if it's valid JSON object
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err == nil {
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	return nil, fmt.Errorf("unrecognized gen_metadata JSON format")
}

func aggregateIfNeeded(meta *GenMetadata) {
	if meta.UncachedInputTokens == 0 && meta.CachedInputTokens == 0 && meta.OutputTokens == 0 && len(meta.Turns) > 0 {
		for _, t := range meta.Turns {
			meta.UncachedInputTokens += t.UncachedInputTokens
			meta.CachedInputTokens += t.CachedInputTokens
			meta.OutputTokens += t.OutputTokens
		}
	}
	if meta.TotalTokens == 0 {
		meta.TotalTokens = meta.UncachedInputTokens + meta.CachedInputTokens + meta.OutputTokens
	}
}

func resolveGenMetadataPath(dataDir string) string {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataDir = filepath.Join(home, ".gemini", "antigravity")
	}

	// Check if dataDir itself is an existing file
	if fi, err := os.Stat(dataDir); err == nil && !fi.IsDir() {
		return dataDir
	}

	candidate1 := filepath.Join(dataDir, "gen_metadata.json")
	if _, err := os.Stat(candidate1); err == nil {
		return candidate1
	}

	candidate2 := filepath.Join(dataDir, "gen_metadata")
	if _, err := os.Stat(candidate2); err == nil {
		return candidate2
	}

	return ""
}
