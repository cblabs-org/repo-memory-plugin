# core/index

## Ruled out, failure signatures and gotchas

### [seed-3a98c0c9] When writing job pipelines, `Housekeeper` must be securely injected via `TickerOption`, as it requires context. Ensure mock harnesses correctly simulate `Usage` return data for the pipeline token budget accounting.
`ruled-out · anchors: internal/core/index/index.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-54dd3e66] Ensure that the episode store has data written using `Write()` for population testing. Initially used a stub that wasn't writing completely before `PopulateFromStore()` was called.
`ruled-out · anchors: internal/core/index/index.go, internal/core/index/populate.go, internal/core/index/index_test.go`

### [seed-a67dc99c] When writing tests with `cobra.Command`, one must strictly bind any stdout calls (`fmt.Fprintln()`) to `cmd.OutOrStdout()` and set `rootCmd.SetOut(buf)` instead of directly using `os.Stdout`. Otherwise output capture assertions fail.
`ruled-out · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

### [seed-d378e842] Note that extraction requires `sync.Mutex` during payload combination; race conditions can appear if `ExtractionResult` arrays are joined outside the lock.
`ruled-out · anchors: internal/core/index/index.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

## Decisions

### [seed-1efa4e49] Chose to maintain `pipeline.go` as the core orchestrator, directly interacting with `jobs.Store` and passing payloads sequentially between the pipeline components, ensuring that duplicate idempotency keys result in no-op processing per the workorder specifications.
`decision · anchors: internal/core/index/index.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-370ea357] Preserved existing `Rebuild` function signature and added `RebuildV1Index` alias so callers like `cmd/repo-memory/cmd_reindex.go` compile without changes.
`decision · anchors: internal/core/index/index.go, internal/core/index/entries.go, internal/core/index/entry_anchors.go, internal/core/index/rebuild.go, internal/core/index/v2_test.go`

### [seed-39911f56] Date logic: For indexing, if an episode's start date is zero, we fall back to the end date. Also used `time.RFC3339` for string serialization in the `date` unindexed column since `time.Time` support in Go's `sql` package may behave unexpectedly without proper mapping, whereas text is highly deterministic in SQLite.
`decision · anchors: internal/core/index/index.go, internal/core/index/populate.go, internal/core/index/index_test.go`

### [seed-4c5a859b] Chose to output to a `tabwriter` that explicitly directs to `cmd.OutOrStdout()` to prevent breaking execution loops in Cobra testing.
`decision · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

### [seed-7399e38c] `GetEntry` and scope listings hydrate anchors from `entry_anchors` so caller receives fully populated `dossiers.Entry` objects.
`decision · anchors: internal/core/index/index.go, internal/core/index/entries.go, internal/core/index/entry_anchors.go, internal/core/index/rebuild.go, internal/core/index/v2_test.go`

### [seed-c1392cd3] Anchors are stored normalized in `entry_anchors`, with `LookupByAnchor` executing a `LIKE ?` prefix query and `LookupByAnchorExact` performing exact match.
`decision · anchors: internal/core/index/index.go, internal/core/index/entries.go, internal/core/index/entry_anchors.go, internal/core/index/rebuild.go, internal/core/index/v2_test.go`

### [seed-dcc31349] Updated `install_test.go` since the `search` stub now correctly throws an error when invoked outside a valid memory-enabled directory.
`decision · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

### [seed-f9a780a4] Used FTS5 external content table (`content=entries, content_rowid=rowid`) with SQLite triggers for automatic insert, update, and delete synchronization between `entries` and `entries_fts`.
`decision · anchors: internal/core/index/index.go, internal/core/index/entries.go, internal/core/index/entry_anchors.go, internal/core/index/rebuild.go, internal/core/index/v2_test.go`

## Open issues

### [seed-1c3bab6e] (none)
`open-issue · anchors: internal/core/index/index.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`
