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
	StepIndex       int             `json:"step_index"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	CreatedAt       string          `json:"created_at"`
	Content         string          `json:"content"`
	Thinking        string          `json:"thinking"`
	ToolCalls       json.RawMessage `json:"tool_calls"`
	Media           json.RawMessage `json:"media"`
	TruncatedFields []string        `json:"truncated_fields"`
	
	// Flags
	IsCheckpoint bool
	IsGap        bool
	GapSize      int // Number of missing steps
}

// ParseTranscript parses a conversation transcript from the given directory.
// It prioritizes transcript_full.jsonl, then chunks, then transcript.jsonl.
func ParseTranscript(baseDir string) ([]Step, error) {
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

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

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
