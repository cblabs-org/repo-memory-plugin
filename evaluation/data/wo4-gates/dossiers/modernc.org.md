# modernc.org

## Ruled out, failure signatures and gotchas

### [seed-c6513a63] Direct imports of concrete harness adapters from `internal/core` are forbidden by `boundary_test.go`. Interface injection via `SessionAttributor` avoided boundary violations while allowing full concrete functionality.
`ruled-out · anchors: file:///, modernc.org/sqlite, conversation_summaries.db, tick.SessionAttributor, internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, spool.Entry, hooks.go, antigravity.NewSessionAttributor(), tick.NewTicker, cmd/repo-memory/cmd_tick.go`

## Decisions

### [seed-0c419ff6] Subagents with empty or missing `workspace_uris` recursively inherit their enrolled repository attribution from their parent conversation up the hierarchy tree.
`decision · anchors: file:///, modernc.org/sqlite, conversation_summaries.db, tick.SessionAttributor, internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, spool.Entry, hooks.go, antigravity.NewSessionAttributor(), tick.NewTicker, cmd/repo-memory/cmd_tick.go`

### [seed-1eba9bad] When de-duplicating between app and CLI databases, the record with the latest timestamp is preferred for status/activity, while preserving non-empty workspace paths and parent IDs from either store.
`decision · anchors: file:///, modernc.org/sqlite, conversation_summaries.db, tick.SessionAttributor, internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, spool.Entry, hooks.go, antigravity.NewSessionAttributor(), tick.NewTicker, cmd/repo-memory/cmd_tick.go`

### [seed-7967253a] Maintained strict core/adapter boundary boundaries per SPEC §18.2: `internal/core/tick` defines the `SessionAttributor` interface and does not import concrete adapters (`boundary_test.go` passed 100% with 0 violations across 61 files).
`decision · anchors: file:///, modernc.org/sqlite, conversation_summaries.db, tick.SessionAttributor, internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, spool.Entry, hooks.go, antigravity.NewSessionAttributor(), tick.NewTicker, cmd/repo-memory/cmd_tick.go`

## Open issues

### [seed-69baf7ff] None.
`open-issue · anchors: file:///, modernc.org/sqlite, conversation_summaries.db, tick.SessionAttributor, internal/core/tick/sweep.go, internal/core/tick/sweep.go, internal/core/tick/tick.go, spool.Entry, hooks.go, antigravity.NewSessionAttributor(), tick.NewTicker, cmd/repo-memory/cmd_tick.go`
