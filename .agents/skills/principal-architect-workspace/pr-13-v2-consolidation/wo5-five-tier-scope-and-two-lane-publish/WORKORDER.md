---
name: pr-13-v2-consolidation-wo5
description: Handoff instructions for Work Order 5 (Five-Tier Scope Storage, Branch-Merge Promotion & Two-Lane Worktree Publish)
complexity: complex
---

# WORKORDER: wo5-five-tier-scope-and-two-lane-publish

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Step 7 of the Stage 2 Warm Batch Consolidation pipeline (`SPEC.md` §5.4, §6.4 step 7, §6.5, §6.8; `docs/design-v2-distillation-retrieval.md` §6 step 7):
1. Update `internal/core/config/layout.go` with `user-dossiers/{ops,dossiers}` and `worktrees` under `<data-dir>/<repo-id>/`, and ensure `.memory/.gitignore` ignores `local/` and `.cache/`.
2. Create `internal/core/gitops/worktree.go` & `worktree_test.go` implementing Two-Lane git worktree publishing (`PublishToMemoryWorktree`) on branch `memory/pending` under `<data-dir>/<repo-id>/worktrees/<clone-id>/` without disturbing the user's primary checkout.
3. Add multi-reader index rebuild support (`RebuildFromOpLogs`) in `internal/core/index/rebuild.go`.
4. Create `internal/core/consolidate/publish.go` & `publish_test.go` implementing Five-Tier recording (`TierUser`, `TierBranch`, `TierRepo`), automatic branch-merge promotion (`PromoteMergedBranchOps`), and multi-tier SQLite index rebuilding (`RebuildMultiTierIndex`).

## Context & Architectural Why
- **Five-Tier State Scope Hierarchy (`VD17`, `SPEC.md` §5.4):**
  - `Tier 3 (user:)`: workstation-local paths, hostnames, credentials, and personal preferences are stored in `<data-dir>/<repo-id>/user-dossiers/{ops,dossiers}` and never enter the shared git repository.
  - `Tier 4 (branch:)`: notes whose anchored files exist only on an unmerged feature branch are stored in gitignored `.memory/local/ops/<branch-slug>/` and `.memory/local/dossiers/<branch-slug>/`. When the branch merges into the default branch, `PromoteMergedBranchOps` promotes those op batches to `.memory/ops/`, deletes the `.memory/local/*/<branch-slug>/` directories, and re-projects `.memory/dossiers/` + `.memory/ATLAS.md`.
  - `Tier 5 (repo:)`: shared repository ops live in `.memory/ops/` and render to `.memory/dossiers/` + `.memory/ATLAS.md`.
- **Two-Lane Publish (`SPEC.md` §6.5 & §6.8):** Local projection makes new entries immediately queryable via `index.db`, while shared `repo:` op batches and rendered dossiers are committed onto branch `memory/pending` inside a dedicated git worktree (`<data-dir>/<repo-id>/worktrees/<clone-id>/`) so the user's active working tree is never switched or dirtied by background git commits.

## Strict Boundary Box
- **Allowed Files to Create/Modify:**
  - `internal/core/config/layout.go`
  - `internal/core/index/rebuild.go`
  - `internal/core/gitops/worktree.go`
  - `internal/core/gitops/worktree_test.go`
  - `internal/core/consolidate/publish.go`
  - `internal/core/consolidate/publish_test.go`
- **Forbidden Actions:**
  - Do NOT modify any files outside the allowed list above.
  - Do NOT change git branches or rebase.
  - `internal/core` MUST NOT import `internal/harness/antigravity`.
  - **External GitHub Hygiene:** NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or `/usr/local/google/...`) in code, comments, or commit messages.
  - Use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"` when committing.

## Tooling Constraints
- Verify with:
  - `go test -v -race ./internal/core/config/... ./internal/core/index/... ./internal/core/gitops/... ./internal/core/consolidate/...`
  - `go vet ./internal/core/...`
  - `GOOS=windows go build ./...`

## Line-Item Instructions

- [ ] **1. Update `internal/core/config/layout.go`:**
  - Add `"user-dossiers"`, `filepath.Join("user-dossiers", "ops")`, `filepath.Join("user-dossiers", "dossiers")`, and `"worktrees"` to `DataDirSubdirectories`.
  - Add `EnsureMemoryGitignore(repoRoot string) error` in `internal/core/config/layout.go` (and call it from `EnsureRepoLayout`):
    - Ensures `<repoRoot>/.memory/.gitignore` exists and contains `.cache/`, `.enrolled`, and `local/` (checking line-by-line or substring and appending any missing entries with trailing newline, without clobbering existing lines).

- [ ] **2. Add `RebuildFromOpLogs` in `internal/core/index/rebuild.go`:**
  - Add `func (idx *Index) RebuildFromOpLogs(ctx context.Context, opsReaders ...*ops.Reader) error`:
    - Filters out nil readers; if no non-nil readers are passed, returns an error `"ops reader cannot be nil"`.
    - Projects each reader via `dossiers.NewProjector(r).Project(ctx)` and merges the resulting dossiers (combining `Entries` and `History` by entry ID/scope) and alias maps across all readers in order, then executes the existing single-transaction SQLite rebuild.
    - Keep `RebuildFromOpLog(ctx context.Context, opsReader *ops.Reader) error` calling `idx.RebuildFromOpLogs(ctx, opsReader)` so all existing callers remain 100% compatible.

- [ ] **3. Implement `internal/core/gitops/worktree.go` and `internal/core/gitops/worktree_test.go`:**
  - Define `MemoryPendingBranch = "memory/pending"` and `WorktreePublishResult`:
    ```go
    type WorktreePublishResult struct {
    	WorktreePath string `json:"worktree_path"`
    	Branch       string `json:"branch"`
    	CommitSHA    string `json:"commit_sha,omitempty"`
    	Committed    bool   `json:"committed"`
    }
    ```
  - Implement `CloneID(repoRoot string) string`:
    - Computes a deterministic 12-hex-character SHA-256 prefix of `filepath.Clean(repoRoot)`.
  - Implement `EnsureMemoryWorktree(ctx context.Context, dataDir, repoID, repoRoot string) (string, error)`:
    - Computes `worktreePath := filepath.Join(dataDir, repoID, "worktrees", CloneID(repoRoot))`.
    - Verifies `repoRoot` is a git repository (`git -C <repoRoot> rev-parse --verify HEAD`).
    - If `worktreePath` does not already contain a valid `.git` file/dir, runs `git -C <repoRoot> worktree prune` and `git -C <repoRoot> worktree add -B memory/pending <worktreePath> HEAD` (creating parent dirs first).
    - Returns `worktreePath, nil`.
  - Implement `PublishToMemoryWorktree(ctx context.Context, dataDir, repoID, repoRoot string, batches []ops.OpBatch) (*WorktreePublishResult, error)`:
    - If `len(batches) == 0`, returns `&WorktreePublishResult{Branch: MemoryPendingBranch, Committed: false}, nil`.
    - Calls `EnsureMemoryWorktree(ctx, dataDir, repoID, repoRoot)`.
    - Writes each `batch` in `batches` into `filepath.Join(worktreePath, ".memory", "ops")` via `ops.NewWriter(...)`.
    - Also syncs any existing shared ops from `filepath.Join(repoRoot, ".memory", "ops")` into `filepath.Join(worktreePath, ".memory", "ops")` if not already present so the worktree projection is complete.
    - Projects all ops in `filepath.Join(worktreePath, ".memory", "ops")` via `dossiers.NewProjector` (setting default `Purpose` when empty on non-empty dossiers) and writes `.memory/dossiers/`, `.history.md`, and `.memory/ATLAS.md` into `filepath.Join(worktreePath, ".memory")` via `dossiers.NewWriter`.
    - Stages `.memory/ops`, `.memory/dossiers`, and `.memory/ATLAS.md` in `worktreePath` (`git -C <worktreePath> add -- .memory`).
    - Checks `git -C <worktreePath> status --porcelain -- .memory`. If non-empty, commits with `git -C <worktreePath> -c user.name=repo-memory -c user.email=repo-memory@local commit -m "chore(memory): consolidate op batch"` and resolves `CommitSHA` (`git -C <worktreePath> rev-parse HEAD`).
    - Never modifies the checked-out branch or staged index of `repoRoot`.
  - Write unit tests in `internal/core/gitops/worktree_test.go` verifying worktree creation on `memory/pending`, batch + dossier commit inside the worktree, idempotency across multiple publishes, and that `repoRoot`'s active branch and `git status` remain untouched.

- [ ] **4. Implement `internal/core/consolidate/publish.go` and `internal/core/consolidate/publish_test.go`:**
  - Implement `EnsureMemoryGitignore(repoRoot string) error` delegating to `config.EnsureMemoryGitignore(repoRoot)`.
  - Implement Five-Tier storage directory helpers:
    - `UserTierBaseDir(dataDir, repoID string) string` -> `filepath.Join(dataDir, repoID, "user-dossiers")`
    - `BranchTierOpsDir(repoRoot, branchSlug string) string` -> `filepath.Join(repoRoot, ".memory", "local", "ops", SanitizeBranchSlug(branchSlug))`
    - `BranchTierDossiersDir(repoRoot, branchSlug string) string` -> `filepath.Join(repoRoot, ".memory", "local", "dossiers", SanitizeBranchSlug(branchSlug))`
    - `RepoTierBaseDir(repoRoot string) string` -> `filepath.Join(repoRoot, ".memory")`
  - Implement `RecordBranchTier(ctx context.Context, repoRoot, branchSlug string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error)`:
    - Ensures `.memory/.gitignore` has `local/` via `EnsureMemoryGitignore(repoRoot)`.
    - Writes `validOps` to `opsDir := BranchTierOpsDir(repoRoot, branchSlug)` and projects dossiers.
    - Renders `.md`, `.history.md`, and `ATLAS.md` into `BranchTierDossiersDir(repoRoot, branchSlug)` (writing `<scope>.md`, `<scope>.history.md`, and `ATLAS.md` directly under `.memory/local/dossiers/<branch-slug>/` as well as via `RecordAndRenderDirs` so both `.memory/local/dossiers/<branch-slug>/<scope>.md` and `dossiers.NewWriter` conventions work seamlessly).
  - Implement `PromoteMergedBranchOps(ctx context.Context, repoRoot string) (*BranchPromotionResult, error)`:
    - Define `BranchPromotionResult`:
      ```go
      type BranchPromotionResult struct {
      	PromotedBranches []string      `json:"promoted_branches,omitempty"`
      	PromotedBatches  []ops.OpBatch `json:"promoted_batches,omitempty"`
      }
      ```
    - Scans `<repoRoot>/.memory/local/ops/` for `<branch-slug>` subdirectories.
    - Determines whether `<branch-slug>` should be promoted (`isBranchReadyForPromotion(repoRoot, branchSlug, batches)`):
      1. Runs `git -C <repoRoot> branch --merged HEAD` when `HEAD` is on the default branch (`main` or `master` or `resolveDefaultBranch(repoRoot)`) and checks if any merged branch name sanitizes (`SanitizeBranchSlug`) to `branchSlug`; OR
      2. When `HEAD` is on the default branch (`main` or `master` or `resolveDefaultBranch(repoRoot)`) and the git branch ref was already deleted after merge, checks whether all file anchors referenced in `batches` now exist on disk under `repoRoot` (`len(anchors) > 0` and every `StripAnchorSymbol(anchor)` exists under `repoRoot`).
    - For each promoted `<branch-slug>`:
      - Copies every `.json` batch file from `.memory/local/ops/<branch-slug>/<yyyy-mm>/<id>.json` into `<repoRoot>/.memory/ops/<yyyy-mm>/<id>.json` (preserving batch IDs and JSON content).
      - Removes `<repoRoot>/.memory/local/ops/<branch-slug>` and `<repoRoot>/.memory/local/dossiers/<branch-slug>`.
      - Appends `branchSlug` to `PromotedBranches` and its batches to `PromotedBatches`.
    - If `len(PromotedBatches) > 0`, re-projects `<repoRoot>/.memory/ops` and writes updated `.memory/dossiers/` and `.memory/ATLAS.md` via `RecordAndRenderTier(ctx, RepoTierBaseDir(repoRoot), nil, nil, nil, time.Now().UTC())`.
  - Implement `RebuildMultiTierIndex(ctx context.Context, idx *index.Index, dataDir, repoID, repoRoot, currentBranch string) error`:
    - Collects `*ops.Reader` instances for:
      1. `<repoRoot>/.memory/ops` (if directory exists),
      2. `<repoRoot>/.memory/local/ops/<SanitizeBranchSlug(currentBranch)>` (if `currentBranch != ""` and directory exists),
      3. `<dataDir>/<repoID>/user-dossiers/ops` (if `dataDir != ""` && `repoID != ""` and directory exists).
    - If at least one ops directory exists, calls `idx.RebuildFromOpLogs(ctx, readers...)`.
  - Write unit tests in `internal/core/consolidate/publish_test.go` covering:
    - `TestEnsureMemoryGitignore` (`local/`, `.cache/`, `.enrolled` idempotency).
    - `TestFiveTierIsolation_UserBranchRepo`: verifies `user:` ops write only to `<dataDir>/<repoID>/user-dossiers/`, `branch:` ops write only to `.memory/local/{ops,dossiers}/<branch-slug>/`, and `repo:` ops write to `.memory/{ops,dossiers}/`.
    - `TestPromoteMergedBranchOps_AndMultiTierIndex`: creates a git repo on `main`, switches to feature branch `feat/new-thing`, records a `branch:` op batch anchored to a file created on `feat/new-thing`, verifies `RebuildMultiTierIndex` includes the `branch:` entry while on `feat/new-thing`, merges `feat/new-thing` into `main`, runs `PromoteMergedBranchOps`, and verifies that:
      1. The batch file moved to `.memory/ops/<yyyy-mm>/<batch>.json`,
      2. `.memory/local/ops/feat-new-thing` and `.memory/local/dossiers/feat-new-thing` were removed,
      3. `.memory/dossiers/` and `.memory/ATLAS.md` were updated, and
      4. `RebuildMultiTierIndex` on `main` indexes both the promoted `repo:` entry and `user:` entries.

## Verification Requirement
Run all tests (`go test -v -race ./internal/core/config/... ./internal/core/index/... ./internal/core/gitops/... ./internal/core/consolidate/...`), `go vet ./internal/core/...`, and `GOOS=windows go build ./...`. Commit your changes on `pr-13-v2-consolidation/wo5-five-tier-scope-and-two-lane-publish`, then **STOP and send a message back to the Principal Architect**.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
- Decision description and rationale

### Dead Ends & Warnings
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
- Finding description and suggested action

### Blockers
(none)
