package jobs

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Standard Job Kinds defined in SPEC §4.3.
type Kind string

const (
	KindCompact        Kind = "compact"
	KindTriage         Kind = "triage"
	KindExtract        Kind = "extract"
	KindReconcile      Kind = "reconcile"
	KindVerify         Kind = "verify"
	KindPublish        Kind = "publish"
	KindBrief          Kind = "brief"
	KindDejavu         Kind = "dejavu"
	KindRecheck        Kind = "recheck"
	KindReport         Kind = "report"
	KindCapture        Kind = "capture"
	KindSweep          Kind = "sweep"
	KindSeedDocs       Kind = "seed-docs"
	KindDreamFreshness Kind = "dream-freshness"
)

// Valid returns true if the Kind is recognized.
func (k Kind) Valid() bool {
	switch k {
	case KindCompact, KindTriage, KindExtract, KindReconcile, KindVerify,
		KindPublish, KindBrief, KindDejavu, KindRecheck, KindReport,
		KindCapture, KindSweep, KindSeedDocs, KindDreamFreshness:
		return true
	default:
		return false
	}
}

// Standard Job Priorities. Higher integer means higher scheduling priority.
const (
	PriorityLow         = 0
	PriorityNormal      = 10
	PriorityBatch       = 20
	PriorityInteractive = 100 // briefs, déjà-vu (SPEC §4.3 step 6)
)

// Standard Job States defined in SPEC §4.3.
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateDone      State = "done"
	StateFailed    State = "failed"
	StateDead      State = "dead"
	StateSubmitted State = "submitted" // Reports submitted to IDE conversation (SPEC §4.3)
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
	InputTokens     int    `json:"input_tokens,omitempty"`
	OutputTokens    int    `json:"output_tokens,omitempty"`
	ThinkingTokens  int    `json:"thinking_tokens,omitempty"`
	CacheReadTokens int    `json:"cache_read_tokens,omitempty"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
	Model           string `json:"model,omitempty"`
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

	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, c)
}

// Job represents a durable unit of asynchronous work in the job table (SPEC §4.3).
type Job struct {
	ID             string     `json:"id"`
	Kind           Kind       `json:"kind"`
	Priority       int        `json:"priority"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
	State          State      `json:"state"`
	LeaseExpires   *time.Time `json:"lease_expires,omitempty"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"`
	Cost           *Cost      `json:"cost,omitempty"`
	Payload        string     `json:"payload,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Filter defines criteria for listing jobs.
type Filter struct {
	State State
	Kind  Kind
	Limit int
}

// Common sentinel errors.
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrDuplicateJob      = errors.New("job with idempotency key already exists")
	ErrInvalidTransition = errors.New("invalid job state transition")
	ErrInvalidJob        = errors.New("invalid job specification")
	ErrNoJobAvailable    = errors.New("no eligible job available for leasing")
	ErrLeaseExpired      = errors.New("job lease has expired")
)
