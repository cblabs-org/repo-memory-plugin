# PR-07-v2-storage: Op Log, Dossiers, Index, v1 Removal & Seeding

> **Programme:** repo-memory
> **PR:** pr-07-v2-storage
> **Base branch:** `main` (after PR-06 X-gate passed at `6065c8d`)
> **Status:** 🔄 Active — WO1/6
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)
> **Human Approval:** Autonomous execution authority granted (2026-10-06)

## Objective

Build the storage layer and a way to produce dossiers **without** turn notes or consolidation. This lets the read side be tested (X6-lite in PR-09) before distillation is built.

**Key deliverables:**
1. An append-only op log replacing the v1 events store
2. Dossier schema, renderer, and atlas
3. Scopes and aliases for routing
4. SQLite index migration (claims→entries)
5. v1 pipeline stage removal (keep executor, job chaining, local publish)
6. Seeding: produce dossiers from PA reports and evidence without turn notes

## Design & SPEC Sections

- **v2 Design rev 3:** §5.5, §7.1–7.5 (Op log, dossiers, entries, atlas, index), §6 (Consolidation — splits/aliases)
- **SPEC:** §5.6 (Index), §6 (Knowledge management), §6.8 (Two-lane publish)

## Dependencies

- **Base:** `main` at `6065c8d` (PR-06 merged, X-gate passed)
- **Carries forward:** Headless executor, job chaining, local-publish worktree (PR-05a/05b)
- **Retires:** v1 events store (`.memory/events/`), v1 pipeline stages (triage, extract, reconcile, verify), claims store, views generator
- **Informs:** PR-08 delivery (needs dossiers), PR-09 X6-lite (needs time-cut seeded dossiers)

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-op-log](wo1-op-log/WORKORDER.md) | Op types, file schema, layout `.memory/ops/<yyyy-mm>/<ulid>.json`, writer/reader/iterator, validation rules, v1 events retirement | standard | 🔄 Active |
| 2 | wo2-dossier-schema-renderer | Dossier kinds, entry schema (8 types), renderer, atlas, history, cap linting (~3k tokens), hand-edit detection → ops | architect | ☐ Planned |
| 3 | wo3-scopes-aliases | Declared scopes in `config.yaml`, derived module scopes (entity extractor), aliases from `split` ops, alias resolution for `memory.brief` | standard | ☐ Planned |
| 4 | wo4-index-migration | Replace claims/anchors/relations tables → entries/anchors/error-patterns/aliases/history + FTS5 over entries. Rebuild from op log via pointer-file swap. | standard | ☐ Planned |
| 5 | wo5-v1-removal | Remove `internal/core/pipeline/{triage,extract,reconcile,verify}`, views generator, tick wiring. **Keep** executor, job chaining, local-publish worktree, 429 handling. Tests green. | standard | ☐ Planned |
| 6 | wo6-seeding | (a) PA completion-report parser → notes. (b) Lean-agent one-off evidence pass → ops. (c) `--before <commit>` time cut for X6. (d) `source: seed` tagging, human review. | architect | ☐ Planned |

## Exit Criteria

- Op files round-trip: write → read → render → re-render gives identical dossiers.
- History files, the atlas and cap linting work; hand edits become ops.
- The index rebuilds from the op log; anchor lookups and FTS work; `reindex` works.
- Scope derivation gives sensible module assignments on the dogfood repo.
- v1 stages removed and tests green; the executor and publish code are reused.
- Time-cut seeded dossiers produced for the dogfood repo and reviewed by the user.

## Modules & Components

| Module | Package | Status |
|---|---|---|
| Op Log | `internal/core/ops/` | WO1 — new |
| Dossiers | `internal/core/dossiers/` | WO2 — new |
| Scopes & Aliases | `internal/core/config/scopes.go` | WO3 — new |
| Index (v2) | `internal/core/index/` | WO4 — migrate |
| v1 Pipeline | `internal/core/pipeline/{triage,extract,reconcile,verify}` | WO5 — remove |
| v1 Events | `internal/core/events/` | WO1 — retire/archive |
| v1 Claims | `internal/core/claims/` | WO5 — remove |
| v1 Views | `internal/core/views/` | WO5 — remove |
| Seeding | `internal/core/seed/` | WO6 — new |

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| Op log schema doesn't support all needed op types | Design from v2 §7.1; validate against dossier renderer in WO2 | WO1 |
| Seeded dossiers too shallow for X6-lite | PA report parser + lean-agent pass; user review before X6 | WO6 |
| v1 removal breaks existing functionality | Keep executor and publish; run all tests after removal | WO5 |
| Index migration loses FTS capability | Build new schema alongside old, swap atomically | WO4 |

## Session Log

### 2026-10-06 — Project kickoff (Chief Architect)
- Project workspace activated. 6 WOs decomposed per PROGRAM-PLAN.md.
- Base: `main` at `6065c8d` after PR-06 X-gate passed.
- WO1 (op-log) spawned as first worker.
- **Existing codebase survey:**
  - v1 events: `internal/core/events/` — ReviewEvent/AnswerEvent models, store (~240 lines)
  - v1 claims: `internal/core/claims/` + `claims/store/` — claim store with inverse relation index
  - v1 index: `internal/core/index/` — SQLite FTS5 over episodes (~560 lines)
  - v1 pipeline: `internal/core/pipeline/{triage,extract,reconcile,verify,publish,compact}` (~2100 lines total)
  - v1 views: `internal/core/views/`
  - Live `.memory/` dir: `claims/`, `events/`, `archive/v1-claims/`, `config.yaml`, `B5_EVALUATION.md`, `views/`
