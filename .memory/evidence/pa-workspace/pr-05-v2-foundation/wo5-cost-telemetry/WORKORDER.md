---
name: pr-05-v2-foundation-wo5
description: Handoff instructions for Work Order 5 — cost telemetry, gen_metadata reader, quota guard
complexity: standard
---

# WO5: Cost Telemetry, gen_metadata Reader & Quota Guard

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Executor results are stored with `Cost{}` (all zeros). The executor *already* parses `usage` from `agy` JSON output (InputTokens, OutputTokens, ThinkingTokens, CacheReadTokens) into `harness.Usage`, but nothing copies those values into the `jobs.Cost` that gets persisted. Fix this, and add two new readers needed by the spike experiments (X1, X3).

## Strict Boundary Box
- **Modify:** `internal/core/jobs/types.go` (extend `Cost` struct)
- **Modify:** `internal/core/tick/pipeline.go` (ensure cost is persisted from executor results)
- **Create:** `internal/core/telemetry/` package (gen_metadata reader, quota guard)
- **Create:** `cmd/repo-memory/cmd_usage.go` (optional CLI for quota check)
- **DO NOT** modify the evidence package, hooks, spool, episode store, entities, claims, index, delivery, or MCP server
- **DO NOT** modify the executor itself — it already parses usage correctly

## Tooling Constraints
- Use `go build ./...` and `go vet ./...`
- Run `go test ./internal/core/jobs/... ./internal/core/telemetry/...`

## Context

### The broken cost path
1. The executor (`internal/harness/antigravity/executor.go`) parses `usage` from `agy` JSON output into `harness.Usage` (line 127-131):
   - `InputTokens`, `OutputTokens`, `ThinkingTokens`, `CacheReadTokens`
2. The executor returns a `harness.ExecResult` with `Usage harness.Usage`
3. But wherever pipeline jobs are completed (in `pipeline.go`), the `Cost` written to the job is either `nil` or `Cost{}` — nobody copies `result.Usage` into `job.Cost`
4. The `Cost` struct itself is also missing `ThinkingTokens` and `CacheReadTokens` fields

### What needs fixing
1. **Extend `Cost`**: Add `ThinkingTokens` and `CacheReadTokens` fields
2. **Wire the cost**: After executor runs complete, copy `result.Usage` fields into the job's `Cost` before calling `Complete()`
3. **gen_metadata reader**: A read-only function that reads the `gen_metadata` file from the Antigravity data directory. This file contains per-turn token usage for IDE sessions (uncached input, cached input, output). Mark as experimental. Located at `~/.gemini/antigravity/gen_metadata.json` or similar.
4. **Quota guard**: A function that calls `agy -p "/usage" --output-format json` (zero tokens, just an API query) and returns the current quota state (used/remaining in the 5-hour and daily pools). This becomes a shared library for the spike experiments.

### harness.Usage type
Check `internal/harness/types.go` for the `Usage` struct — it already has all four token fields. The `Cost` struct in `jobs/types.go` just needs to match it.

## Line-Item Instructions

- [x] **Extend `jobs.Cost`** in `internal/core/jobs/types.go`:
  - Add `ThinkingTokens int` and `CacheReadTokens int` JSON fields
  - Add a `FromUsage(u harness.Usage) Cost` convenience function

- [x] **Wire cost into pipeline**: In `internal/core/tick/pipeline.go`, wherever executor results are processed and jobs are completed, copy the usage from the result into the job's cost. Look for calls to `p.store.Complete(ctx, job.ID, ...)` — the second argument is `*Cost`. Ensure it's populated from the executor result's `Usage`.

- [x] **Create `internal/core/telemetry/` package** with:
  - `gen_metadata.go`: A `ReadGenMetadata(dataDir string) (*GenMetadata, error)` function. The gen_metadata file records per-turn usage for IDE conversations. Parse the JSON and return a struct with token counts. Mark the function with a `// Experimental: undocumented Antigravity internal, may change without notice` comment. If the file doesn't exist, return nil (not an error).
  - `quota.go`: A `CheckQuota(ctx context.Context) (*QuotaStatus, error)` function that shells out to `agy -p "/usage" --output-format json`, parses the response, and returns a struct with the quota pools and remaining capacity. This costs zero tokens. If `agy` is not in PATH, return an error.

- [x] **Add tests**:
  - Test `Cost.FromUsage` round-trips correctly
  - Test `ReadGenMetadata` with a sample file and with a missing file
  - Test `CheckQuota` error handling (at minimum, test that it handles `agy` not in PATH)

- [x] **Verify:** `go build ./...`, `go vet ./...`, and tests pass. Commit when clean.

## Verification Requirement
Run `go build ./...`, `go vet ./...`, and `go test ./internal/core/jobs/... ./internal/core/telemetry/...`. Commit when clean.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
Complete

### Changes Made
1. **`internal/core/jobs/types.go`**:
   - Extended `jobs.Cost` with `ThinkingTokens int` and `CacheReadTokens int` JSON fields matching `harness.Usage`.
   - Added `FromUsage(u harness.Usage) Cost` and `(c Cost) ToUsage() harness.Usage` conversion helpers.
   - Added `internal/core/jobs/cost_test.go` verifying complete round-trip conversion, JSON serialization, and SQLite driver `Value()` / `Scan()`.
   - Updated `internal/core/jobs/store_test.go` to assert `ErrNoJobAvailable`.
2. **`internal/core/tick/pipeline.go`**:
   - Implemented `usageTracker` decorating `harness.Housekeeper` per stage invocation to accumulate token usage across triage, extract, reconcile, and verify stages.
   - Updated stage job completions to persist populated `jobs.Cost` via `p.store.Complete(ctx, job.ID, &cost)` and track against `p.checkBudget(&cost)`.
   - Added `TestPipelineCostPersistence` in `internal/core/tick/pipeline_test.go` verifying that housekeeper usage is correctly captured and persisted.
3. **`internal/core/telemetry/` package**:
   - `gen_metadata.go`: Implemented `ReadGenMetadata(dataDir string) (*GenMetadata, error)` with experimental comment, graceful handling of non-existent files (`nil, nil`), path resolution for directory or file arguments, and flexible object / turn array parsing. Added unit tests covering missing files, valid objects, turn arrays, and corrupt files.
   - `quota.go`: Implemented `CheckQuota(ctx context.Context) (*QuotaStatus, error)` querying `agy -p "/usage" --output-format json` at zero tokens, detecting missing `agy` binary in PATH, and parsing groups and buckets with reset times. Added unit tests for PATH detection, parsing, error responses, and live execution.
4. **CLI**:
   - Added `cmd/repo-memory/cmd_usage.go` providing `repo-memory usage [--json]` CLI command and registered in `cmd/repo-memory/main.go`. Added tests in `cmd/repo-memory/cmd_usage_test.go`.

### Decisions Made
- Used a decorator pattern (`usageTracker`) around `harness.Housekeeper` in `pipeline.go` so token usage is captured directly without changing the housekeeper interface or stage constructor signatures.
- Supported both object and array layouts in `ReadGenMetadata` to accommodate varied Antigravity internal metadata formats, auto-aggregating totals if top-level counts are zero.
- Preserved hierarchical groups in `QuotaStatus.Groups` while also flattening buckets into `QuotaStatus.Buckets` with helper methods (`GetBucket`, `RemainingFraction`) for convenient access by spike experiments.

### Dead Ends & Warnings
None.

### Out-of-Scope Findings
- `cmd/repo-memory/cmd_search_test.go` fails during full repository tests due to global directory manipulation (`os.Chdir(tempDir)`).
- `internal/harness/antigravity/attribution_test.go` failed in full suite execution due to an idle-time check flake.

### Blockers
None.
