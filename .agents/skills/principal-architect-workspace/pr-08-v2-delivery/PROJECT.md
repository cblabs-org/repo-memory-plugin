# PR-08: Channels, Moments & Injection Ledger (v2 Delivery)

> **Programme:** repo-memory
> **PR:** pr-08-v2-delivery
> **Base branch:** PR-07-v2-storage
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10 (P0, §7.5)
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Deliver dossier entries precisely and keep them in context:
- persistent `systemMessage` for knowledge that lasts;
- `ephemeralMessage` for hints at the moment of need;
- deny-once gates;
- the injection ledger, with compaction and rewind re-injection;
- the pull tools.

## Design & SPEC Sections

- **v2 Design Sections:** §8.1–8.6
  - §8.1 Delivery Channels (`systemMessage`, `ephemeralMessage`, deny-once gates)
  - §8.2 Moments (Task start, First entry, First touch, About to edit, Command error)
  - §8.3 Ranking, deduplication, and anchor checking
  - §8.4 Compaction detection & rewind re-injection (X11)
  - §8.5 MCP Server Pull Tools
  - §8.6 Delivery budgets and token enforcement
- **SPEC Sections:** §7.5 (Risky-action gates carried forward), P0
- **File & Package Paths:**
  - `internal/core/delivery/` (Ledger, delivery engine, moment triggers, compaction re-injection, exposure recording)
  - `internal/mcpserver/` (MCP pull tools: `memory.brief`, `memory.why`, `memory.history`, `memory.search`, `memory.note`, `memory.stats`)

## Dependencies & Gate Preconditions

- **Base branch:** `PR-07-v2-storage`
- **Preconditions:**
  - PR-07-v2-storage complete (op log, rendered dossiers, rebuildable index, seeded entries, v1 pipeline removed).
  - Platform mechanics confirmed by PR-06-v2-spike: X5 delivery channels (`systemMessage`/`userMessage` persistence) and X9 hook exit codes contract.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-ledger-engine` | `internal/core/delivery/`. **Ledger:** entry IDs, channel, step index, content hash and tags (`[repo-memory <id>]`), stored with the conversation's mark state. **Engine:** a fast anchor check (failures marked `suspect` and not pushed); ranking (exact anchor > directory; invariant > failure-signature > ruled-out > rest; verified > unverified; most evidence); budget enforcement per channel. The exposure list (entries pushed or pulled) is recorded here, so the echo filter has data from day one. | architect | ☐ Planned |
| 2 | `wo2-persistent-pushes` | `PreInvocation` as `systemMessage`, or as `userMessage` if X5 says so. **Task start** (`initialNumSteps <= 1`): relevant dossiers from prompt terms and branch diff paths, ≤ 500 tokens. **First entry** into a dossier: module-wide invariants, ≤ 150 tokens, once per dossier per conversation. **First touch** of an anchor: file- or symbol-specific entries, ≤ 150 tokens, once per anchor per conversation. Branch diff paths are precomputed in the tick, not computed in the hook. A one-line correction is posted when the agent's own edit breaks an injected entry's anchor. | architect | ☐ Planned |
| 3 | `wo3-ephemeral-and-errors` | **About-to-edit pointer**, ≤ 50 tokens. **Error matching:** on `PostToolUse` for a failed command, read the output from the transcript's `GENERIC` step at `stepIdx`. A match needs the error pattern **and** either the command prefix or an active dossier scope. Deliver the fix as `ephemeralMessage`, ≤ 200 tokens. | standard | ☐ Planned |
| 4 | `wo4-gates` | `PreToolUse` deny-once for entries marked `gate: edit` ("[REPO-MEMORY `<id>`] why / do. Re-issue the edit after checking this."), acknowledged per conversation and entry. `force_ask` for invariants that need a human. Risky-action gates (SPEC §7.5) carried forward. Follows the X9 contract. | standard | ☐ Planned |
| 5 | `wo5-compaction-reinjection` | On every `PreInvocation`, read the transcript from the last offset. Detect compaction (a `CHECKPOINT` after an injection's step) and rewind (transcript shorter, or the step's timestamp changed). Re-inject one consolidated `systemMessage` of still-relevant entries, ≤ 1k tokens; skip tags that survive in the checkpoint text. Re-arm first-entry and first-touch dedupe. **X11.** | architect | ☐ Planned |
| 6 | `wo6-pull-tools` | `internal/mcpserver/`. MCP server tools: `memory.brief` (dossier-based, resolving aliases), `memory.why` (entry with short evidence spans), `memory.history`, `memory.search` (FTS over entries), `memory.note` (to the inbox), `memory.stats`. Pulls are recorded in the exposure list. | standard | ☐ Planned |

## Exit Criteria

- X5 confirmed in production code; **X11:** detection on ≥ 95% of checkpoints, with re-injected entries present afterwards.
- All moments deliver correctly, budgets are respected, ranking is correct, and suspect entries are not pushed.
- Gates deny once, then allow the retry.
- **Hook latency (PD24):** with all delivery features on, `PreInvocation`/`PreToolUse`/`PostToolUse` p95 ≤ 60 ms when nothing is injected and ≤ 150 ms when injecting, measured on the dogfood repo.
- All MCP tools work against dossiers; exposure lists are populated from pushes and pulls.

## Human Tasks

- None specific to this PR.

## Risks

- **Hook latency exceeded:** Delivery adds work to every `PreInvocation`. Mitigation: Budgets set as explicit exit criteria (PD24: p95 ≤ 60 ms non-injecting, ≤ 150 ms injecting); precompute branch diff paths in the tick rather than hook; non-enrolled fast path (≤ 5 ms).
- **Injections lost to compaction or rewind:** Compaction or context pruning can erase previously injected guidance. Mitigation: Injection ledger tracking and re-injection engine (wo5 / X11) re-injecting up to 1k tokens of still-relevant entries after checkpoints/rewinds.
- **Echoes come back as confirmation:** Pushed entries repeated by the model might be misinterpreted as newly discovered knowledge. Mitigation: Exposure list recorded from pushes and pulls in wo1/wo6 for consumption by the downstream echo filter (PR-11).
- **Antigravity turns on Knowledge Items:** Platform could introduce conflicting delivery channels. Mitigation: Detect platform Knowledge Items; publish into them or switch off overlapping delivery.

## Session Log

*(empty, ready for use)*
