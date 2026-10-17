/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/metrics/types.go
 * Description: Defines the Record interface and structured JSONL telemetry
 *               record types for capture health, hook execution, tick health,
 *               budget exhaustion, transcript measurement, and generic records.
 *
 * Exported Types / Functions:
 * - MetricType:                      Identifies the schema kind of a telemetry record.
 * - MetricTypeCaptureHealth:         Telemetry kind for session capture health.
 * - MetricTypeHookExecution:         Telemetry kind for hook invocation latency.
 * - MetricTypeTickHealth:            Telemetry kind for scheduled tick execution health.
 * - MetricTypeBudgetExhaustion:      Telemetry kind for pipeline stage budget exhaustion.
 * - MetricTypeTranscriptMeasurement: Telemetry kind for transcript size and compaction stats.
 * - MetricTypeCustom:                Telemetry kind for custom arbitrary records.
 * - Record:                          Common interface implemented by all telemetry records.
 * - BaseRecord:                      Common metadata embedded in all telemetry records.
 *
 *   Record (Record):
 *   - GetTimestamp():  Returns the UTC timestamp when the record was created.
 *   - GetMetricType(): Returns the MetricType classification of the record.
 *   - GetRepoID():     Returns the repository identifier associated with the record.
 *   - GetSessionID():  Returns the session identifier associated with the record.
 *
 * - CaptureHealth:                   Session capture telemetry from sweep and hooks.
 *   - NewCaptureHealth():            Constructs an initialized CaptureHealth record.
 *   - AddSeen():                     Increments the count of sessions inspected.
 *   - AddCaptured():                 Increments the count of sessions captured.
 *   - AddSkipped():                  Increments the count of skipped sessions and records the reason.
 *   - AddMissed():                   Increments the count of missed sessions and records the reason.
 *   - CaptureRate():                 Returns the ratio of captured sessions to seen sessions.
 * - HookExecution:                   Hook invocation latency and cumulative session timing record.
 *   - NewHookExecution():            Constructs a HookExecution record from a duration.
 * - TickHealth:                      Scheduled tick execution duration, budget, and backlog record.
 *   - NewTickHealth():               Constructs an initialized TickHealth record.
 * - BudgetExhaustion:                Pipeline stage budget consumption and exhaustion record.
 *   - NewBudgetExhaustion():         Constructs a BudgetExhaustion record.
 * - TranscriptMeasurement:           Transcript byte, token, tool traffic, and compaction record.
 * - GenericRecord:                   Fallback record for unmarshaling arbitrary JSONL metrics.
 *
 *   Record (Record):
 *   - GetTimestamp():  Returns the UTC timestamp when the record was created (via BaseRecord).
 *   - GetMetricType(): Returns the MetricType classification of the record (via BaseRecord).
 *   - GetRepoID():     Returns the repository identifier associated with the record (via BaseRecord).
 *   - GetSessionID():  Returns the session identifier associated with the record (via BaseRecord).
 *
 *   Unmarshaler (json.Unmarshaler):
 *   - UnmarshalJSON(): Unmarshals BaseRecord fields and retains extra keys in Data. [json.Unmarshaler]
 */

package metrics

import (
	"encoding/json"
	"time"
)

// MetricType defines the kind of telemetry record.
type MetricType string

const (
	MetricTypeCaptureHealth         MetricType = "capture_health"         // Session capture health record.
	MetricTypeHookExecution         MetricType = "hook_execution"         // Hook execution latency record.
	MetricTypeTickHealth            MetricType = "tick_health"            // Scheduled tick health record.
	MetricTypeBudgetExhaustion      MetricType = "budget_exhaustion"      // Pipeline budget exhaustion record.
	MetricTypeTranscriptMeasurement MetricType = "transcript_measurement" // Transcript footprint and compaction measurement record.
	MetricTypeCustom                MetricType = "custom"                 // Custom or unstructured telemetry record.
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
	Timestamp time.Time  `json:"timestamp"`            // UTC timestamp when the metric event was recorded.
	Type      MetricType `json:"type"`                 // Discriminator tag selecting the concrete JSONL schema (e.g. "capture_health").
	RepoID    string     `json:"repo_id,omitempty"`    // Repository identifier associated with the event.
	SessionID string     `json:"session_id,omitempty"` // Optional session or conversation identifier.
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
	BaseRecord                      // Embedded common telemetry metadata.
	SessionsSeen     int            `json:"sessions_seen"`          // Number of sessions inspected during the pass.
	SessionsCaptured int            `json:"sessions_captured"`      // Number of sessions successfully captured.
	SessionsSkipped  int            `json:"sessions_skipped"`       // Number of sessions intentionally skipped.
	SessionsMissed   int            `json:"sessions_missed"`        // Number of sessions missed due to errors.
	SkipReasons      map[string]int `json:"skip_reasons,omitempty"` // Counts of skipped sessions keyed by reason.
	MissReasons      map[string]int `json:"miss_reasons,omitempty"` // Counts of missed sessions keyed by reason.
	DurationMS       float64        `json:"duration_ms,omitempty"`  // Duration of the capture pass in milliseconds.
	Harness          string         `json:"harness,omitempty"`      // Harness identifier (e.g. "antigravity").
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
	BaseRecord                            // Common timestamp, repo_id, and session_id header for hook telemetry.
	EventType           string            `json:"event_type"`                      // Hook lifecycle event (e.g. "stop", "pre_invocation", "pre_tool_use", "post_tool_use").
	DurationMS          float64           `json:"duration_ms"`                     // Wall-clock execution time of this hook invocation in milliseconds.
	CumulativeSessionMS float64           `json:"cumulative_session_ms,omitempty"` // Cumulative hook execution time across the session in milliseconds.
	Harness             string            `json:"harness,omitempty"`               // Agent harness adapter ("antigravity" or "claude-code") that invoked the hook.
	Metadata            map[string]string `json:"metadata,omitempty"`              // Optional hook-specific diagnostic key-value pairs.
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
	BaseRecord                                 // Common timestamp and repo_id header for scheduled tick telemetry.
	TickID                   string            `json:"tick_id,omitempty"`                      // Unique identifier for this tick run.
	DurationMS               float64           `json:"duration_ms"`                            // Total tick execution duration in milliseconds.
	BudgetExhausted          bool              `json:"budget_exhausted"`                       // True if the tick stopped early due to budget exhaustion.
	BudgetLimit              int64             `json:"budget_limit,omitempty"`                 // Configured budget limit for the tick.
	BudgetUsed               int64             `json:"budget_used,omitempty"`                  // Budget units consumed during the tick.
	SkippedOverlaps          int               `json:"skipped_overlaps"`                       // Count of overlapping tick runs skipped due to active lock.
	JobBacklog               map[string]int    `json:"job_backlog,omitempty"`                  // Queued job counts keyed by job kind.
	SessionsCaughtBySweep    int               `json:"sessions_caught_by_sweep"`               // Number of uncaptured sessions recovered by sweep.
	SpoolToDeliveryLatencyMS float64           `json:"spool_to_delivery_latency_ms,omitempty"` // Latency in milliseconds from spooling to delivery.
	Metadata                 map[string]string `json:"metadata,omitempty"`                     // Optional tick-level diagnostic attributes.
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
	BaseRecord        // Common timestamp and repo_id header for budget exhaustion events.
	Stage     string `json:"stage"`            // Pipeline stage name (e.g. "housekeeping", "triage", "extraction", "reconcile", "verify", "tick").
	Limit     int64  `json:"limit"`            // Configured budget ceiling for the stage.
	Consumed  int64  `json:"consumed"`         // Amount of budget consumed.
	Unit      string `json:"unit"`             // Unit of measurement (e.g. "tokens", "runs", "time_ms", "calls").
	Exhausted bool   `json:"exhausted"`        // True when Consumed >= Limit.
	Reason    string `json:"reason,omitempty"` // Optional human-readable reason for budget termination.
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
	BaseRecord                        // Common timestamp, repo_id, and session_id header for transcript sizing metrics.
	TranscriptID             string  `json:"transcript_id,omitempty"`              // Conversation directory name or transcript identifier.
	Source                   string  `json:"source,omitempty"`                     // Transcript source variant ("monolithic_full", "chunked_full", "compact").
	IsSubagent               bool    `json:"is_subagent"`                          // True if the conversation was spawned as a subagent.
	StepCount                int     `json:"step_count"`                           // Number of parsed JSONL steps in the transcript.
	RawBytes                 int64   `json:"raw_bytes"`                            // Total raw byte size of the transcript stream.
	RawChars                 int64   `json:"raw_chars"`                            // Total character count across content, thinking, and tool calls.
	EstimatedTokens          int64   `json:"estimated_tokens"`                     // Estimated token count before compaction (~4 chars/token).
	ToolTrafficBytes         int64   `json:"tool_traffic_bytes"`                   // Byte count attributable to tool calls and tool outputs.
	ToolTrafficChars         int64   `json:"tool_traffic_chars"`                   // Character count attributable to tool calls and tool outputs.
	ToolTrafficTokens        int64   `json:"tool_traffic_tokens"`                  // Estimated token count attributable to tool traffic.
	ToolTrafficPct           float64 `json:"tool_traffic_pct"`                     // Percentage of total characters attributable to tool traffic.
	CompactedChars           int64   `json:"compacted_chars,omitempty"`            // Character count remaining after deterministic compaction.
	CompactedTokens          int64   `json:"compacted_tokens,omitempty"`           // Estimated token count after deterministic compaction.
	CompactionRatio          float64 `json:"compaction_ratio,omitempty"`           // Ratio of RawChars to CompactedChars.
	TruncationExpansionRatio float64 `json:"truncation_expansion_ratio,omitempty"` // Ratio of full transcript bytes to compact transcript bytes.
}

// GenericRecord is a fallback record for unmarshaling arbitrary JSONL metrics.
type GenericRecord struct {
	BaseRecord                // Common timestamp, metric type, repo_id, and session_id decoded from raw JSONL.
	Data       map[string]any `json:"-"` // Additional non-base fields preserved from the raw JSON object.
}

// UnmarshalJSON unmarshals BaseRecord and preserves remaining fields in Data.
func (g *GenericRecord) UnmarshalJSON(b []byte) error {
	// --- Step 1: Unmarshal standard BaseRecord metadata fields ---
	type Alias BaseRecord
	var base Alias
	if err := json.Unmarshal(b, &base); err != nil {
		return err
	}
	g.BaseRecord = BaseRecord(base)

	// --- Step 2: Unmarshal remaining dynamic fields into Data ---
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
