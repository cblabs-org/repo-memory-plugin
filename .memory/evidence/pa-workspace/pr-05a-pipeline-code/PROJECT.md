# PR-05a: Pipeline Code

> **Programme:** repo-memory
> **PR:** pr-05a-pipeline-code
> **Base branch:** `main`
> **Status:** Completed
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

The knowledge pipeline: triage, extract, reconcile and verify, plus the local publish lane. Each LLM stage is a pure function dispatched to `agy -p`. This PR merges once deterministic and recorded-executor tests pass.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-executors` | `pr-05a-pipeline-code/wo1-executors` | ☑ Complete | standard | Execution Worker |
| 2 | `wo2-triage` | `pr-05a-pipeline-code/wo2-triage` | ☑ Complete | architect | Execution Worker |
| 3 | `wo3-extract` | `pr-05a-pipeline-code/wo3-extract` | ☑ Complete | architect | Execution Worker |
| 4 | `wo4-reconcile` | `pr-05a-pipeline-code/wo4-reconcile` | ☑ Complete | architect | Execution Worker |
| 5 | `wo5-verify` | `pr-05a-pipeline-code/wo5-verify` | ☑ Complete | standard | Execution Worker |
| 6 | `wo6-local-publish` | `pr-05a-pipeline-code/wo6-local-publish` | ☑ Complete | standard | Execution Worker |
| 7 | `wo7-orchestration` | `pr-05a-pipeline-code/wo7-orchestration` | ☑ Complete | standard | Execution Worker |

## Exit Criteria
- Reconciled claims land in the local lane and are queryable from the index
- Duplicate jobs are no-ops via idempotency keys
- All stage tests pass against recorded executor responses
- Decision-marker windows and P13 precondition covered by fixture tests

## Session Log

- **WO1 (Executors)**: Complete. Implemented `Housekeeper` using `agy -p` and a `RecordedHousekeeper` for testing. Excluded housekeeping runs from capture sweep.
  - **Dead end/Finding**: `agy` rejected `--output-format json` when `--input-format stream-json` was used; the worker used `--output-format stream-json` instead. The CLI also ignores some input event names in stream-json mode, which may need investigation later.
- **WO2 (Triage)**: Complete. Implemented rule-based triage, keep/skip rules, salience windows, synopsis generation, and the `Housekeeper` LLM fallback for ambiguous episodes.
- **WO3 (Extract)**: Complete. Implemented `packer.go` grouping by session/subagent and capping to 60k tokens. Implemented `extractor.go` to call the Housekeeper and process Stage 3, parsing into structured candidates/gaps with a max of 20 per episode. Enforced P13 evidence independence locally in code.
- **WO4 (Reconcile)**: Complete. Implemented Stage 4 (`reconciler.go`) as a single writer coordinating 7 operations and anchor lookups (≤30). Strictly enforced the P13 precondition (checking anchored outcomes) for support operations.
- **WO5 (Verify)**: Complete. Implemented Stage 5 (`verifier.go`) to resolve anchors, compute fingerprints, verify claim behavior on code snippets via Housekeeper, and apply branch scoping.
  - **Dead end/Finding**: The worker duplicated `git` branch queries via `exec.Command` in `verifier.go` because the `gitops` package was out-of-scope. We should consider centralizing these queries into `gitops` in a later project.
- **WO6 (Local Publish)**: Complete. Implemented `local.go` to write claims to `pending/`, mark unreviewed, and do security gating.
  - **Dead end/Finding**: `index.go` lacks an incremental `InsertClaim` method. The worker defined an `Indexer` interface locally to avoid editing `index.go`, which we will wire up in the orchestration layer.
- **WO7 (Orchestration)**: Complete. Wired Triage, Extract, Reconcile, Verify, Publish into the tick's job processing logic. Implemented concurrency (extraction parallel, reconcile/verify single-writer), daily budgets via E11 logic, and idempotency keys. Added `InsertClaim` to `index.go` and wired it to `LocalPublisher`.
  - **Dead end/Finding**: When writing job pipelines, `Housekeeper` must be securely injected via `TickerOption`. Ensure mock harnesses correctly simulate `Usage` return data for the pipeline token budget accounting. Extraction requires `sync.Mutex` during payload combination to avoid race conditions.

### Project Wrap-Up
The knowledge pipeline (stages 2-5 + local publish lane) is fully implemented as a pure function dispatched to `agy -p`. Concurrency rules and cost recording are correctly sequenced in the tick job processing. Tests run cleanly against the recorded executor cassettes. The architecture holds up to the requirements of the SPEC, delivering structured and safely parsed knowledge pipeline artifacts with strict deterministic fallback constraints. 

Unresolved items for future iterations:
- Refactor the duplicated `git branch` queries into the central `gitops` utility.
- Add robust `agy` CLI streaming behavior parsing (from WO1's observation).
