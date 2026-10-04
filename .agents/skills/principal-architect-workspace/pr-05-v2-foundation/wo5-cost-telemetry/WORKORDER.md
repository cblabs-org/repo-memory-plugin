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

- [ ] **Extend `jobs.Cost`** in `internal/core/jobs/types.go`:
  - Add `ThinkingTokens int` and `CacheReadTokens int` JSON fields
  - Add a `FromUsage(u harness.Usage) Cost` convenience function

- [ ] **Wire cost into pipeline**: In `internal/core/tick/pipeline.go`, wherever executor results are processed and jobs are completed, copy the usage from the result into the job's cost. Look for calls to `p.store.Complete(ctx, job.ID, ...)` — the second argument is `*Cost`. Ensure it's populated from the executor result's `Usage`.

- [ ] **Create `internal/core/telemetry/` package** with:
  - `gen_metadata.go`: A `ReadGenMetadata(dataDir string) (*GenMetadata, error)` function. The gen_metadata file records per-turn usage for IDE conversations. Parse the JSON and return a struct with token counts. Mark the function with a `// Experimental: undocumented Antigravity internal, may change without notice` comment. If the file doesn't exist, return nil (not an error).
  - `quota.go`: A `CheckQuota(ctx context.Context) (*QuotaStatus, error)` function that shells out to `agy -p "/usage" --output-format json`, parses the response, and returns a struct with the quota pools and remaining capacity. This costs zero tokens. If `agy` is not in PATH, return an error.

- [ ] **Add tests**:
  - Test `Cost.FromUsage` round-trips correctly
  - Test `ReadGenMetadata` with a sample file and with a missing file
  - Test `CheckQuota` error handling (at minimum, test that it handles `agy` not in PATH)

- [ ] **Verify:** `go build ./...`, `go vet ./...`, and tests pass. Commit when clean.

## Verification Requirement
Run `go build ./...`, `go vet ./...`, and `go test ./internal/core/jobs/... ./internal/core/telemetry/...`. Commit when clean.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(pending)
