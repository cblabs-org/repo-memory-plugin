/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/telemetry/gen_metadata.go
 * Description: Reads and parses per-turn token usage telemetry from Antigravity
 *              gen_metadata files.
 *
 * Exported Types / Functions:
 * - TurnMetadata:       Captures token metrics for a single IDE turn.
 * - GenMetadata:        Captures aggregate and per-turn token usage for IDE sessions.
 * - ReadGenMetadata():  Reads and parses gen_metadata from disk if present.
 * - ParseGenMetadata(): Parses raw JSON bytes into a GenMetadata summary.
 */

package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TurnMetadata captures token metrics for a single IDE turn.
type TurnMetadata struct {
	TurnID              string `json:"turn_id,omitempty"`               // Optional unique identifier for the turn.
	TurnIndex           int    `json:"turn_index,omitempty"`            // Zero-based index of the turn within the session.
	UncachedInputTokens int    `json:"uncached_input_tokens,omitempty"` // Input tokens billed without context caching.
	CachedInputTokens   int    `json:"cached_input_tokens,omitempty"`   // Input tokens served from context cache.
	OutputTokens        int    `json:"output_tokens,omitempty"`         // Generated completion tokens for the turn.
	TotalTokens         int    `json:"total_tokens,omitempty"`          // Combined input and output tokens for the turn.
}

// GenMetadata captures per-turn token usage for IDE sessions.
type GenMetadata struct {
	UncachedInputTokens int            `json:"uncached_input_tokens,omitempty"` // Aggregate uncached input tokens across turns.
	CachedInputTokens   int            `json:"cached_input_tokens,omitempty"`   // Aggregate cached input tokens across turns.
	OutputTokens        int            `json:"output_tokens,omitempty"`         // Aggregate output tokens across turns.
	TotalTokens         int            `json:"total_tokens,omitempty"`          // Total tokens consumed across the session.
	Turns               []TurnMetadata `json:"turns,omitempty"`                 // Per-turn token usage breakdown.
}

// ReadGenMetadata reads per-turn token usage from the Antigravity gen_metadata file.
// If dataDir is empty, it checks ~/.gemini/antigravity.
// If the file does not exist, it returns nil, nil without error.
//
// Experimental: undocumented Antigravity internal, may change without notice.
func ReadGenMetadata(dataDir string) (*GenMetadata, error) {
	// --- Step 1: Resolve the candidate gen_metadata file path ---
	targetPath := resolveGenMetadataPath(dataDir)
	if targetPath == "" {
		return nil, nil
	}

	// --- Step 2: Read and parse the JSON metadata file ---
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
	// --- Step 1: Try parsing as a top-level GenMetadata object ---
	var meta GenMetadata
	if err := json.Unmarshal(data, &meta); err == nil && (meta.UncachedInputTokens > 0 || meta.CachedInputTokens > 0 || meta.OutputTokens > 0 || len(meta.Turns) > 0) {
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	// --- Step 2: Try parsing as a JSON array of TurnMetadata records ---
	var turns []TurnMetadata
	if err := json.Unmarshal(data, &turns); err == nil && len(turns) > 0 {
		meta = GenMetadata{
			Turns: turns,
		}
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	// --- Step 3: Fall back to generic JSON object validation ---
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err == nil {
		aggregateIfNeeded(&meta)
		return &meta, nil
	}

	return nil, fmt.Errorf("unrecognized gen_metadata JSON format")
}

// aggregateIfNeeded computes top-level token totals from meta.Turns when top-level counters are zero.
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

// resolveGenMetadataPath locates gen_metadata.json or gen_metadata inside dataDir (or ~/.gemini/antigravity).
func resolveGenMetadataPath(dataDir string) string {
	// --- Step 1: Default to ~/.gemini/antigravity when dataDir is empty ---
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataDir = filepath.Join(home, ".gemini", "antigravity")
	}

	// --- Step 2: Probe direct file path and known gen_metadata filenames ---
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
