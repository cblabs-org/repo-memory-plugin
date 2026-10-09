# PR-01b: Experiment Spike

> **Programme:** repo-memory
> **PR:** pr-01b-experiment-spike
> **Base branch:** `main`
> **Status:** ✅ Complete (4/4 WOs)
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.9
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Answer experiments E1–E12 with the probe hook and a few `agy -p` runs, before any capture code is written. The deliverable is documentation: one report per experiment in `docs/experiments/`, plus a summary recording which SPEC sections each confirms or changes. No code is merged here.

## Work Order Ledger

| # | WO | Branch | Status | Complexity |
|---|---|---|---|---|
| 1 | `wo1-transcript-and-hooks` | `pr-01b-experiment-spike/wo1-transcript-and-hooks` | ✅ Complete | interactive |
| 2 | `wo2-delivery-surfaces` | `pr-01b-experiment-spike/wo1-transcript-and-hooks` | ✅ Complete | interactive |
| 3 | `wo3-runtime-and-cost` | `pr-01b-experiment-spike/wo3-runtime-and-cost` | ✅ Complete | interactive |
| 4 | [`wo4-spec-reconciliation`](wo4-spec-reconciliation/WORKORDER.md) | `pr-01b-experiment-spike/wo4-spec-reconciliation` | ✅ Complete | standard |

> **Human approval of decomposition:** Confirmed by user at project creation.

## Exit Criteria

- [x] Each experiment has a documented answer with evidence in `docs/experiments/`
- [ ] SPEC updated (after approval) for any changed assumptions → **Proposed diffs in [`SPEC-CHANGES.md`](../../../docs/experiments/SPEC-CHANGES.md); user to apply**
- [x] E5 and E11 have preliminary figures, to be re-confirmed in PR-07 and PR-05b

## Session Log

### 2026-10-01T21:19+01:00 — WO1 Complete: Transcript & Hooks (E12, E3, E1)
**Summary**: Transcript is append-only with zero-lag hook reads; `ephemeralMessage` works but is per-invocation only and not a viable push channel.
**Findings**:
- **E12:** Append-only in normal flow. Step-index gaps from internal dispatch steps. Out-of-order writes from parallel tool calls. Session resets within same file.
- **E03:** Zero lag at both `PreInvocation` and `PreToolUse`. Current turn's `thinking` flushed before `PreToolUse` fires. `transcriptPath` → `transcript_full.jsonl`.
- **E01:** `ephemeralMessage` per-invocation transient. `invocationNum` resets per user prompt. Malformed responses silently ignored. Architect brief: `ephemeralMessage` is unacceptable (4–20k wasted tokens/prompt).
**Cross-cutting decisions**: `transcriptPath` always points to `transcript_full.jsonl`; `invocationNum` resets per prompt (not session-scoped); `injectSteps` is the correct format.
**Dead ends**: Wrong hook response formats (`ephemeral_message` vs `injectSteps`) silently ignored — silent failure is the dominant failure mode.

### 2026-10-01T22:34+01:00 — WO2 Complete: Delivery Surfaces (E2, E4, E9, E8)
**Summary**: Include resolution works repo-root-relative; `model_decision` rules are pull-based; deny+reason is isolated per call; subagents fire ALL hooks with own identity.
**Findings**:
- **E02:** `@[label](path)` resolves relative to repo root, inlined into `user_rules`. 24KB/file, 20k-token aggregate. Refresh per-session only.
- **E04:** `trigger: model_decision` pull-based. `activation:` key silently ignored.
- **E09:** `deny`+`reason` delivered verbatim; parallel denials isolated; advisory → 3 retries, policy → 2. `overwrite` rewrites args non-destructively.
- **E08:** Subagents fire ALL hooks with own `conversationId`/`transcriptPath`. Plugin can inject context into workers directly.
**Cross-cutting decisions**: Includes confirmed as the primary always-on channel; native-floor rules cost a tool call per rule; `overwrite` is a candidate for non-blocking advisory; subagent hook coverage enables per-worker delivery.

### 2026-10-01T22:56+01:00 — WO3 Complete: Runtime & Cost (E6, E5, E7, E10, E11)
**Summary**: Hook latency is fast for enrolled repos (~3ms p95) but non-enrolled path is slow (~57ms). Sidecars are IDE-only. `--project` flag silently ignored. ~17k token floor with no prefix caching. Zero rate limiting observed.
**Findings**:
- **E05:** Enrolled: p50=3ms, p95=6ms. Non-enrolled: p50=57ms ❌. Kill-switch: 2.8ms.
- **E06:** Sidecars are IDE-managed child processes. Perfect 60s intervals. CLI does not run sidecars.
- **E07:** `--project` silently ignored. CLI/IDE have separate DBs. Isolation via run dir + env var.
- **E10:** Fixed overhead: ~16.8k tokens (Flash 3.8), ~9.9k (Flash 3.6, 41% cheaper). No prefix caching.
- **E11:** 25+ calls in 15min, zero throttling. Pro-tier subscription.
**Cross-cutting decisions**: Kill-switch first in hook fast-path; sidecars IDE-only → CLI needs cron backstop; conversation reuse critical for cost; use cheapest flash model.

### 2026-10-01T23:08+01:00 — WO4 Complete: Spec Reconciliation
**Summary**: All 12 experiments synthesized into `SUMMARY.md` and 16 proposed SPEC edits in `SPEC-CHANGES.md`. Zero contradictions across experiments. 3 confirmed, 9 changed/new-info.
**Cross-cutting decisions**: Silent failure is the platform's dominant behavior pattern — every configuration surface (hooks, rules, includes, CLI flags) fails silently on invalid input. SPEC must document exact schemas and validation requirements.
**Surfaced issues**:
1. Follow-up E1b needed: does `userMessage` in `injectSteps` persist across invocations?
2. Non-enrolled hook latency (~57ms) needs O(1) directory marker optimization
3. CLI-only users need `repo-memory schedule install` for cron/launchd backstop
4. Sidecar sleep/wake and overlap behavior remain untested

### 2026-10-01T23:08+01:00 — Project Wrap-Up
**Architectural outcome**: The experiment spike answered all 12 P0 experiments before any capture code was written. The SPEC's core assumptions held (append-only transcripts, zero-lag hooks, subagent parity, viable cost model), but several mechanisms needed correction: `ephemeralMessage` is not a viable push channel for bulk context (per-invocation only), `--project` isolation doesn't work, non-enrolled hook latency needs optimization, and sidecars are IDE-only. 16 concrete SPEC edits are proposed and ready for the user to apply before PR-02a starts.
**Lessons for future projects**:
- Silent failure is Antigravity's default behavior. Always validate configurations before deploying. Build canary tests.
- Interactive WOs (WO1–WO3) are efficient for empirical experiments — the user drives the session while the PA stays strategic.
- The standard WO (WO4) spawned on `flash` model performed the synthesis work effectively at lower cost.
**Unresolved items**:
- E12 branch/edit UI behavior not directly tested (inferred from session resets)
- E06 sleep/wake and command overlap untested (need physical interaction)
- E1b (`userMessage` persistence) — follow-up experiment for PR-07
- User must review and apply `SPEC-CHANGES.md` diffs to `SPEC.md` before PR-02a starts
