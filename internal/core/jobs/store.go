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
	DefaultMaxAttempts          = 5
	DefaultLeaseDuration        = 5 * time.Minute
	DefaultBusyTimeout          = 5 * time.Second
	DefaultInteractivePriority  = PriorityInteractive
)

// Stats summarizes job counts grouped by state.
type Stats struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Done      int `json:"done"`
	Failed    int `json:"failed"`
	Dead      int `json:"dead"`
	Submitted int `json:"submitted"`
	Total     int `json:"total"`
}

// ReclaimResult contains detailed counts of reclaimed jobs.
type ReclaimResult struct {
	Requeued int
	Deadened int
	Total    int
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
	db                   *sql.DB
	maxAttempts          int
	defaultLeaseDuration time.Duration
	busyTimeout          time.Duration
}

// Open opens or creates a SQLite job database at path, applying WAL mode, busy_timeout,
// and table initialization.
func Open(path string, opts ...Option) (*Store, error) {
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
func NewStore(db *sql.DB, opts ...Option) (*Store, error) {
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
func (s *Store) Enqueue(ctx context.Context, job *Job) (*Job, error) {
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

// Get fetches a job by ID.
func (s *Store) Get(ctx context.Context, id string) (*Job, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

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
func (s *Store) GetByIdempotencyKey(ctx context.Context, key string) (*Job, error) {
	if key == "" {
		return nil, fmt.Errorf("%w: idempotency key cannot be empty", ErrInvalidJob)
	}

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
// Returns (nil, nil) if no job is currently eligible.
func (s *Store) AcquireNext(ctx context.Context, leaseDuration time.Duration, kinds ...Kind) (*Job, error) {
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
func (s *Store) RenewLease(ctx context.Context, id string, leaseDuration time.Duration) error {
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}
	if leaseDuration <= 0 {
		leaseDuration = s.defaultLeaseDuration
	}

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)

	query := `
UPDATE jobs
SET lease_expires = ?, updated_at = ?
WHERE id = ? AND state = 'running';
`
	res, err := s.db.ExecContext(ctx, query, leaseExpires, now, id)
	if err != nil {
		return fmt.Errorf("renewing lease for %s: %w", id, err)
	}

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
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// If max attempts reached, transition to dead
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

	// Transient failure: transition to failed with backoff
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

	// 1. Mark jobs that exceeded max attempts as dead
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

	// 2. Re-queue jobs with remaining attempts
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

// Delete removes a job by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidJob)
	}

	query := "DELETE FROM jobs WHERE id = ?;"
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting job %s: %w", id, err)
	}

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
	query := "SELECT state, COUNT(*) FROM jobs GROUP BY state;"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return Stats{}, fmt.Errorf("querying stats: %w", err)
	}
	defer rows.Close()

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

func scanJob(scanner interface{ Scan(dest ...any) error }) (*Job, error) {
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

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}

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
	result, err := s.db.ExecContext(ctx, "UPDATE jobs SET state = ?, attempts = 0, error = NULL, result = NULL WHERE kind = ?", StateQueued, kind)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}
