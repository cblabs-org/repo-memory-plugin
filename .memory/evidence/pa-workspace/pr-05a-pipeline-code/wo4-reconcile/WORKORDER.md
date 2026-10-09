---
name: pr-05a-pipeline-code-wo4
description: Handoff instructions for Work Order 4 (Reconcile)
complexity: architect
---

# WORKORDER: wo4-reconcile

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 4 of the knowledge pipeline: Reconcile (`internal/core/pipeline/reconcile/`). Perform anchor lookup (≤ 30), support the 7 reconciliation operations, and implement the P13 mechanical precondition (after an exposure, support needs an outcome that touched an anchored file or symbol, or ran the claim's `enforced_by` check, before the reconciler's judgement applies). It must be a single writer process.

## Strict Boundary Box
- Do not implement any other pipeline stages (extract, verify, publish, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/core/pipeline/reconcile/`, `prompts/reconcile/`, and related test files.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Use recorded executor cassettes for tests (e.g., `fake.NewFakeHousekeeper`).

## Line-Item Instructions
- [ ] Implement `internal/core/pipeline/reconcile/reconciler.go` as a single writer that coordinates the reconciliation process.
- [ ] Implement anchor lookup limiting to ≤ 30 anchors.
- [ ] Support the 7 reconciliation operations as defined in the SPEC §6.6.
- [ ] Enforce the P13 mechanical precondition: after an exposure, support needs an outcome that touched an anchored file/symbol or ran the claim's `enforced_by` check before applying the LLM reconciler's judgement.
- [ ] Create `prompts/reconcile/prompt.md` to guide the LLM Housekeeper through the reconciliation process.
- [ ] Write unit tests verifying the single writer behavior, the 7 operations, the anchor lookup limit, and the P13 precondition.

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
- `internal/core/pipeline/reconcile/reconciler.go` — Added the Reconciler to coordinate stage 4 of the pipeline, supporting the 7 reconciliation operations and P13 rule.
- `internal/core/pipeline/reconcile/reconciler_test.go` — Unit tests for the reconciler covering single writer lock, 7 operations, P13 rule, and anchor limits.
- `prompts/reconcile/prompt.md` — Reconciler prompt added.

### Decisions Made
- Enforced P13 (Evidence independence) strictly after LLM outputs a `support` operation. The reconciler looks up the matching claim ID, verifies if the corresponding episode had an exposure, and if so, ensures an outcome touched the anchored file or the `enforced_by` command before allowing the support to persist.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
