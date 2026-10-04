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
	Steps         []harness.Step
	Checkpoints   map[string]string
	Started       time.Time
	Ended         time.Time
	RawBytes      int64
	RawChars      int64
	StepCount     int
	HasCheckpoint bool
}

type rawStep struct {
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
}

type toolCallItem struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
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

// OpenTranscript opens a transcript stream from either a file path or conversation directory.
func OpenTranscript(pathOrDir string) (io.ReadCloser, error) {
	fi, err := os.Stat(pathOrDir)
	if err != nil {
		return nil, fmt.Errorf("stat transcript path %s: %w", pathOrDir, err)
	}

	if !fi.IsDir() {
		return os.Open(pathOrDir)
	}

	// If directory has .system_generated/logs subpath
	sysLogs := filepath.Join(pathOrDir, ".system_generated", "logs")
	if sysStat, err := os.Stat(sysLogs); err == nil && sysStat.IsDir() {
		return OpenTranscript(sysLogs)
	}

	// 1. Monolithic transcript_full.jsonl
	fullPath := filepath.Join(pathOrDir, "transcript_full.jsonl")
	if f, err := os.Open(fullPath); err == nil {
		return f, nil
	}

	// 2. Chunks under chunks/transcript_full/
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

	// 3. Compact transcript.jsonl fallback
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
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*50) // up to 50MB per line

	parsed := &ParsedTranscript{
		Checkpoints: make(map[string]string),
	}

	prevHadToolCall := false

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

		// Parse timestamps
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

		// Check if checkpoint step
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

		// Determine tool output
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

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("reading transcript stream: %w", err)
		}
	}

	return parsed, nil
}

func parseTimestamp(val string) (time.Time, error) {
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

func normalizeStep(rs rawStep, isToolOutput, isCheckpoint bool) harness.Step {
	step := harness.Step{
		Index: rs.StepIndex,
	}

	if isCheckpoint {
		step.Kind = harness.StepKindNote
		step.Text = rs.Content
		return step
	}

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
			case "view_file":
				step.Kind = harness.StepKindRead
				if f, ok := call.Args["AbsolutePath"].(string); ok {
					step.File = f
				}
			case "replace_file_content", "write_to_file":
				step.Kind = harness.StepKindEdit
				if f, ok := call.Args["TargetFile"].(string); ok {
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
