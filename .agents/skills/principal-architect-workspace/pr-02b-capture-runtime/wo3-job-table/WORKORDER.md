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
- [ ] Create `internal/core/jobs/` package.
- [ ] Implement SQLite job table setup with WAL mode and `busy_timeout`.
- [ ] Define the schema: `id`, `kind`, `priority`, `idempotency_key`, `state`, `lease_expires`, `attempts`, `next_attempt_at`, `cost`.
- [ ] Implement state machine transitions (`queued` -> `running` -> `done`/`failed` -> `dead`).
- [ ] Implement lease recovery logic.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
### Changes Made
### Decisions Made
### Dead Ends & Warnings
### Out-of-Scope Findings
### Blockers
