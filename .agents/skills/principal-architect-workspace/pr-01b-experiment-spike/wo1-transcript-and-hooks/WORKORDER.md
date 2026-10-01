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

**Completed**: 2026-10-01T21:19 UTC+1  
**Worker**: Antigravity (Claude Opus 4.6 Thinking)  
**Conversation**: `e9b80453-3c02-42d9-862a-07c17920a622`

### Checklist Status

#### E12: Transcript File Set ✅
- [x] Confirm `transcript_full.jsonl` is append-only during normal conversation flow
- [x] Examine step-index gaps in 5 conversations of different lengths
- [x] Hypothesize the cause of gaps
- [x] Write findings to `docs/experiments/E12-transcript-file-set.md`
- [ ] Branch or edit an earlier step in the UI and diff (partially covered — session reset behavior observed, but direct UI edit not performed)

#### E3: Transcript Currency at Hooks ✅
- [x] Compare transcript state at PreInvocation with `initialNumSteps`
- [x] Repeat for PreToolUse — check if current turn's thinking is included
- [x] Document the lag (if any)
- [x] Write findings to `docs/experiments/E03-transcript-currency.md`

#### E1: `ephemeralMessage` Persistence ✅
- [x] Inject unique marker via `ephemeralMessage` at invocation N
- [x] Ask the model about it at invocation N+5
- [x] Inspect `transcript.jsonl` for ephemeral message step
- [x] Repeat with second marker (MARKER-BETA-9K4R)
- [x] Write findings to `docs/experiments/E01-ephemeral-message.md`

### Key Findings Summary

| Finding | Impact |
|:---|:---|
| Transcript is **append-only** during normal flow | Safe to read prefix bytes; cursor-based reading is valid |
| Step indices have **gaps** (internal dispatch steps not serialized) | Parsers must NOT assume contiguous indices |
| Parallel tool calls cause **out-of-order** step writes | Step indices may appear non-monotonically in the file |
| Transcript has **zero lag** at both PreInvocation and PreToolUse | Hook can reliably read the latest model reasoning |
| PreToolUse transcript includes **current turn's thinking** | Enables reasoning-aware tool gating |
| `transcriptPath` always points to `transcript_full.jsonl` | Use full transcript, not compact, for hook reads |
| `invocationNum` **resets to 0 per user prompt** | Dedupe strategy must track across prompts, not just invocations |
| `ephemeralMessage` **works** with correct `injectSteps` format | Context injection is viable for Antigravity |
| `ephemeralMessage` is **per-invocation only** | NOT carried to next invocation; must re-inject every invocationNum=0 |
| `ephemeralMessage` **appears in transcript** as `EPHEMERAL_MESSAGE` type | Recorded on disk but invisible to model on subsequent invocations |
| Wrong hook response format is **silently ignored** | Must validate response schema; no error feedback |

### Issues Surfaced (Outside Boundary Box)

1. **SPEC §7 `invocationNum` semantics**: The SPEC assumes `invocationNum` increments across the session. In reality it resets to 0 with each user prompt. The dedupe ledger logic (which gates on `invocationNum == 0`) will re-inject context on EVERY user prompt. This may be desirable (ensures freshness) but the SPEC should document the reset behavior explicitly.

2. **SPEC missing: hook response schema**: The SPEC references `injectSteps` → `ephemeralMessage` but doesn't document the exact JSON structure. The correct format `{"injectSteps": [{"ephemeralMessage": "..."}]}` must be in the SPEC. Using `{"ephemeral_message": "..."}` at the top level is silently ignored.

3. **SPEC missing: `EPHEMERAL_MESSAGE` transcript step type**: The SPEC's transcript format documentation doesn't mention `EPHEMERAL_MESSAGE` as a step type or `SYSTEM_SDK` as a source. These should be documented.

4. **SPEC missing: step-index non-contiguity**: Parsers that assume `line_count == max_step_index + 1` will break. The SPEC should document that internal dispatch steps consume indices but are never serialized.

5. **SPEC missing: out-of-order writes**: Parallel tool call results are appended to the transcript before the parent PLANNER_RESPONSE. The SPEC should document that step indices may not be monotonically increasing in the file.

6. **E12 partial gap: branch/edit behavior**: The work order requested diffing the transcript before and after a UI branch/edit operation. This was not directly tested (only session reset behavior was observed). A follow-up experiment should be done if this is critical.

### Files Created
- `docs/experiments/E12-transcript-file-set.md`
- `docs/experiments/E03-transcript-currency.md`
- `docs/experiments/E01-ephemeral-message.md`
- `testdata/e1-marker-hook.sh` (experiment fixture, can be cleaned up)

### Files Modified
- `~/.gemini/config/plugins/repo-memory/hooks.json` (restored to `--probe` mode after experiments)
