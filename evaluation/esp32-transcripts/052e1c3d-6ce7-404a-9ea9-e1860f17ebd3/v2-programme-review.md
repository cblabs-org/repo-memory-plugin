# repo-memory v2: Programme Upissue — Review Package

> **Chief Architect Session:** 2026-10-04
> **Documents Updated:**
> - [`PROGRAM.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/programme-repo-memory/PROGRAM.md) — programme overview, PR sequence, gates, session log
> - [`PROGRAM-PLAN.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/programme-repo-memory/PROGRAM-PLAN.md) — detailed plan with all WOs, exit criteria, experiments
> - [`MEMORY.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/memory/MEMORY.md) — PA boot registry

---

## Summary of Changes

### What happened
The v1 post-hoc pipeline (triage → extract → reconcile → verify) produced accurate but shallow claims at ~112k input tokens per episode. The [v2 design](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/design-v2-distillation-retrieval.md) fundamentally changes the distillation and retrieval approach.

### Programme structure: v1 → v2

```
v1 (13 PRs)                          v2 (17 PRs)
────────────                          ────────────
PR-01  Go Skeleton          ✅        (unchanged)
PR-01b Experiments           ✅        (unchanged)
PR-02a Transcript→Episode    ✅        (unchanged)
PR-02b Capture Runtime       ✅        (unchanged)
PR-03  Episode Search        ✅        (unchanged)
PR-04  Entities/Claims/Index ✅        (unchanged)
PR-05a v1 Pipeline           ✅ ──→    ARCHIVED (superseded)
PR-05b v1 Dogfood            ✅ ──→    ARCHIVED (informed v2)
                                       ┌─────────────────────┐
PR-06  Publish/Review/Brief  ──→       │ PR-05-v2  Spike     │
PR-07  Delivery              ──→       │ PR-06-v2  Distil    │
PR-08  Learning Loop         ──→       │ PR-07-v2  Storage   │
PR-09  PA Integration        ──→       │ PR-08-v2  Consolidate│
PR-10  Backfill              ──→       │ PR-09-v2  Delivery  │
                                       │ PR-10-v2  Pilot     │
                                       │ PR-11-v2  Learning  │
                                       │ PR-12-v2  PA Integ  │
                                       │ PR-13-v2  Backfill  │
                                       └─────────────────────┘
```

### Key numbers

| Metric | v1 | v2 |
|---|---|---|
| Total PRs | 13 | 17 (8 complete + 9 new) |
| New WOs to write | ~40 (PRs 06–10) | ~50 (PRs 05-v2 through 13-v2) |
| Pipeline LLM calls/episode | ~7 (112k tokens) | ~0.2 (turn note: warm cache; fallback: lean agent) |
| Storage | Per-file claims | Dossier pages + op log |
| Delivery | Designed but unbuilt | `systemMessage` persistent + `ephemeralMessage` hints |

### Gate structure

| Gate | PR | Condition | If it fails |
|---|---|---|---|
| **X-gate** | PR-05-v2-spike exit | Platform assumptions validated (Stop continue, lean agent, systemMessage, exit codes) | Revise v2 design |
| **Quality gate (X6)** | PR-10-v2-pilot exit | B1 replay: dossiers beat no-memory | **STOP the project** |
| **Ship gate** | PR-13-v2-backfill exit | All subsystems covered; cost measured | — |

### v2 PR Summary (9 new PRs)

| PR | WOs | Objective |
|---|---|---|
| **PR-05-v2-spike** | 5 | Validate: Stop continue (X1), note quality (X2), quota (X3), channels (X5), lean agent (X8), exit codes (X9) |
| **PR-06-v2-distillation** | 7 | Turn notes (Stop gate + harvest), marks, free distillations, fallback, preparation layer, subagent registry |
| **PR-07-v2-storage** | 6 | Op log, dossiers, atlas, scopes/aliases, index migration, v1 archive |
| **PR-08-v2-consolidation** | 7 | Echo filter, routing, dedup, anchor check, LLM ops (lean agent), validate, publish |
| **PR-09-v2-delivery** | 7 | Injection ledger, systemMessage/ephemeralMessage, all moments, compaction re-injection, MCP tools |
| **PR-10-v2-pilot** | 5 | Dogfood + esp32 pilot, dossier review, **B1 replay (X6 kill criterion)**, cost report |
| **PR-11-v2-learning** | 6 | Close conditions, gardening, coverage, elicitation, metrics, replay tuning |
| **PR-12-v2-pa-integration** | 4 | Episode summaries, WO briefs, PA compatibility mode (cross-repo) |
| **PR-13-v2-backfill** | 5 | Full import, doc/git import, inferred rationale, re-projection |

---

## Decisions Requiring Approval

> [!IMPORTANT]
> These are the key decisions embedded in the v2 plan. Please confirm or revise.

1. **VD1: Archive PR-05a/05b, branch v2 from PR-04's tip.** The v1 pipeline code stays in-tree but deprecated until v2 is validated. Entity extraction, index, and events store carry forward.

2. **VD2: Experiments first (PR-05-v2-spike).** The v2 design relies on platform behaviour not yet tested (`Stop` continue, `systemMessage` persistence, lean agent overhead). We validate before building.

3. **VD4: Kill criterion at X6.** If dossiers don't beat no-memory on the B1 replay, the project stops. Is this strict enough? The v2 design asks the same question (§14 Q7).

4. **VD7: Turn notes via `Stop` continue.** The note appears at the end of milestone turns. Is the visibility acceptable?

5. **VD8: `systemMessage` for persistent pushes.** Relies on X5 confirming that `systemMessage` persists across invocations and survives until compaction. If it doesn't, the delivery design needs revision.

6. **VD9: Op log as source of truth.** Dossiers are rendered views; ops are the durable record. Hand edits to dossiers are converted into ops. Is this workable for reviewing memory PRs?

---

## Next Steps

Once approved:
1. I archive PR-05a and PR-05b branches (`git branch -m ... archive/...`)
2. I create the PR-05-v2-spike project boundary and work orders
3. PR-05-v2-spike is interactive (X1–X3, X5, X8, X9 are all experiments you run), so it gets bridge prompts for each WO
4. After the X-gate passes, I spawn PAs for the implementation PRs (PR-06 through PR-13)
