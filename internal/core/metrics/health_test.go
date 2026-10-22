/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package metrics_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/metrics"
)

func TestCaptureHealth_Metrics(t *testing.T) {
	ch := metrics.NewCaptureHealth("repo-alpha")
	ch.AddSeen(20)
	ch.AddCaptured(15)
	ch.AddSkipped(3, "recursion_guard")
	ch.AddSkipped(1, "active_turn")
	ch.AddMissed(1, "deleted_before_sweep")

	if ch.SessionsSeen != 20 {
		t.Errorf("expected 20 seen, got %d", ch.SessionsSeen)
	}
	if ch.SessionsCaptured != 15 {
		t.Errorf("expected 15 captured, got %d", ch.SessionsCaptured)
	}
	if ch.SessionsSkipped != 4 {
		t.Errorf("expected 4 skipped, got %d", ch.SessionsSkipped)
	}
	if ch.SessionsMissed != 1 {
		t.Errorf("expected 1 missed, got %d", ch.SessionsMissed)
	}
	if rate := ch.CaptureRate(); math.Abs(rate-0.75) > 0.001 {
		t.Errorf("expected capture rate 0.75, got %.4f", rate)
	}
	if ch.SkipReasons["recursion_guard"] != 3 {
		t.Errorf("expected 3 recursion_guard skips, got %d", ch.SkipReasons["recursion_guard"])
	}

	// Multiple records aggregation
	ch2 := metrics.NewCaptureHealth("repo-alpha")
	ch2.AddSeen(10)
	ch2.AddCaptured(9)
	ch2.AddSkipped(1, "recursion_guard")

	summary := metrics.AggregateCaptureHealth([]metrics.CaptureHealth{ch, ch2})
	if summary.TotalSessionsSeen != 30 {
		t.Errorf("expected 30 total seen, got %d", summary.TotalSessionsSeen)
	}
	if summary.TotalSessionsCaptured != 24 {
		t.Errorf("expected 24 total captured, got %d", summary.TotalSessionsCaptured)
	}
	if summary.TotalSessionsSkipped != 5 {
		t.Errorf("expected 5 total skipped, got %d", summary.TotalSessionsSkipped)
	}
	if summary.TotalSessionsMissed != 1 {
		t.Errorf("expected 1 total missed, got %d", summary.TotalSessionsMissed)
	}
	if math.Abs(summary.CaptureRate-0.80) > 0.001 {
		t.Errorf("expected 0.80 overall capture rate, got %.4f", summary.CaptureRate)
	}
}

func TestHookExecution_LatenciesAndPercentiles(t *testing.T) {
	// Create mock executions for 'stop' and 'pre_tool_use'
	var records []metrics.HookExecution
	stopDurations := []float64{10, 15, 20, 25, 30, 35, 40, 50, 80, 120} // ms
	for i, d := range stopDurations {
		exec := metrics.NewHookExecution("repo-1", "session-A", "stop", time.Duration(d*1000)*time.Microsecond)
		exec.CumulativeSessionMS = float64((i + 1) * 30)
		records = append(records, exec)
	}

	preToolDurations := []float64{2, 3, 4, 5, 8} // ms
	for _, d := range preToolDurations {
		exec := metrics.NewHookExecution("repo-1", "session-B", "pre_tool_use", time.Duration(d*1000)*time.Microsecond)
		records = append(records, exec)
	}

	agg := metrics.AggregateHookExecutions(records)

	stopStats, ok := agg.PerEventType["stop"]
	if !ok {
		t.Fatal("missing 'stop' stats")
	}
	if stopStats.Count != 10 {
		t.Errorf("expected 10 stop executions, got %d", stopStats.Count)
	}
	if stopStats.MinDurationMS != 10.0 {
		t.Errorf("expected min 10.0, got %f", stopStats.MinDurationMS)
	}
	if stopStats.MaxDurationMS != 120.0 {
		t.Errorf("expected max 120.0, got %f", stopStats.MaxDurationMS)
	}
	// P50 should be between 30 and 35
	if stopStats.P50DurationMS < 30 || stopStats.P50DurationMS > 35 {
		t.Errorf("unexpected P50: %f", stopStats.P50DurationMS)
	}
	// P95 should be >= 80
	if stopStats.P95DurationMS < 80 {
		t.Errorf("unexpected P95: %f", stopStats.P95DurationMS)
	}

	// Cumulative session verification
	if agg.CumulativePerSession["session-A"] != 300.0 { // last cumulative value
		t.Errorf("expected 300.0 cumulative for session-A, got %f", agg.CumulativePerSession["session-A"])
	}
	if agg.CumulativePerSession["session-B"] != 22.0 { // sum of preToolDurations 2+3+4+5+8
		t.Errorf("expected 22.0 cumulative for session-B, got %f", agg.CumulativePerSession["session-B"])
	}
}

func TestTickHealth_AndBudgetExhaustion(t *testing.T) {
	t1 := metrics.NewTickHealth("repo-1", 120*time.Millisecond)
	t1.SkippedOverlaps = 1
	t1.JobBacklog["sweep"] = 3
	t1.JobBacklog["compact"] = 1
	t1.SessionsCaughtBySweep = 2
	t1.SpoolToDeliveryLatencyMS = 450.0

	t2 := metrics.NewTickHealth("repo-1", 200*time.Millisecond)
	t2.BudgetExhausted = true
	t2.JobBacklog["sweep"] = 1
	t2.SessionsCaughtBySweep = 0
	t2.SpoolToDeliveryLatencyMS = 350.0

	summary := metrics.AggregateTickHealth([]metrics.TickHealth{t1, t2})
	if summary.TotalTicks != 2 {
		t.Errorf("expected 2 total ticks, got %d", summary.TotalTicks)
	}
	if summary.BudgetExhaustionCount != 1 {
		t.Errorf("expected 1 budget exhaustion, got %d", summary.BudgetExhaustionCount)
	}
	if summary.TotalSkippedOverlaps != 1 {
		t.Errorf("expected 1 skipped overlap, got %d", summary.TotalSkippedOverlaps)
	}
	if summary.TotalSessionsCaughtBySweep != 2 {
		t.Errorf("expected 2 caught by sweep, got %d", summary.TotalSessionsCaughtBySweep)
	}
	if math.Abs(summary.MeanSpoolToDeliveryMS-400.0) > 0.001 {
		t.Errorf("expected 400.0 mean spool latency, got %f", summary.MeanSpoolToDeliveryMS)
	}
	if summary.AverageBacklog["sweep"] != 2.0 {
		t.Errorf("expected average backlog for sweep to be 2.0, got %f", summary.AverageBacklog["sweep"])
	}

	// Budget exhaustion tests
	b1 := metrics.NewBudgetExhaustion("repo-1", "triage", 50000, 52000, "tokens", "hit daily limit")
	b2 := metrics.NewBudgetExhaustion("repo-1", "extraction", 60000, 30000, "tokens", "under limit")
	b3 := metrics.NewBudgetExhaustion("repo-1", "triage", 50000, 50000, "tokens", "cap reached")

	if !b1.Exhausted {
		t.Error("expected b1 to be exhausted")
	}
	if b2.Exhausted {
		t.Error("expected b2 NOT to be exhausted")
	}
	if !b3.Exhausted {
		t.Error("expected b3 to be exhausted")
	}

	bSummary := metrics.AggregateBudgetExhaustion([]metrics.BudgetExhaustion{b1, b2, b3})
	if bSummary.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", bSummary.TotalEvents)
	}
	if bSummary.ExhaustedEvents != 2 {
		t.Errorf("expected 2 exhausted events, got %d", bSummary.ExhaustedEvents)
	}
	if bSummary.ExhaustionByStage["triage"] != 2 {
		t.Errorf("expected 2 triage exhaustions, got %d", bSummary.ExhaustionByStage["triage"])
	}
}

func TestReader_TypedReads(t *testing.T) {
	var buf bytes.Buffer
	writer := metrics.NewWriterFromIO(&buf)

	_ = writer.WriteCaptureHealth(metrics.NewCaptureHealth("repo-1"))
	_ = writer.WriteHookExecution(metrics.NewHookExecution("repo-1", "s1", "stop", 10*time.Millisecond))
	_ = writer.WriteTickHealth(metrics.NewTickHealth("repo-1", 50*time.Millisecond))
	_ = writer.WriteBudgetExhaustion(metrics.NewBudgetExhaustion("repo-1", "sweep", 100, 110, "runs", "max runs reached"))

	reader := metrics.NewReaderFromIO(bytes.NewReader(buf.Bytes()))

	chList, err := reader.ReadCaptureHealth(time.Time{})
	if err != nil || len(chList) != 1 {
		t.Fatalf("expected 1 CaptureHealth, got %d (err: %v)", len(chList), err)
	}

	hookList, err := reader.ReadHookExecutions(time.Time{})
	if err != nil || len(hookList) != 1 {
		t.Fatalf("expected 1 HookExecution, got %d (err: %v)", len(hookList), err)
	}

	tickList, err := reader.ReadTickHealth(time.Time{})
	if err != nil || len(tickList) != 1 {
		t.Fatalf("expected 1 TickHealth, got %d (err: %v)", len(tickList), err)
	}

	budgetList, err := reader.ReadBudgetExhaustions(time.Time{})
	if err != nil || len(budgetList) != 1 {
		t.Fatalf("expected 1 BudgetExhaustion, got %d (err: %v)", len(budgetList), err)
	}
}
