---
name: pr-02b-capture-runtime-wo3
description: Handoff instructions for Work Order 3
complexity: standard
---

# pr-02b-capture-runtime / WO3 - job-table

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the SQLite job table for tracking asynchronous work.

## Strict Boundary Box
- Do not implement the tick loop itself.
- Keep the database schema aligned with the specification.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [x] Create `internal/core/jobs/` package.
- [x] Implement SQLite job table setup with WAL mode and `busy_timeout`.
- [x] Define the schema: `id`, `kind`, `priority`, `idempotency_key`, `state`, `lease_expires`, `attempts`, `next_attempt_at`, `cost`.
- [x] Implement state machine transitions (`queued` -> `running` -> `done`/`failed` -> `dead`).
- [x] Implement lease recovery logic.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `go.mod`, `go.sum` — Added pure-Go SQLite driver `modernc.org/sqlite` (cgo-free, compliant with SPEC §18.4).
- `internal/core/jobs/types.go` — Defined domain models and types: `Kind` (covering pipeline stages and asynchronous work), `Priority` levels (interactive, batch, normal, low), `State` (`queued`, `running`, `done`, `failed`, `dead`, `submitted`), `Cost` struct (tokens, duration, model) implementing `sql.Scanner` and `driver.Valuer`, `Job` struct aligning with the schema specification, and domain sentinel errors.
- `internal/core/jobs/schema.go` — Defined SQLite table schema DDL with all 9 specified fields (`id`, `kind`, `priority`, `idempotency_key`, `state`, `lease_expires`, `attempts`, `next_attempt_at`, `cost`) plus payload, error, and timestamps. Added unique index on `idempotency_key`, scheduler index on priority and state, and partial index on active leases.
- `internal/core/jobs/state.go` — Implemented state machine transition validator (`ValidateTransition`, `CanTransition`) enforcing permitted lifecycle paths, and exponential backoff calculator (`DefaultBackoff`) with attempt scaling and 15-minute cap.
- `internal/core/jobs/store.go` — Implemented SQLite `Store`:
  - Initialization enforcing WAL mode (`PRAGMA journal_mode = WAL;`) and busy timeout (`PRAGMA busy_timeout = 5000;`).
  - `Enqueue`: Enqueues jobs with ULID generation; enforces duplicate deduplication via idempotency keys (duplicates are no-ops).
  - `AcquireNext`: Atomically leases the highest-priority eligible job for execution.
  - `RenewLease`: Extends lease duration for running jobs.
  - `Complete`: Marks jobs done, records `Cost`, and clears leases.
  - `Fail`: Records errors, clears lease, transitions to `failed` with exponential `next_attempt_at` backoff, or marks `dead` when `maxAttempts` is reached.
  - `MarkDead`: Manually transitions non-retryable jobs to `dead`.
  - `ReclaimExpiredLeases`: Atomically reclaims expired leases, re-queuing jobs with attempts remaining and deadening jobs exceeding max attempts.
  - `List`, `Delete`, and `Stats`: Querying and monitoring helpers.
- `internal/core/jobs/state_test.go` — Unit tests for state transitions, terminal states, and backoff calculations.
- `internal/core/jobs/store_test.go` — Integration tests for SQLite WAL pragmas, schema structure, idempotency under concurrency, full lifecycle transitions, priority scheduling, and lease reclamation.

### Decisions Made
- Pure-Go SQLite: Selected `modernc.org/sqlite` per SPEC §18.4 to maintain a cgo-free single binary.
- Idempotency deduplication: Used unique partial index `WHERE idempotency_key IS NOT NULL AND idempotency_key != ''` so that non-keyed jobs can be enqueued without conflict, while keyed jobs are strictly deduplicated and return existing jobs on duplicate insertion.
- Two-tier lease reclamation: `ReclaimExpiredLeases` atomically handles both recoverable jobs (reset to `queued` with cleared lease) and crash-looping jobs (transitioned to `dead` once `maxAttempts` is reached).

### Dead Ends & Warnings
- When using SQLite in-memory databases (`:memory:`), `PRAGMA journal_mode = WAL;` returns `"memory"`. The store gracefully handles this while ensuring file-backed databases always run in `wal` mode.

### Out-of-Scope Findings
- (none)

### Blockers
(none)
