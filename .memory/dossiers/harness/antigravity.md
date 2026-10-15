# harness/antigravity

## Ruled out, failure signatures and gotchas

### [seed-30763995] Parsing of Checkpoints and Gap Steps via `transcript.go` caused index offsetting. Gap steps have StepIndex `0`, so they must be explicitly excluded from step index threshold calculations.
`ruled-out · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

## Decisions

### [seed-01c0c6b5] Used `"decision": "force_ask"` for gated entries as instructed in WO4 Line 130 to prompt user confirmation rather than repeated hard blocks before ledger persistence is added in WO5.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-1c868e1f] Fixed early `return nil` in `cmd/repo-memory/cmd_hook.go` line 88 on non-enrolled `PreToolUse` fast path, fixing `TestFastPath_NonEnrolledLatency` and `TestProbe_Disabled_FastPath`.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-203c1800] Bypassed the fast-path explicitly for `PreInvocation` in `cmd_hook.go` to ensure the hook fired.
`decision · anchors: internal/harness/antigravity/v2_spike_channels.go, cmd/repo-memory/cmd_hook.go, docs/experiments/X5-channels.md`

### [seed-3ace6d21] Extracted unique touched workspace paths by unmarshaling `step.ToolCalls` and filtering paths inside the workspace.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

### [seed-505cbd33] Ignored GAP steps (StepIndex 0) when calculating next valid step index after a checkpoint during compaction detection to avoid mistakenly purging active injections.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-815aa635] Used the exported `delivery.RankEntries` and `delivery.SelectWithinBudget` directly instead of a non-existent method on `delivery.Engine` for ranking entries.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-db3299bd] Used `config.DeriveModuleScopes` to determine scopes of touched files.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

## Open issues

### [seed-3d68e661] A pre-existing flaky test `TestSessionAttributor_Integration` was found in `attribution_test.go`. It has been left untouched since it is outside the WO boundary box.
`open-issue · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-411b878c] Tests `TestFastPath_NonEnrolledLatency`, `TestProbe_Disabled_FastPath`, and `TestSessionAttributor_Integration` fail out-of-the-box (pre-existing issue where PreToolUse fastpath outputs multiple lines). Surfaced for the Principal Architect.
`open-issue · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

### [seed-f124f548] `internal/harness/antigravity/attribution_test.go:483` (`TestSessionAttributor_Integration` from PR-02b WO5) checks `fi.ModTime()` of newly created test transcript files against a hardcoded frozen past timestamp `2026-10-02`, which fails when tests run with current system clock. Should be updated with `os.Chtimes` or dynamic relative time.
`open-issue · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`
