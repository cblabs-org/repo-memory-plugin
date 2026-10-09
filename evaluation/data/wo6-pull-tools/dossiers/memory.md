# memory

## Ruled out, failure signatures and gotchas

### [seed-de536fc6] None.
`ruled-out · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, ~/.claude/projects/, subagents/*.jsonl, <data-dir>/<repo-id>/raw/claude-code/, memory/, claudecode.CopyTranscripts, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`

## Decisions

### [seed-5a8e8744] Used a decoupled registration pattern (`tick.RegisterCopier`) to keep `internal/core` free of adapter dependencies.
`decision · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, ~/.claude/projects/, subagents/*.jsonl, <data-dir>/<repo-id>/raw/claude-code/, memory/, claudecode.CopyTranscripts, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`

## Open issues

### [seed-c309f6be] Transcript parsing and compaction are deferred to Release 2 (R2.1) per SPEC §18.6 rule 11.
`open-issue · anchors: internal/core/tick/sweep.go, internal/core/tick/tick.go, internal/harness/claudecode/copier.go, ~/.claude/projects/, subagents/*.jsonl, <data-dir>/<repo-id>/raw/claude-code/, memory/, claudecode.CopyTranscripts, cmd/repo-memory/cmd_tick.go, internal/harness/claudecode/copier_test.go`
