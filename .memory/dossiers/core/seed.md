# core/seed

## Ruled out, failure signatures and gotchas

### [seed-c0864a54] Raw `CHECKPOINT` lines parsed by `ParseTranscript` have `StepIndex == 0` unless derived from the preceding step; relying on `steps[checkpointIdx].StepIndex` directly caused `LastHandledCheckpointStep` to remain `0` instead of the actual checkpoint position.
`ruled-out · kind: episodic · anchors: internal/core/seed/convert.go, internal/core/seed/seed_test.go, internal/core/seed/gc_test.go, internal/core/delivery/ledger.go, internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/preinvocation_test.go, internal/core/dossiers/projector.go, internal/core/dossiers/writer.go, internal/core/dossiers/dossiers_test.go, internal/core/seed/parser.go, internal/core/seed/scanner.go, internal/core/seed/seed_test.go, internal/core/index/index.go`

## Decisions

### [seed-109fea35] Used `ruled-out` type with prefix `rul-` as default mapping for Dead Ends.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/convert.go, internal/core/seed/scanner.go, cmd/repo-memory/cmd_seed.go, cmd/repo-memory/main.go, internal/core/seed/seed_test.go`

### [seed-63f7d11a] Filtered module-derivation anchor inputs in `RunOpsGC` to multi-segment non-dot paths (`strings.Contains(a, "/") && !strings.HasPrefix(a, ".")`) so root-level files (`Makefile`, `.gitignore`) and dot-directories (`.agents/...`, `.memory/...`) map cleanly to `"root"` without creating spurious `.agents` or `Makefile` module scopes during scope repair.
`decision · kind: episodic · anchors: internal/core/seed/convert.go, internal/core/seed/parser.go, internal/core/seed/gc.go, internal/core/seed/gc_test.go, cmd/repo-memory/cmd_ops_gc.go, cmd/repo-memory/main.go, .memory/ops, .agents/skills/principal-architect-workspace`

### [seed-8ae39b13] Used `ops.NewBatch` to properly emit batched ops from the seeded list.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/convert.go, internal/core/seed/scanner.go, cmd/repo-memory/cmd_seed.go, cmd/repo-memory/main.go, internal/core/seed/seed_test.go`

### [seed-aeb2bf53] Implemented two-layer anchor path filtering: syntactic validation in `extractBacktickPaths` so pure parser callers reject code expressions and external/home paths without requiring a filesystem root, paired with repository-tree validation (`git ls-tree` at `BeforeCommit` or `os.Stat`) in `ScanAndParse` when `RepoRoot` is provided.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/scanner.go, internal/core/seed/seed_test.go`

### [seed-b70d0bb4] Derived `CheckpointStepIdx` in `DetectCompaction` from the preceding non-checkpoint step index when a raw `CHECKPOINT` line has `StepIndex == 0`, and reset `LastHandledCheckpointStep = -1` on rewind when no checkpoint remains or `LastHandledCheckpointStep > maxRemainingStepIdx`.
`decision · kind: episodic · anchors: internal/core/seed/convert.go, internal/core/seed/seed_test.go, internal/core/seed/gc_test.go, internal/core/delivery/ledger.go, internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/preinvocation_test.go, internal/core/dossiers/projector.go, internal/core/dossiers/writer.go, internal/core/dossiers/dossiers_test.go, internal/core/seed/parser.go, internal/core/seed/scanner.go, internal/core/seed/seed_test.go, internal/core/index/index.go`

### [seed-f5fb667a] Preserved unmarshaled `InjectedSet` in `loadLedgerLocked` whenever `"injected_set"` is present in JSON (even when empty `{}`), falling back to reconstructing from post-checkpoint non-`about-to-edit:` `Injections` only when loading legacy JSON files where `InjectedSet == nil`.
`decision · kind: episodic · anchors: internal/core/seed/convert.go, internal/core/seed/seed_test.go, internal/core/seed/gc_test.go, internal/core/delivery/ledger.go, internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/preinvocation_test.go, internal/core/dossiers/projector.go, internal/core/dossiers/writer.go, internal/core/dossiers/dossiers_test.go, internal/core/seed/parser.go, internal/core/seed/scanner.go, internal/core/seed/seed_test.go, internal/core/index/index.go`
