---
name: pr-01b-wo1
description: Handoff instructions for Work Order 1 — Transcript and Hook Experiments
complexity: interactive
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions. Your task is strictly limited to executing the checklist below.

## Objective
Run experiments E12 (remaining), E3, and E1 to answer open questions about the Antigravity transcript file set and hook behavior. Each experiment produces a documented report in `docs/experiments/`.

## Strict Boundary Box
- **DO NOT** modify any Go source code in `cmd/`, `internal/`, or `packaging/`.
- **DO NOT** create or modify SPEC.md — that happens in WO4.
- You **may** create files under `docs/experiments/` and use the probe hook (`repo-memory hook antigravity <event> --probe`) for data collection.
- You **may** create temporary test fixtures under `testdata/` if needed for observation.

## Prerequisites
1. Build the probe hook: `go build -o ./repo-memory ./cmd/repo-memory`
2. Install it: `./repo-memory install --harness antigravity`
3. Enrol this repository: `./repo-memory init`

## Line-Item Instructions

### E12: Transcript file set (remaining parts)
- [ ] Open a new Antigravity session in this repository. After each model turn, check whether `transcript_full.jsonl` has grown (compare byte count and last step index). Confirm it is **append-only** during normal conversation flow.
- [ ] Branch or edit an earlier step in the Antigravity UI. Diff the transcript file before and after. Document whether the file is **rewritten** (not append-only) in this case.
- [ ] Examine the step-index gaps present in both `transcript.jsonl` and `transcript_full.jsonl`. Count the gaps in 5 conversations of different lengths. Hypothesize the cause (e.g., tool calls, system messages, checkpoint steps).
- [ ] Write findings to `docs/experiments/E12-transcript-file-set.md`.

### E3: Transcript currency at hooks
- [ ] With the probe hook installed, make a model call and check the probe log. Compare the last step index in the transcript file at the moment `PreInvocation` fires with the `initialNumSteps` value from the hook payload.
- [ ] Repeat for `PreToolUse` — is the transcript current enough to include the current turn's reasoning (the `thinking` that produced the tool call)?
- [ ] Document the lag (if any) between transcript writes and hook execution.
- [ ] Write findings to `docs/experiments/E03-transcript-currency.md`.

### E1: `ephemeralMessage` persistence
- [ ] Using the probe hook, inject a unique marker string via `ephemeralMessage` at invocation N (e.g., "MARKER-ALPHA-7X2Q"). 
- [ ] Continue the conversation for 5+ more invocations. Ask the model "Do you recall seeing the text MARKER-ALPHA-7X2Q?" at invocation N+5.
- [ ] Inspect `transcript.jsonl` — does the ephemeral message appear as a step?
- [ ] Repeat with a second marker to confirm consistency.
- [ ] Write findings to `docs/experiments/E01-ephemeral-message.md`.

## Verification Requirement
Each experiment report must include:
1. **Question** (from the SPEC)
2. **Method** (what was done)
3. **Raw evidence** (file sizes, step indices, transcript excerpts — redacted if needed)
4. **Answer** (clear yes/no/qualified answer)
5. **SPEC impact** (which sections are confirmed or need changes)

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
