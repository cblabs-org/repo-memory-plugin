# PR-07-v2-storage: Op Log, Dossiers, Index, v1 Removal & Seeding

> **Programme:** repo-memory
> **PR:** pr-07-v2-storage
> **Base branch:** `main` (after PR-06 X-gate passed at `6065c8d`)
> **Status:** ✅ **Complete** — GitHub PR #9 merged 2026-10-06.
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
| 1 | [wo1-op-log](wo1-op-log/WORKORDER.md) | Op types, file schema, layout `.memory/ops/<yyyy-mm>/<ulid>.json`, writer/reader/iterator, validation rules, v1 events retirement | standard | ✅ Complete |
| 2 | [wo2-dossier-schema-renderer](wo2-dossier-schema-renderer/WORKORDER.md) | Dossier kinds, entry schema (8 types), renderer, atlas, history, cap linting (~3k tokens), hand-edit detection → ops | architect | ✅ Complete |
| 3 | [wo3-scopes-aliases](wo3-scopes-aliases/WORKORDER.md) | Declared scopes in `config.yaml`, derived module scopes (entity extractor), aliases from `split` ops, alias resolution for `memory.brief` | standard | ✅ Complete |
| 4 | [wo4-index-migration](wo4-index-migration/WORKORDER.md) | Replace claims/anchors/relations tables → entries/anchors/error-patterns/aliases/history + FTS5 over entries. Rebuild from op log via pointer-file swap. | standard | ✅ Complete |
| 5 | [wo5-v1-removal](wo5-v1-removal/WORKORDER.md) | Remove `internal/core/pipeline/{triage,extract,reconcile,verify}`, views generator, tick wiring. **Keep** executor, job chaining, local-publish worktree, 429 handling. Tests green. | standard | ✅ Complete |
| 6 | [wo6-seeding](wo6-seeding/WORKORDER.md) | (a) PA completion-report parser → notes. (b) Lean-agent one-off evidence pass → ops. (c) `--before <commit>` time cut for X6. (d) `source: seed` tagging, human review. | architect | ✅ Complete |

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

### 2026-10-06 — WO1 complete (op-log)
- Worker delivered `internal/core/ops/` (1824 lines across 8 files):
  - `op.go`: Op/OpBatch structs, ULID helpers, provenance constants
  - `validate.go`: Per-type validation with clear errors
  - `writer.go`: Month-sharded JSON writer
  - `reader.go`: Batch reader with validation
  - `iterator.go`: Chronological iterator with IterateAfter, IterateBefore (git time-cut for X6), IterateBeforeTime, push/pull interfaces, context cancellation
  - `migrate.go`: v1 events → archive helper
  - `ops_test.go`: 10 test suites, comprehensive coverage
- Build ✅, tests ✅, vet ✅
- Worker also produced `docs/addendum-a-provenance-and-hardening.md` and minor SPEC edits
- Committed at `e9ce662`, pushed.
- WO2 (dossier-schema-renderer) spawned immediately after, `pro` model for `architect` complexity.

### 2026-10-06 — WO2 complete (dossier-schema-renderer)
- Worker delivered `internal/core/dossiers/` (1032 lines across 7 files):
  - `entry.go`: Entry struct with 8 types (decision, invariant, ruled-out, failure-signature, gotcha, procedure, open-issue, external-behaviour)
  - `dossier.go`: Dossier struct with Purpose, required/optional sections
  - `projector.go`: Op log → Entry projection with `ProjectBefore` time-cut (470 lines)
  - `renderer.go`: Deterministic Markdown rendering, atlas generation
  - `writer.go`: Dossier + history + atlas file writer
  - `lint.go`: Cap enforcement (~3k tokens per dossier)
  - `handedit.go`: Detection of manual dossier edits → conversion to ops
- Build ✅, tests ✅

### 2026-10-06 — WO3 complete (scopes-aliases)
- Worker delivered `internal/core/config/scopes.go` (798 lines):
  - `ScopeResolver` combining declared scopes (from config.yaml), derived module scopes (from entity extractor file anchors), and aliases (from split ops)
  - Alias resolution for `memory.brief`
  - Module scope rollup heuristic
- Build ✅, tests ✅ (481 lines of tests)

### 2026-10-06 — WO4 complete (index-migration)
- Worker delivered v2 index tables in `internal/core/index/`:
  - `entries.go`: CRUD for entries table (450 lines)
  - `entry_anchors.go`: Anchor-to-entry mapping (114 lines)
  - `index.go`: Updated schema with `entries`, `entries_fts` (FTS5), `entry_anchors`, `error_patterns`, `aliases`, `entry_history`
  - `rebuild.go`: `RebuildFromOpLog` — single-transaction rebuild, op projection outside lock
  - Retained: `episodes_fts`, `entities`
- Build ✅, tests ✅ (638 lines of v2 tests)

### 2026-10-06 — WO5 complete (v1-removal)
- Worker removed ~2500 lines of v1 pipeline stages:
  - Removed: `internal/core/pipeline/{triage,extract,reconcile,verify}`, `internal/core/views/`, `internal/core/claims/`, `internal/core/events/`, `cmd/repo-memory/cmd_views.go`
  - Retained: `internal/core/pipeline/publish/` (local-publish worktree), `internal/core/pipeline/compact/`, executor, job chaining, 429 handling
  - Gutted `internal/core/tick/pipeline.go`: v1 stage calls replaced with log message
- Build ✅, tests ✅

### 2026-10-06 — WO6 complete (seeding)
- Worker delivered `internal/core/seed/` (560 lines across 4 files) and `cmd/repo-memory/cmd_seed.go`:
  - `parser.go`: PA completion-report parser (backtick-token extraction from Decisions Made, Dead Ends, Changes Made, Out-of-Scope Findings)
  - `convert.go`: Report → ops converter with deterministic content-addressed IDs (`seed-<hash>`)
  - `scanner.go`: Workorder scanner with git commit timestamps on ops; `--before <commit>` time cut
  - CLI subcommand `repo-memory seed`
  - Idempotent (content-addressed IDs prevent duplicate entries)
- **Seeding result:** 174 entries from 49 PA workorders, with module-based scopes and git commit timestamps
- Build ✅, tests ✅

### 2026-10-07 — Project Wrap-Up
**Architectural outcome:** The v2 storage layer is fully built. The system now has:
- An append-only op log as the single source of truth (6 op types, month-sharded)
- Dossiers rendered deterministically from ops, with history files and an atlas
- A scope system combining declared, derived, and aliased scopes
- A SQLite index rebuilt from the op log (entries, FTS5, anchors, error patterns, aliases)
- v1 pipeline stages fully removed; executor, job chaining, and local publish retained
- A seeding path that produces dossiers from PA reports without needing the full distillation pipeline

**Lessons for future projects:**
- The `architect` complexity WOs (WO2, WO6) needed `pro` model to handle the design-sensitive schema work
- Content-addressed seeding IDs are critical for idempotent re-seeding across sessions
- Time-cut projection (`ProjectBefore`, `IterateBefore`) is essential for X6 — built into both ops and dossiers from the start

**Unresolved items:** None — all 6 WOs complete. The seeded dossiers (174 entries) are ready for PR-08 delivery integration and PR-09 X6-lite testing.
