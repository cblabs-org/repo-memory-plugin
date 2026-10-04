---
name: pr-05b-dogfood-pilot-wo4
description: Handoff instructions for Work Order 4
complexity: interactive
---

# pr-05b-dogfood-pilot / WO4 - B5 Evaluation

## Role Definition
You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Your task is strictly limited to executing the checklist below interactively with the user.

## Objective
Produce a B5 evaluation sheet by sampling ≥20 claims from `.memory/claims/`. Present the results to the user for review. This validates the extraction quality against the PA's structured records as ground truth.

## Strict Boundary Box
- DO NOT modify the pipeline logic.
- DO NOT generate views.
- Focus strictly on generating an evaluation sheet (e.g. `B5_EVALUATION.md` in `.memory/`) based on a sample of claims, and presenting it to the user.

## Tooling Constraints
- Read from `.memory/claims/` to gather data.

## Line-Item Instructions
- [ ] Sample ≥20 claims from `.memory/claims/`.
- [ ] Construct an evaluation sheet/matrix mapping these claims to their ground truth episodes to measure extraction accuracy.
- [ ] Write the evaluation to `.memory/B5_EVALUATION.md`.
- [ ] Present the summary of the evaluation to the user for review.
- [ ] Ask the user if they approve the extraction quality based on this sample.
- [ ] Once the user approves, write the Completion Report and provide the user with the PA return prompt.

## Verification Requirement
The worker must interactively present the evaluation to the user.

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
- Decision description and rationale

### Dead Ends & Warnings
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
- Finding description and suggested action

### Blockers
(none)
