# harness/antigravity

## Ruled out, failure signatures and gotchas

### [seed-30763995] Parsing of Checkpoints and Gap Steps via `transcript.go` caused index offsetting. Gap steps have StepIndex `0`, so they must be explicitly excluded from step index threshold calculations.
`ruled-out · kind: episodic · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-670fd05e] `entries_fts` indexes `dossier_scope` in addition to `id` and `text`, and `RankEntries` awards `+100` (`anchorMatch = "scope"`) to unanchored entries (`len(e.Anchors) == 0`). When testing `MomentFirstEntry` independently of `MomentTaskStart`, ensure the initial `userPrompt` at `MomentTaskStart` does not contain tokens matching the `dossier_scope` of unanchored entries intended to fire first at `MomentFirstEntry`.
`ruled-out · kind: episodic · anchors: internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation_test.go`

## Decisions

### [seed-01c0c6b5] Used `"decision": "force_ask"` for gated entries as instructed in WO4 Line 130 to prompt user confirmation rather than repeated hard blocks before ledger persistence is added in WO5.
`decision · kind: episodic · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-1c868e1f] Fixed early `return nil` in `cmd/repo-memory/cmd_hook.go` line 88 on non-enrolled `PreToolUse` fast path, fixing `TestFastPath_NonEnrolledLatency` and `TestProbe_Disabled_FastPath`.
`decision · kind: episodic · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-203c1800] Bypassed the fast-path explicitly for `PreInvocation` in `cmd_hook.go` to ensure the hook fired.
`decision · kind: episodic · anchors: internal/harness/antigravity/v2_spike_channels.go, cmd/repo-memory/cmd_hook.go, docs/experiments/X5-channels.md`

### [seed-3ace6d21] Extracted unique touched workspace paths by unmarshaling `step.ToolCalls` and filtering paths inside the workspace.
`decision · kind: episodic · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

### [seed-505cbd33] Ignored GAP steps (StepIndex 0) when calculating next valid step index after a checkpoint during compaction detection to avoid mistakenly purging active injections.
`decision · kind: episodic · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-6b2f4c2f] In `compaction.go`, `BuildReinjection` operates strictly on `lost` injection records that were already selected and injected earlier in the conversation; passing `minScore = 0` to `SelectWithinBudget` preserves rank ordering and token budgeting while preventing unverified `gotcha`/`decision`/`procedure` entries from being silently filtered out when `targetAnchors` is `nil`.
`decision · kind: episodic · anchors: internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation_test.go`

### [seed-815aa635] Used the exported `delivery.RankEntries` and `delivery.SelectWithinBudget` directly instead of a non-existent method on `delivery.Engine` for ranking entries.
`decision · kind: episodic · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-8b551d6d] In `preinvocation.go`, when a single file is touched in a turn (e.g. `internal/core/auth/token.go`), `config.DeriveModuleScopes([]string{rel})` rolls single-file directories (`fileCount <= 2`) up to their parent (`core`), whereas repository-wide seeding creates submodule dossiers (e.g. `core/auth`) whenever the directory has `> 2` files in the repo. Including both the direct module scope (`config.DeriveModuleScopes([]string{rel, rel + ".1", rel + ".2"})`) and the rolled-up scope (`config.DeriveModuleScopes(relTouchedFiles)`) ensures `MomentFirstEntry` matches submodule dossiers as well as rolled-up parent dossiers.
`decision · kind: episodic · anchors: internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation_test.go`

### [seed-db3299bd] Used `config.DeriveModuleScopes` to determine scopes of touched files.
`decision · kind: episodic · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

## Open issues

### [seed-3d68e661] A pre-existing flaky test `TestSessionAttributor_Integration` was found in `attribution_test.go`. It has been left untouched since it is outside the WO boundary box.
`open-issue · kind: semantic · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go`

### [seed-411b878c] Tests `TestFastPath_NonEnrolledLatency`, `TestProbe_Disabled_FastPath`, and `TestSessionAttributor_Integration` fail out-of-the-box (pre-existing issue where PreToolUse fastpath outputs multiple lines). Surfaced for the Principal Architect.
`open-issue · kind: semantic · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/enrolment_test.go`

### [seed-b634fade] None (both findings discovered during WO5 verification were incorporated into WO5 with Principal Architect approval).
`open-issue · kind: semantic · anchors: internal/harness/antigravity/preinvocation.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation_test.go`

### [seed-f124f548] `internal/harness/antigravity/attribution_test.go:483` (`TestSessionAttributor_Integration` from PR-02b WO5) checks `fi.ModTime()` of newly created test transcript files against a hardcoded frozen past timestamp `2026-10-02`, which fails when tests run with current system clock. Should be updated with `os.Chtimes` or dynamic relative time.
`open-issue · kind: semantic · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`
