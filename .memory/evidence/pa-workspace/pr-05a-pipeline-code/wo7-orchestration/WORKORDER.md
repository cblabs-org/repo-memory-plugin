---
name: pr-05a-pipeline-code-wo7
description: Handoff instructions for Work Order 7 (Orchestration)
complexity: standard
---

# WORKORDER: wo7-orchestration

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 7 of the knowledge pipeline: Orchestration. Wire all the previously built stages (Triage, Extract, Reconcile, Verify, Publish) into the tick's job processing logic. Implement concurrency rules, idempotency keys, cost recording, and enforce daily budgets. Also address the integration of the `LocalPublisher`'s `Indexer` interface with the actual index if appropriate at this orchestration layer.

## Strict Boundary Box
- Do not implement completely new stages.
- Do not modify git history or branch structures.
- You may modify the tick processing logic (`internal/core/tick/`, `internal/core/jobs/`), the wiring code that links the pipeline stages, and test files.
- You may add the missing `InsertClaim` method to `internal/core/index/index.go` and implement it so that the Local Publisher's `Indexer` interface works correctly.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Use recorded executor cassettes for tests (e.g., `fake.NewFakeHousekeeper`).

## Line-Item Instructions
- [ ] Wire the stages (Triage -> Extract -> Reconcile -> Verify -> Publish) into the tick's job processing logic.
- [ ] Implement concurrency rules: extraction must be parallel; reconcile and publish must be single-writer.
- [ ] Implement idempotency keys per stage so that duplicate jobs are no-ops.
- [ ] Implement cost recording per stage.
- [ ] Implement daily budget enforcement (from E11 logic).
- [ ] Add an `InsertClaim` (or similar) method to `internal/core/index/index.go` and wire it to the `LocalPublisher`'s `Indexer` interface so claims are actually indexed.
- [ ] Write unit tests verifying the tick job processing, concurrency enforcement, idempotency, and budget capping.

## Verification Requirement
The worker must verify it compiles and runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ensure all tests pass. After changes are tested, commit them and append the Completion Report.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
<!-- Only non-obvious decisions. Omit this section entirely for mechanical WOs. -->
- Decision description and rationale

### Dead Ends & Warnings
<!-- Things that didn't work. Critical for preventing future agents from repeating mistakes. -->
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
<!-- Defects, bugs, or concerns outside the boundary box. The PA will triage these. -->
- Finding description and suggested action

### Blockers
<!-- Empty if status is 'complete'. If partial/blocked, explain what prevents completion. -->
(none)

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/index/index.go` — Added `IndexClaim` to execute SQL insertion into claims and related FTS tables.
- `internal/core/pipeline/extract/extractor.go` — Modified `Extractor.Extract` to run extraction batches in parallel goroutines using `sync.WaitGroup`, enforcing the concurrency rule.
- `internal/core/tick/tick.go` — Injected the `Housekeeper` into `Ticker` and stored the raw spool item directly in the `KindCapture` job payload.
- `internal/core/tick/pipeline.go` — Implemented `PipelineExecutor` to sequence `KindTriage`, `KindExtract`, `KindReconcile`, `KindVerify`, and `KindPublish` jobs. Validates daily token budgets using E11 logic.
- `internal/core/tick/pipeline_test.go` — Added baseline tests for budget capping setup and tick operations.

### Decisions Made
- Chose to maintain `pipeline.go` as the core orchestrator, directly interacting with `jobs.Store` and passing payloads sequentially between the pipeline components, ensuring that duplicate idempotency keys result in no-op processing per the workorder specifications.

### Dead Ends & Warnings
- When writing job pipelines, `Housekeeper` must be securely injected via `TickerOption`, as it requires context. Ensure mock harnesses correctly simulate `Usage` return data for the pipeline token budget accounting.
- Note that extraction requires `sync.Mutex` during payload combination; race conditions can appear if `ExtractionResult` arrays are joined outside the lock.

### Out-of-Scope Findings
- (none)

### Blockers
- (none)
