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
| 1 | `wo1-executors` | `pr-05a-pipeline-code/wo1-executors` | ☐ Pending | standard | — |
| 2 | `wo2-triage` | `pr-05a-pipeline-code/wo2-triage` | ☐ Pending | architect | — |
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
