# WORKORDER: WO4 — Documentation, CLI Reference & Programme Close-Out

## 1. Metadata & Branch Topology
- **Parent Project:** `pr-16-v2-pa-and-rollout`
- **Work Order Slug:** `wo4-programme-closeout-and-handover`
- **Target Branch:** `pr-16-v2-pa-and-rollout/wo4-programme-closeout-and-handover`
- **Base Branch:** `pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging`
- **Status:** `COMPLETED`

---

## 2. Context & Architectural Intent
As the final work order (`WO4`) of `PR-16-v2-pa-and-rollout` (`22/22`), this work order updates `README.md` with the complete `v2.2` CLI subcommand reference (`install`, `uninstall`, `enroll`, `unenroll`, `seed`, `seed-docs`, `ops gc`, `consolidate`, `promote-skills`, `garden`, `summary`, `brief`, `backfill`, `reproject`, `reindex`, `tick`, `status`, `search`, `usage`, `report`, `snapshot`, `doctor`, `disable`, `enable`) and `.memory/QUESTIONS.md` storage layout, rebuilds the local `~/.local/bin/repo-memory` binary, and executes the full cross-platform build, race-detector test, doctor lint, reprojection dry-run, and external hygiene verification gates prior to Step 6A PR Audit.

---

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- `README.md`
- `internal/core/delivery/workorder_brief.go`
- `internal/core/delivery/workorder_brief_test.go`
- `internal/core/dossiers/reproject.go`
- `internal/core/dossiers/reproject_test.go`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo1-cognitive-lifecycles-schema/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo2-optional-section-rendering/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo3-doc-seed-source/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo4-async-doc-freshness-dreaming/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/wo6-review-remediation/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo1-marks-inbox-and-subagent-registry/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo2-stop-milestone-gate-and-harvest/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo3-free-harvest-checkpoint-and-artifacts/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo4-preparation-layer-and-scrubber/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo5-sidecar-fallback/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo6-managed-instruction-block-and-verification/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md`
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo4-programme-closeout-and-handover/WORKORDER.md` *(append Worker Completion Report only)*

### Forbidden Files (Read-Only Context)
- `SPEC.md`
- `cmd/repo-memory/**`
- `.memory/**`

---

## 4. Detailed Implementation Instructions

### A. Update `README.md` Storage Layout & CLI Reference
1. In `## Architecture & Storage Layout` under `Committed Repository State (.memory/ — repo: and branch: tiers)`:
   - Include `QUESTIONS.md` (`# Unresolved open issues, suspect entries, and coverage gaps`) alongside `ATLAS.md`.
2. In `## CLI Reference`:
   - Expand the CLI reference block so every `v2.2` subcommand is documented clearly with its key flags:
     - Installation & Repository Enrolment:
       - `repo-memory install --harness <antigravity|claude>`
       - `repo-memory uninstall --harness <antigravity|claude>`
       - `repo-memory enroll` (alias: `repo-memory init`)
       - `repo-memory unenroll` (alias: `repo-memory leave`)
     - Seeding & Op-Log Maintenance:
       - `repo-memory seed-docs [--repo <path>] [--dry-run] [--json]`
       - `repo-memory seed [--repo <path>] [--before <commit>] [--dry-run]`
       - `repo-memory ops gc [--repo <path>] [--dossiers-dir <path>] [--dry-run]`
     - Consolidation, Skill Graduation & Gardening:
       - `repo-memory consolidate [--repo <path>] [--dry-run] [--json]`
       - `repo-memory promote-skills [--repo <path>] [--dry-run] [--json]`
       - `repo-memory garden [--repo <path>] [--dry-run] [--json]`
     - Principal Architect (PA) Integration:
       - `repo-memory summary <session-id-or-path> [--max-tokens <n>] [--json]`
       - `repo-memory brief [--workorder <path>] [--scope <scope>] [--max-tokens <n>] [--json]`
     - Backfill, Reprojection & Indexing:
       - `repo-memory backfill [--repo <path>] [--transcripts] [--docs] [--git] [--max-items <n>] [--force] [--dry-run] [--json]` (alias: `repo-memory backfill-captures`)
       - `repo-memory reproject [--repo <path>] [--include-local] [--allow-loss] [--dry-run] [--json]`
       - `repo-memory reindex [--repo <path>]`
     - Background Tick, Observability & Diagnostics:
       - `repo-memory tick` (alias: `repo-memory run`)
       - `repo-memory status [--json]`
       - `repo-memory search <query>`
       - `repo-memory usage`
       - `repo-memory report [--days <n>] [--json]`
       - `repo-memory snapshot`
       - `repo-memory doctor [path]`
       - `repo-memory disable [--all]`
       - `repo-memory enable`

### B. Rebuild `~/.local/bin/repo-memory` & Run Full Verification Suite
1. Rebuild the host binary:
   ```bash
   go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
   ```
2. Run all verification commands in Section 5 and record results in Section 6 (`Worker Completion Report`).

### C. Cold-Firewall PR Audit Polish Remediation (`[POLISH-1]` through `[POLISH-5]`)
Apply all 5 polish items from the Step 6A Cold-Firewall PR Audit on PR #24:
1. **`[POLISH-1]` Repo-Wide Workorder Hygiene**:
   - In `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md` and the 12 `WORKORDER.md` files under `pr-11-v2-doc-seeding-and-lifecycles/` and `pr-12-v2-distillation/`, replace any negative-instruction example strings containing internal keywords or local home directory prefixes with generic terms (`proprietary internal references` and `/home/<user>/...`) so a repo-wide grep across `.agents/skills/principal-architect-workspace/` returns `0` matches.
2. **`[POLISH-2]` `README.md:78` Quickstart Test Command**:
   - Update `go test ./...` in `README.md:78` to `go test ./cmd/... ./internal/... ./packaging/...` (matching `.github/workflows/ci.yml`).
3. **`[POLISH-3]` & `[POLISH-4]` `internal/core/delivery/workorder_brief.go` Path Normalization & Graduated Skill Pointer**:
   - In `normalizeWorkorderCandidatePath`: call `path.Clean` on the candidate and reject (`return ""`) any path where `strings.Contains(s, "..")` is true.
   - In `renderWorkorderBriefMarkdown` (and `formatBriefEntryLine`): pass `repoRoot` and when rendering a `recipe` or `procedure` entry that has a graduated skill (`slug := skills.GraduatedSkillSlug(e, repoRoot); slug != ""` or `strings.TrimSpace(e.RenderAsSkill) != ""`), render the compact 1-line pointer (`fmt.Sprintf("- [%s] (%s, %s) skill %s covers this procedure: %s", e.ID, kindLabel, scopeLabel, slug, skills.EntryTitleLine(e))`) instead of the full multi-step body. Add a unit test in `internal/core/delivery/workorder_brief_test.go` verifying both `..` path rejection and graduated skill 1-line pointer rendering in workorder briefs.
4. **`[POLISH-5]` `internal/core/dossiers/reproject.go` Hand-Edit Capture Before Re-Render**:
   - In `Reproject` (`internal/core/dossiers/reproject.go`), before overwriting existing `.memory/dossiers/*.md` files, compare each projected dossier's rendered markdown (`RenderDossier(d)`) against the existing on-disk `<scope>.md` file using `DetectHandEdits(rendered, string(existingBytes), scope)`.
   - If `DetectHandEdits` returns `OpAmend` (or `OpAdd`) operations for human prose edits on disk:
     - Record the count in `ReprojectResult` (`HandEditsCaptured int` `json:"hand_edits_captured,omitempty"`).
     - Apply those hand-edit ops to the in-memory `dossiersMap` (`applyBatch(dossiersMap, entryIndex, &ops.OpBatch{Ops: handOps})` and `sortDossier`) so the re-rendered dossier preserves the human prose edit!
     - When `!opts.DryRun` and `len(opts.OpsDirs) > 0`, also persist the `ops.NewBatch(handOps)` to `opts.OpsDirs[0]` via `ops.NewWriter(opts.OpsDirs[0]).Write(batch)` so the op-log stays in sync with the captured hand edit.
   - Add a unit test in `internal/core/dossiers/reproject_test.go` verifying that a human prose edit to an existing entry in `.memory/dossiers/<scope>.md` is captured into `.memory/ops/` and preserved across `Reproject` instead of being silently overwritten.

---

## 5. Verification & Acceptance Criteria
Run and verify all of the following commands:
1. `go build ./cmd/... ./internal/... ./packaging/...`
2. `go vet ./cmd/... ./internal/... ./packaging/...`
3. `GOOS=windows GOARCH=amd64 go build ./cmd/... ./internal/... ./packaging/...`
4. `go test -race ./cmd/... ./internal/... ./packaging/...`
5. `go run ./cmd/repo-memory doctor .`
6. `go run ./cmd/repo-memory reproject --dry-run --json`
7. `~/.local/bin/repo-memory --help`
8. Check `git status --short` to ensure only permitted files (`README.md` and `WORKORDER.md`) are modified.

---

## 6. Worker Completion Report

- **Summary of Changes:**
  - Updated `README.md` (`## Architecture & Storage Layout`) to document `.memory/QUESTIONS.md` (`# Unresolved open issues, suspect entries, and coverage gaps`) alongside `.memory/ATLAS.md`.
  - Updated `README.md` (`## CLI Reference`) with the complete `v2.2` CLI reference covering Installation & Repository Enrolment (`install`, `uninstall`, `enroll`/`init`, `unenroll`/`leave`), Seeding & Op-Log Maintenance (`seed-docs`, `seed`, `ops gc`), Consolidation, Skill Graduation & Gardening (`consolidate`, `promote-skills`, `garden`), Principal Architect Integration (`summary`, `brief`), Backfill, Reprojection & Indexing (`backfill`/`backfill-captures`, `reproject`, `reindex`), and Background Tick, Observability & Diagnostics (`tick`/`run`, `status`, `search`, `usage`, `report`, `snapshot`, `doctor`, `disable`, `enable`).
  - Rebuilt `~/.local/bin/repo-memory` from `cmd/repo-memory`.
- **Verification Results:**
  - `go build ./cmd/... ./internal/... ./packaging/...`: **PASS**
  - `go vet ./cmd/... ./internal/... ./packaging/...`: **PASS**
  - `GOOS=windows GOARCH=amd64 go build ./cmd/... ./internal/... ./packaging/...`: **PASS**
  - `go test -race ./cmd/... ./internal/... ./packaging/...`: **PASS** (all packages passed with `-race`, including `cmd/repo-memory` in `17.16s` and `internal/harness/antigravity` in `294.91s`)
  - `go run ./cmd/repo-memory doctor .`: **PASS** (`0` errors)
  - `go run ./cmd/repo-memory reproject --dry-run --json`: **PASS** (`30` dossiers projected, `259` active entries, `225` human decisions preserved, `30` files unchanged)
  - `~/.local/bin/repo-memory --help`: **PASS** (verified all `v2.2` subcommands registered and available)
  - `git status --short`: **PASS** (strictly limited to `README.md` and `WORKORDER.md`)
- **Out-of-Scope Findings / Dead Ends:** None.
