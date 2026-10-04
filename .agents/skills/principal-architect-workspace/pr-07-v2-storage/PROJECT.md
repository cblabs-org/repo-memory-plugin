# PR-07-v2-storage: Op Log, Dossiers, Index, v1 Removal & Seeding

> **Programme:** repo-memory
> **PR:** pr-07-v2-storage
> **Base branch:** `pr-06-v2-spike`
> **Status:** ☐ Planned
> **SPEC:** [SPEC.md](../../../../SPEC.md) v1.0 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Build the storage layer and a way to produce dossiers **without** turn notes or consolidation. That way the read side can be tested (X6-lite) before distillation is built.

## Design & SPEC Sections

- **v2 Design rev 3:** §5.5 (Seeding from PA reports & evidence), §7.1–7.5 (Op log, dossier schema & rendering, history, atlas, hand-edit reconciliation), §6 (Splits and aliases)
- **SPEC:** Storage layer updates, op-log format, dossier schema, and retirement of v1 events store

## Dependencies & Gate Preconditions

- **Base branch:** `pr-06-v2-spike`
- **Dependencies:** PR-06-v2-spike
- **Gate Preconditions:** **The X-gate must have passed.** All validation exit criteria from PR-06 (X1, X9, X5, X8, X3) must be met and the proposed SPEC v1.0 edits must be approved by the user before work on PR-07 begins.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-op-log` | `internal/core/ops/`: op types (`add`, `amend`, `supersede`, `retire`, `move`, `split`), op file schema (one JSON file per batch, named by ULID), and the layout `.memory/ops/<yyyy-mm>/<ulid>.json`. Writer, reader and iterator; validation rules per op type. **Retire the v1 events store** (`.memory/events/`, PR-04 wo7). Migrate anything worth keeping into ops; otherwise archive it (PD26). | standard | ☐ Planned |
| 2 | `wo2-dossier-schema-renderer` | `internal/core/dossiers/`: dossier kinds; the entry schema (ID, the eight types, required fields per type, anchors, evidence links, verified stamp, `closes_when`); required sections (Purpose and boundaries; Invariants; Ruled out, failure signatures and gotchas) plus optional sections omitted when empty; a cap of about 3k tokens. Render dossiers and `ATLAS.md` from the op log, fully or incrementally; superseded and retired entries go to `.history.md`. Detect hand edits and convert them to ops. | architect | ☐ Planned |
| 3 | `wo3-scopes-aliases` | `internal/core/config/scopes.go`: declared scopes (topics, external systems, procedures) in `.memory/config.yaml`; derived module scopes (package directory rolled up to a parent until there are enough entries, using the entity extractor); aliases from `split` ops; alias resolution for `memory.brief`. | standard | ☐ Planned |
| 4 | `wo4-index-migration` | `internal/core/index/`: replace the claims, anchors and relations tables with tables for entries, anchors (with fingerprints), error patterns (command prefix, pattern, dossier scope), aliases and history. Add FTS5 over entries, synopses and checkpoint summaries. Rebuild from the op log via pointer-file swap; update `repo-memory reindex`. | standard | ☐ Planned |
| 5 | `wo5-v1-removal` | Remove the v1 stages (`internal/core/pipeline/triage`, `extract`, `reconcile`, `verify`), the views generator and their tick wiring. **Keep** the headless executor, job chaining, local-publish worktree and 429 handling from PR-05a/05b, generalised for the lean agent. No broken imports; tests green. | standard | ☐ Planned |
| 6 | `wo6-seeding` | A path to dossiers without turn notes. (a) A deterministic PA completion-report parser ("Decisions Made", "Dead Ends & Warnings" → notes, `source: pa`). (b) A one-off lean-agent seeding pass over evidence (checkpoint summaries, artifacts, compacted transcripts) that emits ops directly, with output validated in Go. (c) A **`--before <commit>` time cut** that excludes all evidence at or after a commit, as X6 requires. Seeded entries are tagged `source: seed` and reviewed by a human before use. | architect | ☐ Planned |

## Exit Criteria

- Op files round-trip: write → read → render → re-render gives identical dossiers.
- History files, the atlas and cap linting work; hand edits become ops.
- The index rebuilds from the op log; anchor lookups and FTS work; `reindex` works.
- Scope derivation gives sensible module assignments on the dogfood repo.
- v1 stages removed and tests green; the executor and publish code are reused.
- Time-cut seeded dossiers produced for the dogfood repo and reviewed by the user.

## Human Tasks

- Review the time-cut seeded dossiers (needed by PR-07 exit).

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| **v1 removal breaks imports or builds** | Verify executor and publish code reuse; ensure no broken imports; keep tests green | PR-07 |
| **Scope derivation issues** | Verify scope derivation gives sensible module assignments on the dogfood repo | PR-07 |
| **Several developers publish at once** | Op log never conflicts (ULID-named batches); dossiers re-render deterministically | PR-07 |

## Session Log

*(Empty, ready for use)*
