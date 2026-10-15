# core/tick

## Ruled out, failure signatures and gotchas

### [seed-c6513a63] Direct imports of concrete harness adapters from `internal/core` are forbidden by `boundary_test.go`. Interface injection via `SessionAttributor` avoided boundary violations while allowing full concrete functionality.
`ruled-out · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, hooks.go, cmd/repo-memory/cmd_tick.go`

## Decisions

### [seed-0c419ff6] Subagents with empty or missing `workspace_uris` recursively inherit their enrolled repository attribution from their parent conversation up the hierarchy tree.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, hooks.go, cmd/repo-memory/cmd_tick.go`

### [seed-1eba9bad] When de-duplicating between app and CLI databases, the record with the latest timestamp is preferred for status/activity, while preserving non-empty workspace paths and parent IDs from either store.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, hooks.go, cmd/repo-memory/cmd_tick.go`

### [seed-5a8e8744] Used a decoupled registration pattern (`tick.RegisterCopier`) to keep `internal/core` free of adapter dependencies.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`

### [seed-7967253a] Maintained strict core/adapter boundary boundaries per SPEC §18.2: `internal/core/tick` defines the `SessionAttributor` interface and does not import concrete adapters (`boundary_test.go` passed 100% with 0 violations across 61 files).
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, hooks.go, cmd/repo-memory/cmd_tick.go`

### [seed-e29c518c] Sweep extracts workspaces directly from `transcript.jsonl` files iteratively to map non-spooled runs to their enrolled repos.
`decision · anchors: internal/core/tick/lock_unix.go, lock_windows.go, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`

### [seed-eda4000f] Used `go:build` flags for platform specific `flock` and `LockFileEx` lock mechanisms.
`decision · anchors: internal/core/tick/lock_unix.go, lock_windows.go, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`

## Open issues

### [seed-c309f6be] Transcript parsing and compaction are deferred to Release 2 (R2.1) per SPEC §18.6 rule 11.
`open-issue · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`
