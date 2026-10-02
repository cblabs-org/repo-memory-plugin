# PR-05a: Pipeline Code

> **Programme:** repo-memory
> **PR:** pr-05a-pipeline-code
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

The knowledge pipeline: triage, extract, reconcile and verify, plus the local publish lane. Each LLM stage is a pure function dispatched to `agy -p`. This PR merges once deterministic and recorded-executor tests pass.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-executors` | `pr-05a-pipeline-code/wo1-executors` | ☑ Complete | standard | Execution Worker |
| 2 | `wo2-triage` | `pr-05a-pipeline-code/wo2-triage` | ☑ Complete | architect | Execution Worker |
| 3 | `wo3-extract` | `pr-05a-pipeline-code/wo3-extract` | ☐ Pending | architect | — |
| 4 | `wo4-reconcile` | `pr-05a-pipeline-code/wo4-reconcile` | ☐ Pending | architect | — |
| 5 | `wo5-verify` | `pr-05a-pipeline-code/wo5-verify` | ☐ Pending | standard | — |
| 6 | `wo6-local-publish` | `pr-05a-pipeline-code/wo6-local-publish` | ☐ Pending | standard | — |
| 7 | `wo7-orchestration` | `pr-05a-pipeline-code/wo7-orchestration` | ☐ Pending | standard | — |

## Exit Criteria
- Reconciled claims land in the local lane and are queryable from the index
- Duplicate jobs are no-ops via idempotency keys
- All stage tests pass against recorded executor responses
- Decision-marker windows and P13 precondition covered by fixture tests

## Session Log

- **WO1 (Executors)**: Complete. Implemented `Housekeeper` using `agy -p` and a `RecordedHousekeeper` for testing. Excluded housekeeping runs from capture sweep.
  - **Dead end/Finding**: `agy` rejected `--output-format json` when `--input-format stream-json` was used; the worker used `--output-format stream-json` instead. The CLI also ignores some input event names in stream-json mode, which may need investigation later.
- **WO2 (Triage)**: Complete. Implemented rule-based triage, keep/skip rules, salience windows, synopsis generation, and the `Housekeeper` LLM fallback for ambiguous episodes.
