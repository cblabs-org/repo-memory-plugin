---
name: pr-05a-pipeline-code-wo6
description: Handoff instructions for Work Order 6 (Local Publish)
complexity: standard
---

# WORKORDER: wo6-local-publish

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 6 of the knowledge pipeline: Local Publish (`internal/core/pipeline/publish/local.go`). Write new or changed claims to the `pending/` local lane, index them immediately so they are available, mark them as `unreviewed`, and implement security gating to prevent publishing sensitive or malicious content.

## Strict Boundary Box
- Do not implement any other pipeline stages (extract, reconcile, verify, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/core/pipeline/publish/local.go`, related pipeline interfaces if needed, and related test files.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Tests should not make real LLM calls; use fake/mock equivalents or data fixtures.

## Line-Item Instructions
- [ ] Implement `internal/core/pipeline/publish/local.go`.
- [ ] Ensure that new or changed claims are written to the `<data-dir>/<repo-id>/pending/` directory.
- [ ] Implement immediate indexing for published claims so they are queryable.
- [ ] Ensure all claims published locally are explicitly marked as `unreviewed`.
- [ ] Implement security and external-provenance gating to block sensitive or unverified content from being published.
- [ ] Write unit tests verifying the local publish behavior, including directory writing, indexing triggers, `unreviewed` markings, and security gating.

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

### Status
complete

### Changes Made
- `internal/core/pipeline/publish/local.go` — implemented `LocalPublisher` handling claim writing to `pending/`, security and provenance gating, and trigger indexing.
- `internal/core/pipeline/publish/local_test.go` — added unit tests covering publish behavior and gating checks.

### Decisions Made
- Used simple string matching for checking sensitive keywords.
- Implemented `Indexer` interface to allow the Local Publisher to trigger indexing immediately without tightly coupling to `index.go`.

### Dead Ends & Warnings
- `index.go` currently only exposes `InsertEpisode` but not individual claim indexing. We delegate that to the provided `Indexer` interface which caller logic must connect.

### Out-of-Scope Findings
- `index.go` needs a way to insert individual claims incrementally. Currently, it lacks an `InsertClaim` method. This needs to be built at the application layer or added to the `Index` type natively.

### Blockers
(none)
