package metrics_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/metrics"
)

func TestMeasureTranscript_Fixtures(t *testing.T) {
	fixturesBase := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1")

	// 1. Full Normal
	normalDir := filepath.Join(fixturesBase, "full_normal")
	normalMeas, err := metrics.MeasureTranscriptDir(normalDir)
	if err != nil {
		t.Fatalf("failed to measure full_normal: %v", err)
	}
	if normalMeas.Source != string(metrics.SourceMonolithicFull) {
		t.Errorf("expected source %s, got %s", metrics.SourceMonolithicFull, normalMeas.Source)
	}
	if normalMeas.StepCount != 2 { // 2 JSON steps (CHECKPOINT skipped)
		t.Errorf("expected 2 steps, got %d", normalMeas.StepCount)
	}
	if normalMeas.RawBytes == 0 {
		t.Error("expected non-zero raw bytes")
	}

	// 2. Full Chunked (merged chunks concatenated in lexical order)
	chunkedDir := filepath.Join(fixturesBase, "full_chunked")
	chunkedMeas, err := metrics.MeasureTranscriptDir(chunkedDir)
	if err != nil {
		t.Fatalf("failed to measure full_chunked: %v", err)
	}
	if chunkedMeas.Source != string(metrics.SourceChunkedFull) {
		t.Errorf("expected source %s, got %s", metrics.SourceChunkedFull, chunkedMeas.Source)
	}
	if chunkedMeas.StepCount != 2 {
		t.Errorf("expected 2 steps across chunks, got %d", chunkedMeas.StepCount)
	}

	// 3. Compact fallback
	compactDir := filepath.Join(fixturesBase, "compact")
	compactMeas, err := metrics.MeasureTranscriptDir(compactDir)
	if err != nil {
		t.Fatalf("failed to measure compact: %v", err)
	}
	if compactMeas.Source != string(metrics.SourceCompact) {
		t.Errorf("expected source %s, got %s", metrics.SourceCompact, compactMeas.Source)
	}
}

func TestMeasureTranscript_CompactionRatio_Ge4x(t *testing.T) {
	// Construct a synthetic full transcript with realistic tool traffic, command outputs, file reads, and device output
	var sb strings.Builder

	// Step 0: User input
	s0 := map[string]any{
		"step_index": 0, "source": "USER_EXPLICIT", "type": "USER_INPUT", "status": "DONE",
		"content": "Please run the test suite and fix any failing unit tests in main.go",
	}
	b0, _ := json.Marshal(s0)
	sb.Write(b0)
	sb.WriteByte('\n')

	// Step 1: Model planner response with run_command tool call
	cmdOutput := strings.Repeat("PASS: test_case_alpha\n", 100) +
		"FAIL: test_case_beta (expected true, got false)\n" +
		strings.Repeat("PASS: test_case_gamma\n", 100) +
		"FAILURES: 1 of 201 failed\n"

	s1 := map[string]any{
		"step_index": 1, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "DONE",
		"thinking": "We need to run the test suite using `go test ./...` to discover failures.\n",
		"tool_calls": []map[string]any{
			{
				"name": "run_command",
				"args": map[string]any{"CommandLine": "go test -v ./..."},
			},
		},
	}
	b1, _ := json.Marshal(s1)
	sb.Write(b1)
	sb.WriteByte('\n')

	// Step 2: Tool execution output
	s2 := map[string]any{
		"step_index": 2, "source": "MODEL", "type": "GENERIC", "status": "DONE",
		"content": cmdOutput,
	}
	b2, _ := json.Marshal(s2)
	sb.Write(b2)
	sb.WriteByte('\n')

	// Step 3: Model views file (view_file tool call)
	fileContent := strings.Repeat("package main\n\nfunc Helper() bool {\n    return true\n}\n", 150)
	s3 := map[string]any{
		"step_index": 3, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "DONE",
		"tool_calls": []map[string]any{
			{
				"name": "view_file",
				"args": map[string]any{"AbsolutePath": "/path/to/main.go"},
			},
		},
	}
	b3, _ := json.Marshal(s3)
	sb.Write(b3)
	sb.WriteByte('\n')

	// Step 4: File content output
	s4 := map[string]any{
		"step_index": 4, "source": "MODEL", "type": "GENERIC", "status": "DONE",
		"content": fileContent,
	}
	b4, _ := json.Marshal(s4)
	sb.Write(b4)
	sb.WriteByte('\n')

	// Step 5: Replace file content (edit)
	s5 := map[string]any{
		"step_index": 5, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "DONE",
		"tool_calls": []map[string]any{
			{
				"name": "replace_file_content",
				"args": map[string]any{
					"TargetFile":         "/path/to/main.go",
					"ReplacementContent": fileContent + "\nfunc Fix() {}",
				},
			},
		},
	}
	b5, _ := json.Marshal(s5)
	sb.Write(b5)
	sb.WriteByte('\n')

	// Step 6: Edit result
	s6 := map[string]any{
		"step_index": 6, "source": "MODEL", "type": "GENERIC", "status": "DONE",
		"content": "File updated successfully.",
	}
	b6, _ := json.Marshal(s6)
	sb.Write(b6)
	sb.WriteByte('\n')

	meas, steps, err := metrics.MeasureTranscript(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("failed to measure transcript: %v", err)
	}

	if len(steps) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(steps))
	}

	t.Logf("Measured synthetic full transcript: raw chars=%d, kept chars=%d, compaction ratio=%.2fx, tool traffic=%.1f%%",
		meas.RawChars, meas.CompactedChars, meas.CompactionRatio, meas.ToolTrafficPct)

	// Verify compaction ratio >= 4x
	if meas.CompactionRatio < 4.0 {
		t.Errorf("expected compaction ratio >= 4.0x, got %.2fx", meas.CompactionRatio)
	}

	// Tool traffic should dominate (typically > 80%)
	if meas.ToolTrafficPct < 70.0 {
		t.Errorf("expected tool traffic pct >= 70%%, got %.1f%%", meas.ToolTrafficPct)
	}
}

func TestReMeasureSection441_SurvivingTranscripts(t *testing.T) {
	// Re-measure surviving transcripts from disk if present (SPEC §4.4.1)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("unable to locate user home directory")
	}

	brainDir := filepath.Join(homeDir, ".gemini", "antigravity", "brain")
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		t.Logf("brain directory %s not present or inaccessible, testing with fixtures only", brainDir)
		fixturesBase := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1")
		dirs := []string{
			filepath.Join(fixturesBase, "full_normal"),
			filepath.Join(fixturesBase, "full_chunked"),
		}
		report, err := metrics.MeasureDirectories(dirs)
		if err != nil {
			t.Fatalf("measuring fixtures failed: %v", err)
		}
		table := metrics.FormatSection441Table(report)
		t.Logf("Fixture Section 4.4.1 table:\n%s", table)
		return
	}

	var candidateDirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		logsDir := filepath.Join(brainDir, e.Name(), ".system_generated", "logs")
		fullPath := filepath.Join(logsDir, "transcript_full.jsonl")
		chunksDir := filepath.Join(logsDir, "chunks", "transcript_full")

		if _, err := os.Stat(fullPath); err == nil {
			candidateDirs = append(candidateDirs, logsDir)
		} else if _, err := os.Stat(chunksDir); err == nil {
			candidateDirs = append(candidateDirs, logsDir)
		}

		if len(candidateDirs) >= 50 {
			// Measure a representative sample of 50 conversations for test speed
			break
		}
	}

	if len(candidateDirs) == 0 {
		t.Skip("no surviving conversation logs found in brain directory")
	}

	t.Logf("Re-measuring §4.4.1 transcript sizes on %d surviving full transcripts...", len(candidateDirs))
	report, err := metrics.MeasureDirectories(candidateDirs)
	if err != nil {
		t.Fatalf("measuring surviving transcripts failed: %v", err)
	}

	table := metrics.FormatSection441Table(report)
	t.Logf("\n=== Re-measured SPEC §4.4.1 Transcripts Table ===\n%s", table)

	// Validate re-measurement outputs
	if report.TotalTranscripts != len(candidateDirs) {
		t.Errorf("expected %d measured transcripts, got %d", len(candidateDirs), report.TotalTranscripts)
	}

	// Tool traffic percentage is expected to be high (> 70%)
	if report.Combined.ToolTrafficPct < 60.0 {
		t.Errorf("combined tool traffic unexpectedly low: %.1f%%", report.Combined.ToolTrafficPct)
	}

	// Verify compaction ratio >= 4x
	if report.Combined.CompactionRatio < 4.0 {
		t.Errorf("combined compaction ratio unexpectedly below 4x: %.2fx", report.Combined.CompactionRatio)
	}

	t.Logf("Re-measurement validated: Median tokens=%d, Mean tokens=%d, Tool traffic=%.1f%%, Compaction ratio=%.2fx",
		report.Combined.MedianTokens, report.Combined.MeanTokens, report.Combined.ToolTrafficPct, report.Combined.CompactionRatio)
}
