package metrics

import (
	"encoding/json"
	"time"
)

// MetricType defines the kind of telemetry record.
type MetricType string

const (
	MetricTypeCaptureHealth        MetricType = "capture_health"
	MetricTypeHookExecution        MetricType = "hook_execution"
	MetricTypeTickHealth           MetricType = "tick_health"
	MetricTypeBudgetExhaustion     MetricType = "budget_exhaustion"
	MetricTypeTranscriptMeasurement MetricType = "transcript_measurement"
	MetricTypeCustom               MetricType = "custom"
)

// Record is the common interface implemented by all telemetry records.
type Record interface {
	GetTimestamp() time.Time
	GetMetricType() MetricType
	GetRepoID() string
	GetSessionID() string
}

// BaseRecord provides common metadata for telemetry records.
type BaseRecord struct {
	Timestamp time.Time  `json:"timestamp"`
	Type      MetricType `json:"type"`
	RepoID    string     `json:"repo_id,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
}

// GetTimestamp returns the record timestamp.
func (b BaseRecord) GetTimestamp() time.Time {
	return b.Timestamp
}

// GetMetricType returns the record telemetry type.
func (b BaseRecord) GetMetricType() MetricType {
	return b.Type
}

// GetRepoID returns the repository identifier.
func (b BaseRecord) GetRepoID() string {
	return b.RepoID
}

// GetSessionID returns the session identifier.
func (b BaseRecord) GetSessionID() string {
	return b.SessionID
}

// CaptureHealth records session capture telemetry from sweep and hooks (SPEC §10.4).
type CaptureHealth struct {
	BaseRecord
	SessionsSeen     int            `json:"sessions_seen"`
	SessionsCaptured int            `json:"sessions_captured"`
	SessionsSkipped  int            `json:"sessions_skipped"`
	SessionsMissed   int            `json:"sessions_missed"`
	SkipReasons      map[string]int `json:"skip_reasons,omitempty"`
	MissReasons      map[string]int `json:"miss_reasons,omitempty"`
	DurationMS       float64        `json:"duration_ms,omitempty"`
	Harness          string         `json:"harness,omitempty"`
}

// NewCaptureHealth initializes a CaptureHealth record.
func NewCaptureHealth(repoID string) CaptureHealth {
	return CaptureHealth{
		BaseRecord: BaseRecord{
			Timestamp: time.Now().UTC(),
			Type:      MetricTypeCaptureHealth,
			RepoID:    repoID,
		},
		SkipReasons: make(map[string]int),
		MissReasons: make(map[string]int),
	}
}

// AddSeen increments sessions seen count.
func (c *CaptureHealth) AddSeen(n int) {
	c.SessionsSeen += n
}

// AddCaptured increments sessions captured count.
func (c *CaptureHealth) AddCaptured(n int) {
	c.SessionsCaptured += n
}

// AddSkipped increments sessions skipped count with an optional reason.
func (c *CaptureHealth) AddSkipped(n int, reason string) {
	c.SessionsSkipped += n
	if reason != "" {
		if c.SkipReasons == nil {
			c.SkipReasons = make(map[string]int)
		}
		c.SkipReasons[reason] += n
	}
}

// AddMissed increments sessions missed count with an optional reason.
func (c *CaptureHealth) AddMissed(n int, reason string) {
	c.SessionsMissed += n
	if reason != "" {
		if c.MissReasons == nil {
			c.MissReasons = make(map[string]int)
		}
		c.MissReasons[reason] += n
	}
}

// CaptureRate returns the ratio of captured sessions to seen sessions (0.0 to 1.0).
func (c CaptureHealth) CaptureRate() float64 {
	if c.SessionsSeen == 0 {
		return 0
	}
	return float64(c.SessionsCaptured) / float64(c.SessionsSeen)
}

// HookExecution records hook invocation latency and cumulative session execution time (SPEC §10.4).
type HookExecution struct {
	BaseRecord
	EventType             string            `json:"event_type"` // e.g. "stop", "pre_invocation", "pre_tool_use", "post_tool_use"
	DurationMS            float64           `json:"duration_ms"`
	CumulativeSessionMS   float64           `json:"cumulative_session_ms,omitempty"`
	Harness               string            `json:"harness,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

// NewHookExecution initializes a HookExecution record.
func NewHookExecution(repoID, sessionID, eventType string, duration time.Duration) HookExecution {
	return HookExecution{
		BaseRecord: BaseRecord{
			Timestamp: time.Now().UTC(),
			Type:      MetricTypeHookExecution,
			RepoID:    repoID,
			SessionID: sessionID,
		},
		EventType:  eventType,
		DurationMS: float64(duration.Microseconds()) / 1000.0,
	}
}

// TickHealth records scheduled tick execution health, duration, and backlog (SPEC §10.4).
type TickHealth struct {
	BaseRecord
	TickID                    string            `json:"tick_id,omitempty"`
	DurationMS                float64           `json:"duration_ms"`
	BudgetExhausted           bool              `json:"budget_exhausted"`
	BudgetLimit               int64             `json:"budget_limit,omitempty"`
	BudgetUsed                int64             `json:"budget_used,omitempty"`
	SkippedOverlaps           int               `json:"skipped_overlaps"`
	JobBacklog                map[string]int    `json:"job_backlog,omitempty"`
	SessionsCaughtBySweep     int               `json:"sessions_caught_by_sweep"`
	SpoolToDeliveryLatencyMS  float64           `json:"spool_to_delivery_latency_ms,omitempty"`
	Metadata                  map[string]string `json:"metadata,omitempty"`
}

// NewTickHealth initializes a TickHealth record.
func NewTickHealth(repoID string, duration time.Duration) TickHealth {
	return TickHealth{
		BaseRecord: BaseRecord{
			Timestamp: time.Now().UTC(),
			Type:      MetricTypeTickHealth,
			RepoID:    repoID,
		},
		DurationMS: float64(duration.Microseconds()) / 1000.0,
		JobBacklog: make(map[string]int),
	}
}

// BudgetExhaustion records budget exhaustion events per stage or run (SPEC §4.4.1, §10.4).
type BudgetExhaustion struct {
	BaseRecord
	Stage     string `json:"stage"` // e.g. "housekeeping", "triage", "extraction", "reconcile", "verify", "tick"
	Limit     int64  `json:"limit"`
	Consumed  int64  `json:"consumed"`
	Unit      string `json:"unit"` // e.g. "tokens", "runs", "time_ms", "calls"
	Exhausted bool   `json:"exhausted"`
	Reason    string `json:"reason,omitempty"`
}

// NewBudgetExhaustion initializes a BudgetExhaustion record.
func NewBudgetExhaustion(repoID, stage string, limit, consumed int64, unit string, reason string) BudgetExhaustion {
	return BudgetExhaustion{
		BaseRecord: BaseRecord{
			Timestamp: time.Now().UTC(),
			Type:      MetricTypeBudgetExhaustion,
			RepoID:    repoID,
		},
		Stage:     stage,
		Limit:     limit,
		Consumed:  consumed,
		Unit:      unit,
		Exhausted: consumed >= limit,
		Reason:    reason,
	}
}

// TranscriptMeasurement records transcript footprint, token estimation, and compaction statistics (§4.4.1).
type TranscriptMeasurement struct {
	BaseRecord
	TranscriptID              string  `json:"transcript_id,omitempty"`
	Source                    string  `json:"source,omitempty"` // "monolithic_full", "chunked_full", "compact"
	IsSubagent                bool    `json:"is_subagent"`
	StepCount                 int     `json:"step_count"`
	RawBytes                  int64   `json:"raw_bytes"`
	RawChars                  int64   `json:"raw_chars"`
	EstimatedTokens           int64   `json:"estimated_tokens"`
	ToolTrafficBytes          int64   `json:"tool_traffic_bytes"`
	ToolTrafficChars          int64   `json:"tool_traffic_chars"`
	ToolTrafficTokens         int64   `json:"tool_traffic_tokens"`
	ToolTrafficPct            float64 `json:"tool_traffic_pct"`
	CompactedChars            int64   `json:"compacted_chars,omitempty"`
	CompactedTokens           int64   `json:"compacted_tokens,omitempty"`
	CompactionRatio           float64 `json:"compaction_ratio,omitempty"`
	TruncationExpansionRatio  float64 `json:"truncation_expansion_ratio,omitempty"` // full bytes / compact bytes
}

// GenericRecord is a fallback record for unmarshaling arbitrary JSONL metrics.
type GenericRecord struct {
	BaseRecord
	Data map[string]any `json:"-"`
}

// UnmarshalJSON unmarshals BaseRecord and preserves remaining fields in Data.
func (g *GenericRecord) UnmarshalJSON(b []byte) error {
	type Alias BaseRecord
	var base Alias
	if err := json.Unmarshal(b, &base); err != nil {
		return err
	}
	g.BaseRecord = BaseRecord(base)
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	delete(raw, "timestamp")
	delete(raw, "type")
	delete(raw, "repo_id")
	delete(raw, "session_id")
	g.Data = raw
	return nil
}
