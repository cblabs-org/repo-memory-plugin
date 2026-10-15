# Work Order: wo6-review-remediation

> **Project:** pr-10b-delivery-and-storage-fixes · **WO:** 6 (PR #18 Audit Remediation)
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification`
> **Base:** `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` (`5de0ed7`)
> **Complexity:** standard

## Objective

Remediate all 5 findings (`[MAJOR-1]`, `[MAJOR-2]`, `[MINOR-1]`, `[MINOR-2]`, `[MINOR-3]`) from the Tier 2b Cold-Firewall PR Audit on GitHub PR #18 (`pr-10b-delivery-and-storage-fixes`).

## Audit Findings & Required Remediations

### 1. `[MAJOR-1]` Placeholder `"text": "(none)"` entries survived in `.memory/` and `evaluation/data/*/`
- **Files:** `internal/core/seed/convert.go`, `internal/core/seed/seed_test.go`, `internal/core/seed/gc_test.go`, `.memory/`, `evaluation/data/*/`
- **Fix:**
  1. In `IsJunkText(text string) bool` (`internal/core/seed/convert.go:21-36`), strip surrounding punctuation/parentheses or explicitly add `"(none)"`, `"(none)."`, `"(n/a)"`, `"(tbd)"`, `"(todo)"`, `"-"` to the `switch lower` case list so any parenthesized placeholder (`(none)`, `(none).`, `(n/a)`, etc.) returns `true`.
  2. Add `"(none)"`, `"(none)."`, `"(n/a)"`, `"-"` test cases to `TestReportToOps_JunkFiltered` (`internal/core/seed/seed_test.go`) and `TestRunOpsGC_PrunesJunkStaleAndRepairsScopes` (`internal/core/seed/gc_test.go`).
  3. Rebuild `~/.local/bin/repo-memory`, run `~/.local/bin/repo-memory ops gc --ops-dir .memory/ops --dossiers-dir .memory` and across all 8 `evaluation/data/<wo>` snapshots (`--ops-dir evaluation/data/<wo>/ops --dossiers-dir evaluation/data/<wo>`), and run `~/.local/bin/repo-memory reindex .`.
  4. Verify `git grep -i '"text": "(none)' .memory/ evaluation/data/` and `git grep "(none)" .memory/dossiers/ evaluation/data/*/dossiers/` return 0 matches.

### 2. `[MAJOR-2]` Cross-process `ModifyLedger` / `loadLedgerLocked` resurrects cleared `InjectedSet` keys after compaction and rewind
- **Files:** `internal/core/delivery/ledger.go`, `internal/harness/antigravity/ledger_store.go`, `internal/harness/antigravity/preinvocation.go`, `internal/harness/antigravity/compaction.go`, `internal/harness/antigravity/preinvocation_test.go`
- **Fix:**
  1. In `internal/core/delivery/ledger.go`:
     - Remove `,omitempty` from `InjectedSet map[string]bool` (`json:"injected_set"`) and `LastHandledCheckpointStep int` (`json:"last_handled_checkpoint_step"`) so an empty `InjectedSet` (`"injected_set": {}`) and `LastHandledCheckpointStep = -1` are always explicitly persisted in JSON.
  2. In `internal/harness/antigravity/ledger_store.go` (`loadLedgerLocked`, lines 191–196):
     - Remove the unconditional `for _, inj := range ledger.Injections { ledger.InjectedSet[inj.EntryID] = true }` loop that overwrote `InjectedSet` on every load!
     - Instead, only when `ledger.InjectedSet == nil` (i.e. legacy JSON file that did not have `"injected_set"`), initialize `ledger.InjectedSet = make(map[string]bool)` and backfill only `for _, inj := range ledger.Injections { if inj.StepIndex > ledger.LastHandledCheckpointStep && !strings.HasPrefix(inj.EntryID, "about-to-edit:") { ledger.InjectedSet[inj.EntryID] = true } }`. When `ledger.InjectedSet != nil` after `json.Unmarshal`, keep the unmarshaled `ledger.InjectedSet` as-is!
  3. In `internal/harness/antigravity/preinvocation.go`:
     - When `DetectRewind(steps, ledger.LastKnownStepCount)` fires (lines 127–145):
       - If `ledger.LastHandledCheckpointStep > maxRemainingStepIdx`, reset `ledger.LastHandledCheckpointStep = -1`.
     - When `DetectCompaction` fires and `BuildReinjection` returns a non-nil `reinjStep` (lines 221–226):
       - Call `ledger.RearmDedup()` and set `rearmed = true`. (Note: any entries re-injected in `reinjStep` or subsequently injected in the same invocation should have `ledger.MarkInjected(id)` called if they were re-injected, or if `RearmDedup()` leaves `InjectedSet` clear for new moments, ensure `ModifyLedger` copies `for k, v := range ledger.InjectedSet { if v { l.InjectedSet[k] = true } }` after `if rearmed { l.RearmDedup() }`).
     - In `ModifyLedger` (lines 393–431):
       - If `rewound` is true, filter `l.Injections` to `inj.StepIndex <= maxRemainingStepIdx`, set `l.LastProcessedStepIndex = ledger.LastProcessedStepIndex`, and set `l.LastHandledCheckpointStep = ledger.LastHandledCheckpointStep`.
       - Copy `for k, v := range ledger.InjectedSet { if v { l.InjectedSet[k] = true } }`.
  4. In `internal/harness/antigravity/preinvocation_test.go`:
     - In `TestPreInvocation_MultiInvocationLifecycle` (and/or `TestPreInvocation_CompactionAndReinjection`), add explicit assertions that call `LoadLedger(dataDir, repoID, convID)` after compaction and after rewind and verify that pre-checkpoint entries that were NOT re-injected are NOT resurrected into `loaded.InjectedSet`, and `loaded.LastHandledCheckpointStep` is reset to `-1` when rewinding past a checkpoint.

### 3. `[MINOR-1]` `Projector` / `Writer` leaves empty shell `*Dossier` structs when all entries are moved out of a scope
- **Files:** `internal/core/dossiers/projector.go`, `internal/core/dossiers/writer.go`, `internal/core/dossiers/dossiers_test.go`
- **Fix:**
  1. In `moveEntry` (`internal/core/dossiers/projector.go:349-381`), if `found` is true and after removing `entryID` from `fromD.Entries` both `len(fromD.Entries) == 0` and `len(fromD.History) == 0` (and `fromD.Title == ""` and `fromD.Purpose == ""`), `delete(dossiers, fromScope)` so empty shell source dossiers do not linger in the projected map.
  2. In `Writer.WriteAll` (`internal/core/dossiers/writer.go:48-72`), also guard: if `len(d.Entries) == 0 && len(d.History) == 0`, skip writing the empty `.md` file and skip adding a 0-entry `0001-01-01T00:00:00Z` row to `atlas.Dossiers`.
  3. Update `TestProjector_ReAddUnderDifferentScopeMovesEntry` in `internal/core/dossiers/dossiers_test.go` to verify that when the sole entry in `scope-a` moves to `scope-b`, `WriteAll` does not emit an empty `scope-a.md` file or a 0-entry `scope-a` row in `ATLAS.md`.

### 4. `[MINOR-2]` `IsValidAnchorPathSyntax` and `buildRepoPathValidator` reject `path#symbol` and `path:symbol` anchors
- **Files:** `internal/core/seed/parser.go`, `internal/core/seed/scanner.go`, `internal/core/seed/seed_test.go`
- **Fix:**
  1. Add a helper `stripAnchorSymbol(p string) (filePart string, ok bool)` in `internal/core/seed/parser.go`:
     - After rejecting `http:` / `https:` and `://`, if `p` contains `#` or `:` (at most one symbol separator, after a non-empty file path part), split `p` into `filePart` and `symPart`:
       - Require `symPart != ""` and that `symPart` contains only valid identifier/line characters (`[a-zA-Z0-9_.\-]+`, no `/` or metacharacters).
       - Validate `filePart` against all the path syntax rules in `IsValidAnchorPathSyntax` (checking `allowedSingleSegmentExts` on `filepath.Ext(filePart)` and rejecting `:` inside `filePart`).
     - In `extractBacktickPaths`, preserve the full `path#symbol` / `path:symbol` token (with trailing `/` stripped from `filePart` if any).
  2. In `buildRepoPathValidator` (`internal/core/seed/scanner.go:58-103`), strip any `#symbol` or `:symbol` suffix from `p` before cleaning and looking up `filePart` in `known[clean]` (`git ls-tree`) or `os.Stat(filepath.Join(opts.RepoRoot, clean))`.
  3. Add unit test cases in `TestParseCompletionReport_FiltersNonPathTokens` and `TestScanAndParse_ValidatesRepoPathsAndBeforeCommitTimestamp` (`internal/core/seed/seed_test.go`) verifying that `internal/real/file.go#MyFunc` and `SPEC.md#Section` are accepted while `http://example.com/foo` and `foo:bar:baz` are rejected.

### 5. `[MINOR-3]` `NewReadOnlyIndex` omits `_pragma=busy_timeout(5000)`
- **Files:** `internal/core/index/index.go`
- **Fix:**
  - In `NewReadOnlyIndex(dbPath string)` (`internal/core/index/index.go:43-50`), update the DSN from `fmt.Sprintf("file:%s?mode=ro", dbPath)` to `fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath)`.

## Verification Criteria

- [x] `go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...` passes.
- [x] `go vet ./cmd/... ./internal/... ./evaluation/cmd/build-index/...` passes.
- [x] `GOOS=windows go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...` passes.
- [x] `go test -race ./cmd/... ./internal/...` passes across all 25 testable packages.
- [x] `git grep -i '"text": "(none)' .memory/ evaluation/data/` returns 0 matches.
- [x] `~/.local/bin/repo-memory` is rebuilt and `~/.local/bin/repo-memory reindex .` succeeds.

---

## Completion Report

### Status
✅ Complete

### Changes Made
- `internal/core/seed/convert.go`, `internal/core/seed/seed_test.go`, `internal/core/seed/gc_test.go`, `.memory/`, `evaluation/data/*/` — updated `IsJunkText` to match parenthesized/punctuated placeholders (`(none)`, `(none).`, `(n/a)`, `(tbd)`, `(todo)`, `-`), added unit tests, and ran `repo-memory ops gc` across `.memory/` and all 8 `evaluation/data/<wo>` snapshots (`[MAJOR-1]`)
- `internal/core/delivery/ledger.go`, `internal/harness/antigravity/ledger_store.go`, `internal/harness/antigravity/compaction.go`, `internal/harness/antigravity/preinvocation.go`, `internal/harness/antigravity/preinvocation_test.go` — removed `,omitempty` from `InjectedSet` and `LastHandledCheckpointStep`, preserved unmarshaled `InjectedSet` on `loadLedgerLocked` without overwriting from historical `Injections`, derived `CheckpointStepIdx` from preceding step index on `CHECKPOINT` lines, and reset `LastHandledCheckpointStep = -1` when rewinding past a checkpoint (`[MAJOR-2]`)
- `internal/core/dossiers/projector.go`, `internal/core/dossiers/writer.go`, `internal/core/dossiers/dossiers_test.go` — deleted empty shell source dossiers in `moveEntry` and skipped 0-entry/0-history dossiers in `Writer.WriteAll` (`[MINOR-1]`)
- `internal/core/seed/parser.go`, `internal/core/seed/scanner.go`, `internal/core/seed/seed_test.go` — added `stripAnchorSymbol` and `isValidAnchorSymbol` so `path#symbol` and `path:symbol` anchors are accepted by `IsValidAnchorPathSyntax`, `extractBacktickPaths`, and `buildRepoPathValidator` (`[MINOR-2]`)
- `internal/core/index/index.go` — added `_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)` to `NewReadOnlyIndex` (`[MINOR-3]`)

### Decisions Made
- Preserved unmarshaled `InjectedSet` in `loadLedgerLocked` whenever `"injected_set"` is present in JSON (even when empty `{}`), falling back to reconstructing from post-checkpoint non-`about-to-edit:` `Injections` only when loading legacy JSON files where `InjectedSet == nil`.
- Derived `CheckpointStepIdx` in `DetectCompaction` from the preceding non-checkpoint step index when a raw `CHECKPOINT` line has `StepIndex == 0`, and reset `LastHandledCheckpointStep = -1` on rewind when no checkpoint remains or `LastHandledCheckpointStep > maxRemainingStepIdx`.

### Dead Ends & Warnings
- Raw `CHECKPOINT` lines parsed by `ParseTranscript` have `StepIndex == 0` unless derived from the preceding step; relying on `steps[checkpointIdx].StepIndex` directly caused `LastHandledCheckpointStep` to remain `0` instead of the actual checkpoint position.

### Out-of-Scope Findings
- None.
