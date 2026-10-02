package metrics

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

	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/compact"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// TranscriptSourceType represents the source variant of a transcript.
type TranscriptSourceType string

const (
	SourceMonolithicFull TranscriptSourceType = "monolithic_full"
	SourceChunkedFull    TranscriptSourceType = "chunked_full"
	SourceCompact        TranscriptSourceType = "compact"
)

// multiReadCloser joins multiple readers and closes them all upon Close().
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

// OpenTranscriptStream opens a transcript stream from a conversation directory in priority order (§6.2):
// 1. transcript_full.jsonl (complete, untruncated)
// 2. chunks/transcript_full/*.jsonl (concatenated in lexical order as a continuous byte stream)
// 3. transcript.jsonl (compact fallback)
func OpenTranscriptStream(dir string) (io.ReadCloser, TranscriptSourceType, error) {
	fullPath := filepath.Join(dir, "transcript_full.jsonl")
	if f, err := os.Open(fullPath); err == nil {
		return f, SourceMonolithicFull, nil
	}

	chunksDir := filepath.Join(dir, "chunks", "transcript_full")
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
					return nil, "", fmt.Errorf("opening chunk %s: %w", name, err)
				}
				readers = append(readers, cf)
				closers = append(closers, cf)
			}
			return &multiReadCloser{
				Reader:  io.MultiReader(readers...),
				closers: closers,
			}, SourceChunkedFull, nil
		}
	}

	compactPath := filepath.Join(dir, "transcript.jsonl")
	if f, err := os.Open(compactPath); err == nil {
		return f, SourceCompact, nil
	}

	return nil, "", fmt.Errorf("no transcript source found in %s", dir)
}

// rawStep mirrors the raw JSONL step structure in Antigravity transcripts.
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

// MeasureTranscript analyzes a transcript stream, measuring raw bytes, characters, estimated tokens,
// tool traffic proportion, and compaction ratio (SPEC §4.4.1, §5.1).
func MeasureTranscript(r io.Reader) (*TranscriptMeasurement, []harness.Step, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*50) // up to 50MB per line

	var totalBytes int64
	var totalChars int64
	var toolChars int64
	var toolBytes int64
	var stepCount int

	var steps []harness.Step
	prevHadToolCall := false
	isSubagent := false

	compactor := compact.NewCompactor()

	for scanner.Scan() {
		line := scanner.Bytes()
		lineLen := int64(len(line)) + 1 // include newline
		totalBytes += lineLen

		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" || trimmed == "CHECKPOINT" {
			continue
		}

		var rs rawStep
		if err := json.Unmarshal(line, &rs); err != nil {
			// Count raw bytes even if unmarshal fails on non-JSON line
			continue
		}

		stepCount++
		cLen := int64(len(rs.Content))
		thLen := int64(len(rs.Thinking))
		tcLen := int64(len(rs.ToolCalls))
		stepChars := cLen + thLen + tcLen
		totalChars += stepChars

		// Detect subagent indicators
		if stepCount <= 3 {
			if strings.Contains(rs.Content, "subagent_reminder") ||
				strings.Contains(rs.Content, "You are running as a subagent") ||
				strings.Contains(rs.Content, "invoked by a caller agent") ||
				strings.Contains(rs.Content, "<SYSTEM_MESSAGE>\n[Message]") {
				isSubagent = true
			}
		}

		// Detect tool traffic
		isToolStep := false
		if tcLen > 0 {
			isToolStep = true
			toolChars += tcLen
			toolBytes += tcLen
		}
		if prevHadToolCall && (rs.Type == "GENERIC" || rs.Type == "TOOL_OUTPUT" || rs.Type == "COMMAND_OUTPUT") {
			isToolStep = true
			toolChars += cLen
			toolBytes += cLen
		} else if strings.Contains(strings.ToLower(rs.Source), "tool") || rs.Type == "TOOL_OUTPUT" || rs.Type == "COMMAND_OUTPUT" {
			isToolStep = true
			toolChars += cLen
			toolBytes += cLen
		}

		prevHadToolCall = tcLen > 0

		// Normalize to harness.Step for compaction
		normStep := normalizeRawStep(rs, isToolStep)
		steps = append(steps, normStep)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading transcript stream: %w", err)
	}

	// Calculate tokens (~4 chars per token standard rule of thumb)
	estTokens := totalChars / 4
	toolTokens := toolChars / 4

	var toolTrafficPct float64
	if totalChars > 0 {
		toolTrafficPct = (float64(toolChars) / float64(totalChars)) * 100.0
	}

	// Run compaction
	compacted := compactor.Compact(steps)
	var keptChars int64
	for _, s := range compacted {
		keptChars += int64(len(s.Text) + len(s.Excerpt) + len(s.Command) + len(s.Outcome))
	}
	compactedTokens := keptChars / 4

	var compactionRatio float64
	if keptChars > 0 {
		compactionRatio = float64(totalChars) / float64(keptChars)
	}

	m := &TranscriptMeasurement{
		BaseRecord: BaseRecord{
			Timestamp: time.Now().UTC(),
			Type:      MetricTypeTranscriptMeasurement,
		},
		IsSubagent:        isSubagent,
		StepCount:         stepCount,
		RawBytes:          totalBytes,
		RawChars:          totalChars,
		EstimatedTokens:   estTokens,
		ToolTrafficBytes:  toolBytes,
		ToolTrafficChars:  toolChars,
		ToolTrafficTokens: toolTokens,
		ToolTrafficPct:    toolTrafficPct,
		CompactedChars:    keptChars,
		CompactedTokens:   compactedTokens,
		CompactionRatio:   compactionRatio,
	}

	return m, steps, nil
}

// normalizeRawStep converts a rawStep into a harness.Step.
func normalizeRawStep(rs rawStep, isTool bool) harness.Step {
	step := harness.Step{
		Index: rs.StepIndex,
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
				step.Text = string(rs.ToolCalls)
			}
			return step
		}
	}

	if isTool {
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

// MeasureTranscriptDir reads and measures the transcript within a conversation directory.
func MeasureTranscriptDir(dir string) (*TranscriptMeasurement, error) {
	stream, src, err := OpenTranscriptStream(dir)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	measurement, _, err := MeasureTranscript(stream)
	if err != nil {
		return nil, err
	}

	measurement.Source = string(src)
	measurement.TranscriptID = filepath.Base(dir)

	// If parent has .system_generated/subagents, check if this dir is a subagent
	parentDir := filepath.Dir(dir)
	subagentsDir := filepath.Join(parentDir, ".system_generated", "subagents")
	if entries, err := os.ReadDir(subagentsDir); err == nil && len(entries) > 0 {
		// Parent spawned subagents
	}
	// Check if this dir itself is named in some other conversation's subagents folder
	brainDir := filepath.Dir(parentDir)
	if filepath.Base(parentDir) == "brain" || filepath.Base(brainDir) == "brain" {
		// It's in brain/<id>
	}

	// Compare with compact transcript.jsonl if present to compute truncation ratio
	compactPath := filepath.Join(dir, "transcript.jsonl")
	if fi, err := os.Stat(compactPath); err == nil && fi.Size() > 0 {
		if measurement.RawBytes > 0 {
			measurement.TruncationExpansionRatio = float64(measurement.RawBytes) / float64(fi.Size())
		}
	}

	return measurement, nil
}

// CategoryStats summarizes measurements for a transcript group.
type CategoryStats struct {
	Name            string  `json:"name"`
	Count           int     `json:"count"`
	MedianTokens    int64   `json:"median_tokens"`
	MeanTokens      int64   `json:"mean_tokens"`
	ToolTrafficPct  float64 `json:"tool_traffic_pct"`
	CompactionRatio float64 `json:"compaction_ratio"`
	MedianRawBytes  int64   `json:"median_raw_bytes"`
	MeanRawBytes    int64   `json:"mean_raw_bytes"`
}

// SizeReport captures transcript size measurements and compaction telemetry across conversations (§4.4.1).
type SizeReport struct {
	TotalTranscripts int                      `json:"total_transcripts"`
	TopLevel         CategoryStats            `json:"top_level"`
	Subagent         CategoryStats            `json:"subagent"`
	Combined         CategoryStats            `json:"combined"`
	Measurements     []*TranscriptMeasurement `json:"measurements,omitempty"`
}

// MeasureDirectories measures transcripts in the specified conversation directories and builds a SizeReport.
func MeasureDirectories(dirs []string) (*SizeReport, error) {
	var topLevel []*TranscriptMeasurement
	var subagents []*TranscriptMeasurement
	var all []*TranscriptMeasurement

	for _, d := range dirs {
		m, err := MeasureTranscriptDir(d)
		if err != nil {
			// Skip directories without transcripts
			continue
		}
		all = append(all, m)
		if m.IsSubagent {
			subagents = append(subagents, m)
		} else {
			topLevel = append(topLevel, m)
		}
	}

	if len(all) == 0 {
		return nil, errors.New("no valid transcripts found to measure")
	}

	report := &SizeReport{
		TotalTranscripts: len(all),
		TopLevel:         summarizeGroup("Antigravity top-level (re-measured)", topLevel),
		Subagent:         summarizeGroup("Antigravity subagent (re-measured)", subagents),
		Combined:         summarizeGroup("Combined", all),
		Measurements:     all,
	}

	return report, nil
}

func summarizeGroup(name string, list []*TranscriptMeasurement) CategoryStats {
	if len(list) == 0 {
		return CategoryStats{Name: name}
	}

	var tokensList []int64
	var rawBytesList []int64
	var sumTokens int64
	var sumBytes int64
	var sumToolTrafficPct float64
	var sumCompactionRatio float64

	for _, m := range list {
		tokensList = append(tokensList, m.EstimatedTokens)
		rawBytesList = append(rawBytesList, m.RawBytes)
		sumTokens += m.EstimatedTokens
		sumBytes += m.RawBytes
		sumToolTrafficPct += m.ToolTrafficPct
		if m.CompactionRatio > 0 {
			sumCompactionRatio += m.CompactionRatio
		}
	}

	sort.Slice(tokensList, func(i, j int) bool { return tokensList[i] < tokensList[j] })
	sort.Slice(rawBytesList, func(i, j int) bool { return rawBytesList[i] < rawBytesList[j] })

	n := int64(len(list))
	medianTokens := tokensList[len(tokensList)/2]
	medianBytes := rawBytesList[len(rawBytesList)/2]

	return CategoryStats{
		Name:            name,
		Count:           len(list),
		MedianTokens:    medianTokens,
		MeanTokens:      sumTokens / n,
		ToolTrafficPct:  sumToolTrafficPct / float64(n),
		CompactionRatio: sumCompactionRatio / float64(n),
		MedianRawBytes:  medianBytes,
		MeanRawBytes:    sumBytes / n,
	}
}

// FormatSection441Table formats a SizeReport into the markdown table specified in SPEC §4.4.1.
func FormatSection441Table(report *SizeReport) string {
	var sb strings.Builder
	sb.WriteString("| Transcripts | Count | Median | Mean | Tool traffic | Compaction Ratio |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")

	writeRow := func(c CategoryStats) {
		if c.Count == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %dk tokens | %dk tokens | %.0f%% | %.1fx |\n",
			c.Name,
			c.Count,
			c.MedianTokens/1000,
			c.MeanTokens/1000,
			c.ToolTrafficPct,
			c.CompactionRatio,
		))
	}

	writeRow(report.TopLevel)
	writeRow(report.Subagent)
	writeRow(report.Combined)

	return sb.String()
}
