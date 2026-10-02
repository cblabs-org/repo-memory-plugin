package metrics_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/metrics"
)

func TestWriter_InMemory(t *testing.T) {
	var buf bytes.Buffer
	writer := metrics.NewWriterFromIO(&buf)

	health := metrics.NewCaptureHealth("test-repo")
	health.AddSeen(10)
	health.AddCaptured(8)
	health.AddSkipped(1, "recursion_guard")
	health.AddMissed(1, "transcript_expired")

	if err := writer.WriteCaptureHealth(health); err != nil {
		t.Fatalf("failed to write capture health: %v", err)
	}

	hook := metrics.NewHookExecution("test-repo", "sess-1", "stop", 25*time.Millisecond)
	if err := writer.WriteHookExecution(hook); err != nil {
		t.Fatalf("failed to write hook execution: %v", err)
	}

	tick := metrics.NewTickHealth("test-repo", 150*time.Millisecond)
	tick.JobBacklog["sweep"] = 2
	if err := writer.WriteTickHealth(tick); err != nil {
		t.Fatalf("failed to write tick health: %v", err)
	}

	budget := metrics.NewBudgetExhaustion("test-repo", "housekeeping", 100000, 105000, "tokens", "daily cap reached")
	if err := writer.WriteBudgetExhaustion(budget); err != nil {
		t.Fatalf("failed to write budget exhaustion: %v", err)
	}

	meas := metrics.TranscriptMeasurement{
		TranscriptID:    "tr-1",
		EstimatedTokens: 52000,
		ToolTrafficPct:  86.5,
		CompactionRatio: 4.2,
	}
	if err := writer.WriteTranscriptMeasurement(meas); err != nil {
		t.Fatalf("failed to write transcript measurement: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 5 {
		t.Fatalf("expected 5 JSONL lines, got %d", len(lines))
	}

	// Verify each line is valid JSON with proper type
	expectedTypes := []metrics.MetricType{
		metrics.MetricTypeCaptureHealth,
		metrics.MetricTypeHookExecution,
		metrics.MetricTypeTickHealth,
		metrics.MetricTypeBudgetExhaustion,
		metrics.MetricTypeTranscriptMeasurement,
	}

	for i, line := range lines {
		var rec metrics.GenericRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i, err)
		}
		if rec.Type != expectedTypes[i] {
			t.Errorf("line %d: expected type %s, got %s", i, expectedTypes[i], rec.Type)
		}
	}
}

func TestWriter_FileBackedAndRotation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metrics-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoID := "repo-xyz"
	writer, err := metrics.NewWriter(tempDir, repoID, metrics.WithSyncOnWrite(true))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	m := metrics.NewCaptureHealth(repoID)
	m.AddSeen(5)
	m.AddCaptured(5)
	if err := writer.WriteCaptureHealth(m); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if err := writer.Sync(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Verify file was created in metrics directory with YYYY-MM-DD.jsonl
	metricsDir := filepath.Join(tempDir, repoID, "metrics")
	today := time.Now().UTC().Format("2006-01-02") + ".jsonl"
	expectedPath := filepath.Join(metricsDir, today)

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected metrics file at %s: %v", expectedPath, err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty metrics file")
	}

	// Reader verification
	reader, err := metrics.NewReader(tempDir, repoID)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read all records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != metrics.MetricTypeCaptureHealth {
		t.Errorf("expected type %s, got %s", metrics.MetricTypeCaptureHealth, records[0].Type)
	}
}

func TestWriter_FixedFileName(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metrics-fixed-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoID := "repo-abc"
	writer, err := metrics.NewWriter(tempDir, repoID, metrics.WithFileName("custom_events.jsonl"))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	hook := metrics.NewHookExecution(repoID, "s1", "stop", 10*time.Millisecond)
	if err := writer.WriteHookExecution(hook); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	_ = writer.Close()

	customPath := filepath.Join(tempDir, repoID, "metrics", "custom_events.jsonl")
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("expected file at %s: %v", customPath, err)
	}
}

func TestWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	writer := metrics.NewWriterFromIO(&buf)

	concurrency := 25
	writesPerGoroutine := 40
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				h := metrics.NewHookExecution("repo", fmt.Sprintf("sess-%d", workerID), "pre_invocation", time.Duration(j)*time.Millisecond)
				if err := writer.WriteHookExecution(h); err != nil {
					t.Errorf("worker %d write %d failed: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	expectedCount := concurrency * writesPerGoroutine
	if len(lines) != expectedCount {
		t.Fatalf("expected %d lines, got %d", expectedCount, len(lines))
	}

	for idx, line := range lines {
		var rec metrics.HookExecution
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("line %d malformed JSON: %v", idx, err)
		}
	}
}
