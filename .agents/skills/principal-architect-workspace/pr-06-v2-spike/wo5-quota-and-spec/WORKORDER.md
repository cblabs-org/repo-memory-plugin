---
name: pr-06-v2-spike-wo5
description: X3 quota measurement, spike summary, and SPEC v1.0 edit proposals
complexity: standard
---

# WO5: X3 Quota, Spike Summary, and SPEC Edits

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Run X3 quota measurement, summarise ALL spike findings (X1, X3, X5, X8, X9), list every v2 design section confirmed or changed, and propose SPEC v1.0 edits. Document in `docs/experiments/X3-quota.md` and `docs/experiments/spike-summary.md`.

## Strict Boundary Box
- **Create:** `docs/experiments/X3-quota.md`, `docs/experiments/spike-summary.md`
- **DO NOT** modify any production code
- **DO NOT** modify hook handlers, pipeline, evidence, enrollment, or install code

## Context

### Prior experiment findings to incorporate

**X1 (Stop-Continue) — from WO1 interactive session:**
Read the completion report: `.agents/skills/principal-architect-workspace/pr-06-v2-spike/wo1-stop-continue/WORKORDER.md`
Key findings:
- Stop hook continuation works for harvesting turn notes
- `terminationReason=NO_TOOL_CALL`, `fullyIdle=true`, `executionNum=0` gate is correct
- `parentConversationId` absent from Stop payload — NONE self-filtering is natural solution
- Plugin enablement bug: `agent_permissions` disabled the plugin (fixed in PR-06b)
- hooks.json keys must be PascalCase

**X9 (Hook Contracts) — from WO2:**
Read: `.agents/skills/principal-architect-workspace/pr-06-v2-spike/wo2-hook-contracts/WORKORDER.md`
Key findings:
- PreToolUse is fail-CLOSED (empty/malformed = implicit deny)
- Other hooks fail-OPEN
- 35 contract tests written

**X5 (Channels) — from WO3:**
Read: `docs/experiments/X5-channels.md`
Key finding: **systemMessage and userMessage NOT supported by Antigravity PreInvocation**. Must find alternative delivery mechanism.

**X8 (Lean Agent) — from WO4:**
Read: `docs/experiments/X8-lean-agent.md`
Key findings:
- File-based agent resolution fails dynamically (needs daemon restart or define_subagent)
- inheritCustomizations: false correctly prevents hook loading
- --json-schema supported for output constraints

### X3 quota background
The v2 design (§4.4) uses `/usage` to check remaining quota before running pipeline jobs. The API returns `remaining_fraction` (~0.1% resolution) for the cap nearest to being reached.

## Line-Item Instructions

### 1. X3 Quota Measurement

- [ ] **Check current quota state:**
  ```bash
  agy /usage 2>&1 || echo "agy /usage not available"
  ```
  If `/usage` is not a valid command, check the Antigravity SDK or MCP tools for quota information.

- [ ] **Document the quota API:** What fields are returned? What does `remaining_fraction` look like? What is `window`?

- [ ] **Measure conversation token costs:**
  Read the transcripts from the X5 and X8 test conversations to extract token usage:
  - Check `gen_metadata` fields in PLANNER_RESPONSE steps
  - Look for `input_tokens`, `output_tokens`, `cached_tokens`
  - If not in transcripts, document that token metrics are not exposed in the transcript format

- [ ] **Write `docs/experiments/X3-quota.md`** with:
  - Quota API findings (available fields, resolution, window behavior)
  - Token measurement methodology and results
  - Recommendations for quota guard implementation

### 2. Spike Summary

- [ ] **Create `docs/experiments/spike-summary.md`** consolidating all findings:

  ```markdown
  # V2 Spike Summary (PR-06)

  ## Experiment Results

  | Experiment | Question | Result | Impact |
  |---|---|---|---|
  | X1 | Stop-continue for turn notes | ✅ Works | Gate: terminationReason=NO_TOOL_CALL, fullyIdle, executionNum=0 |
  | X3 | Quota visibility | ... | ... |
  | X5 | systemMessage/userMessage channels | ❌ Not supported | Must redesign delivery (PR-08) |
  | X8 | Lean agent for distillation | ⚠ Partial | File resolution fails; use define_subagent |
  | X9 | Hook contracts | ✅ Documented | PreToolUse fail-closed; others fail-open |

  ## V2 Design Sections: Confirmed vs Changed

  | Section | Status | Notes |
  |---|---|---|
  | §2.3 Plugin loading | ✅ Confirmed | Split architecture works (PR-06b) |
  | §4.4 Quota guard | ... | ... |
  | §5.1 Distillation agent | ⚠ Changed | Must use define_subagent, not file-based |
  | §8.1 Delivery via systemMessage | ❌ Changed | systemMessage not supported; need alternative |
  | ... | ... | ... |
  ```

- [ ] **For each v2 design section** (§1 through §10), state whether confirmed, changed, or untested
- [ ] **List platform assumptions** that were validated or invalidated
- [ ] **List the open questions** that remain for PR-07+

### 3. SPEC v1.0 Edit Proposals

- [ ] **Read the current SPEC:** `SPEC.md`
- [ ] **Read the v2 design:** `docs/design-v2-distillation-retrieval.md`
- [ ] **Propose edits** based on spike findings. Format as a diff-style list:
  ```markdown
  ### Proposed SPEC Edits

  1. **§X.Y — [section name]**
     - Current: "..."
     - Proposed: "..."
     - Rationale: [which experiment proved this]

  2. **§X.Y — [section name]**
     ...
  ```
- [ ] Focus on:
  - Delivery mechanism change (systemMessage → alternative)
  - Distillation agent spawning (define_subagent, not file-based)
  - Hook contract documentation (fail-closed vs fail-open)
  - Enrollment model (init→enroll)
  - Split plugin architecture
  - Any quota findings

- [ ] **DO NOT modify SPEC.md directly** — propose edits for user approval

## Verification Requirement
- All experiment reports reference transcript evidence or prior WO findings
- Spike summary covers all 5 experiments
- SPEC edit proposals are actionable with clear rationale
- `go build ./...` still passes (no code changes)

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(pending)
