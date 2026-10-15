# Work Order: wo4-ops-cleanup-and-rebuild

> **Project:** pr-10b-delivery-and-storage-fixes · **WO:** 4 of 5
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo4-ops-cleanup-and-rebuild`
> **Base:** `pr-10b-delivery-and-storage-fixes/wo3-seeder-fixes`
> **Complexity:** standard

## Objective

Resolve GitHub Issue **#16** by implementing `repo-memory ops gc` (`internal/core/seed/gc.go` + `cmd/repo-memory/cmd_ops_gc.go`), cleaning junk placeholder ops, stale entry `seed-b1617c95`, and invalid non-repo-path anchors/scopes from `evaluation/data/*/` and `.memory/`, re-rendering dossiers and `ATLAS.md`, removing the `deleteJunkEntries()` stopgap from `evaluation/cmd/build-index/main.go`, rebuilding `~/.local/bin/repo-memory`, and re-indexing `<data-dir>/<repo-id>/index.db`.

## Context

1. **GitHub Issue #16 (`Migration: clean junk placeholder entries from existing ops logs`):**
   - Existing ops logs in `evaluation/data/*/ops/` (and newly seeded `.memory/ops/`) need a first-class GC/migration mechanism (`repo-memory ops gc`) that:
     1. Prunes ops whose `fields["text"]` matches `isJunkText` (`None.`, `None`, `(pending)`, `N/A`, `pending`, `tbd`, `todo`, `None encountered...`, `None identified...`, `No issues...`, `Nothing to...`).
     2. Prunes known-false stale entry IDs — specifically `seed-b1617c95` (*"Antigravity completely ignores `systemMessage` and `userMessage` fields returned from `PreInvocation`..."*, contradicted by `8d7ad7b`).
     3. Sanitizes `fields["anchors"]` on remaining ops using `IsValidAnchorPathSyntax` (from WO3) so bogus tokens (`*claims.Claim`, `cand.EpisodeID`, `*_test.go`, `<data-dir>/<repo-id>/metrics/*.jsonl`, `~/.gemini/...`, `modernc.org/...`) are stripped, and re-resolves polluted `dossier_scope` values (`<data-dir>/<repo-id>`, `modernc.org`, `.agents`, `pending`, `~`) via `config.ScopeResolver` (falling back to `"root"` when no valid scope remains).
     4. Deletes batch `.json` files whose `ops` array becomes empty after pruning, or writes back updated batch files.
     5. When `--dossiers-dir <dir>` is set, removes stale `<dir>/dossiers` and `<dir>/ATLAS.md`, projects dossiers from the cleaned `--ops-dir` via `dossiers.NewProjector`, and writes fresh `<dir>/dossiers/` and `<dir>/ATLAS.md` via `dossiers.NewWriter`.
   - Once `ops gc` is implemented and run across `evaluation/data/*/` and `.memory/`, remove the `deleteJunkEntries()` stopgap from `evaluation/cmd/build-index/main.go` (Issue #16 item 4) and remove the stray root-level binary `build-index` and one-off script `render_dossiers.go`.

2. **Binary Rebuild & Re-index:**
   - Rebuild and install `~/.local/bin/repo-memory` (`go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`).
   - Seed `.memory/ops/` from `.agents/skills/principal-architect-workspace/`, run `repo-memory ops gc --ops-dir .memory/ops --dossiers-dir .memory`, and run `~/.local/bin/repo-memory reindex .` so the repository's own `.memory/ops/`, `.memory/dossiers/`, `.memory/ATLAS.md`, and SQLite `index.db` are clean and up to date.

## Files to Modify / Create

| File | Action | Why |
|---|---|---|
| `internal/core/seed/convert.go` | Modify | Export `IsJunkText(text string) bool` (keep `isJunkText` alias if needed) for use by `ops gc`. |
| `internal/core/seed/parser.go` | Modify | Export `IsValidAnchorPathSyntax(p string) bool` for use by `ops gc`. |
| `internal/core/seed/gc.go` | Create | Implement `GCOptions`, `GCStats`, `DefaultStaleEntryIDs` (`"seed-b1617c95"`), and `RunOpsGC(ctx, opts)` (`#16`). |
| `internal/core/seed/gc_test.go` | Create | Unit tests verifying `RunOpsGC` prunes junk ops, prunes `seed-b1617c95`, strips invalid anchors, repairs polluted scopes, deletes empty batch files, and re-renders dossiers/ATLAS. |
| `cmd/repo-memory/cmd_ops_gc.go` | Create | Add `repo-memory ops gc` CLI subcommand (`--ops-dir`, `--dossiers-dir`, `--config`, `--prune-id`, `--dry-run`) and register `opsCmd` on `rootCmd`. |
| `evaluation/cmd/build-index/main.go` | Modify | Remove the `deleteJunkEntries()` stopgap function and call (`#16` item 4). |
| `build-index` | Delete | Remove stray compiled binary tracked at repository root. |
| `render_dossiers.go` | Delete | Remove one-off root script superseded by `repo-memory ops gc --dossiers-dir`. |
| `evaluation/data/*/{ops,dossiers,ATLAS.md}` | Modify | Clean all 8 `evaluation/data/<wo>` snapshots using `repo-memory ops gc` and re-render their `dossiers/` and `ATLAS.md`. |
| `.memory/{ops,dossiers,ATLAS.md}` | Create/Modify | Seed `.memory/ops/`, run `repo-memory ops gc --ops-dir .memory/ops --dossiers-dir .memory`, and rebuild `<data-dir>/<repo-id>/index.db`. |

## Atomic Implementation Checklist

- [x] **Step 1: Export helpers & implement `RunOpsGC` in `internal/core/seed/gc.go` (`#16`)**
  - Export `IsJunkText` in `internal/core/seed/convert.go` and `IsValidAnchorPathSyntax` in `internal/core/seed/parser.go`.
  - Create `internal/core/seed/gc.go` with:
    - `DefaultStaleEntryIDs = []string{"seed-b1617c95"}`
    - `GCOptions{OpsDir, DossiersDir, ConfigPath string; PruneIDs []string; DryRun bool}`
    - `GCStats{BatchesScanned, BatchesModified, BatchesDeleted, OpsRemoved, AnchorsRemoved, ScopesRepaired int}`
    - `RunOpsGC(ctx context.Context, opts GCOptions) (*GCStats, error)`:
      - Walk all `.json` files in `opts.OpsDir`.
      - Pass 1: Read all batches, filter out ops where `IsJunkText(text)` or `pruneSet[op.EntryID]` is true, filter each surviving op's `fields["anchors"]` (handling both `[]string` and `[]any` from JSON unmarshaling) using `IsValidAnchorPathSyntax`, and collect all surviving valid anchors across all batches.
      - Pass 2: Build `ScopeResolver` (`config.LoadScopeConfig(opts.ConfigPath)` + `config.DeriveModuleScopes(allValidAnchors)`). For each surviving op, if `isPollutedScope(op.DossierScope)` is true (e.g. contains `<`, `>`, `~`, `*`, `..`, or equals `modernc.org`, `.agents`, `pending`, or starts with `.`) or if `resolved := resolver.ResolveScope(cleanAnchors, nil)` is non-empty and `isPollutedScope(op.DossierScope)`:
        - Set `op.DossierScope = resolved` if `resolved != ""`, else `"root"`.
      - Write modified batches back with `json.MarshalIndent(batch, "", "  ")` (or `os.Remove` if `len(batch.Ops) == 0`).
      - If `opts.DossiersDir != ""` and `!opts.DryRun`:
        - Remove `filepath.Join(opts.DossiersDir, "dossiers")` and `filepath.Join(opts.DossiersDir, "ATLAS.md")`.
        - Project dossiers from `ops.NewReader(opts.OpsDir)` via `dossiers.NewProjector(reader).Project(ctx)` and write them via `dossiers.NewWriter(opts.DossiersDir).WriteAll(dossierMap)`.
- [x] **Step 2: Add `repo-memory ops gc` CLI command in `cmd/repo-memory/cmd_ops_gc.go` and unit tests in `internal/core/seed/gc_test.go`**
  - Create `cmd/repo-memory/cmd_ops_gc.go` registering `ops` parent command and `gc` subcommand (`repo-memory ops gc`) with flags `--ops-dir` (default `.memory/ops`), `--dossiers-dir` (default `""`), `--config` (default `.memory/config.yaml`), `--prune-id` (`StringSlice`), and `--dry-run`.
  - Add `TestRunOpsGC_PrunesJunkStaleAndRepairsScopes` in `internal/core/seed/gc_test.go`.
- [x] **Step 3: Remove `deleteJunkEntries` stopgap and root-level stray files**
  - Remove `deleteJunkEntries` from `evaluation/cmd/build-index/main.go`.
  - `git rm build-index render_dossiers.go`.
- [x] **Step 4: Rebuild `~/.local/bin/repo-memory`, clean `evaluation/data/*/`, seed `.memory/`, and re-index**
  - Build and install `~/.local/bin/repo-memory`: `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`.
  - Run `~/.local/bin/repo-memory ops gc` across each of the 8 `evaluation/data/<wo>` directories (`--ops-dir evaluation/data/<wo>/ops --dossiers-dir evaluation/data/<wo>`).
  - Run `~/.local/bin/repo-memory seed --pa-dir .agents/skills/principal-architect-workspace/ --ops-dir .memory/ops/` followed by `~/.local/bin/repo-memory ops gc --ops-dir .memory/ops --dossiers-dir .memory` and `~/.local/bin/repo-memory reindex .`.
  - Verify `git grep "seed-b1617c95" .memory/ evaluation/data/` returns 0 matches and no `<data-dir>`, `modernc.org.md`, `.agents.md`, or `pending.md` files exist under `.memory/dossiers/` or `evaluation/data/*/dossiers/`.

## Verification Criteria

- [x] `go test -v ./internal/core/seed/... ./cmd/repo-memory/...` passes.
- [x] `go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...` and `go vet ./cmd/... ./internal/...` pass.
- [x] `git grep "seed-b1617c95" .memory/ evaluation/data/` returns no matches.
- [x] `~/.local/bin/repo-memory reindex .` succeeds and builds a valid `index.db`.

## Anti-Patterns — Do NOT

- **Do NOT** modify archived historical transcripts in `evaluation/repo-memory-transcripts/`, `evaluation/esp32-transcripts/`, or `evaluation/results/archive/`.
- **Do NOT** include any internal paths or external hygiene violations in code or comments.

---

## Completion Report

### Status
✅ Complete

### Changes Made
- `internal/core/seed/convert.go` — Exported `IsJunkText` (preserving `isJunkText` wrapper) for reuse in `ops gc`.
- `internal/core/seed/parser.go` — Exported `IsValidAnchorPathSyntax` (preserving `isValidAnchorPathSyntax` wrapper) for reuse in `ops gc`.
- `internal/core/seed/gc.go` — Implemented `DefaultStaleEntryIDs` (`"seed-b1617c95"`), `GCOptions`, `GCStats`, and `RunOpsGC` to prune placeholder/stale ops, strip non-path anchor tokens, repair polluted `dossier_scope` values via `config.ScopeResolver`, remove empty batch files, and re-project `dossiers/` and `ATLAS.md`.
- `internal/core/seed/gc_test.go` — Added `TestRunOpsGC_PrunesJunkStaleAndRepairsScopes` covering junk/stale pruning, anchor sanitization, scope repair, empty batch deletion, and dossier/ATLAS re-rendering.
- `cmd/repo-memory/cmd_ops_gc.go` — Added `repo-memory ops gc` CLI subcommand (`--ops-dir`, `--dossiers-dir`, `--config`, `--prune-id`, `--dry-run`).
- `cmd/repo-memory/main.go` — Registered `opsCmd` on `rootCmd`.
- `evaluation/cmd/build-index/main.go` — Removed the `deleteJunkEntries()` stopgap (`#16` item 4).
- `build-index`, `render_dossiers.go` — Removed stray root-level compiled binary and one-off rendering script.
- `evaluation/data/*/` — Cleaned all 8 `evaluation/data/<wo>` snapshots via `repo-memory ops gc` and re-rendered their `dossiers/` and `ATLAS.md`.
- `.memory/` — Seeded `.memory/ops/` from `.agents/skills/principal-architect-workspace/`, ran `repo-memory ops gc --ops-dir .memory/ops --dossiers-dir .memory`, and rebuilt `<data-dir>/<repo-id>/index.db` via `~/.local/bin/repo-memory reindex .`.

### Decisions Made
- Filtered module-derivation anchor inputs in `RunOpsGC` to multi-segment non-dot paths (`strings.Contains(a, "/") && !strings.HasPrefix(a, ".")`) so root-level files (`Makefile`, `.gitignore`) and dot-directories (`.agents/...`, `.memory/...`) map cleanly to `"root"` without creating spurious `.agents` or `Makefile` module scopes during scope repair.

### Dead Ends & Warnings
- None.

### Out-of-Scope Findings
- None.
