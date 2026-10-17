/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/jobs/types.go
 * Description: Defines domain types, job kinds, priorities, states, execution
 *               cost metrics, query filters, and sentinel errors for the job queue.
 *
 * Exported Types / Functions:
 * - Kind:                Enumerates supported background job kinds.
 *   - Valid():           Reports whether the Kind is a recognized job kind.
 * - KindCompact:         Job kind for transcript compaction.
 * - KindTriage:          Job kind for episode triage.
 * - KindExtract:         Job kind for candidate memory extraction.
 * - KindReconcile:       Job kind for reconciling candidate claims against existing entries.
 * - KindVerify:          Job kind for verifying candidate claims.
 * - KindPublish:         Job kind for publishing verified entries and regenerating views.
 * - KindBrief:           Job kind for generating module dossier briefs.
 * - KindDejavu:          Job kind for matching recurring failure signatures.
 * - KindRecheck:         Job kind for re-evaluating suspect or stale entries.
 * - KindReport:          Job kind for delivering background reports to an IDE conversation.
 * - KindCapture:         Job kind for capturing raw session episodes.
 * - KindSweep:           Job kind for sweeping unindexed or missed sessions.
 * - KindSeedDocs:        Job kind for bootstrapping Day-1 dossiers from repository docs.
 * - KindDreamFreshness:  Job kind for background freshness and drift checks.
 * - KindFallback:        Job kind for fallback distillation passes.
 * - KindConsolidate:     Job kind for consolidating inbox notes into dossiers.
 * - PriorityLow:         Lowest scheduling priority (0).
 * - PriorityNormal:      Standard background job priority (10).
 * - PriorityBatch:       Elevated batch job priority (20).
 * - PriorityInteractive: Highest priority for interactive briefs and déjà-vu checks (100).
 * - State:               Enumerates job lifecycle states.
 *   - Valid():           Reports whether the State is recognized.
 *   - IsTerminal():      Reports whether the State is final (done or dead).
 * - StateQueued:         Job is waiting to be leased by a worker.
 * - StateRunning:        Job is currently leased and executing.
 * - StateDone:           Job completed successfully.
 * - StateFailed:         Job failed transiently and awaits backoff retry.
 * - StateDead:           Job exhausted all attempts or failed permanently.
 * - StateSubmitted:      Report job has been submitted to an IDE conversation.
 * - Cost:                Token, duration, and model usage metrics for a job execution run.
 *
 *   Lifecycle:
 *   - FromUsage():       Constructs a Cost from a harness.Usage value.
 *
 *   Own Public API:
 *   - ToUsage():         Converts the Cost back into a harness.Usage value.
 *
 *   Valuer (driver.Valuer):
 *   - Value():           Serializes Cost as a JSON string for SQLite storage. [driver.Valuer]
 *
 *   Scanner (sql.Scanner):
 *   - Scan():            Deserializes a JSON string or byte slice from SQLite into Cost. [sql.Scanner]
 *
 * - Job:                 Represents a durable unit of asynchronous work in the jobs table.
 * - Filter:              Selection criteria for listing jobs by state, kind, and limit.
 * - ErrJobNotFound:      Sentinel error when a requested job ID or key does not exist.
 * - ErrDuplicateJob:     Sentinel error when a duplicate idempotency key is encountered.
 * - ErrInvalidTransition: Sentinel error when a job state transition is disallowed.
 * - ErrInvalidJob:       Sentinel error when a job specification is missing required fields.
 * - ErrNoJobAvailable:   Sentinel error when no eligible job is ready for leasing.
 * - ErrLeaseExpired:     Sentinel error when renewing a lease that is no longer active.
 */

package jobs

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Kind identifies the category of asynchronous work performed by a Job (SPEC §4.3).
type Kind string

const (
	KindCompact        Kind = "compact"         // Compacts raw session transcripts into bounded episodes.
	KindTriage         Kind = "triage"          // Triages episodes for memory-worthy signal.
	KindExtract        Kind = "extract"         // Extracts candidate claims from triaged episodes.
	KindReconcile      Kind = "reconcile"       // Reconciles extracted claims against existing dossier entries.
	KindVerify         Kind = "verify"          // Verifies candidate claims against repository state.
	KindPublish        Kind = "publish"         // Publishes verified entries and regenerates memory views.
	KindBrief          Kind = "brief"           // Synthesizes a module dossier brief.
	KindDejavu         Kind = "dejavu"          // Evaluates recurring failure signatures (déjà-vu).
	KindRecheck        Kind = "recheck"         // Re-verifies stale or suspect dossier entries.
	KindReport         Kind = "report"          // Submits a report to an active or idle IDE conversation.
	KindCapture        Kind = "capture"         // Captures an agent session transcript into the episode store.
	KindSweep          Kind = "sweep"           // Sweeps conversation directories for uncaptured sessions.
	KindSeedDocs       Kind = "seed-docs"       // Seeds initial dossier entries from repository documentation.
	KindDreamFreshness Kind = "dream-freshness" // Audits dossier entries for code drift during idle ticks.
	KindFallback       Kind = "fallback"        // Runs sidecar fallback distillation when stop-hook notes are absent.
	KindConsolidate    Kind = "consolidate"     // Consolidates spooled inbox notes into canonical dossiers.
)

// Valid returns true if the Kind is recognized.
func (k Kind) Valid() bool {
	switch k {
	case KindCompact, KindTriage, KindExtract, KindReconcile, KindVerify,
		KindPublish, KindBrief, KindDejavu, KindRecheck, KindReport,
		KindCapture, KindSweep, KindSeedDocs, KindDreamFreshness,
		KindFallback, KindConsolidate:
		return true
	default:
		return false
	}
}

// Standard Job Priorities. Higher integer means higher scheduling priority.
const (
	PriorityLow         = 0   // Lowest scheduling priority for opportunistic background tasks.
	PriorityNormal      = 10  // Default priority for standard pipeline jobs.
	PriorityBatch       = 20  // Priority for batch maintenance and sweep jobs.
	PriorityInteractive = 100 // Highest priority for latency-sensitive briefs and déjà-vu checks (SPEC §4.3 step 6).
)

// State represents the lifecycle state of a Job in the durable job table (SPEC §4.3).
type State string

const (
	StateQueued    State = "queued"    // Waiting in the queue to be acquired by a worker.
	StateRunning   State = "running"   // Currently leased and executing under an active lease timeout.
	StateDone      State = "done"      // Completed successfully (terminal).
	StateFailed    State = "failed"    // Failed transiently and scheduled for retry after NextAttemptAt.
	StateDead      State = "dead"      // Permanently failed after exhausting attempts or hitting a fatal error (terminal).
	StateSubmitted State = "submitted" // Report submitted to an IDE conversation awaiting idle completion (SPEC §4.3).
)

// Valid returns true if the State is recognized.
func (s State) Valid() bool {
	switch s {
	case StateQueued, StateRunning, StateDone, StateFailed, StateDead, StateSubmitted:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the State is a final state that will not be retried.
func (s State) IsTerminal() bool {
	return s == StateDone || s == StateDead
}

// Cost represents token, duration, and model metrics for execution runs (SPEC §4.3, §4.4).
type Cost struct {
	InputTokens     int    `json:"input_tokens,omitempty"`      // Prompt/input tokens consumed.
	OutputTokens    int    `json:"output_tokens,omitempty"`     // Completion/output tokens generated.
	ThinkingTokens  int    `json:"thinking_tokens,omitempty"`   // Reasoning/thinking tokens consumed.
	CacheReadTokens int    `json:"cache_read_tokens,omitempty"` // Prompt tokens served from context cache.
	DurationMS      int64  `json:"duration_ms,omitempty"`       // Wall-clock execution duration in milliseconds.
	Model           string `json:"model,omitempty"`             // Model identifier used for the run.
}

// FromUsage converts a harness.Usage into a Cost.
func FromUsage(u harness.Usage) Cost {
	return Cost{
		InputTokens:     u.InputTokens,
		OutputTokens:    u.OutputTokens,
		ThinkingTokens:  u.ThinkingTokens,
		CacheReadTokens: u.CacheReadTokens,
	}
}

// ToUsage converts a Cost into a harness.Usage.
func (c Cost) ToUsage() harness.Usage {
	return harness.Usage{
		InputTokens:     c.InputTokens,
		OutputTokens:    c.OutputTokens,
		ThinkingTokens:  c.ThinkingTokens,
		CacheReadTokens: c.CacheReadTokens,
	}
}

// Value implements driver.Valuer for SQLite storage as JSON.
func (c *Cost) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshaling cost: %w", err)
	}
	return string(bytes), nil
}

// Scan implements sql.Scanner for deserializing JSON from SQLite.
func (c *Cost) Scan(src any) error {
	// --- Step 1: Handle nil SQL values and extract raw JSON bytes ---
	if src == nil {
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into Cost", src)
	}

	// --- Step 2: Unmarshal non-empty JSON payload into the Cost receiver ---
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, c)
}

// Job represents a durable unit of asynchronous work in the job table (SPEC §4.3).
type Job struct {
	ID             string     `json:"id"`                        // Unique ULID job identifier.
	Kind           Kind       `json:"kind"`                      // Category of background work to execute.
	Priority       int        `json:"priority"`                  // Scheduling priority (higher runs first).
	IdempotencyKey string     `json:"idempotency_key,omitempty"` // Optional deduplication key preventing duplicate enqueues.
	State          State      `json:"state"`                     // Lifecycle state (queued, running, done, failed, or dead).
	LeaseExpires   *time.Time `json:"lease_expires,omitempty"`   // UTC expiration timestamp for an active worker lease.
	Attempts       int        `json:"attempts"`                  // Number of execution attempts made so far.
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"` // Earliest UTC time when a failed job may be retried.
	Cost           *Cost      `json:"cost,omitempty"`            // Token and latency cost recorded upon completion.
	Payload        string     `json:"payload,omitempty"`         // JSON-encoded job arguments.
	LastError      string     `json:"last_error,omitempty"`      // Diagnostic message from the most recent failure.
	CreatedAt      time.Time  `json:"created_at"`                // UTC timestamp when the job was first enqueued.
	UpdatedAt      time.Time  `json:"updated_at"`                // UTC timestamp of the most recent state update.
}

// Filter defines criteria for listing jobs.
type Filter struct {
	State State // Optional state filter (empty matches all states).
	Kind  Kind  // Optional job kind filter (empty matches all kinds).
	Limit int   // Maximum number of jobs to return (<= 0 means no limit).
}

// Common sentinel errors returned by job store and state operations.
var (
	ErrJobNotFound       = errors.New("job not found")                         // Returned when no job matches the given ID or key.
	ErrDuplicateJob      = errors.New("job with idempotency key already exists") // Returned when an idempotency key conflicts.
	ErrInvalidTransition = errors.New("invalid job state transition")          // Returned when a state transition violates the state machine.
	ErrInvalidJob        = errors.New("invalid job specification")             // Returned when a job has an empty ID, invalid kind, or invalid state.
	ErrNoJobAvailable    = errors.New("no eligible job available for leasing") // Returned by AcquireNext when no job is ready to run.
	ErrLeaseExpired      = errors.New("job lease has expired")                 // Returned when renewing a lease that is no longer active.
)
