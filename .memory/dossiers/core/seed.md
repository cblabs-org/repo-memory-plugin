# core/seed

## Decisions

### [seed-109fea35] Used `ruled-out` type with prefix `rul-` as default mapping for Dead Ends.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/convert.go, internal/core/seed/scanner.go, cmd/repo-memory/cmd_seed.go, cmd/repo-memory/main.go, internal/core/seed/seed_test.go`

### [seed-8ae39b13] Used `ops.NewBatch` to properly emit batched ops from the seeded list.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/convert.go, internal/core/seed/scanner.go, cmd/repo-memory/cmd_seed.go, cmd/repo-memory/main.go, internal/core/seed/seed_test.go`

### [seed-aeb2bf53] Implemented two-layer anchor path filtering: syntactic validation in `extractBacktickPaths` so pure parser callers reject code expressions and external/home paths without requiring a filesystem root, paired with repository-tree validation (`git ls-tree` at `BeforeCommit` or `os.Stat`) in `ScanAndParse` when `RepoRoot` is provided.
`decision · kind: episodic · anchors: internal/core/seed/parser.go, internal/core/seed/scanner.go, internal/core/seed/seed_test.go`
