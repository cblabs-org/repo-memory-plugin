/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/transcript.go
 * Description: Parses Antigravity conversation transcripts from transcript_full.jsonl,
 *              chunked transcript_full directories, or compact transcript.jsonl files,
 *              synthesizing CHECKPOINT and gap markers.
 *
 * Exported Types / Functions:
 * - Step:              Single parsed action or marker in an Antigravity transcript.
 *   - Classification(): Returns a simplified actor ("user", "model", "system") and action pair.
 * - ParseTranscript(): Reads and parses all transcript steps from a conversation log directory.
 */

package antigravity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Step represents a single action in the conversation.
type Step struct {
	StepIndex       int             `json:"step_index"`       // 0-based step index in the trajectory.
	Source          string          `json:"source"`           // Step origin (e.g., "USER_EXPLICIT", "MODEL", "SYSTEM").
	Type            string          `json:"type"`             // Step type (e.g., "USER_INPUT", "PLANNER_RESPONSE").
	Status          string          `json:"status"`           // Step completion status (e.g., "DONE", "ERROR").
	CreatedAt       string          `json:"created_at"`       // ISO 8601 creation timestamp string.
	Content         string          `json:"content"`          // Primary text content of the step.
	Thinking        string          `json:"thinking"`         // Internal model chain-of-thought text.
	ToolCalls       json.RawMessage `json:"tool_calls"`       // Raw JSON array of tool calls made in this step.
	Media           json.RawMessage `json:"media"`            // Raw JSON array of attached media references.
	TruncatedFields []string        `json:"truncated_fields"` // Field names truncated in compact transcripts.

	// Flags
	IsCheckpoint bool // True when this entry represents a raw CHECKPOINT compaction marker line.
	IsGap        bool // True when this entry represents a synthetic gap marker for missing step indices.
	GapSize      int  // Number of missing steps represented by a synthetic gap marker.
}

// ParseTranscript parses a conversation transcript from the given directory.
// It prioritizes transcript_full.jsonl, then chunks, then transcript.jsonl.
func ParseTranscript(baseDir string) ([]Step, error) {
	// --- Step 1: Locate and open the highest-fidelity transcript source in baseDir ---
	var reader io.ReadCloser

	fullPath := filepath.Join(baseDir, "transcript_full.jsonl")
	if f, err := os.Open(fullPath); err == nil {
		reader = f
	} else if os.IsNotExist(err) {
		// try chunks
		chunksDir := filepath.Join(baseDir, "chunks", "transcript_full")
		entries, err := os.ReadDir(chunksDir)
		if err == nil {
			var chunkFiles []string
			for _, entry := range entries {
				if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
					chunkFiles = append(chunkFiles, entry.Name())
				}
			}
			if len(chunkFiles) > 0 {
				sort.Strings(chunkFiles)
				var readers []io.Reader
				var closers []io.Closer
				for _, cf := range chunkFiles {
					f, err := os.Open(filepath.Join(chunksDir, cf))
					if err != nil {
						// cleanup
						for _, c := range closers {
							c.Close()
						}
						return nil, err
					}
					readers = append(readers, f)
					closers = append(closers, f)
				}
				reader = &multiReadCloser{
					Reader:  io.MultiReader(readers...),
					closers: closers,
				}
			}
		}

		if reader == nil {
			// fallback to compact
			compactPath := filepath.Join(baseDir, "transcript.jsonl")
			if f, err := os.Open(compactPath); err == nil {
				reader = f
			} else {
				return nil, fmt.Errorf("no transcript source found in %s", baseDir)
			}
		}
	} else {
		return nil, err
	}

	defer reader.Close()

	// --- Step 2: Scan JSONL lines, synthesizing CHECKPOINT and gap steps ---
	var steps []Step
	scanner := bufio.NewScanner(reader)
	// Some jsonl lines could be very long
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*50) // up to 50MB per line

	expectedIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "CHECKPOINT" {
			steps = append(steps, Step{IsCheckpoint: true})
			continue
		}

		var step Step
		if err := json.Unmarshal([]byte(line), &step); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}

		if step.StepIndex > expectedIndex {
			steps = append(steps, Step{
				IsGap:   true,
				GapSize: step.StepIndex - expectedIndex,
			})
		}

		expectedIndex = step.StepIndex + 1
		steps = append(steps, step)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return steps, nil
}

// multiReadCloser concatenates multiple readers and closes all underlying file handles on Close.
type multiReadCloser struct {
	io.Reader             // Concatenated reader over ordered chunk files.
	closers   []io.Closer // Open chunk file handles to close when reading completes.
}

// Close closes all underlying chunk readers and returns the first error encountered.
func (m *multiReadCloser) Close() error {
	var firstErr error
	for _, c := range m.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Classification returns a simplified actor and action classification for the step.
func (s *Step) Classification() (actor string, action string) {
	if s.Source == "USER_EXPLICIT" {
		actor = "user"
	} else if s.Source == "MODEL" {
		actor = "model"
	} else {
		actor = "system"
	}

	action = s.Type
	return actor, action
}
