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

### WO1 — Transcript & Hooks (E12, E3, E1)
- **E12:** Transcript is append-only during normal flow. Step-index gaps caused by internal dispatch steps (never serialized). Out-of-order writes from parallel tool calls. Session resets within the same file.
- **E03:** Zero lag at both `PreInvocation` and `PreToolUse`. Current turn's `thinking` is flushed before `PreToolUse` fires. `transcriptPath` always points to `transcript_full.jsonl`.
- **E01:** `ephemeralMessage` is per-invocation transient — not carried to subsequent invocations. `invocationNum` resets per user prompt, not per session. Malformed responses silently ignored. **Architect brief:** task brief via `ephemeralMessage` is unacceptable (4–20k wasted tokens/prompt); need E1b to test `userMessage` persistence, or fall back to pull-only architecture.

### WO2 — Delivery Surfaces (E2, E4, E9, E8)
- **E02:** `@[label](path)` includes resolve relative to repo root, inline into `user_rules`. 24KB/file cap, 20k-token aggregate budget. Refresh per-session only.
- **E04:** `trigger: model_decision` rules are pull-based — description shown as pointer, model decides to `view_file`. `activation:` key silently ignored; correct key is `trigger:`.
- **E09:** `deny`+`reason` delivers reason verbatim; parallel denials isolated; advisory wording → 3 retries, policy → 2. `overwrite` rewrites tool args non-destructively with notice.
- **E08:** Subagents fire ALL hooks with own `conversationId` and `transcriptPath`. Plugin can inject context into workers directly via hooks.
- **Cross-cutting:** Silent failure is the default. `{}` for PreToolUse = implicit deny. Hook command paths resolve relative to `hooks.json` directory. `hooks.json` is hot-reloaded mid-session.

