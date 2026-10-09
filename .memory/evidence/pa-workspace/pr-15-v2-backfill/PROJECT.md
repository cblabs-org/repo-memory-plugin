# PR-15: Full Import & Re-projection

> **Programme:** repo-memory
> **PR:** pr-15-v2-backfill
> **Base branch:** PR-14-v2-pa-integration
> **Status:** ☐ Planned
> **SPEC:** [SPEC.md](../../../SPEC.md) §14
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Full backfill of historical repository knowledge and implementation of op-log re-projection tooling:
- Ingest all surviving captured evidence through the fallback, throttled by the daily budget.
- Import design decision logs, ADRs, guides, and experiment reports as notes.
- Import commits, PR descriptions, and reverts as refutation evidence.
- Run a lean-agent inferred-rationale pass for subsystems with no rationale coverage (tagged `hypothesis`/`inferred`).
- Implement `repo-memory reproject` for op-log replay under a new pipeline version, producing a diff memory PR while preserving human decisions by entry ID and content hash.

> [!NOTE]
> **Transcripts and Pruning (PD10, PD17):** Only transcripts captured before pruning exist. Historical backfill relies on surviving transcripts snapshot in PR-05 / PD17, structured documentation, and git history.

## Design & SPEC Sections

- **SPEC:** §14 (Migration and backfill), §15.1 (P5 Full backfill)
- **v2 Design:** §7.4 (Why pages, rendered from an op log)

## Dependencies & Gate Preconditions

- **Base branch:** `PR-14-v2-pa-integration`
- **Dependencies & Preconditions:**
  - PR-14-v2-pa-integration complete.
  - Storage op log and dossier schema established (PR-07-v2).
  - Sidecar fallback and preparation layer operational (PR-10-v2).
  - Op log replay, consolidation, and two-lane publishing functional (PR-11-v2).
  - Candidate transcripts and artifacts preserved prior to conversation pruning via PR-05 snapshot and PD17 capture-only enrolment.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-transcript-import` | All captured evidence, through the fallback, throttled by the daily budget. Note: only transcripts captured before pruning exist (PD10, PD17). | standard | ☐ Planned |
| 2 | `wo2-doc-import` | Design decision logs, ADRs, guides and experiment reports, imported as notes | standard | ☐ Planned |
| 3 | `wo3-git-import` | Commits, PR descriptions, and reverts as refutation evidence | standard | ☐ Planned |
| 4 | `wo4-inferred-rationale` | A lean-agent pass for subsystems with no rationale coverage. Results are tagged `hypothesis`/`inferred`. | standard | ☐ Planned |
| 5 | `wo5-reprojection` | `repo-memory reproject`: an op-log replay under a new pipeline version, producing a diff memory PR. Human decisions are re-applied by entry ID and content hash. | architect | ☐ Planned |

## Exit Criteria

- Every subsystem has a dossier; the coverage report is published; historical defects end retired or superseded; re-projection preserves every human decision.

## Human Tasks

- Review memory PR generated from backfill and re-projection (ongoing memory-PR review process from PR-11 onward).

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| Conversation pruning (~500 kept, ~19 days) destroys pilot, backfill and X6 inputs | Artifact capture and snapshot in PR-05; PD17 capture-only enrolment of esp32 | PR-05, user |

## Session Log

*(Empty, ready for use)*
