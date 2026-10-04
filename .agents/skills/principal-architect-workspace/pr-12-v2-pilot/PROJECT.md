# PR-12: Pilot & Confirmatory X6

> **Programme:** repo-memory
> **PR name:** pr-12-v2-pilot
> **Base branch:** `PR-11-v2-consolidation`
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Run full v2 on the dogfood repository (`repo-memory-plugin`) and enrol the pilot repository (`esp32-app-foundation`). Review generated dossiers against ground truth, run the confirmatory X6 replay experiment comparing distilled dossiers against no-memory and PA documentation at lower token cost, and produce comprehensive cost reporting against v1 and PA baselines.

## Design & SPEC Sections

- **v2 Design Document:**
  - [§12 (Validation plan: X6 B1 replay)](../../../docs/design-v2-distillation-retrieval.md#12-validation-plan): Confirmatory X6 experiment running the pre-registered protocol with time-cut dossiers produced by the full distillation pipeline instead of seeding.
  - [§9 (Cost model)](../../../docs/design-v2-distillation-retrieval.md#9-cost-model): Token consumption and cost telemetry across turn notes, fallback, consolidation, and delivery compared to v1 (~112k per episode) and PA (~5k produce + ~9k load) baselines.
- **SPEC v0.10:**
  - §12 (Evaluation & Metrics)

## Dependencies & Gate Preconditions

- **Base branch:** `PR-11-v2-consolidation`
- **Dependencies:** PR-11 consolidation and two-lane publishing functional; PR-08 delivery channels active; PR-05/PD17 captured evidence available for `esp32-app-foundation`.
- **Gate Preconditions:** Early kill gate (X6-lite from PR-09) must have passed prior to PR-10.
- **Gate (Exit):**
  - **Confirmatory X6:** distilled dossiers beat no-memory and match PA docs at fewer tokens. If not, stop or fall back to seeded or PA-sourced dossiers.

## Work Order Ledger

| # | WO name | Repository | Scope | Complexity | Status |
|---|---|---|---|---|---|
| 1 | `wo1-dogfood-run` | repo-memory-plugin | Full v2 on this repo: turn notes, fallback, consolidation. Measure cost per note, per fallback call and per consolidation batch from the telemetry. | standard | ☐ Planned |
| 2 | `wo2-pilot-enrolment` | esp32-app-foundation | Full enrolment with a reviewed scope config, as a PR in that repo. Run the fallback and seeding over captured evidence (available since the PD17 capture-only enrolment). | standard | ☐ Planned |
| 3 | `wo3-dossier-review` | repo-memory-plugin | Review dossiers from both repos against ground truth. Score precision per type, missed knowledge and stale entries. | interactive | ☐ Planned |
| 4 | `wo4-x6-confirmatory` | repo-memory-plugin | Re-run the X6 protocol with time-cut dossiers produced by the full pipeline instead of seeding. | interactive | ☐ Planned |
| 5 | `wo5-cost-report` | repo-memory-plugin | Cost per session (turn notes, fallback, consolidation, delivery), quota impact, and comparison with v1 (~112k per episode) and the PA (~5k to produce + ~9k to load). | standard | ☐ Planned |

## Exit Criteria

- **Confirmatory X6:** distilled dossiers beat no-memory and match PA docs at fewer tokens. If not, stop or fall back to seeded or PA-sourced dossiers.
- Precision ≥ 0.8 for pushed entries; cost within budget; pilot dossiers reviewed by the user.

## Human Tasks

- Review pilot dossiers.
- Review the esp32 scope proposal.
- Decide at the confirmatory gate (continue to PR-13, stop, or fall back to seeded or PA-sourced dossiers).
- Review memory PRs; answer weekly review reports (ongoing).

## Risks

- **Dossiers don't improve outcomes:** Distilled dossiers fail to demonstrate improvements over no-memory or fail to match PA documentation at lower token cost. *Mitigation:* Confirmatory X6 gate at PR-12 — if not passing, stop the project or fall back to seeded or PA-sourced dossiers. (Owner: PR-09, PR-12)
- **X6 leakage:** Dossiers contain the replayed work order's own knowledge. *Mitigation:* Time cut by parent commit for dossiers and PA docs; user review for leakage. (Owner: PR-09, PR-12)
- **X6 too small to detect real effects:** Replay runs lack statistical resolution. *Mitigation:* Pre-registered metric; ≥ 2 seeds per condition; work orders chosen for headroom. (Owner: PR-09, PR-12)
- **Conversation pruning destroys pilot inputs:** Platform pruning removes captured evidence before pilot processing. *Mitigation:* Artifact capture and `snapshot` in PR-05; PD17 capture-only enrolment of esp32-app-foundation. (Owner: PR-05, user)

## Session Log

<!-- Empty, ready for use -->
