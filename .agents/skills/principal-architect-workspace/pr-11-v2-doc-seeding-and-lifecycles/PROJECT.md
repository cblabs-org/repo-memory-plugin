# PR-11: Doc-Seeded Dossiers, Cognitive Lifecycles & `{section?}` Rendering

> **Programme:** repo-memory
> **PR:** pr-11-v2-doc-seeding-and-lifecycles
> **Base branch:** `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` (`14c449c`)
> **Status:** ✅ Complete (6/6 WOs complete — PR #19 APPROVED)
> **Human approval:** Decomposition approved 2026-10-15 as part of the v2.2 Programme Transition Plan (`PROGRAM.md` / `PROGRAM-PLAN.md`, Decisions `VD16` & `VD17`).

## Objective

Implement the `v2.2` Doc-Seeded Dossiers (`repo-memory seed-docs` / `DocSeedSource`), Async Doc Freshness ("Dreaming" in `tick`), the 3 formal Cognitive Memory Lifecycles (`semantic`, `procedural`, `episodic`), and Optional Section Resolution (`{section?}`) per `SPEC.md` v1.0 (§5.3, §5.4, §6.3.5, §6.6, §14.3), `docs/design-v2-distillation-retrieval.md` Rev 4 (§5.2, §7.3, §7.6), and `docs/addendum-a-provenance-and-hardening.md` Rev 3 (§10, §11.1).

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-cognitive-lifecycles-schema](wo1-cognitive-lifecycles-schema/WORKORDER.md) | Add `CognitiveKind` (`semantic` \| `procedural` \| `episodic`, auto-derived via `DeriveCognitiveKind(entryType)` across all 8 canonical entry types plus aliases `contract`/`convention`, `gotcha`/`external-behaviour`, `recipe`/`procedure`, `interface`) and `v2.2` provenance/procedural fields (`SourceDocPath`, `SourceDocBlobSHA`, `SuccessCount`, `FailureCount`, `LastSucceededAt`, `StaleSuspect`, `RenderAsSkill`) to `dossiers.Entry`, `dossiers.Projector`, `ops` (`SourceDocSeed = "doc-seed"`, `EntryTypePrefixes` for `con-`, `rec-`, `err-`, `iss-`), and the SQLite `entries` schema (`internal/core/index/`). Add `KindSeedDocs` and `KindDreamFreshness` to `jobs.Kind` (`internal/core/jobs/types.go`). | standard | ✅ Complete (`c9640fc`) |
| 2 | [wo2-optional-section-rendering](wo2-optional-section-rendering/WORKORDER.md) | Update `internal/core/dossiers/renderer.go` and `handedit.go` to enforce `SPEC.md` §5.4 / `design-v2` §7.3 Optional Section Resolution (`{section?}`) and entry metadata rendering: render `kind: <semantic\|procedural\|episodic>` in the backtick metadata line, group `invariant` + `contract`/`convention`/`interface` under `## Invariants & contracts`, group `recipe`/`procedure` under `## Procedures / Recipes`, omit empty optional sections completely, and preserve backward compatibility with existing tests and `DetectHandEdits`. | standard | ✅ Complete (`16d0be3`) |
| 3 | [wo3-doc-seed-source](wo3-doc-seed-source/WORKORDER.md) | Add `DocSeeds []string` to `config.RepoConfig` (`.memory/config.yaml`, defaulting to `docs/**/*.md` and hand-written `.agents/skills/**/SKILL.md` files without `generated_by: repo-memory`). Implement `DocSeedSource` in `internal/core/seed/doc_seed.go` and the `repo-memory seed-docs` CLI subcommand (`cmd/repo-memory/cmd_seed_docs.go`), extracting structured, code-anchored `invariant`, `contract`, `gotcha`, `recipe`/`procedure`, and `failure-signature` ops tagged with `source: "doc-seed"`, `source_doc_path`, and `source_doc_blob_sha` (`git hash-object`), supporting deterministic markdown extraction, `--dry-run`, and `--json`. | architect | ✅ Complete (`37f2051`) |
| 4 | [wo4-async-doc-freshness-dreaming](wo4-async-doc-freshness-dreaming/WORKORDER.md) | Implement Async Doc Freshness ("Dreaming", `SPEC.md` §6.3.5 / `design-v2` §7.6 / Addendum A §11.1) in `internal/core/tick/dream_freshness.go` and wire into `PipelineExecutor.Run`: compare each `source: "doc-seed"` entry's `source_doc_blob_sha` against the current git blob SHA of `source_doc_path`; when a seeded doc is modified or deleted in git, deterministically re-seed/amend/supersede or retire the affected `doc-seed` entries and rebuild `dossiers/` and `index.db`. | standard | ✅ Complete (`f54c27f`) |
| 5 | [wo5-dogfood-doc-seed-and-verification](wo5-dogfood-doc-seed-and-verification/WORKORDER.md) | Run `repo-memory seed-docs` on `repo-memory-plugin`'s curated docs/skills, re-project `.memory/dossiers/` and `.memory/ATLAS.md` with `{section?}` resolution and `kind:` metadata, rebuild `~/.local/bin/repo-memory` and `<data-dir>/<repo-id>/index.db`, and verify `go build ./...`, `go vet ./...`, `GOOS=windows go build ./...`, and `go test -race ./...`. | standard | ✅ Complete (`572ddb5`) |
| 6 | [wo6-review-remediation](wo6-review-remediation/WORKORDER.md) | Resolve Round 1 Cold-Firewall PR Audit findings (`[MAJOR-1]` polluted dot-prefixed/root-file scopes in `SeedDocs`/`ParseDocToOps`, `[MAJOR-2]` empty `# <scope>` files and `0001-01-01T00:00:00Z` `ATLAS.md` rows in `dossiers.Writer.WriteAll`, `[MINOR-1]` `FilesSkipped` counter when `candidateOps` is empty), clean & re-seed `.memory/`, and rebuild `~/.local/bin/repo-memory` + `index.db`. | standard | ✅ Complete (`23f082d`) |

## Dependencies

- **Base:** `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` (`14c449c`)
- **Preconditions:** `PR-10a` (Specs `v1.0` & Design Rev 4) and `PR-10b` (Delivery persistence & storage/seeder fixes) complete and approved.
- **Downstream:** Feeds `PR-12-v2-distillation` and `PR-15-v2-learning`.

## Exit Criteria

- `dossiers.Entry`, `dossiers.Projector`, `ops`, `index.Index`, and `jobs.Kind` support the 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`), `doc-seed` provenance (`source_doc_path`, `source_doc_blob_sha`), and procedural execution fields (`success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect`, `render_as_skill`).
- `RenderDossier` renders `kind: <semantic|procedural|episodic>` in entry metadata lines and omits empty optional sections per `{section?}`.
- `repo-memory seed-docs` (with `--dry-run` and `--json`) ingests configured markdown docs/skills (`doc_seeds` in `.memory/config.yaml`), validates repository file/symbol anchors, and writes idempotent `source: "doc-seed"` ops with `source_doc_path` and `source_doc_blob_sha`.
- `tick` detects modified or deleted `source_doc_path` files via git blob SHA comparison ("Dreaming") and updates/retires affected `doc-seed` entries automatically.
- `.memory/dossiers/`, `.memory/ATLAS.md`, and `~/.local/bin/repo-memory` are updated and all Go tests (`go test -race ./cmd/... ./internal/...`) pass.

## Session Log

### 2026-10-15 — Project Creation & Human Approval Recorded
- Initialized by Chief Architect stacked on `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` (`14c449c`).
- Human approval of the 5-WO decomposition recorded in `PROGRAM.md` and `PROGRAM-PLAN.md` (`2026-10-15`).

### 2026-10-15 — WO1 (`wo1-cognitive-lifecycles-schema`) Complete (`c9640fc`)
- Executed by Tier 3 Worker `fa92ca29-9fdd-4af7-b6a8-be9c3970987c`.
- Added `CognitiveKind` (`semantic`, `procedural`, `episodic`), `DeriveCognitiveKind`, `EffectiveKind`, canonical `EntryType` constants (`contract`, `convention`, `external-behaviour`, `recipe`), `SourceDocSeed`, and `v2.2` provenance/procedural tracking fields across `dossiers`, `ops`, `delivery`, `index` (with idempotent `PRAGMA table_info(entries)` + `ALTER TABLE` migration and `ListEntriesBySource`), and `jobs.Kind` (`KindSeedDocs`, `KindDreamFreshness`).

### 2026-10-15 — WO2 (`wo2-optional-section-rendering`) Complete (`16d0be3`)
- Executed by Tier 3 Worker `62a89cfd-590c-4dcd-b70d-8d160f411654`.
- Updated `internal/core/dossiers/renderer.go` to enforce `{section?}` Optional Section Resolution (`## Invariants & contracts`, `## Ruled out, failure signatures and gotchas`, `## Decisions`, `## Procedures / Recipes`, `## Interfaces and dependencies`, `## Open issues`), render `kind: <semantic|procedural|episodic>` and `source_doc: <path>` in entry metadata lines, and added deterministic `InferEntryTypeFromID` in `handedit.go`.

### 2026-10-15 — WO3 (`wo3-doc-seed-source`) Complete (`37f2051`)
- Executed by Tier 3 Worker `ac804197-9da5-40da-92eb-5d6eae6f1af2`.
- Added `DocSeeds []string` and `ResolveDocSeedGlobs` in `internal/core/config/config.go`, `DocSeedSource` / `SeedDocs` / `ParseDocToOps` / `ComputeGitBlobSHA` in `internal/core/seed/doc_seed.go`, and `repo-memory seed-docs` CLI in `cmd/repo-memory/cmd_seed_docs.go`.

### 2026-10-15 — WO4 (`wo4-async-doc-freshness-dreaming`) Complete (`f54c27f`)
- Executed by Tier 3 Worker `ced46bcd-66d1-455f-b380-010b272cfcab`.
- Implemented `CheckDocFreshness` / `checkDocFreshness` in `internal/core/tick/dream_freshness.go` and wired into `PipelineExecutor.Run` (`internal/core/tick/pipeline.go`).

### 2026-10-15 — WO5 (`wo5-dogfood-doc-seed-and-verification`) Complete (`572ddb5`)
- Executed by Tier 3 Worker `3682103d-eb9b-4738-b95f-6ad724d18667`.
- Configured `doc_seeds` in `.memory/config.yaml`, refined `ParseDocToOps` to exclude flattened comparison tables and non-code target anchors, dogfooded `repo-memory seed-docs` (12 anchored `doc-seed` entries across `.memory/dossiers/`), re-projected all 30 dossiers with `kind:` and `{section?}` resolution, rebuilt `~/.local/bin/repo-memory` and `<data-dir>/<repo-id>/index.db`, and verified `go test -race ./cmd/... ./internal/...` and `GOOS=windows go build ./cmd/... ./internal/...`.

### 2026-10-15 — WO6 (`wo6-review-remediation`) Complete (`23f082d`)
- Executed by Tier 3 Worker `aeef37a3-f5cd-47ec-804f-eaa6378368d0`.
- Resolved all 3 Round 1 Cold-Firewall PR Audit findings (`[MAJOR-1]`, `[MAJOR-2]`, `[MINOR-1]`), cleaned and re-seeded `.memory/` so `seed-docs` and `ops gc` are mutually idempotent (`scopes_repaired=0`, 27 clean dossiers in `.memory/ATLAS.md`), and rebuilt `~/.local/bin/repo-memory` + `<data-dir>/<repo-id>/index.db`.

### 2026-10-15 — Cold-Firewall PR Audit Round 2 APPROVED & Step 6B Graduation
- Cold-Firewall PR Audit Reviewer verified `675c606` (`23f082d`) on [PR #19](https://github.com/cblabs-org/repo-memory-plugin/pull/19) with verdict `APPROVED (0 blocking, 0 major)`.
- **Distilled Architectural Lessons:**
  - **Mutual Idempotency Between Seeders & `ops gc`:** Any deterministic seeder (`SeedDocs`, `Seed`) must filter `scopeAnchors` (`strings.Contains(a, "/") && !strings.HasPrefix(a, ".")`) and guard derived scopes with `!isPollutedScope(scope)` before falling back to `"root"`, while keeping valid dot-prefixed or root-file anchors (`.memory/config.yaml`, `Makefile`) in `op.Fields["anchors"]`.
  - **Zero-Entry Dossier & `ATLAS.md` Pruning on Retirement:** When all active entries in a dossier are retired (`len(d.Entries) == 0`, `len(d.History) > 0`), `dossiers.Writer.WriteAll` must remove `<scope>.md`, write only `<scope>.history.md`, omit `<scope>` from `ATLAS.md` (preventing `0`-entry `0001-01-01T00:00:00Z` rows), and sweep orphaned `.md` files from `.memory/dossiers/`.
