/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/evidence/parser.go
 * Description: Opens and parses raw Antigravity JSONL transcripts (single file,
 *              chunked directory, or .system_generated/logs layout) into
 *              normalized harness steps and checkpoint sections.
 *
 * Exported Types / Functions:
 * - ParsedTranscript:      Holds normalized steps, checkpoints, timestamps, and byte/char counts.
 * - OpenTranscript():      Opens a transcript reader from a file path or conversation directory.
 * - ParseTranscript():     Opens and parses a transcript from disk.
 * - ParseTranscriptStream(): Parses a transcript JSONL stream from an io.Reader.
 */

package evidence

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// ParsedTranscript represents the parsed steps and extracted metadata from a transcript.
type ParsedTranscript struct {
	Steps         []harness.Step    // Normalized chronological step sequence.
	Checkpoints   map[string]string // Extracted checkpoint section map keyed by section name.
	Started       time.Time         // Earliest step timestamp observed in the stream.
	Ended         time.Time         // Latest step timestamp observed in the stream.
	RawBytes      int64             // Total raw byte count scanned from the JSONL stream.
	RawChars      int64             // Total character count across content, thinking, and tool calls.
	StepCount     int               // Number of valid JSON step lines parsed.
	HasCheckpoint bool              // True if a checkpoint marker or checkpoint step was encountered.
}

// rawStep models a single JSONL line from an Antigravity conversation transcript.
type rawStep struct {
	StepIndex       int             `json:"step_index"`       // Zero-based trajectory step index.
	Source          string          `json:"source"`           // Step origin (e.g., USER_EXPLICIT, MODEL, SYSTEM).
	Type            string          `json:"type"`             // Step category (e.g., USER_INPUT, PLANNER_RESPONSE).
	Status          string          `json:"status"`           // Execution status (e.g., DONE, ERROR).
	CreatedAt       string          `json:"created_at"`       // ISO-8601 creation timestamp string.
	Content         string          `json:"content"`          // Primary text payload or tool output content.
	Thinking        string          `json:"thinking"`         // Internal model reasoning text.
	ToolCalls       json.RawMessage `json:"tool_calls"`       // Raw JSON array of tool call invocations.
	Media           json.RawMessage `json:"media"`            // Raw JSON array of attached media descriptors.
	TruncatedFields []string        `json:"truncated_fields"` // Names of fields truncated in compact transcripts.
}

// toolCallItem represents a single tool invocation inside rawStep.ToolCalls.
type toolCallItem struct {
	Name string         `json:"name"` // Invoked tool name (e.g., run_command, view_file).
	Args map[string]any `json:"args"` // Unstructured JSON key-value parameters passed to the tool invocation.
}

// multiReadCloser concatenates multiple io.Reader chunks and closes all underlying handles.
type multiReadCloser struct {
	io.Reader             // Concatenated reader across all chunk files.
	closers   []io.Closer // Underlying file handles to close when reading finishes.
}

// Close closes every underlying chunk file handle and returns the first error encountered.
func (m *multiReadCloser) Close() error {
	var firstErr error
	for _, c := range m.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// OpenTranscript opens a transcript stream from either a file path or conversation directory.
// The caller must call Close on the returned io.ReadCloser when finished.
func OpenTranscript(pathOrDir string) (io.ReadCloser, error) {
	// --- Step 1: Stat target path and open directly if it is a regular file ---
	fi, err := os.Stat(pathOrDir)
	if err != nil {
		return nil, fmt.Errorf("stat transcript path %s: %w", pathOrDir, err)
	}

	if !fi.IsDir() {
		return os.Open(pathOrDir)
	}

	// --- Step 2: Descend into .system_generated/logs if present ---
	sysLogs := filepath.Join(pathOrDir, ".system_generated", "logs")
	if sysStat, err := os.Stat(sysLogs); err == nil && sysStat.IsDir() {
		return OpenTranscript(sysLogs)
	}

	// --- Step 3: Prefer monolithic transcript_full.jsonl when available ---
	fullPath := filepath.Join(pathOrDir, "transcript_full.jsonl")
	if f, err := os.Open(fullPath); err == nil {
		return f, nil
	}

	// --- Step 4: Fall back to ordered chunk files under chunks/transcript_full/ ---
	chunksDir := filepath.Join(pathOrDir, "chunks", "transcript_full")
	if entries, err := os.ReadDir(chunksDir); err == nil {
		var chunkFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
				chunkFiles = append(chunkFiles, e.Name())
			}
		}
		if len(chunkFiles) > 0 {
			sort.Strings(chunkFiles)
			var readers []io.Reader
			var closers []io.Closer
			for _, name := range chunkFiles {
				cf, err := os.Open(filepath.Join(chunksDir, name))
				if err != nil {
					for _, c := range closers {
						_ = c.Close()
					}
					return nil, fmt.Errorf("opening chunk %s: %w", name, err)
				}
				readers = append(readers, cf)
				closers = append(closers, cf)
			}
			return &multiReadCloser{
				Reader:  io.MultiReader(readers...),
				closers: closers,
			}, nil
		}
	}

	// --- Step 5: Fall back to compact transcript.jsonl ---
	compactPath := filepath.Join(pathOrDir, "transcript.jsonl")
	if f, err := os.Open(compactPath); err == nil {
		return f, nil
	}

	return nil, fmt.Errorf("no transcript source found in %s", pathOrDir)
}

// ParseTranscript parses a transcript from a file path or directory.
func ParseTranscript(pathOrDir string) (*ParsedTranscript, error) {
	rc, err := OpenTranscript(pathOrDir)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return ParseTranscriptStream(rc)
}

// ParseTranscriptStream parses a transcript from an io.Reader stream.
func ParseTranscriptStream(r io.Reader) (*ParsedTranscript, error) {
	// --- Step 1: Configure buffered scanner with a 50 MB line ceiling ---
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*50) // Up to 50 MB per line.

	parsed := &ParsedTranscript{
		Checkpoints: make(map[string]string),
	}

	prevHadToolCall := false

	// --- Step 2: Scan JSONL lines, accumulate footprint stats, and normalize steps ---
	for scanner.Scan() {
		line := scanner.Bytes()
		parsed.RawBytes += int64(len(line)) + 1

		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		if trimmed == "CHECKPOINT" {
			parsed.HasCheckpoint = true
			continue
		}

		var rs rawStep
		if err := json.Unmarshal(line, &rs); err != nil {
			continue
		}

		parsed.StepCount++
		cLen := int64(len(rs.Content))
		thLen := int64(len(rs.Thinking))
		tcLen := int64(len(rs.ToolCalls))
		parsed.RawChars += cLen + thLen + tcLen

		// Update session start and end bounds from step timestamp.
		if rs.CreatedAt != "" {
			t, err := parseTimestamp(rs.CreatedAt)
			if err == nil && !t.IsZero() {
				if parsed.Started.IsZero() || t.Before(parsed.Started) {
					parsed.Started = t
				}
				if parsed.Ended.IsZero() || t.After(parsed.Ended) {
					parsed.Ended = t
				}
			}
		}

		// Detect and extract checkpoint sections when present.
		isCheckpointStep := rs.Type == "CHECKPOINT" ||
			(rs.Source == "SYSTEM" && (strings.Contains(rs.Content, "User Knowledge") ||
				strings.Contains(rs.Content, "Model Knowledge") ||
				strings.Contains(rs.Content, "Work Accomplished") ||
				strings.Contains(rs.Content, "## ")))

		if isCheckpointStep {
			parsed.HasCheckpoint = true
			sections := ParseCheckpointSections(rs.Content)
			for k, v := range sections {
				parsed.Checkpoints[k] = v
			}
		}

		// Identify whether the step carries tool execution output.
		isToolOutput := false
		if prevHadToolCall && (rs.Type == "GENERIC" || rs.Type == "TOOL_OUTPUT" || rs.Type == "COMMAND_OUTPUT") {
			isToolOutput = true
		} else if strings.Contains(strings.ToLower(rs.Source), "tool") || rs.Type == "TOOL_OUTPUT" || rs.Type == "COMMAND_OUTPUT" {
			isToolOutput = true
		}

		normStep := normalizeStep(rs, isToolOutput, isCheckpointStep)
		parsed.Steps = append(parsed.Steps, normStep)

		prevHadToolCall = len(rs.ToolCalls) > 0
	}

	// --- Step 3: Check for stream read errors before returning ---
	if err := scanner.Err(); err != nil {
		if !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("reading transcript stream: %w", err)
		}
	}

	return parsed, nil
}

// parseTimestamp parses RFC3339 or ISO-8601 date/time strings into UTC time.Time values.
func parseTimestamp(val string) (time.Time, error) {
	// --- Step 1: Trim and try standard RFC3339 layouts ---
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC(), nil
	}

	// --- Step 2: Fall back to timezone-less date/time layouts ---
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, val); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time: %s", val)
}

// normalizeStep converts a raw transcript step into a canonical harness.Step.
func normalizeStep(rs rawStep, isToolOutput, isCheckpoint bool) harness.Step {
	// --- Step 1: Handle checkpoint steps directly as note steps ---
	step := harness.Step{
		Index: rs.StepIndex,
	}

	if isCheckpoint {
		step.Kind = harness.StepKindNote
		step.Text = rs.Content
		return step
	}

	// --- Step 2: Classify tool calls by tool name ---
	if len(rs.ToolCalls) > 0 {
		var calls []toolCallItem
		if err := json.Unmarshal(rs.ToolCalls, &calls); err == nil && len(calls) > 0 {
			call := calls[0]
			switch call.Name {
			case "run_command":
				step.Kind = harness.StepKindCommand
				if cmd, ok := call.Args["CommandLine"].(string); ok {
					step.Command = cmd
				}
			case "view_file", "read_file":
				step.Kind = harness.StepKindRead
				if f, ok := call.Args["AbsolutePath"].(string); ok {
					step.File = f
				} else if f, ok := call.Args["Path"].(string); ok {
					step.File = f
				} else if f, ok := call.Args["path"].(string); ok {
					step.File = f
				}
			case "replace_file_content", "write_to_file", "edit_file", "write_file", "multi_replace_file_content", "sed_file", "notebook_edit":
				step.Kind = harness.StepKindEdit
				if f, ok := call.Args["TargetFile"].(string); ok {
					step.File = f
				} else if f, ok := call.Args["Path"].(string); ok {
					step.File = f
				} else if f, ok := call.Args["path"].(string); ok {
					step.File = f
				} else if f, ok := call.Args["NotebookPath"].(string); ok {
					step.File = f
				}
			default:
				step.Kind = harness.StepKindAgent
				step.Action = call.Name
				step.Text = string(rs.ToolCalls)
			}
			return step
		}
	}

	// --- Step 3: Classify tool outputs, user prompts, reasoning, and agent text ---
	if isToolOutput {
		step.Kind = harness.StepKindCommand
		step.Excerpt = rs.Content
		return step
	}

	if rs.Type == "USER_INPUT" || rs.Source == "USER_EXPLICIT" {
		step.Kind = harness.StepKindUser
		step.Text = rs.Content
		return step
	}

	if rs.Thinking != "" {
		step.Kind = harness.StepKindReasoning
		step.Text = rs.Thinking
		return step
	}

	step.Kind = harness.StepKindAgent
	step.Text = rs.Content
	return step
}
