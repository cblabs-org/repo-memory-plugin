# PR-05b: Dogfood Pilot & Pipeline Evaluation

> **Programme:** repo-memory
> **PR:** pr-05b-dogfood-pilot
> **Base branch:** `main`
> **Status:** ✅ Complete
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Run the knowledge pipeline end-to-end on this repo's own history (71 PA-imported episodes). Produce real claims in `.memory/claims/`, generate views (MAP.md, subsystem pages), and evaluate extraction quality against the PA's structured records as ground truth. This validates the entire capture→pipeline→delivery chain before enrolling external repos.

## Adaptation from Programme Plan

The original PR-05b targeted esp32-app-foundation. This dogfood run comes first:
- **Dogfood (this PR):** Run pipeline on repo-memory-plugin's own PA history
- **Pilot (follow-up):** Enrol esp32-app-foundation after validating pipeline quality here

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | [`wo1-subsystem-bootstrap`](wo1-subsystem-bootstrap/WORKORDER.md) | `pr-05b-dogfood-pilot/wo1-subsystem-bootstrap` | ☑ Complete | standard | — |
| 2 | [`wo2-pipeline-run`](wo2-pipeline-run/WORKORDER.md) | `pr-05b-dogfood-pilot/wo2-pipeline-run` | ☑ Complete | architect | — |
| 3 | [`wo3-views-generation`](wo3-views-generation/WORKORDER.md) | `pr-05b-dogfood-pilot/wo3-views-generation` | ☑ Complete | standard | — |
| 4 | [`wo4-b5-evaluation`](wo4-b5-evaluation/WORKORDER.md) | `pr-05b-dogfood-pilot/wo4-b5-evaluation` | ☑ Superseded | interactive | — |

## Human Approval

> **Approved:** yes (via message: # PR-05b: Dogfood Pilot & Pipeline Evaluation)

## Exit Criteria
- ✅ `.memory/claims/` populated with extracted claims from pipeline run (125 claims from 71 episodes)
- ✅ `.memory/views/MAP.md` generated and readable
- ✅ Pipeline cost per episode and per stage recorded (~112k input tokens per episode)
- ☑ B5 precision: **WO4 superseded** by the comprehensive evaluation in [`docs/dogfood-evaluation-01.md`](../../../docs/dogfood-evaluation-01.md) and its Appendix C review
- ✅ User has reviewed claim quality (led to v2 pivot)

## Close-out

**Completed:** 2026-10-04.

WO4 (B5 evaluation) was not executed as a standalone work order. Instead, the evaluation was performed interactively and documented in:
- [`docs/dogfood-evaluation-01.md`](../../../docs/dogfood-evaluation-01.md) — comprehensive solution evaluation with corrected PA costs
- Appendix C — review of the evaluation, including the research survey that informed the v2 design

The evaluation found the v1 pipeline's claims to be accurate but shallow (~112k input tokens per episode), which led directly to the v2 design pivot documented in [`docs/design-v2-distillation-retrieval.md`](../../../docs/design-v2-distillation-retrieval.md).

**Branch:** `pr-05b-dogfood-pilot/wo4-b5-evaluation` holds the executor fixes (`4163dd5`, `960d852`) and all v2 documents. This branch is merged to `main` by PR-05-v2-foundation wo1.

## Session Log
- 2026-10-02 16:55 — Project created. Dogfood-first approach approved by user.
- 2026-10-02 17:02 — WO1 completed. Subsystem assignments fixed to ensure deterministic mapping and `**` globs support in config parsing. Out-of-scope test failures in `TestSearchCommand` and `TestSessionAttributor_Integration` noted for future triage.
- 2026-10-02 18:01 — WO2 completed. Housekeeper wired into ticker. Pipeline ran on 71 episodes. Applied fixes to `executor.go` (piping input via stdin to avoid ARG_MAX issues), `pipeline.go` (session ID resolution), and `extractor.go` (JSON Schema enforcement). Noted out-of-scope issue: pipeline LLM costs are not being written to DB.
- 2026-10-02 18:05 — WO3 completed. Views generator added and `repo-memory views` subcommand wired up. Produced `MAP.md`, `SIGNATURES.md`, etc. from `.memory/claims/`. Noted a potential gitignore issue (`repo-memory` masking `cmd/repo-memory`).
- 2026-10-04 12:17 — Project closed. WO4 superseded by `docs/dogfood-evaluation-01.md` and Appendix C. The evaluation's findings drove the v2 design pivot. Branch merged to `main` by PR-05-v2-foundation wo1.
