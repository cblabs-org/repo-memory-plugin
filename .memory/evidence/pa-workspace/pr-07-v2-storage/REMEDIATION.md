# PR-07 Review Remediation Round 2 — 9 Findings

You are an expert senior software engineer. Fix all 9 review findings.
**DO NOT trigger or act as the principal-architect.**

## Branch
You are on `pr-07-v2-storage/wo6-seeding`. Commit fixes here.

## Findings to Fix

### Finding 1: Prose change bullets become dossier scopes
**File:** `internal/core/seed/parser.go:102`
**Problem:** A bullet like `- Implemented \`internal/core/metrics/writer.go\` providing thread-safe JSONL...` has no ` — ` or ` - ` separator, so the whole sentence becomes `Change.Path`. These garbage paths get fed to `DeriveModuleScopes`.
**Fix:** In the parser, validate that `pathPart` looks like a real file path before accepting it as a Change:
- Must not contain spaces (after trimming backticks)
- Must contain at least one `/` or `.`
- If it doesn't look like a path, skip this bullet (don't add it to Changes)

### Finding 2: Template completion reports still seeded
**File:** `internal/core/seed/parser.go:56`
**Problem:** Most WORKORDERs have an unfenced `## Completion Report` template at the bottom. The code-fence fix only covers fenced examples. When a worker adds a real completion report, there are now two `## Completion Report` headers. The parser hits the template first (with placeholder text like "Decision description and rationale"), then `continue`s on the second header.
**Fix:** Parse the LAST `## Completion Report` section, not the first. Reverse the logic: scan the whole file collecting ALL `## Completion Report` positions (outside code fences), then parse from the LAST one. Also filter out obvious template bullets: skip any bullet whose text exactly matches known template placeholders like "Decision description and rationale", "What was attempted, why it failed, and what to avoid", "Finding description and suggested action".

### Finding 3: Projector stale-scope still broken
**File:** `internal/core/dossiers/projector.go:133`
**Problem:** The fallback only fires when `op.DossierScope` is empty or names a dossier that doesn't exist. After a move/split, the source dossier still exists with other entries, so an amend naming the old scope searches the wrong dossier and misses.
**Fix:** In `applyAmend`, `applyRetire`, `applySupersede`, `applyMove`: ALWAYS prefer `entryIndex[op.EntryID]` when it's set, using `op.DossierScope` only as a last-resort fallback. The current code checks `op.DossierScope` first, then falls back to entryIndex — reverse that priority.

### Finding 4: Deterministic IDs use absolute path
**File:** `internal/core/seed/convert.go:13`
**Problem:** `path` is absolute, so different machines produce different hashes. Also `index`-based hashing means inserting a bullet shifts every later ID.
**Fix:**
1. Make path repo-relative before hashing. In `SeedWithTimeCut`, compute `relPath` from `opts.RepoRoot` and pass that to `ReportToOps` instead of the absolute path.
2. Hash on `(relPath, sectionName, bulletText)` instead of `(path, section, index)`. This is content-addressed and stable across insertions.
3. Change `time.Now()` in `convert.go` to use a timestamp from the report if available (fall back to `time.Now()` if not). For now, use `time.Now()` but document this as a known limitation.

### Finding 5: Duplicate IDs abort rebuild
**File:** `internal/core/index/rebuild.go:40`
**Problem:** The rebuild uses plain `INSERT` on `id TEXT PRIMARY KEY`. If projection produces duplicate IDs (e.g. from DetectHandEdits), the whole transaction fails.
**Fix:** Change `INSERT INTO entries` to `INSERT OR REPLACE INTO entries` (i.e. upsert semantics). This matches the previous `UpsertEntry` behavior.

### Finding 6: Write lock held during projection
**File:** `internal/core/index/rebuild.go:34`
**Problem:** `Project()` runs inside the transaction, holding SQLite's write lock while reading the entire op log. Other writers block and timeout.
**Fix:** Run `projector.Project(ctx)` and `config.BuildAliasMap(opsReader, ctx)` BEFORE `BeginTx`. The transaction only needs to cover the deletes and inserts.

### Finding 7: ProjectBefore was removed
**File:** `internal/core/dossiers/projector.go:21`
**Problem:** The remediation worker removed `ProjectBefore` instead of updating it with entryIndex support. X6-lite (PR-09) needs it.
**Fix:** Restore `ProjectBefore` using a shared `projectFromSeq` helper:
```go
func (p *Projector) projectFromSeq(seq iter.Seq2[*ops.OpBatch, error]) (map[string]*Dossier, error) {
    dossiers := make(map[string]*Dossier)
    entryIndex := make(map[string]string)
    for batch, err := range seq {
        if err != nil { return nil, err }
        applyBatch(dossiers, entryIndex, batch)
    }
    for _, d := range dossiers { sortDossier(d) }
    return dossiers, nil
}

func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error) {
    return p.projectFromSeq(p.reader.Iterate(ctx))
}

func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error) {
    return p.projectFromSeq(p.reader.IterateBefore(ctx, beforeCommit))
}
```

### Finding 8: Errors swallowed in cmd_seed
**File:** `cmd/repo-memory/cmd_seed.go:46`
**Problem:** `LoadScopeConfig` and `ScanAllAnchors` errors are silently swallowed. With deterministic IDs, entries seeded with wrong scopes can't be fixed by re-running.
**Fix:** Return errors from both calls. If `LoadScopeConfig` fails and the file exists, return the error. If the file doesn't exist, that's fine (use empty config). Same for `ScanAllAnchors`.

### Finding 9: Duplicated scan/parse code
**File:** `internal/core/seed/scanner.go:121`
**Problem:** `ScanAllAnchors` is a near-copy of `SeedWithTimeCut`. Each workorder is read and parsed twice.
**Fix:** Refactor: extract a `ScanAndParse(paDir string, opts SeedOptions) ([]*ParsedWorkorder, error)` function that returns all parsed workorders. Both `SeedWithTimeCut` and `ScanAllAnchors` should call this. Define:
```go
type ParsedWorkorder struct {
    Path    string
    RelPath string
    Report  *CompletionReport
}
```
Then `ScanAllAnchors` just collects anchors from the pre-parsed results, and `SeedWithTimeCut` converts and writes.

## Verification

1. `go build ./...` must pass
2. `go test ./internal/core/seed/... ./internal/core/dossiers/... ./internal/core/index/...` must pass
3. `go vet ./...` must pass
4. Clean seed run: `rm -rf /tmp/test-ops3 && go run ./cmd/repo-memory seed --ops-dir /tmp/test-ops3 --pa-dir .agents/skills/principal-architect-workspace/`
5. Verify: no entries with text "Decision description and rationale" (template text)
6. Verify: no scopes with spaces or full sentences
7. Verify: `ProjectBefore` exists and compiles

**CRITICAL:** Before committing, send me a validation report. Do NOT commit until I approve.
