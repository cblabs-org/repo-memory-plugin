# Work Order: wo3-seeder-fixes

> **Project:** pr-10b-delivery-and-storage-fixes · **WO:** 3 of 5
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo3-seeder-fixes`
> **Base:** `pr-10b-delivery-and-storage-fixes/wo2-index-and-projector-fixes`
> **Complexity:** standard

## Objective

Fix GitHub Issue **#10** (seeder extracts arbitrary backticked code tokens, globs, placeholders, and external/home paths as file anchors) and GitHub Issue **#12** (`repo-memory seed --before <sha>` stamps seeded ops with `HEAD`'s `git log` timestamp for the workorder file instead of the timestamp as of `<sha>`).

## Context

1. **GitHub Issue #10 (`internal/core/seed/parser.go` & `internal/core/seed/scanner.go`):**
   - Currently, `extractBacktickPaths(s string)` in `internal/core/seed/parser.go` accepts any backticked substring containing `/` or `.` without spaces:
     ```go
     if (strings.Contains(p, "/") || strings.Contains(p, ".")) && !strings.Contains(p, " ")
     ```
   - This lets through Go expressions and symbols (`*claims.Claim`, `cand.EpisodeID`, `Reader.IterateBefore`, `!config.IsEnrolledFast(cwd)`), globs (`*_test.go`), documentation placeholders (`<data-dir>/<repo-id>/metrics/*.jsonl`), home-directory paths (`~/.gemini/antigravity/hooks/hooks.json`), and external module paths (`modernc.org/sqlite`).
   - Downstream, `cmd_seed.go` passes `c.Path` into `config.DeriveModuleScopes(allAnchors)`, producing invalid dossier scope names (such as `~.md` and `<data-dir>-<repo-id>.md`), and `ReportToOps` attaches those bogus anchors to seeded ops.
   - **Two-Layer Fix:**
     - **Layer 1 (Syntactic Filtering in `parser.go`):** Tighten `extractBacktickPaths` via a helper `isValidAnchorPathSyntax(p string) bool` so pure unit tests (`ParseCompletionReport`) also reject non-path tokens:
       - Trim surrounding whitespace; strip a trailing `/` if present; reject empty strings, `.`, `..`, or strings containing `..` path segments.
       - Reject strings starting with `~`, `/`, `*`, `!`, `<`, `>`, `-`, `$`, or `http:` / `https:`.
       - Reject strings containing any non-path punctuation/metacharacters: `<`, `>`, `*`, `?`, `(`, `)`, `[`, `]`, `{`, `}`, `=`, `;`, `:`, `\`, `"`, `'`, `$`, `|`, `&`, `...`, or `//`.
       - If the path has **no `/`** (a single path segment like `SPEC.md` vs `cand.EpisodeID` or `v2.2`), require `filepath.Ext(p)` (lowercased) to be a recognized source/doc/config file extension (`.go`, `.md`, `.yaml`, `.yml`, `.json`, `.toml`, `.sh`, `.txt`, `.mod`, `.sum`, `.sql`, `.py`, `.js`, `.ts`, `.c`, `.h`, `.cpp`, `.rs`, `.proto`, `.mk`, `.gitignore`) or an exact root filename (`Makefile`, `Dockerfile`, `LICENSE`).
       - If the path **contains `/`**, reject if the first segment contains a `.` in the middle (e.g. `modernc.org/...`, `github.com/...`) — leading-dot repo directories like `.memory/...`, `.agents/...`, `.github/...` are valid and must be allowed.
     - **Layer 2 (Repository Tree Validation in `scanner.go`):** When `opts.RepoRoot != ""` in `ScanAndParse`:
       - Build a helper validator that checks whether a candidate relative path `p` (cleaned via `filepath.Clean(strings.TrimSuffix(p, "/"))`) corresponds to a real file or directory in the repository:
         - When `opts.BeforeCommit != ""`, run `git ls-tree -r --name-only <BeforeCommit>` once in `opts.RepoRoot` to populate a map of all tracked file paths and their parent directory prefixes at `<BeforeCommit>`. A candidate path `p` is valid if it exists in that `<BeforeCommit>` path/dir set (if `git ls-tree` fails, fall back to `os.Stat(filepath.Join(opts.RepoRoot, p))`).
         - When `opts.BeforeCommit == ""`, a candidate path `p` is valid if `os.Stat(filepath.Join(opts.RepoRoot, p))` succeeds (`err == nil`).
       - Filter `report.Changes` in `ScanAndParse` so only validated repository paths remain on `report.Changes`.

2. **GitHub Issue #12 (`internal/core/seed/scanner.go`):**
   - In `ScanAndParse` (`internal/core/seed/scanner.go:103`), commit timestamp lookup currently runs:
     ```go
     cmd := exec.Command("git", "log", "-1", "--format=%cI", "--", relPath)
     ```
     Because it omits `opts.BeforeCommit`, `git log` always queries `HEAD` even when `--before <commit>` is specified, leaking post-cutoff commit timestamps into time-cut seeded ops.
   - **Fix:** When `opts.BeforeCommit != ""`, pass `opts.BeforeCommit` as the revision range argument:
     ```go
     var cmd *exec.Cmd
     if opts.BeforeCommit != "" {
         cmd = exec.Command("git", "log", "-1", "--format=%cI", opts.BeforeCommit, "--", relPath)
     } else {
         cmd = exec.Command("git", "log", "-1", "--format=%cI", "--", relPath)
     }
     ```

## Relevant Institutional Memory

- **Hermetic git commands in tests:** Whenever creating git commits in `seed_test.go`, pass `-c user.name=test -c user.email=test@example.com` and explicit `GIT_COMMITTER_DATE` / `GIT_AUTHOR_DATE` environment variables so tests are deterministic and hermetic.

## Files to Modify

| File | Action | Why |
|---|---|---|
| `internal/core/seed/parser.go` | Modify | Tighten `extractBacktickPaths` with `isValidAnchorPathSyntax` to reject code expressions, globs, `<...>` placeholders, `~/` paths, and external module paths (`#10`). |
| `internal/core/seed/scanner.go` | Modify | Filter `report.Changes` against the repository tree (`git ls-tree` at `BeforeCommit` or `os.Stat` under `RepoRoot`) when `opts.RepoRoot != ""` (`#10`), and pass `opts.BeforeCommit` to `git log -1 --format=%cI` when `opts.BeforeCommit != ""` (`#12`). |
| `internal/core/seed/seed_test.go` | Modify | Add unit tests for `#10` (syntactic filtering + repo tree filtering) and `#12` (`BeforeCommit` historical timestamp vs `HEAD`). |

## Atomic Implementation Checklist

- [x] **Step 1: Syntactic path validation in `internal/core/seed/parser.go` (`#10`)**
  - Implement `isValidAnchorPathSyntax(p string) bool` and apply it in `extractBacktickPaths(s string)` (normalizing a trailing `/` if present, or keeping clean relative paths) so that:
    - Tokens starting with `~`, `/`, `*`, `!`, `<`, `>`, `-`, `$` are rejected.
    - Tokens containing `<`, `>`, `*`, `?`, `(`, `)`, `[`, `]`, `{`, `}`, `=`, `;`, `:`, `\`, `"`, `'`, `$`, `|`, `&`, `...`, `//`, or `..` segments are rejected.
    - Single-segment tokens (no `/`) are accepted only if they match `Makefile`, `Dockerfile`, `LICENSE`, or have a lowercase file extension in the allowed set (`.go`, `.md`, `.yaml`, `.yml`, `.json`, `.toml`, `.sh`, `.txt`, `.mod`, `.sum`, `.sql`, `.py`, `.js`, `.ts`, `.c`, `.h`, `.cpp`, `.rs`, `.proto`, `.mk`, `.gitignore`), rejecting `cand.EpisodeID`, `Reader.IterateBefore`, `*claims.Claim`, `v1.0`, etc.
    - Multi-segment tokens (containing `/`) whose first segment contains a `.` after index 0 (e.g. `modernc.org/sqlite`, `github.com/foo/bar`) are rejected, while leading-dot repo directories (`.memory/...`, `.agents/...`, `.github/...`) are accepted.
- [x] **Step 2: Repository path validation & `--before` timestamp fix in `internal/core/seed/scanner.go` (`#10`, `#12`)**
  - In `ScanAndParse(paDir string, opts SeedOptions)`:
    - When `opts.RepoRoot != ""`, build a path validator:
      - If `opts.BeforeCommit != ""`, run `git ls-tree -r --name-only <opts.BeforeCommit>` (with `cmd.Dir = opts.RepoRoot`) to build a `map[string]bool` containing every file path and every ancestor directory path (`filepath.Dir` loop up to `.`) present in `<opts.BeforeCommit>`.
      - Filter `report.Changes` so a change `c` is retained only if `clean := filepath.Clean(strings.TrimSuffix(c.Path, "/"))` is in the `<BeforeCommit>` tree map (when `BeforeCommit != ""` and `ls-tree` succeeded) or `os.Stat(filepath.Join(opts.RepoRoot, clean))` succeeds.
    - When querying `commitTime` for `relPath`:
      - If `opts.BeforeCommit != ""`, execute `exec.Command("git", "log", "-1", "--format=%cI", opts.BeforeCommit, "--", relPath)`.
      - Otherwise, execute `exec.Command("git", "log", "-1", "--format=%cI", "--", relPath)`.
- [x] **Step 3: Unit tests in `internal/core/seed/seed_test.go` (`#10`, `#12`)**
  - Add `TestParseCompletionReport_FiltersNonPathTokens` testing that `*claims.Claim`, `cand.EpisodeID`, `Reader.IterateBefore`, `!config.IsEnrolledFast(cwd)`, `*_test.go`, `<data-dir>/<repo-id>/metrics/*.jsonl`, `~/.gemini/antigravity/hooks/hooks.json`, and `modernc.org/sqlite` are rejected while `internal/core/seed/parser.go`, `.memory/config.yaml`, and `SPEC.md` are retained.
  - Add `TestScanAndParse_ValidatesRepoPathsAndBeforeCommitTimestamp` creating a temporary git repo (`git init` + `-c user.name=test -c user.email=test@example.com`):
    - Commit 1 at `2026-01-10T10:00:00Z` (`GIT_AUTHOR_DATE` & `GIT_COMMITTER_DATE`): creates `internal/real/file.go` and `.agents/p1/wo1/WORKORDER.md` whose `### Changes Made` mentions both `` `internal/real/file.go` `` and `` `internal/nonexistent/ghost.go` ``.
    - Commit 2 at `2026-06-15T12:00:00Z`: modifies `.agents/p1/wo1/WORKORDER.md` (e.g. appends a comment line) so `HEAD` has a newer commit timestamp for `.agents/p1/wo1/WORKORDER.md`.
    - Run `ScanAndParse` with `RepoRoot: tmpDir, BeforeCommit: commit1SHA`:
      - Assert `len(parsed) == 1`.
      - Assert `parsed[0].Report.Changes` contains only `internal/real/file.go` (and `internal/nonexistent/ghost.go` was filtered out).
      - Assert `parsed[0].CommitTime.UTC()` equals `2026-01-10T10:00:00Z` (Commit 1's timestamp, not Commit 2's `2026-06-15T12:00:00Z`).

## Verification Criteria

- [x] `go test -v ./internal/core/seed/...` passes all existing and new tests.
- [x] `go test ./cmd/... ./internal/...` passes with 0 failures.
- [x] `go vet ./cmd/... ./internal/...` passes cleanly.

## Anti-Patterns — Do NOT

- **Do NOT** break existing `TestParseCompletionReport` (which calls `ParseCompletionReport` without a `RepoRoot`).
- **Do NOT** reject valid dot-prefixed repository directories like `.memory/...` or `.agents/...`.
- **Do NOT** include any internal paths or external hygiene violations in code or comments.

---

## Completion Report

### Status
✅ Complete

### Changes Made
- `internal/core/seed/parser.go` — Added `isValidAnchorPathSyntax` and updated `extractBacktickPaths` to filter out Go expressions, symbol references, globs, template placeholders, home-directory paths, and external module import paths while retaining valid repository paths and dot-prefixed repository directories (`#10`).
- `internal/core/seed/scanner.go` — Added `buildRepoPathValidator` to validate candidate `report.Changes` paths against the repository tree (`git ls-tree` at `BeforeCommit` or `os.Stat` under `RepoRoot`) when `opts.RepoRoot != ""` (`#10`), and passed `opts.BeforeCommit` to `git log -1 --format=%cI` when `opts.BeforeCommit != ""` so `--before` seeding uses cutoff-relative commit timestamps (`#12`).
- `internal/core/seed/seed_test.go` — Added `TestParseCompletionReport_FiltersNonPathTokens` and `TestScanAndParse_ValidatesRepoPathsAndBeforeCommitTimestamp` covering both `#10` and `#12`.

### Decisions Made
- Implemented two-layer anchor path filtering: syntactic validation in `extractBacktickPaths` so pure parser callers reject code expressions and external/home paths without requiring a filesystem root, paired with repository-tree validation (`git ls-tree` at `BeforeCommit` or `os.Stat`) in `ScanAndParse` when `RepoRoot` is provided.

### Dead Ends & Warnings
- None

### Out-of-Scope Findings
- None
