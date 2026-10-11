# core/index

## Ruled out, failure signatures and gotchas

### [seed-54dd3e66] Ensure that the episode store has data written using `Write()` for population testing. Initially used a stub that wasn't writing completely before `PopulateFromStore()` was called.
`ruled-out · anchors: internal/core/index/index.go, internal/core/index/populate.go, internal/core/index/index_test.go`

### [seed-a67dc99c] When writing tests with `cobra.Command`, one must strictly bind any stdout calls (`fmt.Fprintln()`) to `cmd.OutOrStdout()` and set `rootCmd.SetOut(buf)` instead of directly using `os.Stdout`. Otherwise output capture assertions fail.
`ruled-out · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, idx.SearchEpisodes(), cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

## Decisions

### [seed-39911f56] Date logic: For indexing, if an episode's start date is zero, we fall back to the end date. Also used `time.RFC3339` for string serialization in the `date` unindexed column since `time.Time` support in Go's `sql` package may behave unexpectedly without proper mapping, whereas text is highly deterministic in SQLite.
`decision · anchors: internal/core/index/index.go, internal/core/index/populate.go, internal/core/index/index_test.go`

### [seed-4c5a859b] Chose to output to a `tabwriter` that explicitly directs to `cmd.OutOrStdout()` to prevent breaking execution loops in Cobra testing.
`decision · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, idx.SearchEpisodes(), cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

### [seed-dcc31349] Updated `install_test.go` since the `search` stub now correctly throws an error when invoked outside a valid memory-enabled directory.
`decision · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, idx.SearchEpisodes(), cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`

## Open issues

### [seed-5b5f1df6] None
`open-issue · anchors: internal/core/index/index.go, internal/core/index/populate.go, internal/core/index/index_test.go`

### [seed-b18370e2] None
`open-issue · anchors: internal/core/index/index.go, cmd/repo-memory/cmd_search.go, idx.SearchEpisodes(), cmd/repo-memory/cmd_search_test.go, cmd/repo-memory/install_test.go`
