package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

// Reader reads and aggregates telemetry records from <data-dir>/<repo-id>/metrics/*.jsonl (SPEC §10.4).
type Reader struct {
	dirPath  string
	customIO io.Reader
}

// NewReader creates a Reader for the metrics directory under <data-dir>/<repo-id>/metrics.
func NewReader(dataDir, repoID string) (*Reader, error) {
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	if repoID == "" {
		return nil, fmt.Errorf("repoID cannot be empty")
	}

	metricsDir := filepath.Join(dataDir, repoID, "metrics")
	return &Reader{
		dirPath: metricsDir,
	}, nil
}

// NewReaderFromIO creates a Reader backed by an io.Reader.
func NewReaderFromIO(r io.Reader) *Reader {
	return &Reader{
		customIO: r,
	}
}

// ReadAll reads all metric records.
func (r *Reader) ReadAll() ([]GenericRecord, error) {
	return r.ReadSince(time.Time{})
}

// ReadSince reads all metric records with a timestamp >= since.
func (r *Reader) ReadSince(since time.Time) ([]GenericRecord, error) {
	if r.customIO != nil {
		if seeker, ok := r.customIO.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}
		return scanGenericRecords(r.customIO, since)
	}

	entries, err := os.ReadDir(r.dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading metrics directory: %w", err)
	}

	var jsonlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			jsonlFiles = append(jsonlFiles, entry.Name())
		}
	}
	sort.Strings(jsonlFiles)

	var all []GenericRecord
	for _, name := range jsonlFiles {
		filePath := filepath.Join(r.dirPath, name)
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("opening metrics file %s: %w", filePath, err)
		}
		records, err := scanGenericRecords(f, since)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", filePath, err)
		}
		all = append(all, records...)
	}

	return all, nil
}

// ReadCaptureHealth reads all CaptureHealth records.
func (r *Reader) ReadCaptureHealth(since time.Time) ([]CaptureHealth, error) {
	var results []CaptureHealth
	err := r.scanTyped(MetricTypeCaptureHealth, since, func(data []byte) error {
		var rec CaptureHealth
		if err := json.Unmarshal(data, &rec); err != nil {
			return err
		}
		results = append(results, rec)
		return nil
	})
	return results, err
}

// ReadHookExecutions reads all HookExecution records.
func (r *Reader) ReadHookExecutions(since time.Time) ([]HookExecution, error) {
	var results []HookExecution
	err := r.scanTyped(MetricTypeHookExecution, since, func(data []byte) error {
		var rec HookExecution
		if err := json.Unmarshal(data, &rec); err != nil {
			return err
		}
		results = append(results, rec)
		return nil
	})
	return results, err
}

// ReadTickHealth reads all TickHealth records.
func (r *Reader) ReadTickHealth(since time.Time) ([]TickHealth, error) {
	var results []TickHealth
	err := r.scanTyped(MetricTypeTickHealth, since, func(data []byte) error {
		var rec TickHealth
		if err := json.Unmarshal(data, &rec); err != nil {
			return err
		}
		results = append(results, rec)
		return nil
	})
	return results, err
}

// ReadBudgetExhaustions reads all BudgetExhaustion records.
func (r *Reader) ReadBudgetExhaustions(since time.Time) ([]BudgetExhaustion, error) {
	var results []BudgetExhaustion
	err := r.scanTyped(MetricTypeBudgetExhaustion, since, func(data []byte) error {
		var rec BudgetExhaustion
		if err := json.Unmarshal(data, &rec); err != nil {
			return err
		}
		results = append(results, rec)
		return nil
	})
	return results, err
}

// ReadTranscriptMeasurements reads all TranscriptMeasurement records.
func (r *Reader) ReadTranscriptMeasurements(since time.Time) ([]TranscriptMeasurement, error) {
	var results []TranscriptMeasurement
	err := r.scanTyped(MetricTypeTranscriptMeasurement, since, func(data []byte) error {
		var rec TranscriptMeasurement
		if err := json.Unmarshal(data, &rec); err != nil {
			return err
		}
		results = append(results, rec)
		return nil
	})
	return results, err
}

func (r *Reader) scanTyped(mType MetricType, since time.Time, fn func(data []byte) error) error {
	processReader := func(rd io.Reader) error {
		scanner := bufio.NewScanner(rd)
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 1024*1024*10)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var base BaseRecord
			if err := json.Unmarshal(line, &base); err != nil {
				continue
			}
			if base.Type != mType {
				continue
			}
			if !since.IsZero() && base.Timestamp.Before(since) {
				continue
			}

			if err := fn(line); err != nil {
				return err
			}
		}
		return scanner.Err()
	}

	if r.customIO != nil {
		if seeker, ok := r.customIO.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}
		return processReader(r.customIO)
	}

	entries, err := os.ReadDir(r.dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var jsonlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			jsonlFiles = append(jsonlFiles, entry.Name())
		}
	}
	sort.Strings(jsonlFiles)

	for _, name := range jsonlFiles {
		filePath := filepath.Join(r.dirPath, name)
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		err = processReader(f)
		_ = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func scanGenericRecords(rd io.Reader, since time.Time) ([]GenericRecord, error) {
	var results []GenericRecord
	scanner := bufio.NewScanner(rd)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*10)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec GenericRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if !since.IsZero() && rec.Timestamp.Before(since) {
			continue
		}
		results = append(results, rec)
	}

	return results, scanner.Err()
}

// CaptureHealthSummary represents an aggregated overview of capture health telemetry.
type CaptureHealthSummary struct {
	TotalSessionsSeen     int            `json:"total_sessions_seen"`
	TotalSessionsCaptured int            `json:"total_sessions_captured"`
	TotalSessionsSkipped  int            `json:"total_sessions_skipped"`
	TotalSessionsMissed   int            `json:"total_sessions_missed"`
	CaptureRate           float64        `json:"capture_rate"`
	SkipReasons           map[string]int `json:"skip_reasons"`
	MissReasons           map[string]int `json:"miss_reasons"`
}

// AggregateCaptureHealth aggregates multiple CaptureHealth records.
func AggregateCaptureHealth(records []CaptureHealth) CaptureHealthSummary {
	summary := CaptureHealthSummary{
		SkipReasons: make(map[string]int),
		MissReasons: make(map[string]int),
	}

	for _, r := range records {
		summary.TotalSessionsSeen += r.SessionsSeen
		summary.TotalSessionsCaptured += r.SessionsCaptured
		summary.TotalSessionsSkipped += r.SessionsSkipped
		summary.TotalSessionsMissed += r.SessionsMissed

		for reason, count := range r.SkipReasons {
			summary.SkipReasons[reason] += count
		}
		for reason, count := range r.MissReasons {
			summary.MissReasons[reason] += count
		}
	}

	if summary.TotalSessionsSeen > 0 {
		summary.CaptureRate = float64(summary.TotalSessionsCaptured) / float64(summary.TotalSessionsSeen)
	}

	return summary
}

// HookLatencyStats records percentile distribution and cumulative metrics for hook execution.
type HookLatencyStats struct {
	EventType       string  `json:"event_type"`
	Count           int     `json:"count"`
	TotalDurationMS float64 `json:"total_duration_ms"`
	MeanDurationMS  float64 `json:"mean_duration_ms"`
	P50DurationMS   float64 `json:"p50_duration_ms"`
	P95DurationMS   float64 `json:"p95_duration_ms"`
	P99DurationMS   float64 `json:"p99_duration_ms"`
	MinDurationMS   float64 `json:"min_duration_ms"`
	MaxDurationMS   float64 `json:"max_duration_ms"`
}

// HookAggregation contains aggregated hook latency percentiles per event type and cumulative session times (SPEC §10.4).
type HookAggregation struct {
	PerEventType         map[string]HookLatencyStats `json:"per_event_type"`
	CumulativePerSession map[string]float64          `json:"cumulative_per_session"`
	MedianSessionMS      float64                     `json:"median_session_ms"`
	MeanSessionMS        float64                     `json:"mean_session_ms"`
}

// AggregateHookExecutions computes p50, p95, p99 percentiles per event type and session cumulative times.
func AggregateHookExecutions(records []HookExecution) HookAggregation {
	byType := make(map[string][]float64)
	sessionTimes := make(map[string]float64)

	for _, r := range records {
		byType[r.EventType] = append(byType[r.EventType], r.DurationMS)
		if r.SessionID != "" {
			if r.CumulativeSessionMS > 0 {
				if r.CumulativeSessionMS > sessionTimes[r.SessionID] {
					sessionTimes[r.SessionID] = r.CumulativeSessionMS
				}
			} else {
				sessionTimes[r.SessionID] += r.DurationMS
			}
		}
	}

	statsPerType := make(map[string]HookLatencyStats)
	for eventType, durations := range byType {
		if len(durations) == 0 {
			continue
		}
		sort.Float64s(durations)

		count := len(durations)
		var sum float64
		for _, d := range durations {
			sum += d
		}

		p50 := percentile(durations, 0.50)
		p95 := percentile(durations, 0.95)
		p99 := percentile(durations, 0.99)

		statsPerType[eventType] = HookLatencyStats{
			EventType:       eventType,
			Count:           count,
			TotalDurationMS: sum,
			MeanDurationMS:  sum / float64(count),
			P50DurationMS:   p50,
			P95DurationMS:   p95,
			P99DurationMS:   p99,
			MinDurationMS:   durations[0],
			MaxDurationMS:   durations[count-1],
		}
	}

	var sessionVals []float64
	var sumSession float64
	for _, total := range sessionTimes {
		sessionVals = append(sessionVals, total)
		sumSession += total
	}

	var medianSession float64
	var meanSession float64
	if len(sessionVals) > 0 {
		sort.Float64s(sessionVals)
		medianSession = percentile(sessionVals, 0.50)
		meanSession = sumSession / float64(len(sessionVals))
	}

	return HookAggregation{
		PerEventType:         statsPerType,
		CumulativePerSession: sessionTimes,
		MedianSessionMS:      medianSession,
		MeanSessionMS:        meanSession,
	}
}

// TickHealthSummary represents an aggregated overview of tick executions (SPEC §10.4).
type TickHealthSummary struct {
	TotalTicks                 int                `json:"total_ticks"`
	TotalDurationMS            float64            `json:"total_duration_ms"`
	MeanDurationMS             float64            `json:"mean_duration_ms"`
	P50DurationMS              float64            `json:"p50_duration_ms"`
	P95DurationMS              float64            `json:"p95_duration_ms"`
	BudgetExhaustionCount      int                `json:"budget_exhaustion_count"`
	TotalSkippedOverlaps       int                `json:"total_skipped_overlaps"`
	TotalSessionsCaughtBySweep int                `json:"total_sessions_caught_by_sweep"`
	MeanSpoolToDeliveryMS      float64            `json:"mean_spool_to_delivery_ms"`
	AverageBacklog             map[string]float64 `json:"average_backlog"`
}

// AggregateTickHealth computes statistical summaries for tick executions.
func AggregateTickHealth(records []TickHealth) TickHealthSummary {
	if len(records) == 0 {
		return TickHealthSummary{
			AverageBacklog: make(map[string]float64),
		}
	}

	var durations []float64
	var sumDuration float64
	var totalExhausted int
	var totalSkippedOverlaps int
	var totalCaughtBySweep int
	var sumSpoolLatency float64
	var spoolCount int
	backlogSums := make(map[string]float64)

	for _, r := range records {
		durations = append(durations, r.DurationMS)
		sumDuration += r.DurationMS
		if r.BudgetExhausted {
			totalExhausted++
		}
		totalSkippedOverlaps += r.SkippedOverlaps
		totalCaughtBySweep += r.SessionsCaughtBySweep
		if r.SpoolToDeliveryLatencyMS > 0 {
			sumSpoolLatency += r.SpoolToDeliveryLatencyMS
			spoolCount++
		}
		for kind, count := range r.JobBacklog {
			backlogSums[kind] += float64(count)
		}
	}

	sort.Float64s(durations)
	p50 := percentile(durations, 0.50)
	p95 := percentile(durations, 0.95)

	avgBacklog := make(map[string]float64)
	for kind, sum := range backlogSums {
		avgBacklog[kind] = sum / float64(len(records))
	}

	var meanSpool float64
	if spoolCount > 0 {
		meanSpool = sumSpoolLatency / float64(spoolCount)
	}

	return TickHealthSummary{
		TotalTicks:                 len(records),
		TotalDurationMS:            sumDuration,
		MeanDurationMS:             sumDuration / float64(len(records)),
		P50DurationMS:              p50,
		P95DurationMS:              p95,
		BudgetExhaustionCount:      totalExhausted,
		TotalSkippedOverlaps:       totalSkippedOverlaps,
		TotalSessionsCaughtBySweep: totalCaughtBySweep,
		MeanSpoolToDeliveryMS:      meanSpool,
		AverageBacklog:             avgBacklog,
	}
}

// BudgetSummary represents an aggregated overview of budget exhaustion records.
type BudgetSummary struct {
	TotalEvents        int            `json:"total_events"`
	ExhaustedEvents    int            `json:"exhausted_events"`
	ExhaustionByStage  map[string]int `json:"exhaustion_by_stage"`
	TotalLimitByUnit   map[string]int64 `json:"total_limit_by_unit"`
	TotalConsumedByUnit map[string]int64 `json:"total_consumed_by_unit"`
}

// AggregateBudgetExhaustion aggregates budget exhaustion records.
func AggregateBudgetExhaustion(records []BudgetExhaustion) BudgetSummary {
	summary := BudgetSummary{
		TotalEvents:         len(records),
		ExhaustionByStage:   make(map[string]int),
		TotalLimitByUnit:    make(map[string]int64),
		TotalConsumedByUnit: make(map[string]int64),
	}

	for _, r := range records {
		if r.Exhausted {
			summary.ExhaustedEvents++
			summary.ExhaustionByStage[r.Stage]++
		}
		unit := r.Unit
		if unit == "" {
			unit = "tokens"
		}
		summary.TotalLimitByUnit[unit] += r.Limit
		summary.TotalConsumedByUnit[unit] += r.Consumed
	}

	return summary
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1.0 {
		return sorted[len(sorted)-1]
	}

	index := p * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	weight := index - float64(lower)
	return sorted[lower]*(1.0-weight) + sorted[upper]*weight
}
