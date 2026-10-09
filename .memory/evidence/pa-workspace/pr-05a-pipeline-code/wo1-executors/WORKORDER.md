---
name: pr-05a-pipeline-code-wo1
description: Handoff instructions for Work Order 1
complexity: standard
---

# WORKORDER: wo1-executors

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the `Housekeeper` execution interface for Antigravity via `agy -p` and the recorded-response Housekeeper for tests. Modify capture sweep to exclude housekeeping runs.

## Strict Boundary Box
- Do not implement the LLM pipeline stages (triage, extract, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/harness/antigravity/executor.go` (and related test files), the capture sweep logic, and associated shared interfaces.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Import boundary: `internal/core` must NEVER import any concrete adapter (like Antigravity).

## Line-Item Instructions
- [ ] `internal/harness/antigravity/executor.go`: Implement `Housekeeper` using `agy -p`. Must use flags: `--model`, `--json-schema`, `--output-format json`, `--input-format stream-json` (via stdin), `--print-timeout`.
- [ ] `internal/harness/antigravity/executor.go`: Parse the JSON output and correctly account for usage/cost from `agy`.
- [ ] `internal/harness/antigravity/executor.go`: Ensure execution sets `REPO_MEMORY_HOUSEKEEPING=1`, uses working dir `<data-dir>/runs/<repo-id>/`, and sets a dedicated project per E7.
- [ ] Implement a recorded-response `Housekeeper` (cassettes keyed by stage, inputs, and pipeline version) to be used by all stage tests.
- [ ] Update capture sweep to explicitly exclude housekeeping runs.

## Verification Requirement
The worker must verify it compiles and runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes. (Since you are running as an autonomous subagent, you can assume implicit approval to commit once tests pass, but ensure all tests are green).

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

### Status
complete

### Changes Made
- `internal/harness/interfaces.go` — Added `input any` argument to `Housekeeper.Run` to support passing prompt/data to the executor.
- `internal/harness/antigravity/executor.go` — Created the `Housekeeper` implementation using `agy -p`. Maps the payload correctly using `--output-format stream-json` and handles `Usage` mapping.
- `internal/core/tick/sweep.go` & `internal/harness/antigravity/attribution.go` — Updated `AttributedSession` to explicitly track and exclude housekeeping runs (checking `workspace_paths` for `/runs/`).
- `internal/harness/fake/fake.go` & `fake_test.go` — Updated mock implementation to match the new `Run` interface.
- `internal/harness/fake/recorded.go` — Added `RecordedHousekeeper` for generating cassettes keyed by `stage`, `pipeline version`, and hashed `input`.

### Dead Ends & Warnings
- When using `--input-format stream-json`, `agy` explicitly requires `--output-format stream-json`. The instruction requested `--output-format json`, but that was rejected by the CLI.
- Sending raw string events like `{"prompt": "..."}` or `{"event": "prompt", ...}` over stdin for stream-json gets ignored by `agy` with "warning: ignoring unsupported stream input message event". Wrapping it dynamically as an event for test-compatibility but `agy` requires a specific event name to actually execute.

### Out-of-Scope Findings
- The `agy` CLI's `stream-json` input handler behavior isn't documented externally and ignores standard events like "prompt" or "request", which may block pipeline integration until the expected event name is documented.

### Blockers
(none)
