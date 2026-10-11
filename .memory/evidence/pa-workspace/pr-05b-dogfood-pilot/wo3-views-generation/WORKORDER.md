---
name: pr-05b-dogfood-pilot-wo3
description: Handoff instructions for Work Order 3
complexity: standard
---

# pr-05b-dogfood-pilot / WO3 - Views Generation

## Role Definition
You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Generate markdown views (`MAP.md`, subsystem pages, claim indices, `SIGNATURES.md`, `DECISIONS.md`) from the claims generated in WO2. Wire the view generator into the pipeline or add a `repo-memory views` subcommand.

## Strict Boundary Box
- DO NOT run evaluation or modify the pipeline extraction logic.
- Focus strictly on `cmd/repo-memory` (to add a subcommand for views or wire into pipeline) and `internal/core/views` (or similar package where view generation resides).

## Tooling Constraints
- Ensure changes compile: `go build ./cmd/repo-memory`
- Ensure tests pass (at least the ones you touch/create): `go test ./...`
- Execute the view generation on the existing claims to produce the `.memory/views/` files.

## Line-Item Instructions
- [ ] Implement or integrate a view generator that reads `.memory/claims/` and produces `.memory/views/MAP.md`, subsystem pages, `SIGNATURES.md`, and `DECISIONS.md`.
- [ ] Add a `repo-memory views` subcommand to trigger view generation, or wire it securely into the pipeline.
- [ ] Run the view generation over the existing claims.
- [ ] Verify that `.memory/views/` is populated correctly and the files are readable/well-formatted.

## Verification Requirement
The worker must verify the view generation runs successfully and produces the output markdown files. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/views/generator.go` — Added a `Generator` that reads from the claim store and generates `MAP.md`, `SIGNATURES.md`, `DECISIONS.md`, and individual subsystem pages.
- `cmd/repo-memory/cmd_views.go` — Wired up a `repo-memory views` subcommand to trigger the generation of views into `.memory/views/`.

### Decisions Made
- Designed `Generator.Generate()` to filter claims internally by type (e.g. failure signatures vs decisions) and output the respective files so it handles missing/empty collections safely.

### Dead Ends & Warnings
- (none)

### Out-of-Scope Findings
- There are pre-existing upstream test failures in `cmd/repo-memory/cmd_search_test.go`, `internal/core/jobs/store_test.go`, and `internal/harness/antigravity/attribution_test.go`. The PA should investigate these broken tests under a separate work order.
- `cmd/repo-memory` seems to be ignored by `.gitignore` under the rule `repo-memory`, which could cause git to ignore source files in that directory.

### Blockers
(none)
