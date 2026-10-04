# PR-05-v2-foundation: Close-out, Defect Fixes & Capture

> **Programme:** repo-memory
> **PR:** pr-05-v2-foundation
> **Base branch:** `main`
> **Status:** ☐ Planned
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Put the retained infrastructure into a known-good state before any v2 work. Close PR-05b onto `main`, fix the defects found in the plan review, and start capturing what v2 needs before pruning removes it.

## Design & SPEC Sections

- **v2 Design rev 3:** §2.3 (Retained infrastructure), §5.3 (Artifact capture and snapshotting), §9 (Telemetry, usage, cost and gen_metadata)
- **SPEC:** §4.6 (Episodes, evidence store and checkpoint parsing), §6.2 (Enrolment, hooks and fast path)

## Dependencies & Gate Preconditions

- **Base branch:** `main` (which already contains PR-05a, fail-open `PreToolUse` `3c02240`, and FTS `fdb1f69`)
- **Dependencies:** `main` + PR-05b branch (`pr-05b-dogfood-pilot/wo4-b5-evaluation`)
- **Gate Preconditions:** None (Phase 1 and Phase 2 complete; builds on `main`)

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [`wo1-close-pr05b`](wo1-close-pr05b/WORKORDER.md) | Merge the PR-05b branch's commits to `main`: the executor stdin fix (`4163dd5`), the robust JSON extraction and 429/exit-code-3 handling (`960d852`), and all docs (dogfood evaluation with Appendix C, the v2 design rev 1–3). Mark PR-05b complete in its PROJECT.md. Archive `.memory/claims/` to `.memory/archive/v1-claims/`. No other code changes. | mechanical | ☑ Complete |
| 2 | [`wo2-capture-to-evidence`](wo2-capture-to-evidence/WORKORDER.md) | Fix the broken link between capture and episodes. `convertCapturesToTriage` (`internal/core/tick/pipeline.go`) queues triage by conversation ID, but nothing parses or compacts the captured transcript into the episode store: 124 captured, 0 processed. Build episodes, or evidence records, from captured transcripts incrementally. Parse `CHECKPOINT` steps into structured sections (User Knowledge, Model Knowledge, Work Accomplished, …). Backfill the 124 already captured. Remove the v1 capture→triage hand-off. | standard | ☑ Complete |
| 3 | [`wo3-artifact-capture`](wo3-artifact-capture/WORKORDER.md) | Copy conversation artifacts (`brain/<id>/*.md` and `.metadata.json`) into `<data-dir>/<repo-id>/evidence/artifacts/`, at `Stop` and during the sweep, for enrolled repos. Deterministic. Add a `snapshot` command that copies the named conversations' transcripts and artifacts now, for X6 candidates (PD17). | standard | ☑ Complete |
| 4 | [`wo4-hooks-restore`](wo4-hooks-restore/WORKORDER.md) | The installed `~/.gemini/config/plugins/repo-memory/hooks.json` has been `{}` since 2026-10-02 22:45, so no hooks run and capture relies on the sweep. Record why it was emptied. Make `install`/`upgrade` write the hook set safely, with fail-open behaviour verified (empty stdout or `{"decision":"allow"}`; never `{}`). Add the non-enrolled fast path (E05: 57 ms → ≤ 5 ms via a marker or registry cache). Delete the 1.5 GB of probe logs and add rotation for logs and probes. | standard | ☑ Complete |
| 5 | `wo5-cost-telemetry` | Executor results are stored with `Cost{}`. Record `usage` from `agy` JSON output (input, output, thinking, cache read) on every job. Add a read-only `gen_metadata` reader, marked experimental and undocumented, for the uncached, cached and output tokens of IDE turns. X1 and X3 need it. Add a `/usage` reader (`agy -p "/usage" --output-format json`, zero tokens) as a shared quota-guard library. | standard | ☐ Planned |

## Exit Criteria

- `main` contains PR-05a, its two follow-up fixes, the PR-05b fixes and all v2 docs. The PR-05b branch is closed.
- The 124 captured transcripts are readable as episodes or evidence; checkpoints are parsed; new captures flow through.
- Artifacts are captured for enrolled repos; `snapshot` works.
- Hooks are installed and firing in the dogfood repo; `PreToolUse` fails open; non-enrolled hook p95 ≤ 5 ms; probe logs are removed and rotation is in place.
- Job costs are recorded; the `gen_metadata` reader returns per-turn usage; the quota-guard library works.

## Human Tasks

- Confirm why the installed `hooks.json` was emptied on 2026-10-02 (needed by `wo4`).
- *(Context: Decide PD17 — capture-only enrolment of esp32-app-foundation and snapshot of X6 candidate transcripts before pruning removes them).*

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| **Hooks disabled in the live install** | Safe re-enable in PR-05 wo4; fail-open verified; install writes the hook set | PR-05 |
| **Conversation pruning** (~500 kept, ~19 days) destroys pilot, backfill and X6 inputs | Artifact capture and `snapshot` in PR-05; PD17 capture-only enrolment of esp32 | PR-05, user |
| **Hook latency exceeded** | Budgets as exit criteria (PD24); precompute in the tick; non-enrolled fast path | PR-05, PR-08 |

## Human Approval

> **Approved:** yes (2026-10-04, full autonomous implementation authority granted by user)

## Session Log

- 2026-10-04 13:46 — Project approved. Full autonomous implementation authority granted. Starting WO1 (close PR-05b).
