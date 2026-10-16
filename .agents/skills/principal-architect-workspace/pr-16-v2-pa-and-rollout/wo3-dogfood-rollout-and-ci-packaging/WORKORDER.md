# WORKORDER: WO3 — Full Dogfood Rollout & CI/Packaging Verification

## 1. Metadata & Branch Topology
- **Parent Project:** `pr-16-v2-pa-and-rollout`
- **Work Order Slug:** `wo3-dogfood-rollout-and-ci-packaging`
- **Target Branch:** `pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging`
- **Base Branch:** `pr-16-v2-pa-and-rollout/wo2-backfill-and-reproject-cli`
- **Status:** `COMPLETED`

---

## 2. Context & Architectural Intent
With all `v2.2` distillation, retrieval, governance, learning, PA integration (`summary`, `brief`), backfill (`backfill`), and reprojection (`reproject`) capabilities implemented across `PR-10a` through `PR-16 WO2`, `WO3` executes the full dogfood rollout on `repo-memory-plugin` itself and upgrades the GitHub Actions CI pipeline:
1. **Deterministic Reproject Default (`cmd/repo-memory/cmd_reproject.go`)**:
   - Ensure `repo-memory reproject` replays only `<repoRoot>/.memory/ops` by default so reprojection of committed `.memory/` state is 100% deterministic across developer machines and CI runners regardless of host `<dataDir>` contents. Add an optional `--include-local` flag on `reprojectCmd` for callers that explicitly want to overlay `<dataDir>/repos/<repoID>/ops-local` and `<dataDir>/user/ops`.
2. **Full Dogfood Rollout on `repo-memory-plugin` (`.memory/**` and `.agents/skills/rm-*/**`)**:
   - Run `repo-memory seed`, `repo-memory seed-docs`, `repo-memory ops gc --dossiers-dir .memory`, `repo-memory reproject`, `repo-memory promote-skills`, `repo-memory reindex`, and `repo-memory doctor`.
   - Verify **zero second-run churn**: running `seed`, `seed-docs`, `ops gc --dossiers-dir .memory`, `reproject`, and `promote-skills` a second time must produce zero file modifications (`git status --short` unchanged between run 1 and run 2).
   - Verify **strict external hygiene**: ensure zero internal corporate references or local absolute home directory paths appear in `.memory/ops/`, `.memory/dossiers/`, `.memory/ATLAS.md`, or `.memory/QUESTIONS.md`.
3. **CI Pipeline Verification (`.github/workflows/ci.yml`)**:
   - Update `.github/workflows/ci.yml` to run:
     - `go vet ./cmd/... ./internal/... ./packaging/...`
     - `go build -o repo-memory ./cmd/repo-memory` (or `.exe` on Windows)
     - `go test -race ./cmd/... ./internal/... ./packaging/...` on `ubuntu-latest` / `macos-latest` and `go test ./cmd/... ./internal/... ./packaging/...` on `windows-latest`
     - `./repo-memory doctor .` and `./repo-memory reproject --dry-run .` to enforce instruction-block lint, graduated skill lint, and op-log reprojection preservation in CI.

---

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- `cmd/repo-memory/cmd_reproject.go` *(add `--include-local` flag so default `reproject` only replays `.memory/ops`)*
- `.memory/ops/**`
- `.memory/dossiers/**`
- `.memory/ATLAS.md`
- `.memory/QUESTIONS.md`
- `.agents/skills/rm-*/**` *(if any procedural recipes qualify for graduation)*
- `.github/workflows/ci.yml`
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md` *(append Worker Completion Report only)*

### Forbidden Files (Read-Only Context)
- `internal/core/dossiers/reproject.go`
- `internal/core/seed/doc_seed.go`
- `internal/core/seed/gc.go`
- `internal/core/seed/scanner.go`
- `.memory/config.yaml`

---

## 4. Detailed Implementation Instructions

### A. Make `repo-memory reproject` Default to Repo `.memory/ops` (`cmd/repo-memory/cmd_reproject.go`)
- Add `reprojectIncludeLocal bool` (`--include-local`, default `false`).
- Only append `<dataDir>/repos/<repoID>/ops-local`, `<dataDir>/<repoID>/ops-local`, and `<dataDir>/user/ops` to `opsDirs` when `reprojectIncludeLocal` is `true`.

### B. Execute Dogfood Rollout & Verify Second-Run Idempotency
1. Build the CLI binary to a temporary path outside the repo (e.g. `/tmp/repo-memory-wo3`):
   ```bash
   go build -o /tmp/repo-memory-wo3 ./cmd/repo-memory
   ```
2. Run the dogfood pipeline on the repository root:
   ```bash
   /tmp/repo-memory-wo3 seed
   /tmp/repo-memory-wo3 seed-docs
   /tmp/repo-memory-wo3 ops gc --dossiers-dir .memory
   /tmp/repo-memory-wo3 reproject --allow-loss
   /tmp/repo-memory-wo3 promote-skills
   /tmp/repo-memory-wo3 reindex
   /tmp/repo-memory-wo3 doctor .
   ```
   *(Note: `--allow-loss` may be used on the initial `reproject` if `ops gc` pruned polluted scopes/stale IDs from old `.memory/dossiers/`; immediately afterwards, run `/tmp/repo-memory-wo3 reproject` **without** `--allow-loss` and verify it succeeds with 0 lost entries and 0 updated files!)*
3. Verify **zero second-run churn**:
   - Run `/tmp/repo-memory-wo3 seed`, `/tmp/repo-memory-wo3 seed-docs`, `/tmp/repo-memory-wo3 ops gc --dossiers-dir .memory`, `/tmp/repo-memory-wo3 reproject`, and `/tmp/repo-memory-wo3 promote-skills` a second time.
   - Verify `/tmp/repo-memory-wo3 reproject --dry-run --json` reports `"files_added": null`, `"files_updated": null`, `"files_removed": null`, and `"questions_updated": false`.
4. Audit `.memory/` for hygiene:
   - Check that no local absolute paths (such as `/usr/local/google/home/...`) or internal corporate identifiers appear in `.memory/ops/`, `.memory/dossiers/`, `.memory/ATLAS.md`, or `.memory/QUESTIONS.md`. If any workorder completion report seeded a local path into `.memory/ops/`, sanitize or prune it via `ops gc` so `.memory/` is 100% clean.
5. Also smoke-test the new PA commands on the dogfooded repository:
   - Run `/tmp/repo-memory-wo3 brief --workorder .agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md` and verify it outputs a valid `<= 600`-token workorder brief.
   - Remove `/tmp/repo-memory-wo3` when done.

### C. Upgrade GitHub Actions CI (`.github/workflows/ci.yml`)
- Update `.github/workflows/ci.yml` so each matrix OS runs:
  1. `go vet ./cmd/... ./internal/... ./packaging/...`
  2. `go build ./cmd/repo-memory`
  3. `go test -race ./cmd/... ./internal/... ./packaging/...` (on Linux/macOS) and `go test ./cmd/... ./internal/... ./packaging/...` (on Windows)
  4. `go run ./cmd/repo-memory doctor .` and `go run ./cmd/repo-memory reproject --dry-run .`

---

## 5. Verification & Acceptance Criteria
Run and verify all of the following commands:
1. `go build ./cmd/repo-memory`
2. `go vet ./cmd/... ./internal/... ./packaging/...`
3. `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`
4. `go run ./cmd/repo-memory reproject --dry-run --json` (verify 0 files added/updated/removed and `questions_updated: false`)
5. `go run ./cmd/repo-memory doctor .`
6. `go test -race ./cmd/... ./internal/... ./packaging/...`
7. Check `git status --short` to ensure only permitted files are modified/added.

---

## 6. Worker Completion Report

### Summary of Changes
1. **Deterministic Reproject Default (`cmd/repo-memory/cmd_reproject.go`)**:
   - Added `--include-local` (`reprojectIncludeLocal`, default `false`) to `reprojectCmd` so `repo-memory reproject` replays `<repoRoot>/.memory/ops` deterministically by default across developer machines and CI runners without host `<dataDir>` overlay drift, while allowing callers to opt in to overlaying `ops-local` and `user/ops`.
2. **Full Dogfood Rollout on `repo-memory-plugin` (`.memory/**`)**:
   - Executed `seed`, `seed-docs`, `ops gc --dossiers-dir .memory`, `reproject`, `promote-skills`, `reindex`, and `doctor .` across the full `v2.2` repository (`PR-10a` through `PR-16`).
   - Seeded 37 new entries across 16 workorders (`PR-12` through `PR-16`), repaired 7 polluted scopes via `ops gc`, reprojected 30 dossiers (259 active entries, 225 human decisions preserved), and generated `.memory/ATLAS.md` and `.memory/QUESTIONS.md`.
   - Verified zero second-run churn (`diff -u` of `git status --short` and `git diff` sha256 identical between run 1 and run 2; `reproject --dry-run --json` reports `0` added/updated/removed files and `"questions_updated": false`).
   - Verified 0 hygiene hits across `.memory/ops/`, `.memory/dossiers/`, `.memory/ATLAS.md`, and `.memory/QUESTIONS.md`.
   - Smoke-tested `repo-memory brief --workorder .agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo3-dogfood-rollout-and-ci-packaging/WORKORDER.md`.
3. **GitHub Actions CI Upgrade (`.github/workflows/ci.yml`)**:
   - Updated `.github/workflows/ci.yml` to run `go vet ./cmd/... ./internal/... ./packaging/...`, `go build ./cmd/repo-memory`, `go test -race ./cmd/... ./internal/... ./packaging/...` on Linux/macOS (`go test` on Windows), `go run ./cmd/repo-memory doctor .`, and `go run ./cmd/repo-memory reproject --dry-run .`.

### Files Modified / Created
- `cmd/repo-memory/cmd_reproject.go`: Added `--include-local` flag so default `reproject` only replays `.memory/ops`.
- `.github/workflows/ci.yml`: Upgraded CI workflow steps (`go vet`, `go build`, `go test -race`, `doctor .`, and `reproject --dry-run .`).
- `.memory/ATLAS.md`: Regenerated repository memory atlas across 30 dossiers.
- `.memory/QUESTIONS.md`: Generated open-issues and coverage gaps report.
- `.memory/dossiers/**`: Updated and added projected module dossiers (`core/consolidate.md`, `evaluation/eval-v2.md`, `evaluation/eval-v2/rubrics.md`, etc.).
- `.memory/ops/2026-10/*.json`: Added seeded workorder op batches and cleaned stale/polluted scopes via `ops gc`.

### Verification Results
- `go build ./cmd/repo-memory`: **PASS**
- `go vet ./cmd/... ./internal/... ./packaging/...`: **PASS**
- `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`: **PASS**
- `go run ./cmd/repo-memory reproject --dry-run --json`: **PASS** (`30` dossiers projected, `259` active entries, `225` human decisions preserved, `31` files unchanged, `0` added/updated/removed, `questions_updated: false`)
- `go run ./cmd/repo-memory doctor .`: **PASS** (0 errors)
- `go test -race ./cmd/... ./internal/... ./packaging/...`: **PASS** (all packages passed)
