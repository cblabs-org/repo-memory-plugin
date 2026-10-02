---
name: pr-02b-capture-runtime-wo5
description: Handoff instructions for Work Order 5
complexity: standard
---

# pr-02b-capture-runtime / WO5 - session-attribution

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Map conversations to enrolled repos and handle duplicate deduplication.

## Strict Boundary Box
- Do not change tick core logic beyond session attribution.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [ ] Map conversations to enrolled repos via `workspace_uris` in `conversation_summaries.db` (app and CLI).
- [ ] De-duplicate conversations present in both stores.
- [ ] Link subagents to parents via `parent_conversation_id`.

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
