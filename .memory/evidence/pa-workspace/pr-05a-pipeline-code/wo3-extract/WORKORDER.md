---
name: pr-05a-pipeline-code-wo3
description: Handoff instructions for Work Order 3 (Extract)
complexity: architect
---

# WORKORDER: wo3-extract

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 3 of the knowledge pipeline: Extract (`internal/core/pipeline/extract/`). Implement packing to the 60k cap, applying a 20-candidate limit, and extracting candidates with anchors, evidence, and gaps, strictly following P13 (evidence independence).

## Strict Boundary Box
- Do not implement any other pipeline stages (reconcile, verify, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/core/pipeline/extract/`, `prompts/extract/`, and related test files.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Use recorded executor cassettes for tests.

## Line-Item Instructions
- [ ] Implement `internal/core/pipeline/extract/packer.go` to pack episodes into a single prompt up to a 60k token cap. Ensure a parent session is grouped with its subagents first.
- [ ] Implement `internal/core/pipeline/extract/extractor.go` to call the `Housekeeper` using `prompts/extract/`.
- [ ] Ensure the extraction limits candidates to a maximum of 20 per episode in the prompt or processing logic.
- [ ] Enforce all extraction rules from SPEC §6.5: output must include candidates with anchors, evidence, and `gaps`.
- [ ] Ensure P13 (evidence independence) is structurally accommodated during extraction logic, e.g., correctly distinguishing agent text/reasoning from hard tool outcomes and human-authored steps as valid evidence.
- [ ] Write unit tests verifying packing logic, candidate parsing, gap extraction, and evidence extraction rules.

## Verification Requirement
The worker must verify it compiles and runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ensure all tests pass. After changes are tested, commit them and append the Completion Report.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `internal/core/pipeline/extract/packer.go` — Implemented batching logic to pack episodes by token cap and session ID, sorting parents before subagents.
- `internal/core/pipeline/extract/extractor.go` — Added the `Extractor` that invokes the LLM (`Housekeeper`), deserializes candidates, strictly caps output to 20 candidates per episode, and locally enforces P13 by filtering non-human, non-tool evidence.
- `internal/core/pipeline/extract/extractor_test.go` — Wrote unit tests for packing limits, correct grouping, Candidate limit enforcement, and Evidence Independence (P13).
- `prompts/extract/prompt.md` — Drafted a basic prompt providing Extractor persona and constraint rules.

### Decisions Made
- Used heuristic token counting (`bytes / 4`) as standard across the project.
- P13 checks were hardened directly into `extractor.go` rather than purely relying on the LLM, rejecting any cited evidence that isn't `tool_outcome` or `human`.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
