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

**Completed**: 2026-10-01T23:08 UTC+1  
**Worker**: Antigravity Worker Agent  
**Conversation**: `2ab94d0c-029a-4966-9349-bb001dacff78`  
**Status**: Complete & Approved  

### Checklist Status
- [x] Read all experiment reports in `docs/experiments/E*.md` (E01 through E12).
- [x] Create `docs/experiments/SUMMARY.md` with:
  - Full synthesis table (`| Experiment | Status | Answer (1-line) | SPEC Sections Confirmed | SPEC Sections Changed |`).
  - Categorization for each experiment (`confirmed`, `changed`, `new-info`).
- [x] Create `docs/experiments/SPEC-CHANGES.md` with:
  - Proposed edits in unified diff format with line numbers and rationale.
  - Evidence citations mapping to experimental findings.
  - Updated E4 (`trigger: model_decision` pull model) and E12 (append-only proof, step-gap explanation, out-of-order parallel tool logging, session reset index restarts).
- [x] Verify that no experiment result contradicts another.
- [x] Flag any experiments where the answer is ambiguous or needs re-testing.
- [x] Obtain explicit user/principal approval before committing.

### Key Reconciliation Findings Summary

| Area | Finding & Impact |
|:---|:---|
| **E01 (`ephemeralMessage`)** | Strictly per-invocation transient; `invocationNum` resets to 0 on every prompt; `injectSteps` format required; task briefs cannot be injected repeatedly as ephemeral messages; capped at ≤ 200 tokens. |
| **E02 (Include resolution)** | Inlines into `user_rules` relative to `GEMINI.md` (repo root); refreshes on new session; missing targets silently ignored; 24 KB / 20k token limits confirmed. |
| **E03 (Transcript currency)** | Zero lag at `PreInvocation` and `PreToolUse`; model `thinking` and proposed tool arguments are flushed before `PreToolUse` fires; payload points to `transcript_full.jsonl`. |
| **E04 (Rule activation)** | Frontmatter requires `trigger: model_decision` (`activation:` is silently ignored); pull model via conditional rule pointer in system prompt costing 1 tool call (`view_file`). |
| **E05 (Hook latency)** | Enrolled repos pass easily (p50 ~3ms, p95 ~6.1ms ≤ 60ms); globally disabled kill switch is ~2.8ms; non-enrolled repos take ~57ms (failing ≤ 5ms target); requires fast marker or in-memory cache. |
| **E06 (Sidecar behavior)** | Sidecars are IDE-only child processes of `language_server multicall schedule`; CLI does not run sidecars; 60s ticking is rock-solid in IDE; startup jitter ~592ms; `tick.lock` is mandatory; log grows ~14MB/mo. |
| **E07 (Housekeeping isolation)** | `--project` flag is silently ignored by `agy -p`; isolation requires dedicated run directory (`runs/<repo-id>`) and `REPO_MEMORY_HOUSEKEEPING=1`; IDE and CLI DBs are completely separate. |
| **E08 (Subagent hooks)** | Subagents fire all 4 lifecycle hooks with unique `conversationId` and `transcriptPath`; returning `{}` for `PreToolUse` is an implicit deny (must return `{"decision": "allow"}`); establishes Antigravity subagent parity. |
| **E09 (Deny behavior)** | Deny reason delivered verbatim; parallel tool call denials are isolated; advisory wording prompts retries vs policy wording prompts pivot; `overwrite` enables non-blocking rewrites; exit != 0 is implicit deny; hook paths resolve relative to `hooks.json` dir. |
| **E10 (Overhead reduction)** | Irreducible via CLI flags (~16.8k tokens for Gemini 3.8 Flash), but Gemini 3.6 Flash uses ~9.9k tokens (41% reduction); no prefix caching across fresh CLI calls; 10–14s wall-clock latency per call mandates batching. |
| **E11 (Quota & rate limits)** | 0 rate limits across 25+ calls in 15m; CLI wall-clock latency provides natural rate limiting; Pro-tier OAuth subscription confirmed; long-term tracking in PR-05b. |
| **E12 (Transcript file set)** | Append-only growth confirmed via MD5 prefix matching; step-index gaps are unwritten internal dispatch steps; parallel tool calls produce out-of-order step indices in JSONL; session resets restart index at 0. |

### Issues Surfaced (Outside Boundary Box)
1. **Antigravity Silent Failures:** Across E01 (hook schema), E02 (missing include), E04 (rule frontmatter key), and E07 (`--project` flag), Antigravity consistently fails silently without producing stderr or diagnostic warnings. Tooling and installation scripts must generate strictly validated configurations.
2. **Non-Enrolled Hook Latency Tax:** Because the global hook runs on all repositories on the developer machine, non-enrolled repositories currently incur a ~57ms tax on every invocation unless optimized via an O(1) directory marker check (e.g. checking for `.memory/` before parsing registry files).
3. **CLI-Only Background Ticking:** CLI-only users cannot rely on the Antigravity sidecar scheduler. Release 1 must provide documentation or automated setup (`repo-memory schedule install`) for an OS cron/launchd backstop.
4. **Follow-Up Experiment E1b (`userMessage`):** Follow-up needed to determine if `userMessage` within `injectSteps` persists across invocations within a prompt cycle, which could enable persistent hook-based task brief injection.

### Deliverables Created
- `docs/experiments/SUMMARY.md` (committed: `dd07d71`)
- `docs/experiments/SPEC-CHANGES.md` (committed: `dd07d71`)
