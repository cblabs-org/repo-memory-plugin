# PR-01b: Experiment Spike

> **Programme:** repo-memory
> **PR:** pr-01b-experiment-spike
> **Base branch:** `main`
> **Status:** In Progress (WO3 next)
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.9
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Answer experiments E1–E12 with the probe hook and a few `agy -p` runs, before any capture code is written. The deliverable is documentation: one report per experiment in `docs/experiments/`, plus a summary recording which SPEC sections each confirms or changes. No code is merged here.

## Work Order Ledger

| # | WO | Branch | Status | Complexity |
|---|---|---|---|---|
| 1 | `wo1-transcript-and-hooks` | `pr-01b-experiment-spike/wo1-transcript-and-hooks` | ☑ Complete | interactive |
| 2 | `wo2-delivery-surfaces` | `pr-01b-experiment-spike/wo2-delivery-surfaces` | ☑ Complete | interactive |
| 3 | `wo3-runtime-and-cost` | `pr-01b-experiment-spike/wo3-runtime-and-cost` | ☐ Pending | interactive |
| 4 | `wo4-spec-reconciliation` | `pr-01b-experiment-spike/wo4-spec-reconciliation` | ☐ Pending | standard |

## Exit Criteria
- Each experiment has a documented answer with evidence in `docs/experiments/`
- SPEC updated (after approval) for any changed assumptions
- E5 and E11 have preliminary figures, to be re-confirmed in PR-07 and PR-05b

## Session Log

### 2026-10-01T21:19+01:00 — WO1 Complete: Transcript & Hooks (E12, E3, E1)
**Summary**: Transcript is append-only with zero-lag hook reads; `ephemeralMessage` works but is per-invocation only and not a viable push channel.
**Findings**:
- **E12:** Append-only in normal flow. Step-index gaps from internal dispatch steps. Out-of-order writes from parallel tool calls. Session resets within same file.
- **E03:** Zero lag at both `PreInvocation` and `PreToolUse`. Current turn's `thinking` flushed before `PreToolUse` fires. `transcriptPath` → `transcript_full.jsonl`.
- **E01:** `ephemeralMessage` per-invocation transient. `invocationNum` resets per user prompt. Malformed responses silently ignored. Architect brief: `ephemeralMessage` is unacceptable (4–20k wasted tokens/prompt).
**Cross-cutting decisions**: `transcriptPath` always points to `transcript_full.jsonl`; `invocationNum` resets per prompt (not session-scoped); `injectSteps` is the correct format.
**Surfaced for WO4**: SPEC gaps on `invocationNum` semantics, hook response schema, `EPHEMERAL_MESSAGE` step type, step-index non-contiguity, out-of-order writes. E12 branch/edit behavior untested.
**Dead ends**: Wrong hook response formats (`ephemeral_message` vs `injectSteps`) silently ignored — silent failure is the dominant failure mode.

### 2026-10-01T22:34+01:00 — WO2 Complete: Delivery Surfaces (E2, E4, E9, E8)
**Summary**: Include resolution works repo-root-relative; `model_decision` rules are pull-based; deny+reason is isolated per call; subagents fire ALL hooks with own identity.
**Findings**:
- **E02:** `@[label](path)` resolves relative to repo root, inlined into `user_rules`. 24KB/file, 20k-token aggregate. Refresh per-session only.
- **E04:** `trigger: model_decision` pull-based. `activation:` key silently ignored.
- **E09:** `deny`+`reason` delivered verbatim; parallel denials isolated; advisory → 3 retries, policy → 2. `overwrite` rewrites args non-destructively.
- **E08:** Subagents fire ALL hooks with own `conversationId`/`transcriptPath`. Plugin can inject context into workers directly.
**Cross-cutting decisions**: Includes confirmed as the primary always-on channel; native-floor rules cost a tool call per rule; `overwrite` is a candidate for non-blocking advisory; subagent hook coverage enables per-worker delivery.
**Surfaced for WO4**: `{}` = implicit deny (undocumented); hook `command` path resolves relative to `hooks.json` dir; `hooks.json` hot-reloaded mid-session; hook exit non-zero = implicit deny; design questions on `overwrite` for déjà-vu and `send_message` gating; `PostInvocation` hook existence untested.

