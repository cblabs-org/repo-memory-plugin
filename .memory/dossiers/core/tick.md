# core/tick

## Ruled out, failure signatures and gotchas

### [seed-084601c6] When using `agy` as a background LLM execution harness, always map inputs via `-p -` and provide them through `stdin` to avoid maximum argument size limits which truncate the JSON structures and crash processing.
`ruled-out · anchors: internal/core/tick/pipeline.go, internal/harness/antigravity/executor.go`

### [seed-5fd3847d] The pipeline `processReconcile` job sequence scales sequentially which takes roughly 10 seconds per candidate. This creates significant wall-clock latency (e.g. 60+ minutes) for bulk backfilling.
`ruled-out · anchors: internal/core/tick/pipeline.go, internal/harness/antigravity/executor.go`

### [seed-c6513a63] Direct imports of concrete harness adapters from `internal/core` are forbidden by `boundary_test.go`. Interface injection via `SessionAttributor` avoided boundary violations while allowing full concrete functionality.
`ruled-out · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, cmd/repo-memory/cmd_tick.go`

## Decisions

### [seed-0c419ff6] Subagents with empty or missing `workspace_uris` recursively inherit their enrolled repository attribution from their parent conversation up the hierarchy tree.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, cmd/repo-memory/cmd_tick.go`

### [seed-1eba9bad] When de-duplicating between app and CLI databases, the record with the latest timestamp is preferred for status/activity, while preserving non-empty workspace paths and parent IDs from either store.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, cmd/repo-memory/cmd_tick.go`

### [seed-5a8e8744] Used a decoupled registration pattern (`tick.RegisterCopier`) to keep `internal/core` free of adapter dependencies.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`

### [seed-72ca47ce] Explicitly defined a robust `DO NOT USE ANY TOOLS` instruction inside the standard prompt wrapping JSON since the LLM executor defaults to tool-enabled environments and was actively attempting out-of-bounds bash calls.
`decision · anchors: internal/core/tick/pipeline.go, internal/harness/antigravity/executor.go`

### [seed-77325143] Converted `TestBackwardCompatibility_EntitiesAndClaims` in `internal/core/index/v2_test.go` to `TestBackwardCompatibility_Entities` to allow all 11 index tests to pass without referencing `claims`.
`decision · anchors: internal/core/tick/pipeline.go, cmd/repo-memory/cmd_reindex.go, internal/core/index/index.go, internal/core/index/rebuild.go, internal/core/pipeline/publish/local.go, internal/core/index/v2_test.go, internal/core/tick/pipeline_test.go`

### [seed-7967253a] Maintained strict core/adapter boundary boundaries per SPEC §18.2: `internal/core/tick` defines the `SessionAttributor` interface and does not import concrete adapters (`boundary_test.go` passed 100% with 0 violations across 61 files).
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, cmd/repo-memory/cmd_tick.go`

### [seed-b4f1cf3d] Replaced `claims.Claim` with a local `publish.Item` in `internal/core/pipeline/publish/local.go` rather than deleting `publish`, ensuring that the local publish pipeline and tests remained intact and cleanly decoupled from the deleted v1 `claims` package.
`decision · anchors: internal/core/tick/pipeline.go, cmd/repo-memory/cmd_reindex.go, internal/core/index/index.go, internal/core/index/rebuild.go, internal/core/pipeline/publish/local.go, internal/core/index/v2_test.go, internal/core/tick/pipeline_test.go`

### [seed-e29c518c] Sweep extracts workspaces directly from `transcript.jsonl` files iteratively to map non-spooled runs to their enrolled repos.
`decision · anchors: internal/core/tick/lock_unix.go, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`

### [seed-eda4000f] Used `go:build` flags for platform specific `flock` and `LockFileEx` lock mechanisms.
`decision · anchors: internal/core/tick/lock_unix.go, internal/core/tick/tick.go, internal/core/tick/sweep.go, cmd/repo-memory/cmd_tick.go`

### [seed-f1b5b2e2] Added a workaround extraction method in `processReconcile` to lookup the real underlying session ULID via string manipulation of the job's idempotency key since the extracted candidates did not reliably capture it back from the extraction payload arrays.
`decision · anchors: internal/core/tick/pipeline.go, internal/harness/antigravity/executor.go`

## Open issues

### [seed-9387a988] Usage analytics and pipeline LLM costs are not being written to the database; they are currently discarded using a dummy `&jobs.Cost{}` object on completion for every stage.
`open-issue · anchors: internal/core/tick/pipeline.go, internal/harness/antigravity/executor.go`

### [seed-c309f6be] Transcript parsing and compaction are deferred to Release 2 (R2.1) per SPEC §18.6 rule 11.
`open-issue · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`
