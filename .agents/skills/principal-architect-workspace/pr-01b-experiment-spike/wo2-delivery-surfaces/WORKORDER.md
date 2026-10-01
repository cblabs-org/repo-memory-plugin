---
name: pr-01b-wo2
description: Handoff instructions for Work Order 2 — Delivery Surface Experiments
complexity: interactive
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions. Your task is strictly limited to executing the checklist below.

## Objective
Run experiments E2, E4, E9, and E8 to answer open questions about Antigravity's delivery surfaces: include resolution, rule activation, PreToolUse deny behavior, and subagent hook coverage. Each experiment produces a documented report in `docs/experiments/`.

## Strict Boundary Box
- **DO NOT** modify any Go source code in `cmd/`, `internal/`, or `packaging/`.
- **DO NOT** create or modify SPEC.md — that happens in WO4.
- You **may** create files under `docs/experiments/`, temporary rules/includes for testing, and probe hook configurations.

## Prerequisites
- Probe hook installed and this repository enrolled (done in WO1).

## Line-Item Instructions

### E2: Include resolution and refresh
- [ ] Create a test file `.memory/views/MAP-test.md` with a known marker string.
- [ ] Add an include line `@[memory map](.memory/views/MAP-test.md)` to `GEMINI.md` (or create it if absent).
- [ ] Start a new Antigravity session. Ask the model "What does the memory map say?" — verify it sees the marker.
- [ ] Change the marker in `MAP-test.md` (don't restart the session). Start a **new** session and check if the updated content is visible.
- [ ] Test resolution: does the path resolve relative to `GEMINI.md`'s location (repo root)?
- [ ] Note the 24 KB per-file cap and the 20k-token always-on budget from the built-in docs.
- [ ] Clean up the test file and include line.
- [ ] Write findings to `docs/experiments/E02-include-resolution.md`.

### E4: `model_decision` rule activation (remaining check)
- [ ] Create a minimal `model_decision` rule file in `.agents/rules/test-model-decision.md`:
  ```yaml
  ---
  activation: model_decision
  description: "Activated when the user asks about OTA firmware updates or BLE connectivity"
  ---
  This is a test rule for E4. If you see this, the model_decision activation worked.
  ```
- [ ] Start a new session and ask about "OTA firmware updates" — does the rule content appear in the model's context? Check the transcript for evidence of rule loading.
- [ ] Ask about something unrelated (e.g., "write a Python hello world") — the rule should NOT load.
- [ ] Clean up the test rule.
- [ ] Write findings to `docs/experiments/E04-rule-activation.md`.

### E9: `deny` + `reason` behavior
- [ ] Configure the probe hook with `--probe-decision deny --probe-reason "This action was previously tried and failed because the flash partition was misaligned (clm-test-001). If your approach differs, re-issue the call."` for `PreToolUse`.
- [ ] In a session, ask the model to run a command (e.g., `ls -la`). Observe:
  - Does the model see the denial and reason?
  - Does it re-issue the same call or abandon it?
  - What wording in the reason makes the model re-issue vs. give up?
- [ ] Test with a **batch of parallel tool calls** (e.g., ask the model to create 3 files at once). Deny one of them. Observe: does the denial disrupt the other calls?
- [ ] Test alternative wordings:
  - Policy-sounding: "This action is not permitted because..."
  - Advisory: "Warning: this was tried before and failed because..."
  - Instructional: "Re-issue this call if your approach differs. Previous attempt failed because..."
- [ ] Test the `overwrite` decision (if available in the harness) — does the rewrite notice make it usable as a non-blocking advisory?
- [ ] Clean up probe configuration.
- [ ] Write findings to `docs/experiments/E09-deny-behavior.md`.

### E8: Subagent hook coverage
- [ ] With the probe hook installed, spawn a subagent using `invoke_subagent` (a simple task like "list files in this directory").
- [ ] Check the probe logs: did the subagent fire its own `PreInvocation`, `PreToolUse`, and `Stop` hooks?
- [ ] If yes: does it have its own `conversationId` and `transcriptPath`?
- [ ] If no: document that workers are captured by the sweep only, and briefs must go through the WORKORDER, not hooks.
- [ ] Write findings to `docs/experiments/E08-subagent-hooks.md`.

## Verification Requirement
Same format as WO1: Question, Method, Raw Evidence, Answer, SPEC Impact.

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
