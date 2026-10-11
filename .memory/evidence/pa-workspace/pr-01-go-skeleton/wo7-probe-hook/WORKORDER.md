---
name: pr-01-go-skeleton-wo7
description: Handoff instructions for Work Order 7 - Probe Hook
complexity: standard
---

# WORKORDER: WO7 - Probe Hook

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the `--probe` flag for the `hook antigravity <event>` command to support the experiment spike (PR-01b). This records the hook payload and a snapshot of the transcript file set into a probe log, and allows a configurable probe decision (allow/deny/ask) for `PreToolUse`.

## Strict Boundary Box
- DO NOT implement actual production delivery logic. This is purely for the `--probe` flag.
- DO NOT change parsing logic of transcripts.

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.

## Line-Item Instructions
- [ ] Update `cmd/repo-memory/cmd_hook.go` to add a `--probe` boolean flag and a `--probe-decision` string flag (valid options: `allow`, `deny`, `ask`, defaulting to `allow`).
- [ ] Implement `handleProbe(event string, payload []byte)` logic inside the `hook` command execution:
  - If `--probe` is passed, write the raw incoming hook JSON payload to `<data-dir>/<repo-id>/logs/probe_<event>_<timestamp>.json`.
  - Also attempt to locate the active transcript (using `transcriptPath` from the payload) and copy its current snapshot to `<data-dir>/<repo-id>/logs/probe_<event>_<timestamp>_transcript.jsonl`.
  - If the event is `PreToolUse`, parse the payload to extract fields if necessary, and return a JSON payload with the decision specified by `--probe-decision` (e.g. `{"decision": "deny", "reason": "probe deny"}`).
  - For other events, return `{}` (the no-op response).
- [ ] Write unit tests to verify the `--probe` flag writes the necessary files to the correct locations and returns the configured decision for PreToolUse.

## Verification Requirement
- Verify `make build` compiles correctly.
- Verify `go test ./...` passes.
- **CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

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
