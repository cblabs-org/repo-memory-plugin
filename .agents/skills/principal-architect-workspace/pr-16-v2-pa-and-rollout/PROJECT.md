# PR-16-v2-pa-and-rollout: PA Integration, Backfill/Reproject CLI, Dogfood Rollout & Programme Close-Out

> **Programme:** [repo-memory](../programme-repo-memory/PROGRAM.md) ([PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md))
> **Project:** `pr-16-v2-pa-and-rollout`
> **Base branch:** `pr-15-v2-learning/wo5-verification-and-skill-lint` (`cb2bc10`)
> **Status:** 🔄 Active — Approved for execution (`WO1`–`WO4`)
> **Specification:** [SPEC.md](../../../../SPEC.md) v1.0 (§5.5, §7.7, §13, §14) + [v2.2 Design (Rev 4)](../../../../docs/design-v2-distillation-retrieval.md) (§5.5, §7, §8) + [Addendum A (Rev 3)](../../../../docs/addendum-a-provenance-and-hardening.md) (§6 A5, §7 A6, §10 A7, §11 A9)

---

## Objective

Complete the final PR (`22/22`) of the `repo-memory` `v2.2` programme (`PR-16-v2-pa-and-rollout`), delivering Principal Architect integration commands (`summary`, `brief --workorder`), historical backfill and op-log reprojection (`backfill`, `reproject`), full dogfood rollout on `repo-memory-plugin`, and programme close-out:

1. **PA Session Summaries & Pre-Workorder Memory Briefs (`WO1` — `SPEC.md` §13 & `PROGRAM-PLAN.md` PR-14):**
   - Implement `repo-memory summary <session-id>` (`cmd/repo-memory/cmd_summary.go` + `internal/core/episode/summary.go`): produces a deterministic, structured $\le 1.5\text{k}$-token episode summary (`--json`, `--max-tokens 1500`) from a captured episode or transcript (covering Goal/Prompt, Files Edited, Commands & Test Outcomes, Checkpoints/Artifacts, and Harvested Inbox Notes) so a Principal Architect can review a worker session without reading raw transcripts.
   - Implement `repo-memory brief` (`cmd/repo-memory/cmd_brief.go` + `internal/core/delivery/workorder_brief.go`): supports `--scope <scope>` and `--workorder <path>` (`--json`, `--max-tokens 1000`). When `--workorder <path>` is passed, parses target files, symbols, and module scopes from the `WORKORDER.md` file, verifies anchors against the working tree, ranks matching invariants, contracts, gotchas, failure signatures, recipes (or 1-line graduated skill pointers), and ruled-out approaches, prepends the A6 freshness header, and appends coverage gap lines.
2. **Historical Backfill & Op-Log Reprojection (`WO2` — `SPEC.md` §14, `PROGRAM-PLAN.md` PR-15 & Addendum A §6 A5):**
   - Upgrade `cmd/repo-memory/cmd_backfill.go` (`repo-memory backfill`, retaining `backfill-captures` as an alias): supports `--dry-run`, `--json`, `--transcripts`, `--docs`, `--git`, `--max-items`, and `--repo`, throttled by A5 spend guards (`SpendConfig.Backfill`), importing PA workorders/docs, scanning git revert commits as refutation evidence, and queuing uncovered episodes for distillation.
   - Implement `repo-memory reproject` (`cmd/repo-memory/cmd_reproject.go` + `internal/core/dossiers/reproject.go`): replays `.memory/ops/` (plus local branch and user tiers) under the current projector/renderer, verifies that every entry ID, `verified` stamp, and human decision survives, re-renders `.memory/dossiers/`, `.memory/ATLAS.md`, `.memory/QUESTIONS.md`, and graduated `.agents/skills/rm-*/SKILL.md`, and supports `--dry-run` and `--json`.
3. **Full Dogfood Rollout & CI/Packaging Verification (`WO3`):**
   - Execute full dogfood rollout on `repo-memory-plugin` (`repo-memory seed`, `repo-memory seed-docs`, `repo-memory ops gc`, `repo-memory reproject`, `repo-memory promote-skills`, `repo-memory reindex`, `repo-memory doctor`) so `.memory/ATLAS.md`, `.memory/dossiers/`, and `.memory/QUESTIONS.md` cover the complete `v2.2` codebase (`PR-10a` through `PR-16`).
   - Update `.github/workflows/` CI checks to verify `go vet ./...`, `go test -race ./...`, cross-platform builds, and `repo-memory doctor` skill/instruction-block lint.
4. **Documentation, CLI Reference & Programme Close-Out (`WO4`):**
   - Update `README.md` with the complete `v2.2` CLI subcommand reference (`seed`, `seed-docs`, `ops gc`, `consolidate`, `promote-skills`, `garden`, `summary`, `brief`, `backfill`, `reproject`, `status`, `report`, `doctor`).
   - Run full verification (`go build ./...`, `go vet ./...`, `GOOS=windows go build ./...`, `go test -race ./cmd/... ./internal/... ./packaging/...`) and rebuild `~/.local/bin/repo-memory`.

---

## Architecture & Module Map

| Area | Files / Packages | Responsibility |
|---|---|---|
| PA Session Summaries & Workorder Briefs | `internal/core/episode/summary.go`, `internal/core/delivery/workorder_brief.go`, `cmd/repo-memory/cmd_summary.go`, `cmd/repo-memory/cmd_brief.go` | `repo-memory summary <session>` ($\le 1.5\text{k}$ tokens) and `repo-memory brief --workorder <path>` / `--scope <scope>` |
| Backfill & Op-Log Reprojection | `internal/core/dossiers/reproject.go`, `cmd/repo-memory/cmd_backfill.go`, `cmd/repo-memory/cmd_reproject.go` | A5-throttled `repo-memory backfill` (`--dry-run --json`) and deterministic `repo-memory reproject` preserving human decisions |
| Dogfood Rollout & CI | `.memory/`, `.github/workflows/`, `packaging/antigravity/` | Dogfood `.memory/` state (`seed` + `seed-docs` + `reproject`) and CI verification |
| Docs & Close-Out | `README.md`, `.agents/skills/principal-architect-workspace/` | Complete `v2.2` CLI reference and programme close-out |

---

## Work Order Ledger

| # | Work Order | Branch | Scope | Complexity | Status |
|---|---|---|---|---|---|
| 1 | [wo1-pa-summaries-and-workorder-briefs](wo1-pa-summaries-and-workorder-briefs/WORKORDER.md) | `pr-16-v2-pa-and-rollout/wo1-pa-summaries-and-workorder-briefs` | Implement `repo-memory summary <session-id>` ($\le 1.5\text{k}$ tokens) and `repo-memory brief --workorder <path>` / `--scope <scope>` with unit tests. | standard | ☑ Completed (`c52caed`) |
| 2 | [wo2-backfill-and-reproject-cli](wo2-backfill-and-reproject-cli/WORKORDER.md) | `pr-16-v2-pa-and-rollout/wo2-backfill-and-reproject-cli` | Upgrade `repo-memory backfill` (`--dry-run`, `--json`, `--transcripts`, `--docs`, `--git`, A5 spend guards) and implement `repo-memory reproject` (`--dry-run`, `--json`, human-decision preservation check). | standard | ☑ Completed (`6ec6794`) |
| 3 | [wo3-dogfood-rollout-and-ci-packaging](wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md) | `pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging` | Run full dogfood rollout on `repo-memory-plugin` (`seed`, `seed-docs`, `ops gc`, `reproject`, `promote-skills`, `reindex`, `doctor`) and update `.github/workflows/` CI checks. | standard | ☐ Ready |
| 4 | [wo4-programme-closeout-and-handover](wo4-programme-closeout-and-handover/WORKORDER.md) | `pr-16-v2-pa-and-rollout/wo4-programme-closeout-and-handover` | Update `README.md` CLI reference, run full verification (`go build`, `go vet`, `GOOS=windows`, `go test -race`), rebuild `~/.local/bin/repo-memory`, and finalize `PROJECT.md` for Step 6A PR Audit. | standard | ☐ Planned |

---

## Exit Criteria

- `repo-memory summary <session-id>` outputs a structured episode summary bounded within $\le 1.5\text{k}$ tokens (`--json` and human-readable).
- `repo-memory brief --workorder <path>` extracts target files/scopes from a `WORKORDER.md`, verifies anchors, prepends the A6 freshness line, outputs ranked rules/gotchas/recipes, and appends coverage gap lines.
- `repo-memory backfill --dry-run --json` and `repo-memory reproject --dry-run --json` work deterministically, respect A5 spend guards, and preserve all entry IDs and human decisions.
- Dogfood `.memory/` dossiers, `ATLAS.md`, and `QUESTIONS.md` on `repo-memory-plugin` are current and `repo-memory doctor` passes with 0 errors.
- All unit and race tests pass (`go test -race ./cmd/... ./internal/... ./packaging/...`).

## Human Approval
- [x] 4-WO decomposition approved by project owner (2026-10-16).

## Session Log

### 2026-10-16 — Project Initialized
- Initialized `pr-16-v2-pa-and-rollout` stacked on `pr-15-v2-learning/wo5-verification-and-skill-lint` (`5c57d41`).
- Approved 4-WO decomposition (`WO1` PA summaries & workorder briefs, `WO2` backfill & reproject CLI, `WO3` dogfood rollout & CI, `WO4` programme close-out).

### 2026-10-16 — WO1 Completed (`c52caed`)
- Exported `dossiers.EstimateTokens` in `internal/core/dossiers/lint.go`.
- Implemented `internal/core/episode/summary.go` (`BuildSessionSummary`, `SummarizeSessionInput`) and `cmd/repo-memory/cmd_summary.go` (`repo-memory summary <session-id-or-path> [--json] [--max-tokens]`).
- Implemented `internal/core/delivery/workorder_brief.go` (`ExtractWorkorderPaths`, `BuildWorkorderBrief`) and `cmd/repo-memory/cmd_brief.go` (`repo-memory brief [--workorder <path>] [--scope <scope>] [--json] [--max-tokens]`).
- Verified all unit and race tests pass across `internal/core/episode`, `internal/core/delivery`, and `cmd/repo-memory`.

### 2026-10-16 — WO2 Completed (`6ec6794`)
- Implemented `internal/core/tick/backfill.go` (`RunBackfill`, A5 spend guards, PA workspace import, doc seeding, git revert refutation notes) and upgraded `cmd/repo-memory/cmd_backfill.go` (`repo-memory backfill` with `backfill-captures` alias).
- Implemented `internal/core/dossiers/reproject.go` (`Reproject` with `ErrHumanDecisionLost` preservation check) and `cmd/repo-memory/cmd_reproject.go` (`repo-memory reproject` coordinating dossiers, `ATLAS.md`, `QUESTIONS.md`, and graduated skills).
- Verified all unit and race tests pass across `internal/core/tick`, `internal/core/dossiers`, and `cmd/repo-memory`.
