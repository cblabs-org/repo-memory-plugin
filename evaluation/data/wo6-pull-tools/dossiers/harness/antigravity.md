# harness/antigravity

## Ruled out, failure signatures and gotchas

### [seed-084601c6] When using `agy` as a background LLM execution harness, always map inputs via `-p -` and provide them through `stdin` to avoid maximum argument size limits which truncate the JSON structures and crash processing.
`ruled-out · anchors: internal/core/tick/pipeline.go, cand.EpisodeID, epStore.Read(), internal/core/pipeline/extract/extractor.go, jsonschema.Reflect, internal/harness/antigravity/executor.go`

### [seed-0adcc49f] None encountered in PreToolUse handler.
`ruled-out · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, antigravity.HandlePreToolUse, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-30763995] Parsing of Checkpoints and Gap Steps via `transcript.go` caused index offsetting. Gap steps have StepIndex `0`, so they must be explicitly excluded from step index threshold calculations.
`ruled-out · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-49690164] (pending)
`ruled-out · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-5fd3847d] The pipeline `processReconcile` job sequence scales sequentially which takes roughly 10 seconds per candidate. This creates significant wall-clock latency (e.g. 60+ minutes) for bulk backfilling.
`ruled-out · anchors: internal/core/tick/pipeline.go, cand.EpisodeID, epStore.Read(), internal/core/pipeline/extract/extractor.go, jsonschema.Reflect, internal/harness/antigravity/executor.go`

### [seed-620e8ecc] None encountered within the boundary box.
`ruled-out · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, .system_generated/, internal/core/evidence/artifacts_test.go, .system_generated, internal/harness/antigravity/hooks.go, payload.ArtifactDirectoryPath, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-b1617c95] Antigravity completely ignores `systemMessage` and `userMessage` fields returned from `PreInvocation`. The hook gets executed and returns the data, but it is not injected into the transcript.
`ruled-out · anchors: internal/harness/antigravity/v2_spike_channels.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/v2_spike_channels_test.go, docs/experiments/X5-channels.md`

### [seed-d591a9f8] N/A
`ruled-out · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

## Decisions

### [seed-01c0c6b5] Used `"decision": "force_ask"` for gated entries as instructed in WO4 Line 130 to prompt user confirmation rather than repeated hard blocks before ledger persistence is added in WO5.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, antigravity.HandlePreToolUse, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-1c868e1f] Fixed early `return nil` in `cmd/repo-memory/cmd_hook.go` line 88 on non-enrolled `PreToolUse` fast path, fixing `TestFastPath_NonEnrolledLatency` and `TestProbe_Disabled_FastPath`.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, antigravity.HandlePreToolUse, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-203c1800] Bypassed the fast-path explicitly for `PreInvocation` in `cmd_hook.go` to ensure the hook fired.
`decision · anchors: internal/harness/antigravity/v2_spike_channels.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/v2_spike_channels_test.go, docs/experiments/X5-channels.md`

### [seed-3ace6d21] Extracted unique touched workspace paths by unmarshaling `step.ToolCalls` and filtering paths inside the workspace.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-505cbd33] Ignored GAP steps (StepIndex 0) when calculating next valid step index after a checkpoint during compaction detection to avoid mistakenly purging active injections.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-72ca47ce] Explicitly defined a robust `DO NOT USE ANY TOOLS` instruction inside the standard prompt wrapping JSON since the LLM executor defaults to tool-enabled environments and was actively attempting out-of-bounds bash calls.
`decision · anchors: internal/core/tick/pipeline.go, cand.EpisodeID, epStore.Read(), internal/core/pipeline/extract/extractor.go, jsonschema.Reflect, internal/harness/antigravity/executor.go`

### [seed-815aa635] Used the exported `delivery.RankEntries` and `delivery.SelectWithinBudget` directly instead of a non-existent method on `delivery.Engine` for ranking entries.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-a3027bbf] Handled both camelCase (`artifactDirectoryPath`) and snake_case (`artifact_directory_path`) in `StopPayload.Normalize()`.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, .system_generated/, internal/core/evidence/artifacts_test.go, .system_generated, internal/harness/antigravity/hooks.go, payload.ArtifactDirectoryPath, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-b60a7966] (pending)
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-db3299bd] Used `config.DeriveModuleScopes` to determine scopes of touched files.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-e061d96d] Used atomic temp-file write and rename in `CopyArtifacts` to avoid partial reads or corrupt writes.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, .system_generated/, internal/core/evidence/artifacts_test.go, .system_generated, internal/harness/antigravity/hooks.go, payload.ArtifactDirectoryPath, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-f1b5b2e2] Added a workaround extraction method in `processReconcile` to lookup the real underlying session ULID via string manipulation of the job's idempotency key since the extracted candidates did not reliably capture it back from the extraction payload arrays.
`decision · anchors: internal/core/tick/pipeline.go, cand.EpisodeID, epStore.Read(), internal/core/pipeline/extract/extractor.go, jsonschema.Reflect, internal/harness/antigravity/executor.go`

### [seed-f61bac8b] In `snapshot` command, `resolveBrainDir` accepts both conversation IDs and direct directory paths (extracting the basename as conversation ID), making local testing and explicit path inputs seamless.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, .system_generated/, internal/core/evidence/artifacts_test.go, .system_generated, internal/harness/antigravity/hooks.go, payload.ArtifactDirectoryPath, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

## Open issues

### [seed-3d68e661] A pre-existing flaky test `TestSessionAttributor_Integration` was found in `attribution_test.go`. It has been left untouched since it is outside the WO boundary box.
`open-issue · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-411b878c] Tests `TestFastPath_NonEnrolledLatency`, `TestProbe_Disabled_FastPath`, and `TestSessionAttributor_Integration` fail out-of-the-box (pre-existing issue where PreToolUse fastpath outputs multiple lines). Surfaced for the Principal Architect.
`open-issue · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-9387a988] Usage analytics and pipeline LLM costs are not being written to the database; they are currently discarded using a dummy `&jobs.Cost{}` object on completion for every stage.
`open-issue · anchors: internal/core/tick/pipeline.go, cand.EpisodeID, epStore.Read(), internal/core/pipeline/extract/extractor.go, jsonschema.Reflect, internal/harness/antigravity/executor.go`

### [seed-f124f548] `internal/harness/antigravity/attribution_test.go:483` (`TestSessionAttributor_Integration` from PR-02b WO5) checks `fi.ModTime()` of newly created test transcript files against a hardcoded frozen past timestamp `2026-10-02`, which fails when tests run with current system clock. Should be updated with `os.Chtimes` or dynamic relative time.
`open-issue · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, antigravity.HandlePreToolUse, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-f701a196] (pending)
`open-issue · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`
