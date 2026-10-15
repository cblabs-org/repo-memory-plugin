# core/config

## Ruled out, failure signatures and gotchas

### [seed-92402404] Pre-existing failures observed in `cmd/repo-memory` (`TestSearchCommand`) and `internal/harness/antigravity` (`TestSessionAttributor_Integration`).
`ruled-out · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-bcda28c3] Pre-existing failure observed in `internal/core/jobs/store_test.go` (`TestJobLifecycleSuccess:308`) expecting `store.AcquireNext` to return `nil, nil` when no jobs are available rather than `ErrNoJobAvailable`. Left unmodified as it is outside this work order's boundary box.
`ruled-out · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-e344cefe] When testing Windows and Linux data directory resolution on a Darwin host, path separators in unit tests must be handled with platform awareness or mock functions. Used `dataDirForOS` with injected env and home getters to allow cross-platform testing on any host OS.
`ruled-out · anchors: internal/core/config/datadir.go, internal/core/config/config.go, config.yaml, internal/core/config/layout.go, internal/core/config/config_test.go, go.mod, go.sum`

### [seed-e67606f4] Be careful with `filepath.Match` when writing globs, it behaves differently from shell globs for `**`. Standard `.memory/config.yaml` relies heavily on `**`, so the fallback in `AssignSubsystem` was necessary.
`ruled-out · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

## Decisions

### [seed-01f06942] `ScopeResolver.ResolveScope` prioritizes aliases first, then module scopes (path prefix match), then declared scopes (keywords, tool names, commands), resolving through aliases at every step.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/core/config/scopes_test.go`

### [seed-1469d423] Custom `**` matching: Go's `filepath.Match` does not correctly handle `**` for deep recursive directory matching. I added a simple fallback in `AssignSubsystem` to check `strings.HasPrefix` when the glob ends with `/**` to ensure files in deep subdirectories are matched correctly.
`decision · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

### [seed-1e638d55] Anchors and path segments are normalized with forward slashes and underscores converted to hyphens to ensure consistent kebab-case scope names matching the v2 design.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/core/config/scopes_test.go`

### [seed-2e4a38a5] `EnsureStorageLayout`: Implemented as `EnsureStorageLayout(dataDir, repoID string, repoRoot ...string) error`, plus standalone `EnsureDataDirLayout` and `EnsureRepoLayout` helpers. This preserves the 2-argument signature specified in WO3 line-item instructions while allowing caller to optionally pass `repoRoot` for the repository `.memory` structure.
`decision · anchors: internal/core/config/datadir.go, internal/core/config/config.go, config.yaml, internal/core/config/layout.go, internal/core/config/config_test.go, go.mod, go.sum`

### [seed-684b9213] Standardized canonical checkpoint titles (`User Knowledge`, `Model Knowledge`, `Work Accomplished`, `Current Work`, `Outstanding User Requests`, `Files and Code`) while mapping common synonyms (e.g. `Current Work and Next Steps` -> `Current Work`, `User Requests` -> `Outstanding User Requests`) and supporting custom markdown sections.
`decision · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-8af2e631] Alphabetical mapping in `AssignSubsystem`: The config uses overlapping globs for subsystems (e.g. `internal/harness/antigravity/hooks*.go` vs `internal/harness/**`). Since `Subsystems` is a `map[string]SubsystemConfig`, iteration order is random in Go, causing flaky subsystem assignments. I chose to sort the map keys alphabetically before iterating, which made matching deterministic.
`decision · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

### [seed-97de0061] `ScopeKind` connects resolved scope names to `dossiers.DossierKind` (`module`, `topic`, `external`, `procedure`).
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/core/config/scopes_test.go`

### [seed-bd8c65c3] `AliasMap.Resolve` protects against recursive alias loops using both a visited-set cycle detector and a maximum recursion depth of 10.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/core/config/scopes_test.go`

### [seed-c74993cb] `MachineConfig` YAML Unmarshaling: Implemented custom unmarshaler supporting flat snake_case (`housekeeping_executor`), dotted keys (`housekeeping.executor`), and nested maps (`housekeeping: { executor: ... }`) to seamlessly accept all representations mentioned in SPEC.md.
`decision · anchors: internal/core/config/datadir.go, internal/core/config/config.go, config.yaml, internal/core/config/layout.go, internal/core/config/config_test.go, go.mod, go.sum`

### [seed-d510d878] Implemented `internal/core/evidence/` within `internal/core` rather than importing `internal/harness/antigravity`, strictly complying with `internal/core/boundary_test.go` architectural constraints that prohibit core packages from importing harness adapters.
`decision · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-e937646b] Used `episode.Header` for storing extracted checkpoint sections (`map[string]string`) and evidence metadata (`Source`, `Status`, `StepCount`), enabling rapid header inspection without needing full decompression of stored episodes.
`decision · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

## Open issues

### [seed-1a602f9b] These tests were failing outside of the modified `config` boundary box and require follow-up.
`open-issue · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

### [seed-56df9f6f] `cmd/repo-memory` tests: `TestSearchCommand` fails locally with `unable to open database file (14)`.
`open-issue · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

### [seed-b4369427] `internal/harness/antigravity` tests: `TestSessionAttributor_Integration` fails locally with `expected c1 to be idle (modified 1h ago)`.
`open-issue · anchors: internal/core/config/config.go, .memory/config.yaml, internal/core/config/config_test.go, internal/core/config/subsystem.go, internal/core/config/subsystem_test.go`

### [seed-bd7c4a12] `internal/core/jobs/store_test.go`: Line 308 checks `if err != nil` after `AcquireNext`, which returns `ErrNoJobAvailable` on empty queues. Should check `errors.Is(err, jobs.ErrNoJobAvailable)`.
`open-issue · anchors: internal/core/config/layout.go, internal/core/episode/store.go, internal/core/evidence/checkpoint.go, internal/core/evidence/parser.go, internal/core/evidence/builder.go, internal/core/evidence/checkpoint_test.go, parser_test.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`
