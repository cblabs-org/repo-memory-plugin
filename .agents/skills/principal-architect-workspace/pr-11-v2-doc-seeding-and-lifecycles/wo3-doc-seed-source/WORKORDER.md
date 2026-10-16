---
name: pr-11-v2-doc-seeding-and-lifecycles-wo3
description: Implement DocSeedSource (internal/core/seed/doc_seed.go), RepoConfig.DocSeeds + ResolveDocSeedGlobs, and repo-memory seed-docs CLI subcommand
complexity: architect
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md`. This `WORKORDER.md` belongs to parent project `pr-11-v2-doc-seeding-and-lifecycles`, whose 5-WO decomposition is explicitly approved by the human in `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md`. You are authorized to implement and commit directly on the current branch (`pr-11-v2-doc-seeding-and-lifecycles/wo3-doc-seed-source`).
>
> **External GitHub Hygiene:** Never include any proprietary internal references (`cl/`, `b/`, `go/`, or local `/home/<user>/...` paths) in any code, comments, tests, or commit messages. Use git author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Objective
Implement `v2.2` Doc-Seeded Dossiers (`SPEC.md` §6.3.3, `docs/design-v2-distillation-retrieval.md` §7.6, `docs/addendum-a-provenance-and-hardening.md` §11.1): add `DocSeeds []string` and `ResolveDocSeedGlobs` to `internal/core/config/config.go`, implement `DocSeedSource` (`internal/core/seed/doc_seed.go`) with deterministic markdown extraction, git blob SHA provenance (`source_doc_path`, `source_doc_blob_sha`), and idempotent diff reconciliation, and expose `repo-memory seed-docs` (`cmd/repo-memory/cmd_seed_docs.go`).

## Strict Boundary Box
- **Allowed to modify / create:**
  - `internal/core/config/config.go`
  - `internal/core/config/config_test.go` (if adding config tests)
  - `internal/core/seed/doc_seed.go` (new)
  - `internal/core/seed/doc_seed_test.go` (new)
  - `cmd/repo-memory/cmd_seed_docs.go` (new)
  - `cmd/repo-memory/main.go`
- **Do NOT touch:**
  - `internal/core/tick/` (WO4)
  - `.memory/` (WO5)

## Tooling Constraints
- Run tests with `go test -race ./internal/core/config/... ./internal/core/seed/... ./cmd/repo-memory/...`.
- Commit with `git -c user.name="matthewmcneill" -c user.email="97467074+matthewmcneill@users.noreply.github.com" commit ...`.

## Line-Item Instructions

- [x] `internal/core/config/config.go` — Add `DocSeeds` to `RepoConfig` and `ResolveDocSeedGlobs(repoRoot string, cfg RepoConfig) ([]string, error)`:
  1. Add `DocSeeds []string` (`json:"doc_seeds,omitempty" yaml:"doc_seeds,omitempty"`) to `RepoConfig`.
  2. Implement `ResolveDocSeedGlobs(repoRoot string, cfg RepoConfig) ([]string, error)`:
     - If `repoRoot == ""` default to `"."`.
     - Patterns to evaluate: `cfg.DocSeeds` if non-empty; otherwise default to `[]string{"docs/**/*.md", ".agents/skills/**/SKILL.md"}`.
     - Expand patterns across `repoRoot` (supporting both exact relative file paths, standard `filepath.Match` globs, and `**` recursive directory walks under the prefix directory before `**`).
     - Normalize all matched files to repo-relative slash paths (`filepath.ToSlash(rel)`), only keep `.md` files that exist as regular files, and exclude:
       - Any path under `.agents/skills/principal-architect-workspace/` (or containing `/principal-architect-workspace/`)
       - Any path under `docs/evaluations/` or `docs/experiments/` (unless explicitly listed as an exact non-wildcard file path in `cfg.DocSeeds`)
       - Any path under `.memory/`
       - Any file whose content contains `generated_by: repo-memory` in its YAML frontmatter (so graduated `rm-*` skills are never re-ingested as doc seeds).
     - Return deduplicated, lexicographically sorted repo-relative slash paths.

- [x] `internal/core/seed/doc_seed.go` — Implement `DocSeedSource` and deterministic doc seeding / reconciliation:
  1. Define types:
     ```go
     type DocSeedOptions struct {
         RepoRoot string
         OpsDir   string
         DocPaths []string // Optional explicit repo-relative paths; if empty, uses config.ResolveDocSeedGlobs(RepoRoot, RepoCfg)
         RepoCfg  config.RepoConfig
         DryRun   bool
     }

     type DocSeedFileStat struct {
         Path    string `json:"path"`
         BlobSHA string `json:"blob_sha"`
         Added   int    `json:"added"`
         Amended int    `json:"amended"`
         Retired int    `json:"retired"`
         Skipped bool   `json:"skipped"`
     }

     type DocSeedResult struct {
         FilesScanned int               `json:"files_scanned"`
         FilesSeeded  int               `json:"files_seeded"`
         FilesSkipped int               `json:"files_skipped"`
         EntriesAdded int               `json:"entries_added"`
         EntriesAmend int               `json:"entries_amended"`
         EntriesRetire int              `json:"entries_retired"`
         Files        []DocSeedFileStat `json:"files,omitempty"`
         Ops          []ops.Op          `json:"ops,omitempty"`
     }
     ```
  2. Implement `ComputeGitBlobSHA(repoRoot, relPath string, content []byte) string`:
     - If `content == nil` and `repoRoot != ""` and `relPath != ""`, try reading `filepath.Join(repoRoot, filepath.FromSlash(relPath))` (or running `git hash-object <relPath>` in `repoRoot`).
     - Compute the canonical Git blob SHA-1: `sha1.Sum([]byte(fmt.Sprintf("blob %d\x00", len(content)) + string(content)))` formatted as `%x` (which is byte-for-byte identical to `git hash-object` and works both inside git repos and in hermetic temp directories).
  3. Implement `ParseDocToOps(relPath string, content string, blobSHA string, commitTime time.Time, resolver *config.ScopeResolver, isValidRepoPath func(string) bool) []ops.Op`:
     - Deterministically scan `content` line by line:
       - Skip YAML frontmatter (`--- ... ---` at top of file).
       - Track markdown section headings (`#`, `##`, `###`, `####`).
       - Track fenced code blocks (` ``` `): do not treat lines inside code blocks as markdown bullets, but if a short command block (`bash`/`sh` or `$ ` / `repo-memory ` / `go `) immediately follows a bullet or procedural heading, you may attach its first command line to the item context.
       - Extract candidate items from:
         - Markdown bullets (`- ...`, `* ...`) and numbered list items (`1. ...`) outside code fences.
         - Markdown table rows (`| col1 | col2 | ... |`, skipping separator rows like `|---|---|` and header rows like `| Field | Purpose |` or `| # | Change |`).
       - Filter out junk (`IsJunkText`), short/non-actionable items (`len(trimmed) < 25`), or pure table-of-contents links.
       - Extract valid repository anchors from the item text via `extractBacktickPaths(item)` filtered through `isValidRepoPath`. If the item itself has 0 backticked repo paths, inherit any valid backticked repo paths from its current section heading, or if `relPath` is a skill file (`.agents/skills/<name>/SKILL.md`), check any repo paths referenced in the skill.
       - Resolve `dossierScope`:
         - Try `resolver.ResolveScope(anchors, keywords)` (where `keywords` includes words from the current heading and `relPath`).
         - If `dossierScope == ""` and `len(anchors) > 0`, derive module scope from `anchors` (via `config.DeriveModuleScopes(anchors)` -> first module scope name, or `deriveDocDefaultScope(relPath)`).
         - Require that every extracted item has **at least 1 valid repo anchor** (`len(anchors) > 0`) OR matches a declared scope in `resolver` so general prose without repo grounding is skipped.
       - Classify `entry_type` (`dossiers.EntryType`) deterministically:
         - `EntryTypeFailureSignature`: if heading or item text contains error/failure/symptom patterns (`error`, `404`, `429`, `panic`, `fails with`, `symptom`, `mismatch`, `rateLimitExceeded`).
         - `EntryTypeRecipe`: if `strings.HasSuffix(relPath, "SKILL.md")` and item describes a command/step, or heading/text contains procedural/command cues (`procedure`, `recipe`, `workflow`, `command`, `usage`, `go test`, `go build`, `repo-memory `).
         - `EntryTypeGotcha`: if heading or item text contains `gotcha`, `caveat`, `trap`, `quirk`, `warning`, `caution`, `important`, `lifetime`, `scrubbed`, `ignored`.
         - `EntryTypeContract`: if heading or item text contains `contract`, `schema`, `interface`, `convention`, `wire format`.
         - `EntryTypeInvariant`: default for rules/invariants (`must`, `never`, `always`, `invariant`, `rule`, `gate`, `required`, `strictly`) and general architectural constraints.
       - Format `text` cleanly (strip leading bullet/table pipes, collapse excessive whitespace, cap length at ~400 chars) and deduplicate within the same document.
       - Generate deterministic ID:
         `generateDocSeedID(relPath, string(entryType), normalizedKey)` -> `fmt.Sprintf("docseed-%x", sha256.Sum256([]byte(relPath + ":" + string(entryType) + ":" + normalizedKey))[:4])`.
       - Emit `ops.Op` with:
         - `Type: ops.OpAdd`
         - `EntryID: id`
         - `DossierScope: dossierScope`
         - `Timestamp: commitTime` (if zero, `time.Now().UTC()`)
         - `Source: ops.SourceDocSeed`
         - `Fields`:
           - `"entry_type": string(entryType)`
           - `"kind": string(dossiers.DeriveCognitiveKind(entryType))`
           - `"text": text`
           - `"anchors": anchors`
           - `"source_doc_path": filepath.ToSlash(relPath)`
           - `"source_doc_blob_sha": blobSHA`
           - `"evidence": []string{"doc:" + filepath.ToSlash(relPath)}`
  4. Implement `SeedDocs(ctx context.Context, opts DocSeedOptions, scopeCfg *config.ScopeConfig, opsReader *ops.Reader, opsWriter *ops.Writer) (*DocSeedResult, error)`:
     - Project existing active entries from `opsReader` (if `opsReader != nil`) via `dossiers.NewProjector(opsReader).Project(ctx)`, collecting all active entries where `e.Source == ops.SourceDocSeed` grouped by `e.SourceDocPath`.
     - Determine target `docPaths`: if `len(opts.DocPaths) > 0` use `opts.DocPaths`; otherwise call `config.ResolveDocSeedGlobs(opts.RepoRoot, opts.RepoCfg)`.
     - Build `isValidRepoPath := buildRepoPathValidator(SeedOptions{RepoRoot: opts.RepoRoot})`.
     - Build a `ScopeResolver` combining `scopeCfg`, derived module scopes from repository files, and existing aliases.
     - For each `relPath` in `docPaths`:
       - Check if `filepath.Join(opts.RepoRoot, filepath.FromSlash(relPath))` exists on disk:
         - If deleted (`os.IsNotExist(err)`): for every active entry in `existingByDoc[relPath]`, emit `ops.Op{Type: ops.OpRetire, EntryID: e.ID, DossierScope: e.DossierScope, Timestamp: now, Source: ops.SourceDocSeed, Fields: map[string]any{"reason": "source_doc_deleted: " + relPath}}`.
         - If present: read `content`, compute `blobSHA := ComputeGitBlobSHA(opts.RepoRoot, relPath, content)`.
         - If `len(existingByDoc[relPath]) > 0` and every active entry for `relPath` already has `e.SourceDocBlobSHA == blobSHA`: mark file `Skipped: true`, increment `FilesSkipped`, and continue!
         - Otherwise, parse candidate ops via `ParseDocToOps(relPath, string(content), blobSHA, commitTime, resolver, isValidRepoPath)`:
           - For each candidate op:
             - If `existingEntry, ok := existingByID[cand.EntryID]`:
               - If `existingEntry.Text != candText || existingEntry.SourceDocBlobSHA != blobSHA || existingEntry.DossierScope != cand.DossierScope || string(existingEntry.Type) != candType`:
                 emit `ops.OpAmend` (with `cand.Fields` and `DossierScope: cand.DossierScope`, or `OpAdd` since `Projector.applyAdd` automatically moves across scopes and amends existing entries!). Note: emitting `ops.OpAmend` (and `ops.OpMove` if scope changed) or `ops.OpAdd` updates the entry; track it in `EntriesAmend` if `existingEntry` already existed, or `EntriesAdded` if new!
             - Else:
               - Emit `cand` (`ops.OpAdd`) and increment `EntriesAdded`.
           - For any `oldEntry` in `existingByDoc[relPath]` whose `ID` is NOT in the new `candidateOps` set:
             - Emit `ops.Op{Type: ops.OpRetire, EntryID: oldEntry.ID, DossierScope: oldEntry.DossierScope, Timestamp: now, Source: ops.SourceDocSeed, Fields: map[string]any{"reason": fmt.Sprintf("source_doc_updated: %s (%s)", relPath, shortSHA(blobSHA))}}` and increment `EntriesRetire`.
     - If `!opts.DryRun && opsWriter != nil && len(allEmittedOps) > 0`:
       - Write the batch via `opsWriter.Write(ops.NewBatch(allEmittedOps))`.
     - Return `result`.

- [x] `cmd/repo-memory/cmd_seed_docs.go` and `cmd/repo-memory/main.go` — Add `repo-memory seed-docs` CLI subcommand:
  1. Create `cmd/repo-memory/cmd_seed_docs.go` with `seedDocsCmd` (`Use: "seed-docs"`, `Short: "Seed dossier entries from curated repository markdown documentation and skills"`):
     - Flags:
       - `--ops-dir` (default `".memory/ops/"`)
       - `--config` (default `".memory/config.yaml"`)
       - `--dry-run` (default `false`)
       - `--json` (default `false`)
     - In `RunE`:
       - Find repo root via `gitops.FindRepoRoot(".")`.
       - Load `repoCfg, _ := config.LoadRepoConfig(repoRoot)` and `scopeCfg, _ := config.LoadScopeConfig(resolvedConfigPath)`.
       - Call `seed.SeedDocs(cmd.Context(), opts, scopeCfg, opsReader, opsWriter)`.
       - If `!seedDocsDryRun` and `(res.EntriesAdded + res.EntriesAmend + res.EntriesRetire) > 0`, also re-project `.memory/dossiers/` and `.memory/ATLAS.md` via `dossiers.NewProjector(opsReader)` + `dossiers.NewWriter(filepath.Join(repoRoot, ".memory")).WriteAll(dossiersMap)` so running `repo-memory seed-docs` leaves `.memory/dossiers/` and `.memory/ATLAS.md` immediately up to date!
       - If `--json` is set, marshal `res` as indented JSON to stdout; otherwise print a concise summary (`Seeded %d entries (%d added, %d amended, %d retired) from %d/%d docs (%d unchanged skipped)`).
  2. Register `rootCmd.AddCommand(seedDocsCmd)` in `cmd/repo-memory/main.go`.

- [x] `internal/core/seed/doc_seed_test.go` — Add comprehensive unit tests:
  1. `TestResolveDocSeedGlobs`: verifies default globs (`docs/**/*.md`, `.agents/skills/**/SKILL.md`), exclusion of `.agents/skills/principal-architect-workspace/**`, `docs/evaluations/**`, `docs/experiments/**`, and skills containing `generated_by: repo-memory`, plus custom `doc_seeds` globs.
  2. `TestSeedDocs_InitialAndIdempotent`: creates a temp repo with realistic source files (`internal/core/ops/op.go`, `packaging/antigravity/hooks.json`) and markdown docs (`docs/hooks-guide.md`, `.agents/skills/test-skill/SKILL.md`), runs `SeedDocs`, verifies `source: "doc-seed"`, `source_doc_path`, `source_doc_blob_sha`, `kind`, and repo-validated `anchors` are populated, and verifies a second run with unchanged files skips all files (`FilesSkipped == 2`, `len(res2.Ops) == 0`).
  3. `TestSeedDocs_ModifyAndDelete`: modifies `docs/hooks-guide.md` (changing a rule and removing an old bullet) and deletes `.agents/skills/test-skill/SKILL.md`, runs `SeedDocs`, and verifies the modified doc's `source_doc_blob_sha` is updated, removed bullets are retired, and the deleted file's entries are retired with `source_doc_deleted`.

## Verification Requirement
Run:
```bash
go test -race ./internal/core/config/... ./internal/core/seed/... ./cmd/repo-memory/...
```
All tests must pass with zero race warnings. Commit your changes to `pr-11-v2-doc-seeding-and-lifecycles/wo3-doc-seed-source`, then **STOP and send a message back to the Principal Architect** confirming validation. Once approved, append your Completion Report below and commit it.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---
## Completion Report

### Status
✅ Complete

### Changes Made
- `internal/core/config/config.go` — Added `DocSeeds []string` to `RepoConfig`, `DefaultDocSeedGlobs()`, and `ResolveDocSeedGlobs(repoRoot, cfg)` supporting exact paths, standard globs, and `**` recursive walks with exclusions for `.agents/skills/principal-architect-workspace/**`, `docs/evaluations/**`, `docs/experiments/**`, `.memory/**`, and skills containing `generated_by: repo-memory`.
- `internal/core/seed/doc_seed.go` — Implemented `DocSeedOptions`, `DocSeedFileStat`, `DocSeedResult`, `DocSeedSource`, `ComputeGitBlobSHA`, `ParseDocToOps`, and `SeedDocs` for deterministic markdown extraction, repo anchor validation, cognitive lifecycle classification, and idempotent git blob SHA reconciliation (`OpAdd`, `OpAmend`, `OpMove`, `OpRetire`).
- `cmd/repo-memory/cmd_seed_docs.go` — Added `repo-memory seed-docs` CLI subcommand with `--ops-dir`, `--config`, `--dry-run`, and `--json`, automatically re-projecting `.memory/dossiers/` and `.memory/ATLAS.md` when entries change.
- `cmd/repo-memory/main.go` — Registered `seedDocsCmd` on `rootCmd`.
- `internal/core/seed/doc_seed_test.go` — Added unit tests `TestResolveDocSeedGlobs`, `TestSeedDocs_InitialAndIdempotent`, and `TestSeedDocs_ModifyAndDelete`.

### Decisions Made
- When `opts.DocPaths` is empty in `SeedDocs`, previously seeded `source_doc_path` files from active `doc-seed` entries that are no longer matched by `ResolveDocSeedGlobs` (e.g., deleted files) are included in the reconciliation pass so their entries are deterministically retired with `source_doc_deleted: <path>`.
- Extracted a stable normalized key hint (`<heading>:<bold-prefix>` or table key) when present so updating the prose of a named rule or table row amends the existing `docseed-<hash>` entry in place rather than churning entry IDs.

### Dead Ends & Warnings
- (none)

### Out-of-Scope Findings
- (none)

