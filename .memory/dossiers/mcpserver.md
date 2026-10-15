# mcpserver

## Ruled out, failure signatures and gotchas

### [seed-acc7225d] SQLite `entries` table schema does not store a `retired` column (retired entries reside in `entry_history`). Therefore, in `HandleMemoryBrief`, entries retrieved from `ListEntriesByScope` are cross-checked against `GetHistory(ctx, e.ID)` to ensure retired entries are filtered out.
`ruled-out · kind: episodic · anchors: internal/mcpserver/server.go, internal/mcpserver/tools.go, internal/mcpserver/tools_test.go`

## Decisions

### [seed-1c34eec8] In `HandleMemoryWhy`, supported both `entry.Evidence` slice and `entry.Source` fallback to guarantee evidence provenance is printed.
`decision · kind: episodic · anchors: internal/mcpserver/server.go, internal/mcpserver/tools.go, internal/mcpserver/tools_test.go`

### [seed-3511d024] In `HandleMemoryNote`, wrote the op batch to disk via `ops.Writer` and additionally upserted the entry into `idx` (when present) so subsequent queries in the same process can immediately access newly captured notes.
`decision · kind: episodic · anchors: internal/mcpserver/server.go, internal/mcpserver/tools.go, internal/mcpserver/tools_test.go`

### [seed-cffc3736] In `HandleMemoryHistory`, combined queries from `ListEntriesByScope` (for entries marked retired/superseded), `GetHistory` for each entry ID, and direct query of `entry_history` via `WriteTx` with deduplication to ensure retired entries are reliably discovered regardless of whether the index was populated via ops replay or direct insertion.
`decision · kind: episodic · anchors: internal/mcpserver/server.go, internal/mcpserver/tools.go, internal/mcpserver/tools_test.go`

## Open issues

### [seed-6d43540c] `internal/harness/antigravity/attribution_test.go` (`TestSessionAttributor_Integration`) failed due to an idle time assumption (`expected c1 to be idle (modified 1h ago)`). This is an existing timing test failure outside the `internal/mcpserver` boundary box.
`open-issue · kind: semantic · anchors: internal/mcpserver/server.go, internal/mcpserver/tools.go, internal/mcpserver/tools_test.go`
