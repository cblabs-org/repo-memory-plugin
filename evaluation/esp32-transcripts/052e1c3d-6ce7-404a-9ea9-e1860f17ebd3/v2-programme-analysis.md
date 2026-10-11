# repo-memory v2: Programme Analysis & Upissue Plan

> **Principal Architect Session:** 2026-10-03
> **SPEC:** v0.10 → v1.0 (proposed)
> **v2 Design:** [design-v2-distillation-retrieval.md](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/design-v2-distillation-retrieval.md) rev 3

## 1. State of Play

### What's Built (PRs 01 → 05b, 6/13 complete)

| PR | Status | What it delivered |
|---|---|---|
| PR-01 | ✅ Complete | Go skeleton, CLI, interfaces, fake adapter, enrolment, kill switch, CI, probe hook |
| PR-01b | ✅ Complete | All 12 experiments resolved (E01–E12) |
| PR-02a | ✅ Complete | Transcript parser, 5.4× compaction, episode store, commit linkage, metrics |
| PR-02b | ✅ Complete | Spool, Stop hook, job table, tick, sidecar, session attribution, dogfood enrolment |
| PR-03 | ✅ Complete | FTS5 search, memory.ask fallback, PA import, eval harness, B1–B5 datasets |
| PR-04 | ✅ Complete | Entities, claims, index, events store |
| PR-05a | ✅ Complete | v1 pipeline (triage, extract, reconcile, verify), local publish, orchestration |
| PR-05b | 🟡 WO4 pending | Dogfood run done (71 episodes → 125 claims), B5 evaluation pending |

### What the v2 Design Changes

The v2 design replaces the **distillation and retrieval** approach:

| v1 (built) | v2 (proposed) |
|---|---|
| Post-hoc 4-stage pipeline: triage → extract → reconcile → verify | Turn notes (in-context, warm cache) + sidecar fallback + batched consolidation |
| One file per claim (125 atomic claims from dogfood) | Op log → rendered dossiers (~3k tokens per page) |
| Views generator (MAP.md, subsystem pages) | Atlas + dossier renderer |
| `agy -p` executor with default overhead (~17k tokens/call) | Lean Markdown agent (`excludeDefaultComponents`, no tools, ~<500 tokens overhead) |
| All delivery designed but not built | `systemMessage` persistent pushes + `ephemeralMessage` hints + deny-once gates |

### What's Kept Unchanged

All infrastructure PRs remain valid:
- Capture, spool, incremental copy, transcripts (PR-01, PR-02a, PR-02b)
- Episode store, job table, tick, sidecar (PR-02b)
- FTS5 search, eval harness, datasets (PR-03)
- Entity extraction, SQLite index (PR-04 partially)
- Events store → becomes op log (PR-04 wo7 — schema changes needed)

## 2. Impact on the v1 Programme

### PRs That Need No Changes
- **PR-01** through **PR-03**: fully complete, no rework needed
- **PR-04 wo1–wo5**: entities, entity extractor, claim schema, claim store, SQLite index — entities and the index are still used. The claim schema and store are replaced by the op log and dossiers, but the index tables are reused.

### PRs That Are Superseded
- **PR-05a** (pipeline code): triage, extract, reconcile, verify stages are all replaced. The executor concept survives (lean agent), but the pipeline stages don't.
- **PR-05b** (dogfood pilot): the B5 evaluation of v1 claims is no longer the gate. v2 has its own validation plan (X1–X11).
- **PR-06** (shared publish, views, review, brief/ask): views → dossiers; shared publish → op-log publish; brief/ask core → delivery channels. The two-lane publish model survives but the content is different.
- **PR-07** (delivery): completely redesigned — `systemMessage` persistent pushes, injection ledger, compaction re-injection, scoped error matching, first-touch/first-entry dedupe.
- **PR-08** (learning loop): partially redesigned — déjà-vu is now scoped differently; coverage → dossier gaps; elicitation stays.
- **PR-09** (PA integration): the approach changes — dossiers replace hand-written memory, and the PA can read generated summaries. The compatibility-mode concept survives.
- **PR-10** (backfill): transcript import stays, but the target is the op log and dossiers instead of claims. Re-projection is now op-log replay.

### Summary: New PR Structure

The original 13-PR plan had 6 complete (PR-01 → PR-04) + PR-05a complete but now superseded. The remaining 7 PRs (PR-05b → PR-10) are replaced by a new sequence.

## 3. Proposed v2 PR Sequence

Starting from the tip of PR-04 (the last PR whose work is fully retained):

| # | New PR | Objective | New Experiments | WOs est. |
|---|---|---|---|---|
| 1 | **PR-05-v2-spike** | Validate v2 assumptions: Stop continue, note quality, lean agent, systemMessage persistence, quota accounting | X1, X2, X3, X5, X8, X9 | 5–6 |
| 2 | **PR-06-v2-distillation** | Turn notes (Stop gate + harvest), marks, exposure lists, free distillations (artifacts + checkpoints), sidecar fallback (lean agent), note preparation/framing (§5.6), subagent registry | — | 6–7 |
| 3 | **PR-07-v2-storage** | Op log, dossier schema + renderer, atlas, config scopes + derived scopes + aliases, dossier history files, index migration (claims tables → op/entry tables), reindex | — | 5–6 |
| 4 | **PR-08-v2-consolidation** | Consolidation pipeline: echo filter, routing, duplicate merge, anchor check, LLM decide (lean agent), validate + record ops, local + shared publish, branch scoping, gardening | X4, X10 | 6–7 |
| 5 | **PR-09-v2-delivery** | Injection ledger, systemMessage/ephemeralMessage delivery, all moments (task start, first entry, first touch, about-to-edit, error, invariant gate, risky action), compaction detection + re-injection, ranking, budget enforcement | X5 (confirmed), X11 | 6–7 |
| 6 | **PR-10-v2-pilot** | Pilot run on this repo's history + esp32-app-foundation, B1 replay (X6), dossier quality evaluation, cost measurement | X6 | 4–5 |
| 7 | **PR-11-v2-learning** | Close conditions, gardening, coverage model, elicitation, session metrics, replay tuning | — | 5–6 |
| 8 | **PR-12-v2-pa-integration** | Episode summaries, workorder briefs, PA compatibility mode (cross-repo) | — | 4 |
| 9 | **PR-13-v2-backfill** | Full transcript import, doc import, git import, inferred rationale, re-projection via op-log replay | — | 5 |

**Total: 9 new PRs (46–56 WOs), replacing 7 v1 PRs (40 WOs).**

## 4. Key Architecture Decisions

| # | Decision | Rationale |
|---|---|---|
| VD1 | Archive PR-05a and PR-05b branches; start v2 stack from PR-04's tip | v1 pipeline code is superseded. Entity extraction, index and events store from PR-04 carry forward. |
| VD2 | Experiments first (PR-05-v2-spike) before any implementation | v2 relies on unverified platform behaviour (Stop continue, systemMessage persistence, lean agent overhead). Must validate before building. |
| VD3 | Op log replaces claims store; events store schema adapts | The op log is conceptually the same event-sourcing pattern from v1 (PR-04 wo7), but with different event types. |
| VD4 | Kill criterion at X6: if dossiers don't beat no-memory, stop | The entire v2 rests on whether dossier-based delivery improves agent outcomes. If not, the project pivots. |
| VD5 | Entity extractor (PR-04 wo2) carries forward unchanged | Dossier scopes are derived from entities. The extractor feeds into both v1 and v2. |
| VD6 | Lean Markdown agent replaces default `agy -p` executor | `excludeDefaultComponents` + no tools + no customizations drops the fixed overhead dramatically. Must measure in X8. |

## 5. Gate Structure

| Gate | When | Condition |
|---|---|---|
| **X-gate** (spike) | PR-05-v2-spike exit | X1 harvest ≥ 95%; X8 lean overhead measured; X5 channels confirmed; X9 exit codes documented |
| **Quality gate** (pilot) | PR-10-v2-pilot exit | X6: dossiers beat no-memory on B1 replay. **If not, stop.** |
| **Ship gate** | PR-13-v2-backfill exit | All subsystems have dossiers; cost per session measured; coverage reported |
