# WORKORDER: WO4 — Documentation, CLI Reference & Programme Close-Out

## 1. Metadata & Branch Topology
- **Parent Project:** `pr-16-v2-pa-and-rollout`
- **Work Order Slug:** `wo4-programme-closeout-and-handover`
- **Target Branch:** `pr-16-v2-pa-and-rollout/wo4-programme-closeout-and-handover`
- **Base Branch:** `pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging`
- **Status:** `IN_PROGRESS`

---

## 2. Context & Architectural Intent
As the final work order (`WO4`) of `PR-16-v2-pa-and-rollout` (`22/22`), this work order updates `README.md` with the complete `v2.2` CLI subcommand reference (`install`, `uninstall`, `enroll`, `unenroll`, `seed`, `seed-docs`, `ops gc`, `consolidate`, `promote-skills`, `garden`, `summary`, `brief`, `backfill`, `reproject`, `reindex`, `tick`, `status`, `search`, `usage`, `report`, `snapshot`, `doctor`, `disable`, `enable`) and `.memory/QUESTIONS.md` storage layout, rebuilds the local `~/.local/bin/repo-memory` binary, and executes the full cross-platform build, race-detector test, doctor lint, reprojection dry-run, and external hygiene verification gates prior to Step 6A PR Audit.

---

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- `README.md`
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo4-programme-closeout-and-handover/WORKORDER.md` *(append Worker Completion Report only)*

### Forbidden Files (Read-Only Context)
- `SPEC.md`
- `cmd/repo-memory/**`
- `internal/**`
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
*(To be filled in by the Tier 3 Worker before returning control to the Principal Architect)*
