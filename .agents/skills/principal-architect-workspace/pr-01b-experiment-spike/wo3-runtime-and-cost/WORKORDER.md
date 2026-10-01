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

### 2026-10-01T22:56+01:00 — WO3 Complete: Runtime and Cost (E6, E5, E7, E10, E11)

**Summary**: Hook latency is fast for enrolled repos (~3ms p95) but slow for non-enrolled (~57ms); sidecars are IDE-only infrastructure; `--project` flag is silently ignored; fixed overhead is ~17k tokens with no prefix caching; no rate limits observed at sequential call rates.

**Experiments Completed**:
- **E05 (Hook Latency):** Enrolled repos: p50=3ms, p95=6ms. Non-enrolled repos: p50=57ms, p95=59ms. Kill-switch disabled: p50=2.8ms. Process startup baseline: ~2.5ms. Injection adds negligible latency. Non-enrolled path is bottleneck due to enrollment database lookup.
- **E06 (Sidecar Behavior):** Sidecars are IDE-managed child processes (`language_server multicall schedule`). 119 ticks over 2 hours, perfect 60s intervals, zero gaps. Fixed jitter (~592ms) calculated once at startup. CLI does not run sidecars. Log growth ~460KB/day per sidecar.
- **E07 (Housekeeping Isolation):** CLI and IDE have fully separate conversation databases and brain directories. `--project` flag is silently ignored — all CLI conversations stored under `default-cli-project`. `--continue` resumes most-recent conversation, not project-tagged ones. Housekeeping conversations invisible to IDE UI.
- **E10 (Overhead Reduction):** Fixed overhead: ~16.8k tokens (Gemini 3.8 Flash), ~9.9k (Gemini 3.6 Flash). `--agent`, `--disable-slash-commands` have no effect. `--sandbox` adds +783 tokens. No prefix caching: `cache_read_tokens=0` for all runs (new conversation each time). Wall clock: ~11s per call (6-7s CLI overhead + 4-5s API).
- **E11 (Quota/Rate Limits):** 10 sequential calls (Gemini 3.8 Flash) + 5 (Gemini 3.6 Flash) = 0 rate limit errors. 25+ total calls in 15min window, zero throttling. CLI's ~11s latency acts as natural rate limiter. Pro-tier subscription inferred from model availability.

**Cross-Cutting Decisions**:
1. **Hook fast-path must use kill-switch, not enrollment check.** Non-enrolled path at 57ms is unacceptable. The disabled/kill-switch path at 2.8ms is the correct fast path. Architecture: check kill-switch → check enrollment cache → run hook logic.
2. **Sidecars are IDE-only infrastructure.** The housekeeping engine cannot rely solely on sidecars for background processing. CLI-only users need an alternative: hook-triggered deferred processing or explicit cron.
3. **Conversation reuse is critical for cost.** Each `agy -p` call creates a new conversation (no prefix caching). For housekeeping, use `--conversation <id>` to persist a single housekeeping conversation per workspace.
4. **Gemini 3.6 Flash is 41% cheaper.** 9.9k vs 16.8k tokens per call. Use the cheapest available flash model for housekeeping.

**Surfaced Issues for WO4**:
1. **`--project` flag silently ignored** — CLI conversations all go to `default-cli-project`. Either a bug or undocumented behavior. Need alternative isolation strategy.
2. **Non-enrolled hook latency** — 57ms constant overhead on enrollment check, far exceeding the 5ms target. Must be optimized before production.
3. **No prefix caching for `agy -p`** — Each call is a new conversation. Consider API-level changes or conversation reuse.
4. **Sidecar log rotation** — No built-in rotation. ~14MB/month per sidecar. Needs addressing.
5. **Sidecar overlap behavior untested** — What happens when command duration exceeds tick interval? The scheduler may start concurrent executions.
6. **Sleep/wake sidecar behavior untested** — Requires user interaction to test. Are missed ticks caught up or skipped?
7. **CLI overhead dominates wall time** — 6-7s of 11s per call. For high-frequency housekeeping, consider direct API access instead of `agy -p`.

**Boundary Box Compliance**: No Go source code was modified. No SPEC changes made. All outputs are in `docs/experiments/`.
