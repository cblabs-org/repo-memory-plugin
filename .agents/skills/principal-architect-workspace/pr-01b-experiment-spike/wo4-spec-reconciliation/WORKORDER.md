---
name: pr-01b-wo4
description: Handoff instructions for Work Order 4 — Spec Reconciliation
complexity: standard
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Summarise the experiment results from WO1–WO3. List every SPEC section confirmed or changed. Propose concrete SPEC edits for user approval before PR-02a starts.

## Strict Boundary Box
- **DO NOT** modify Go source code.
- **DO NOT** modify SPEC.md directly — produce a proposed diff in `docs/experiments/SPEC-CHANGES.md` for the user to review.
- You **may** read and reference all files in `docs/experiments/`.

## Line-Item Instructions

- [ ] Read all experiment reports in `docs/experiments/E*.md` (E01 through E12).
- [ ] Create `docs/experiments/SUMMARY.md` with:
  - A table: `| Experiment | Status | Answer (1-line) | SPEC Sections Confirmed | SPEC Sections Changed |`
  - For each experiment, categorize as: `confirmed` (SPEC is correct as-is), `changed` (SPEC needs an edit), or `new-info` (SPEC doesn't address this, but should).
- [ ] Create `docs/experiments/SPEC-CHANGES.md` with:
  - For each `changed` experiment, a proposed edit in unified diff format or a clear before/after.
  - Each edit references the experiment report as evidence.
  - Note: E4 and E12 are already partially answered in the SPEC — update those entries with the confirmed/remaining findings.
- [ ] Verify that no experiment result contradicts another.
- [ ] Flag any experiments where the answer is ambiguous or needs re-testing.

## Verification Requirement
- All 12 experiments (E1–E12) are accounted for in the summary.
- Every proposed SPEC change has an experiment reference as evidence.
- **CRITICAL:** Before committing, ask the user if they are happy with the summary and proposed changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
