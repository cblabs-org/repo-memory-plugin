/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/jobs/store.go
 * Description: Implements the SQLite-backed Store for enqueueing, leasing,
 *               renewing, completing, failing, and reclaiming background jobs.
 *
 * Exported Types / Functions:
 * - DefaultMaxAttempts:         Default maximum execution attempts before marking a job dead (5).
 * - DefaultLeaseDuration:       Default worker lease duration for acquired jobs (5m).
 * - DefaultBusyTimeout:         Default SQLite busy_timeout duration (5s).
 * - DefaultInteractivePriority: Default priority assigned to interactive jobs.
 * - Stats:                      Aggregated job counts grouped by lifecycle state.
 * - ReclaimResult:              Breakdown of requeued and deadened job counts from lease recovery.
 * - Option:                     Functional option for configuring a Store.
 * - WithMaxAttempts():          Sets the maximum execution attempts before a job transitions to dead.
 * - WithDefaultLeaseDuration(): Sets the default lease duration for acquired jobs.
 * - WithBusyTimeout():          Sets the SQLite busy_timeout duration.
 * - Store:                      Manages durable job lifecycle in SQLite.
 *
 *   Lifecycle:
 *   - Open():                   Opens or creates a SQLite job database at path in WAL mode.
 *   - NewStore():               Initializes a Store around an existing *sql.DB connection.
 *   - Close():                  Closes the underlying database connection. [io.Closer]
 *
 *   Own Public API:
 *   - DB():                           Returns the underlying *sql.DB handle.
 *   - Enqueue():                      Inserts a new job or returns the existing job on idempotency match.
 *   - Get():                          Fetches a job by its unique ID.
 *   - GetByIdempotencyKey():          Fetches a job by its unique idempotency key.
 *   - AcquireNext():                  Selects and leases the highest-priority eligible job.
 *   - RenewLease():                   Extends the lease expiration of a currently running job.
 *   - Complete():                     Marks a running or submitted job as done and records its cost.
 *   - Fail():                         Marks a job as failed with backoff or transitions it to dead.
 *   - MarkDead():                     Permanently marks a job as dead and clears its lease.
 *   - ReclaimExpiredLeases():         Recovers expired running leases and returns the total reclaimed count.
 *   - ReclaimExpiredLeasesDetailed(): Recovers expired running leases and returns detailed counts.
 *   - List():                         Returns jobs matching a Filter ordered by priority and creation time.
 *   - Delete():                       Removes a job by ID.
 *   - Stats():                        Returns counts of jobs grouped by state.
 *   - RequeueKind():                  Resets all jobs of a given Kind back to the queued state.
 */

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

// Default constants for job store configuration.
const (
	DefaultMaxAttempts         = 5                   // Default maximum execution attempts before a job is marked dead.
	DefaultLeaseDuration       = 5 * time.Minute     // Default worker lease duration when acquiring a job.
	DefaultBusyTimeout         = 5 * time.Second     // Default SQLite busy_timeout wait duration.
	DefaultInteractivePriority = PriorityInteractive // Default scheduling priority for interactive jobs.
)

// Stats summarizes job counts grouped by state.
type Stats struct {
	Queued    int `json:"queued"`    // Number of jobs in queued state.
	Running   int `json:"running"`   // Number of jobs in running state.
	Done      int `json:"done"`      // Number of jobs in done state.
	Failed    int `json:"failed"`    // Number of jobs in failed state awaiting retry.
	Dead      int `json:"dead"`      // Number of jobs in dead state.
	Submitted int `json:"submitted"` // Number of jobs in submitted state.
	Total     int `json:"total"`     // Total number of jobs across all states.
}

// ReclaimResult contains detailed counts of reclaimed jobs.
type ReclaimResult struct {
	Requeued int // Count of expired running jobs returned to queued state.
	Deadened int // Count of expired running jobs transitioned to dead after reaching max attempts.
	Total    int // Total number of expired running jobs processed (Requeued + Deadened).
}

// Option configures Store behavior.
type Option func(*Store)

// WithMaxAttempts sets the maximum execution attempts before a job transitions to dead.
func WithMaxAttempts(max int) Option {
	return func(s *Store) {
		if max > 0 {
			s.maxAttempts = max
		}
	}
}

// WithDefaultLeaseDuration sets the default lease duration for acquired jobs.
func WithDefaultLeaseDuration(d time.Duration) Option {
	return func(s *Store) {
		if d > 0 {
			s.defaultLeaseDuration = d
		}
	}
}

// WithBusyTimeout sets the SQLite busy_timeout duration.
func WithBusyTimeout(d time.Duration) Option {
	return func(s *Store) {
		if d > 0 {
			s.busyTimeout = d
		}
	}
}

// Store manages durable job lifecycle in SQLite (SPEC §4.3).
type Store struct {
	db                   *sql.DB       // Underlying SQLite connection pool.
	maxAttempts          int           // Maximum attempts before a failing job becomes dead.
	defaultLeaseDuration time.Duration // Default lease duration applied by AcquireNext and RenewLease.
	busyTimeout          time.Duration // SQLite busy_timeout configured on connection open.
}

// Open opens or creates a SQLite job database at path, applying WAL mode, busy_timeout,
// and table initialization. The caller must call Close on the returned Store when finished.
func Open(path string, opts ...Option) (*Store, error) {
	// --- Step 1: Validate database path and ensure parent directory exists ---
	if path == "" {
		return nil, fmt.Errorf("database path cannot be empty")
	}

	// Create parent directory if needed (for file-based database)
	if path != ":memory:" && !strings.HasPrefix(path, "file::memory:") {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating database directory %s: %w", dir, err)
		}
	}

	// --- Step 2: Apply store options and construct the SQLite DSN ---
	store := &Store{
		maxAttempts:          DefaultMaxAttempts,
		defaultLeaseDuration: DefaultLeaseDuration,
		busyTimeout:          DefaultBusyTimeout,
	}
	for _, opt := range opts {
		opt(store)
	}

	busyTimeoutMs := int(store.busyTimeout.Milliseconds())
	if busyTimeoutMs <= 0 {
		busyTimeoutMs = 5000
	}

	dsn := path
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn = fmt.Sprintf("%s%s_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dsn, sep, busyTimeoutMs)

	// --- Step 3: Open SQLite connection, enforce PRAGMAs, and initialize schema ---
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	// Enforce WAL mode and busy_timeout explicitly on connection
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting journal_mode to WAL: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", busyTimeoutMs)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting busy_timeout: %w", err)
	}

	if err := InitSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	store.db = db
	return store, nil
}

// NewStore initializes a Store with an already opened database connection.
// The caller remains responsible for closing db (or calling Close on the returned Store).
func NewStore(db *sql.DB, opts ...Option) (*Store, error) {
	// --- Step 1: Validate the database handle and apply functional options ---
	if db == nil {
		return nil, fmt.Errorf("database connection cannot be nil")
	}

	store := &Store{
		db:                   db,
		maxAttempts:          DefaultMaxAttempts,
		defaultLeaseDuration: DefaultLeaseDuration,
		busyTimeout:          DefaultBusyTimeout,
	}
	for _, opt := range opts {
		opt(store)
	}

	// --- Step 2: Ensure the jobs table and indexes exist ---
	if err := InitSchema(db); err != nil {
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return store, nil
}

// DB returns the underlying *sql.DB instance.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Enqueue inserts a new job into the job table.
// If an idempotency key is provided and a job with that key already exists,
// Enqueue returns the existing job without error (SPEC §4.3: "so duplicates are no-ops").
// It returns ErrInvalidJob if job is nil or has an invalid Kind or State.
func (s *Store) Enqueue(ctx context.Context, job *Job) (*Job, error) {
	// --- Step 1: Validate job specification and check idempotency key ---
	if job == nil {
		return nil, fmt.Errorf("%w: job cannot be nil", ErrInvalidJob)
	}
	if !job.Kind.Valid() {
		return nil, fmt.Errorf("%w: invalid kind %q", ErrInvalidJob, job.Kind)
	}

	// Deduplication check via idempotency key
	if job.IdempotencyKey != "" {
		existing, err := s.GetByIdempotencyKey(ctx, job.IdempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, ErrJobNotFound) {
			return nil, fmt.Errorf("checking idempotency key: %w", err)
		}
	}

	// --- Step 2: Populate default ID, initial state, and timestamps ---
	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	if job.State == "" {
		job.State = StateQueued
	}
	if !job.State.Valid() {
		return nil, fmt.Errorf("%w: invalid state %q", ErrInvalidJob, job.State)
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	// --- Step 3: Insert the job row and handle concurrent idempotency races ---
	query := `
INSERT INTO jobs (
    id, kind, priority, idempotency_key, state, lease_expires,
    attempts, next_attempt_at, cost, payload, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`
	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		string(job.Kind),
		job.Priority,
		nullString(job.IdempotencyKey),
		string(job.State),
		nullTime(job.LeaseExpires),
		job.Attempts,
		nullTime(job.NextAttemptAt),
		costValue(job.Cost),
		nullString(job.Payload),
		nullString(job.LastError),
		job.CreatedAt,
		job.UpdatedAt,
	)
	if err != nil {
		// Handle potential race on unique idempotency_key constraint
		if job.IdempotencyKey != "" && strings.Contains(strings.ToLower(err.Error()), "unique") {
			existing, fetchErr := s.GetByIdempotencyKey(ctx, job.IdempotencyKey)
			if fetchErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("inserting job: %w", err)
	}

	return job, nil
}

// Get fetches a job by ID. It returns ErrInvalidJob if id is empty, or ErrJobNotFound if no row matches.
func (s *Store) Get(ctx context.Context, id string) (*Job, error) {
	// --- Step 1: Validate the job ID ---
	if id == "" {
		return nil, fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	// --- Step 2: Query and scan the job row ---
	query := `
SELECT id, kind, priority, idempotency_key, state, lease_expires,
       attempts, next_attempt_at, cost, payload, last_error, created_at, updated_at
FROM jobs
WHERE id = ?;
`
	row := s.db.QueryRowContext(ctx, query, id)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("getting job %s: %w", id, err)
	}

	return job, nil
}

// GetByIdempotencyKey fetches a job by its unique idempotency key.
// It returns ErrInvalidJob if key is empty, or ErrJobNotFound if no row matches.
func (s *Store) GetByIdempotencyKey(ctx context.Context, key string) (*Job, error) {
	// --- Step 1: Validate the idempotency key ---
	if key == "" {
		return nil, fmt.Errorf("%w: idempotency key cannot be empty", ErrInvalidJob)
	}

	// --- Step 2: Query and scan the job row by idempotency_key ---
	query := `
SELECT id, kind, priority, idempotency_key, state, lease_expires,
       attempts, next_attempt_at, cost, payload, last_error, created_at, updated_at
FROM jobs
WHERE idempotency_key = ?;
`
	row := s.db.QueryRowContext(ctx, query, key)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("getting job by idempotency key: %w", err)
	}

	return job, nil
}

// AcquireNext selects and leases the highest-priority eligible job for execution.
// A job is eligible if it is queued, or failed with next_attempt_at <= now.
// It returns ErrNoJobAvailable if no job is currently eligible.
func (s *Store) AcquireNext(ctx context.Context, leaseDuration time.Duration, kinds ...Kind) (*Job, error) {
	// --- Step 1: Resolve lease duration and open a transaction ---
	if leaseDuration <= 0 {
		leaseDuration = s.defaultLeaseDuration
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// --- Step 2: Query the highest-priority eligible job ---
	now := time.Now().UTC()

	var query strings.Builder
	query.WriteString(`
SELECT id, kind, priority, idempotency_key, state, lease_expires,
       attempts, next_attempt_at, cost, payload, last_error, created_at, updated_at
FROM jobs
WHERE (state = 'queued' OR (state = 'failed' AND (next_attempt_at IS NULL OR next_attempt_at <= ?)))
`)

	args := []any{now}

	if len(kinds) > 0 {
		placeholders := make([]string, len(kinds))
		for i, k := range kinds {
			placeholders[i] = "?"
			args = append(args, string(k))
		}
		query.WriteString(fmt.Sprintf(" AND kind IN (%s)", strings.Join(placeholders, ", ")))
	}

	query.WriteString(" ORDER BY priority DESC, created_at ASC LIMIT 1;")

	row := tx.QueryRowContext(ctx, query.String(), args...)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoJobAvailable // No jobs available
		}
		return nil, fmt.Errorf("querying eligible job: %w", err)
	}

	// --- Step 3: Transition the selected job to running with an incremented attempt count ---
	if err := ValidateTransition(job.State, StateRunning); err != nil {
		return nil, fmt.Errorf("acquiring job %s: %w", job.ID, err)
	}

	leaseExpires := now.Add(leaseDuration)
	attempts := job.Attempts + 1

	updateQuery := `
UPDATE jobs
SET state = ?, attempts = ?, lease_expires = ?, next_attempt_at = NULL, updated_at = ?
WHERE id = ? AND state = ?;
`
	res, err := tx.ExecContext(ctx, updateQuery,
		string(StateRunning),
		attempts,
		leaseExpires,
		now,
		job.ID,
		string(job.State),
	)
	if err != nil {
		return nil, fmt.Errorf("updating job to running: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		// Concurrently acquired
		return nil, ErrNoJobAvailable
	}

	// --- Step 4: Commit transaction and return the updated Job ---
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing acquire transaction: %w", err)
	}

	job.State = StateRunning
	job.Attempts = attempts
	job.LeaseExpires = &leaseExpires
	job.NextAttemptAt = nil
	job.UpdatedAt = now

	return job, nil
}

// RenewLease extends the lease duration of a currently running job.
// It returns ErrInvalidJob if id is empty, ErrJobNotFound if the job does not exist,
// ErrInvalidTransition if the job is no longer running, or ErrLeaseExpired if the update affected no rows.
func (s *Store) RenewLease(ctx context.Context, id string, leaseDuration time.Duration) error {
	// --- Step 1: Validate inputs and compute new expiration timestamp ---
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}
	if leaseDuration <= 0 {
		leaseDuration = s.defaultLeaseDuration
	}

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)

	// --- Step 2: Update lease_expires for the running job ---
	query := `
UPDATE jobs
SET lease_expires = ?, updated_at = ?
WHERE id = ? AND state = 'running';
`
	res, err := s.db.ExecContext(ctx, query, leaseExpires, now, id)
	if err != nil {
		return fmt.Errorf("renewing lease for %s: %w", id, err)
	}

	// --- Step 3: Diagnose zero-row updates by inspecting current job state ---
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		job, err := s.Get(ctx, id)
		if err != nil {
			return err
		}
		if job.State != StateRunning {
			return fmt.Errorf("%w: job is in state %q, not running", ErrInvalidTransition, job.State)
		}
		return ErrLeaseExpired
	}

	return nil
}

// Complete marks a running (or submitted) job as successfully done, recording any associated cost.
func (s *Store) Complete(ctx context.Context, id string, cost *Cost) error {
	// --- Step 1: Fetch the job and validate transition to StateDone ---
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(job.State, StateDone); err != nil {
		return fmt.Errorf("completing job %s: %w", id, err)
	}

	// --- Step 2: Persist StateDone, cost metrics, and cleared lease ---
	now := time.Now().UTC()
	query := `
UPDATE jobs
SET state = ?, cost = ?, lease_expires = NULL, updated_at = ?
WHERE id = ?;
`
	_, err = s.db.ExecContext(ctx, query, string(StateDone), costValue(cost), now, id)
	if err != nil {
		return fmt.Errorf("marking job %s done: %w", id, err)
	}

	return nil
}

// Fail marks a running job as failed, recording the error and scheduling backoff retry.
// If the job has reached or exceeded maxAttempts, it transitions directly to dead.
func (s *Store) Fail(ctx context.Context, id string, errMessage string, backoff time.Duration) error {
	// --- Step 1: Load the job and check whether maxAttempts has been reached ---
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// --- Step 2: Transition to StateDead if attempts are exhausted ---
	if job.Attempts >= s.maxAttempts {
		if err := ValidateTransition(job.State, StateDead); err != nil {
			return fmt.Errorf("transitioning job %s to dead: %w", id, err)
		}
		deadQuery := `
UPDATE jobs
SET state = ?, last_error = ?, lease_expires = NULL, next_attempt_at = NULL, updated_at = ?
WHERE id = ?;
`
		_, err := s.db.ExecContext(ctx, deadQuery, string(StateDead), errMessage, now, id)
		if err != nil {
			return fmt.Errorf("marking job %s dead: %w", id, err)
		}
		return nil
	}

	// --- Step 3: Transition to StateFailed and schedule next retry with backoff ---
	if err := ValidateTransition(job.State, StateFailed); err != nil {
		return fmt.Errorf("transitioning job %s to failed: %w", id, err)
	}

	if backoff <= 0 {
		backoff = DefaultBackoff(job.Attempts)
	}
	nextAttempt := now.Add(backoff)

	failQuery := `
UPDATE jobs
SET state = ?, last_error = ?, lease_expires = NULL, next_attempt_at = ?, updated_at = ?
WHERE id = ?;
`
	_, err = s.db.ExecContext(ctx, failQuery, string(StateFailed), errMessage, nextAttempt, now, id)
	if err != nil {
		return fmt.Errorf("marking job %s failed: %w", id, err)
	}

	return nil
}

// MarkDead permanently marks a job as dead, clearing any active leases.
func (s *Store) MarkDead(ctx context.Context, id string, errMessage string) error {
	// --- Step 1: Load the job and verify transition to StateDead ---
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateTransition(job.State, StateDead); err != nil {
		return fmt.Errorf("marking job %s dead: %w", id, err)
	}

	// --- Step 2: Update the job row to StateDead and clear active lease ---
	now := time.Now().UTC()
	query := `
UPDATE jobs
SET state = ?, last_error = ?, lease_expires = NULL, next_attempt_at = NULL, updated_at = ?
WHERE id = ?;
`
	_, err = s.db.ExecContext(ctx, query, string(StateDead), errMessage, now, id)
	if err != nil {
		return fmt.Errorf("marking job %s dead: %w", id, err)
	}

	return nil
}

// ReclaimExpiredLeases scans for running jobs whose lease_expires <= now.
// For jobs that reached maxAttempts, they are marked dead.
// For jobs with remaining attempts, they are reset to queued state.
// Returns the count of reclaimed jobs.
func (s *Store) ReclaimExpiredLeases(ctx context.Context, now time.Time) (int, error) {
	result, err := s.ReclaimExpiredLeasesDetailed(ctx, now)
	if err != nil {
		return 0, err
	}
	return result.Total, nil
}

// ReclaimExpiredLeasesDetailed executes lease recovery and returns breakdown counts.
func (s *Store) ReclaimExpiredLeasesDetailed(ctx context.Context, now time.Time) (ReclaimResult, error) {
	// --- Step 1: Normalize reference timestamp and begin transaction ---
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReclaimResult{}, fmt.Errorf("beginning reclaim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// --- Step 2: Mark expired running jobs that exceeded maxAttempts as dead ---
	deadQuery := `
UPDATE jobs
SET state = ?, lease_expires = NULL, next_attempt_at = NULL,
    last_error = 'lease expired: maximum attempts exceeded', updated_at = ?
WHERE state = 'running'
  AND lease_expires IS NOT NULL
  AND lease_expires <= ?
  AND attempts >= ?;
`
	resDead, err := tx.ExecContext(ctx, deadQuery, string(StateDead), now, now, s.maxAttempts)
	if err != nil {
		return ReclaimResult{}, fmt.Errorf("deadening expired jobs: %w", err)
	}
	deadCount, err := resDead.RowsAffected()
	if err != nil {
		return ReclaimResult{}, fmt.Errorf("checking dead rows affected: %w", err)
	}

	// --- Step 3: Re-queue expired running jobs that still have remaining attempts ---
	requeueQuery := `
UPDATE jobs
SET state = ?, lease_expires = NULL, next_attempt_at = NULL, updated_at = ?
WHERE state = 'running'
  AND lease_expires IS NOT NULL
  AND lease_expires <= ?
  AND attempts < ?;
`
	resRequeue, err := tx.ExecContext(ctx, requeueQuery, string(StateQueued), now, now, s.maxAttempts)
	if err != nil {
		return ReclaimResult{}, fmt.Errorf("requeuing expired jobs: %w", err)
	}
	requeueCount, err := resRequeue.RowsAffected()
	if err != nil {
		return ReclaimResult{}, fmt.Errorf("checking requeued rows affected: %w", err)
	}

	// --- Step 4: Commit transaction and return breakdown counts ---
	if err := tx.Commit(); err != nil {
		return ReclaimResult{}, fmt.Errorf("committing reclaim transaction: %w", err)
	}

	return ReclaimResult{
		Requeued: int(requeueCount),
		Deadened: int(deadCount),
		Total:    int(requeueCount + deadCount),
	}, nil
}

// List returns a list of jobs matching the provided filter.
func (s *Store) List(ctx context.Context, filter Filter) ([]Job, error) {
	// --- Step 1: Build SQL query from filter predicates ---
	var query strings.Builder
	query.WriteString(`
SELECT id, kind, priority, idempotency_key, state, lease_expires,
       attempts, next_attempt_at, cost, payload, last_error, created_at, updated_at
FROM jobs
WHERE 1=1
`)
	var args []any

	if filter.State != "" {
		query.WriteString(" AND state = ?")
		args = append(args, string(filter.State))
	}
	if filter.Kind != "" {
		query.WriteString(" AND kind = ?")
		args = append(args, string(filter.Kind))
	}

	query.WriteString(" ORDER BY priority DESC, created_at ASC")

	if filter.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}

	// --- Step 2: Execute query and scan resulting Job rows ---
	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning listed job: %w", err)
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating listed jobs: %w", err)
	}

	return jobs, nil
}

// Delete removes a job by ID. It returns ErrInvalidJob if id is empty, or ErrJobNotFound if no row was deleted.
func (s *Store) Delete(ctx context.Context, id string) error {
	// --- Step 1: Validate ID and execute DELETE statement ---
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	query := "DELETE FROM jobs WHERE id = ?;"
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting job %s: %w", id, err)
	}

	// --- Step 2: Verify that a row was removed ---
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows affected: %w", err)
	}
	if rows == 0 {
		return ErrJobNotFound
	}

	return nil
}

// Stats returns counts of jobs grouped by state.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	// --- Step 1: Query job counts grouped by state ---
	query := "SELECT state, COUNT(*) FROM jobs GROUP BY state;"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return Stats{}, fmt.Errorf("querying stats: %w", err)
	}
	defer rows.Close()

	// --- Step 2: Accumulate per-state and total job counts ---
	var stats Stats
	for rows.Next() {
		var stateStr string
		var count int
		if err := rows.Scan(&stateStr, &count); err != nil {
			return Stats{}, fmt.Errorf("scanning stats row: %w", err)
		}
		stats.Total += count
		switch State(stateStr) {
		case StateQueued:
			stats.Queued = count
		case StateRunning:
			stats.Running = count
		case StateDone:
			stats.Done = count
		case StateFailed:
			stats.Failed = count
		case StateDead:
			stats.Dead = count
		case StateSubmitted:
			stats.Submitted = count
		}
	}

	return stats, rows.Err()
}

// scanJob scans a single row from a sql.Row or sql.Rows into a Job struct.
func scanJob(scanner interface{ Scan(dest ...any) error }) (*Job, error) {
	// --- Step 1: Scan raw SQL columns into local variables ---
	var j Job
	var kindStr, stateStr string
	var idempotencyKey, costStr, payload, lastError sql.NullString
	var leaseExpires, nextAttemptAt sql.NullTime

	err := scanner.Scan(
		&j.ID,
		&kindStr,
		&j.Priority,
		&idempotencyKey,
		&stateStr,
		&leaseExpires,
		&j.Attempts,
		&nextAttemptAt,
		&costStr,
		&payload,
		&lastError,
		&j.CreatedAt,
		&j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// --- Step 2: Populate nullable fields, UTC timestamps, and JSON Cost ---
	j.Kind = Kind(kindStr)
	j.State = State(stateStr)

	if idempotencyKey.Valid {
		j.IdempotencyKey = idempotencyKey.String
	}
	if payload.Valid {
		j.Payload = payload.String
	}
	if lastError.Valid {
		j.LastError = lastError.String
	}
	if leaseExpires.Valid {
		t := leaseExpires.Time.UTC()
		j.LeaseExpires = &t
	}
	if nextAttemptAt.Valid {
		t := nextAttemptAt.Time.UTC()
		j.NextAttemptAt = &t
	}
	if costStr.Valid && costStr.String != "" {
		var cost Cost
		if err := cost.Scan(costStr.String); err == nil {
			j.Cost = &cost
		}
	}

	return &j, nil
}

// nullString returns nil if s is empty, or s otherwise for SQL NULL storage.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullTime returns nil if t is nil or zero, or t.UTC() otherwise for SQL NULL storage.
func nullTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}

// costValue serializes c via Value() or returns nil when c is nil or fails to marshal.
func costValue(c *Cost) any {
	if c == nil {
		return nil
	}
	val, err := c.Value()
	if err != nil {
		return nil
	}
	return val
}

// RequeueKind resets the state of all jobs of the specified kind to queued,
// so they can be processed again. Returns the number of jobs modified.
func (s *Store) RequeueKind(ctx context.Context, kind Kind) (int, error) {
	now := time.Now().UTC()
	query := "UPDATE jobs SET state = ?, attempts = 0, last_error = NULL, lease_expires = NULL, next_attempt_at = NULL, updated_at = ? WHERE kind = ?"
	result, err := s.db.ExecContext(ctx, query, string(StateQueued), now, string(kind))
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}
