# core/tick

## Decisions

### [seed-e29c518c] Sweep extracts workspaces directly from `transcript.jsonl` files iteratively to map non-spooled runs to their enrolled repos.
`decision · anchors: internal/core/tick/lock_unix.go, lock_windows.go, tick.lock, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`

### [seed-eda4000f] Used `go:build` flags for platform specific `flock` and `LockFileEx` lock mechanisms.
`decision · anchors: internal/core/tick/lock_unix.go, lock_windows.go, tick.lock, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`
