# PR-06-v2-spike: Prototype Spike

> **Programme:** repo-memory
> **PR:** pr-06-v2-spike
> **Base branch:** `pr-05-v2-foundation`
> **Status:** 🔄 **In Review** — GitHub PR #6. All 5 WOs complete. X-gate criteria met. Pending user review of SPEC edits.
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Validate the platform mechanics v2 relies on.
- **Prototype code** (the `Stop` gate and harvest, `systemMessage` injection, the lean agent) lives on a spike branch behind a feature flag (`REPO_MEMORY_V2_SPIKE=1`), installed locally, with a documented rollback.
- **What merges:** only the experiment reports in `docs/experiments/` and the proposed SPEC edits. Production versions are built in PR-07 onward.

## Design & SPEC Sections

- **v2 Design rev 3:** §2.3 (Mechanics), §5.1 (`Stop` gate and harvest), §5.4 (Prompt design and cost), §8.2–8.4 (Delivery channels and moments), §9 (Cost, quota and gen_metadata), §12 (Validation experiments X1, X3, X5, X8, X9)
- **SPEC:** Proposed edits leading toward SPEC v1.0

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-stop-continue](wo1-stop-continue/WORKORDER.md) | **X1.** Stop gate prototype, harvest, PA/worker interaction | interactive | ✅ Complete |
| 2 | [wo2-hook-contracts](wo2-hook-contracts/WORKORDER.md) | **X9.** Hook exit codes, fail-open/closed per type | standard | ✅ Complete |
| 3 | [wo3-channels](wo3-channels/WORKORDER.md) | **X5.** `injectSteps` with `systemMessage`/`userMessage`/`ephemeralMessage` | interactive | ✅ Complete (corrected ×3) |
| 4 | [wo4-lean-agent](wo4-lean-agent/WORKORDER.md) | **X8.** Lean agent discovery, frontmatter validation | interactive | ✅ Complete (corrected) |
| 5 | [wo5-quota-and-spec](wo5-quota-and-spec/WORKORDER.md) | **X3.** Quota API, spike summary, SPEC edits | standard | ✅ Complete |

## Exit Criteria (X-Gate)

| Criterion | Status | Evidence |
|---|---|---|
| X1: harvest ≥ 95% over ≥ 20 requests | ✅ **Met** | 21/21, 100%. Gate: `terminationReason=NO_TOOL_CALL`, `fullyIdle`, `executionNum=0` |
| X9: behaviour documented per hook type | ✅ **Met** | PreToolUse fail-closed; others fail-open. Contract tests updated. |
| X5: channels confirmed or delivery revised | ✅ **Met** | `injectSteps` works from plugin hooks for top-level sessions. No split placement needed. |
| X8: agent discoverable, overhead measured | ✅ **Met** | Valid frontmatter required. File-based agents work with `agy --agent` and `invoke_subagent`. |
| X3: cost of cached tokens understood | ✅ **Met** | `/usage` API provides `remaining_fraction` + `reset_time`. |
| SPEC v1.0 edits proposed and approved | ⏳ **Pending** | Proposed in `docs/experiments/spike-summary.md`. Awaiting user review. |

## Session Log

### 2026-10-05 — WO1 (X1 Stop-continue), WO2 (X9 Hook contracts)
- **X1 result:** 21/21 harvest success rate. Gate conditions validated. Stop continuation cap is 10 (v1.1.9). PA and worker sessions not disrupted — opt-out by role defined.
- **X9 result:** PreToolUse is fail-closed (blocks tool call on exit 1, exit 2, empty stdout). All other hooks (SessionStart, PreInvocation, PostInvocation, PostToolUse, Stop) fail-open.

### 2026-10-05 — WO3 (X5 Channels) — corrected ×3
- **Initial result:** Concluded `systemMessage`/`userMessage` not supported → WRONG. Used bare string instead of nested object, missing `injectSteps` wrapper.
- **Correction 1:** Correct JSON format: `{"injectSteps":[{"systemMessage":{"systemMessage":"...","metadata":{...}}}]}`. All three message types work.
- **Correction 2:** Plugin hooks DO support `injectSteps` for top-level sessions. Initial CLI test missed `--add-dir .`, preventing plugin discovery.
- **Correction 3:** `invoke_subagent` injection is inconclusive, not proven broken. Idle message captured "BANANA-99" (suggesting injection worked), but model later said "no secret code" (likely flash_lite confusion from repeated injections). Transcript analysis unreliable — `SYSTEM_MESSAGE` steps don't appear in transcripts even for confirmed-working injections.

### 2026-10-05 — WO4 (X8 Lean agent) — corrected
- **Initial result:** Agent not discovered → WRONG. Invalid frontmatter silently rejected.
- **Correction:** `name` + `description` REQUIRED (silent drop). `model:` only accepts tier names. Invalid keys cause silent rejection. File-based agents and `invoke_subagent` are NOT separate systems.

### 2026-10-06 — WO5 (X3 Quota, spike summary, SPEC edits)
- **X3 result:** `/usage` API works. `remaining_fraction` + `reset_time` available.
- Spike summary written with all corrections.
- SPEC v1.0 edits proposed.

### 2026-10-06 — PR-06 close-out
- All experiment docs committed with 3 rounds of corrections.
- `docs/antigravity-hooks-reference.md` created with corrected discovery paths.
- GitHub PR #6 staged for review.

### Key Platform Knowledge Captured
1. `injectSteps` works from `<plugin>/hooks.json` for top-level sessions — no split hook placement needed.
2. PreInvocation fires BEFORE subagent prompt delivery (`initialNumSteps=0`, `userPrompt=""`). This is by design.
3. Agent frontmatter traps: silent rejection without `name`+`description`, invalid `model:` values, invalid keys.
4. Hooks discovery: only `<plugin-root>/hooks.json` — `hooks/hooks.json` is Claude Code's convention.
5. Transcript analysis unreliable for verifying `injectSteps` injection.
6. Stop continuation cap: 10 (v1.1.9).

### Decisions Made
- VD8 updated: persistent `systemMessage` confirmed via `injectSteps` from plugin hooks.
- VD6 updated: lean agent works with correct frontmatter; `define_subagent` only for runtime dynamic agents.
- NEW: No split hook placement needed (corrects original spike conclusion).
- NEW: `invoke_subagent` injection inconclusive but acceptable (subagents inherit parent context).
