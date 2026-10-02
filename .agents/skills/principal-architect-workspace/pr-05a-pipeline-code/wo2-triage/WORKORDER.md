---
name: pr-05a-pipeline-code-wo2
description: Handoff instructions for Work Order 2 (Triage)
complexity: architect
---

# WORKORDER: wo2-triage

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 2 of the knowledge pipeline: Triage (`internal/core/pipeline/triage/`). This includes rule-based triage, keep/skip rules, salience windows (including decision-marker windows), synopsis generation, and small-model fallback.

## Strict Boundary Box
- Do not implement any other pipeline stages (extract, reconcile, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/core/pipeline/triage/`, `prompts/triage/`, and related test files.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Verify against the recorded executor cassettes where appropriate.

## Line-Item Instructions
- [x] Implement `internal/core/pipeline/triage/rules.go` to support rule-based triage (keep/skip rules) based on SPEC §6.4. Includes checking memory artefact writes, failures/reverts, decision signals, and the skip-all condition.
- [x] Implement salience windows calculation in `internal/core/pipeline/triage/windows.go` (failure windows, revert windows, human message windows, memory artefact write windows, device/soak outcomes).
- [x] Implement decision-marker windows in `windows.go` to capture reasoning spans (using configurable marker list, capped at 2 per episode).
- [x] Implement synopsis generation in `internal/core/pipeline/triage/synopsis.go` (files touched by subsystem, commits, outcome list, exposures, human messages verbatim).
- [x] Implement the small-model fallback for the ambiguous remainder cases, utilizing the `Housekeeper` interface to call the LLM using `prompts/triage/`.
- [x] Create tests to cover rule-based triage, salience windows, decision-marker windows, and synopsis generation.

## Verification Requirement
The worker must verify it compiles and runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ensure all tests pass. After changes are tested, commit them and append the Completion Report.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/pipeline/triage/rules.go` — Added deterministic keep/skip rules per SPEC §6.4.
- `internal/core/pipeline/triage/windows.go` — Added salience windows calculation including fallback windows.
- `internal/core/pipeline/triage/synopsis.go` — Added episodic synopsis generation (files touched by subsystem, commits, outcomes, exposures, etc).
- `internal/core/pipeline/triage/triage.go` — Added the primary `Triager` struct wrapping the above features, including the `Housekeeper` LLM fallback for ambiguous episodes.
- `internal/core/pipeline/triage/rules_test.go` — Added rule evaluation tests.
- `internal/core/pipeline/triage/windows_test.go` — Added window generation tests.
- `internal/core/pipeline/triage/synopsis_test.go` — Added synopsis generation tests.

### Decisions Made
- Allowed `skip` to only evaluate if `keep` is false in rule engine, so keep takes precedence and we don't accidentally skip-all on an already kept episode.

### Dead Ends & Warnings
- When using `harness.StepKindUser`, the human messages will be parsed as verbatim.
- The `ep.Header.ID` doesn't exist, it is `EpisodeID`.

### Out-of-Scope Findings
- (none)

### Blockers
(none)
