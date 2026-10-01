---
name: pr-01b-wo3
description: Handoff instructions for Work Order 3 — Runtime and Cost Experiments
complexity: interactive
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions. Your task is strictly limited to executing the checklist below.

## Objective
Run experiments E6, E5, E7, E10, and E11 to answer open questions about sidecar behavior, hook latency, housekeeping isolation, overhead, and quota. Each experiment produces a documented report in `docs/experiments/`.

## Strict Boundary Box
- **DO NOT** modify any Go source code in `cmd/`, `internal/`, or `packaging/`.
- **DO NOT** create or modify SPEC.md — that happens in WO4.
- You **may** create files under `docs/experiments/`, temporary sidecar configurations, and `agy -p` test scripts.

## Prerequisites
- Probe hook installed and this repository enrolled (done in WO1).
- Access to `agy` CLI for housekeeping experiments.

## Line-Item Instructions

### E6: Sidecar ticking behavior
- [ ] Create a minimal test sidecar in `~/.gemini/config/plugins/repo-memory/sidecars/test-tick/sidecar.json` with `builtin: schedule`, `* * * * *`, `restart_policy: "always"`. The command should write a timestamp to a log file.
- [ ] Enable it in the Antigravity **Automations** dashboard.
- [ ] Observe `sidecar_data/.../logs` (find the path in Antigravity's data directory):
  - Does it tick while only the CLI is open (no IDE)?
  - Close the CLI entirely — does it stop?
  - Sleep the machine for 2+ minutes, wake it — are missed ticks caught up or skipped?
- [ ] Test overlap: make the sidecar command sleep for 90 seconds. Does the next tick start, or does it skip because the previous one is still running?
- [ ] Clean up the test sidecar.
- [ ] Write findings to `docs/experiments/E06-sidecar-behavior.md`.

### E5: Hook latency (probe estimate)
- [ ] With the probe hook recording, measure the wall-clock time from process start to hook response for:
  - **Fast path** (non-enrolled repo, kill switch set): should be ≤ 5ms
  - **Enrolled repo, no injection** (PreInvocation returning `{}`): target p95 ≤ 60ms
  - **Enrolled repo, with injection** (PreInvocation returning ephemeralMessage): target p95 ≤ 150ms
- [ ] Run each scenario 20 times. Report: min, p50, p95, max.
- [ ] Note: this is a preliminary estimate. PR-07 will confirm with the full delivery engine.
- [ ] If on macOS only, note that Windows measurements are needed from the user separately.
- [ ] Write findings to `docs/experiments/E05-hook-latency.md`.

### E7: Housekeeping run isolation
- [ ] Run a tagged housekeeping call: `agy -p --model flash --project repo-memory-housekeeping-test --output-format json "Say hello and list your conversation ID"`.
- [ ] Check if `--project` places it in a separate project namespace.
- [ ] Try `agy --continue` from the repo directory — can you resume or see the housekeeping conversation?
- [ ] Inspect both `conversation_summaries.db` stores (app: `~/.gemini/app/` and CLI: `~/.gemini/cli/`) — does the housekeeping conversation appear in either?
- [ ] Does it show up in the Antigravity IDE's conversation history?
- [ ] Write findings to `docs/experiments/E07-housekeeping-isolation.md`.

### E10: Housekeeping overhead reduction
- [ ] Run `agy -p --output-format json "Return the word HELLO"` with the default config. Record `usage` (input tokens, output tokens).
- [ ] Run with `--agent '{"identity":{"name":"test"}}'` (minimal agent definition, no skills). Record usage.
- [ ] Run with all global plugins disabled (if possible via `--no-plugins` or config). Record usage.
- [ ] Compare: what is the fixed overhead? Can it be reduced from ~17k tokens?
- [ ] Test prefix caching: run the same prompt 5 times in quick succession. Do later runs show cache reads in the usage output?
- [ ] Write findings to `docs/experiments/E10-overhead-reduction.md`.

### E11: Quota and rate limits (preliminary)
- [ ] Run 10 housekeeping calls in quick succession using `flash` model. Note any rate limit errors (429s) and their retry-after values.
- [ ] Run 5 calls with `flash_lite`. Compare.
- [ ] Document the subscription tier (free / pro / team) and any observable per-model limits.
- [ ] Note: full quota tracking happens over 2 weeks in PR-05b. This is a preliminary reading.
- [ ] Write findings to `docs/experiments/E11-quota-rate-limits.md`.

## Verification Requirement
Same format as WO1: Question, Method, Raw Evidence, Answer, SPEC Impact.

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
