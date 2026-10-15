# harness/antigravity

## Ruled out, failure signatures and gotchas

### [seed-084601c6] When using `agy` as a background LLM execution harness, always map inputs via `-p -` and provide them through `stdin` to avoid maximum argument size limits which truncate the JSON structures and crash processing.
`ruled-out · anchors: internal/core/tick/pipeline.go, internal/core/pipeline/extract/extractor.go, internal/harness/antigravity/executor.go`

### [seed-30763995] Parsing of Checkpoints and Gap Steps via `transcript.go` caused index offsetting. Gap steps have StepIndex `0`, so they must be explicitly excluded from step index threshold calculations.
`ruled-out · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-4c2dd2e9] When using `--input-format stream-json`, `agy` explicitly requires `--output-format stream-json`. The instruction requested `--output-format json`, but that was rejected by the CLI.
`ruled-out · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, fake_test.go, internal/harness/fake/recorded.go`

### [seed-54b99462] Go was not installed initially on the host machine; installed Go 1.27.1 via Homebrew (`brew install go`).
`ruled-out · anchors: go.mod, go.sum, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go`

### [seed-5fd3847d] The pipeline `processReconcile` job sequence scales sequentially which takes roughly 10 seconds per candidate. This creates significant wall-clock latency (e.g. 60+ minutes) for bulk backfilling.
`ruled-out · anchors: internal/core/tick/pipeline.go, internal/core/pipeline/extract/extractor.go, internal/harness/antigravity/executor.go`

### [seed-64752358] Sending raw string events like `{"prompt": "..."}` or `{"event": "prompt", ...}` over stdin for stream-json gets ignored by `agy` with "warning: ignoring unsupported stream input message event". Wrapping it dynamically as an event for test-compatibility but `agy` requires a specific event name to actually execute.
`ruled-out · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, fake_test.go, internal/harness/fake/recorded.go`

## Decisions

### [seed-01c0c6b5] Used `"decision": "force_ask"` for gated entries as instructed in WO4 Line 130 to prompt user confirmation rather than repeated hard blocks before ledger persistence is added in WO5.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-1c868e1f] Fixed early `return nil` in `cmd/repo-memory/cmd_hook.go` line 88 on non-enrolled `PreToolUse` fast path, fixing `TestFastPath_NonEnrolledLatency` and `TestProbe_Disabled_FastPath`.
`decision · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-203c1800] Bypassed the fast-path explicitly for `PreInvocation` in `cmd_hook.go` to ensure the hook fired.
`decision · anchors: internal/harness/antigravity/v2_spike_channels.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/v2_spike_channels_test.go, docs/experiments/X5-channels.md`

### [seed-2c90e2ae] Used standard library `go/parser` and `go/token` in `internal/core/boundary_test.go` instead of external dependencies for maximum speed and portability.
`decision · anchors: go.mod, go.sum, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go`

### [seed-3ace6d21] Extracted unique touched workspace paths by unmarshaling `step.ToolCalls` and filtering paths inside the workspace.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-4e193660] Followed cherry-pick order exactly as specified in the work order.
`decision · anchors: pr-05b-dogfood-pilot/wo4-b5-evaluation, pr-05-v2-foundation/wo1-close-pr05b, .memory/claims, .memory/archive/v1-claims, docs/dogfood-evaluation-01.md, docs/design-v2-distillation-retrieval.md, docs/experiments/SPEC-CHANGES.md, internal/harness/antigravity/executor.go`

### [seed-505cbd33] Ignored GAP steps (StepIndex 0) when calculating next valid step index after a checkpoint during compaction detection to avoid mistakenly purging active injections.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-72ca47ce] Explicitly defined a robust `DO NOT USE ANY TOOLS` instruction inside the standard prompt wrapping JSON since the LLM executor defaults to tool-enabled environments and was actively attempting out-of-bounds bash calls.
`decision · anchors: internal/core/tick/pipeline.go, internal/core/pipeline/extract/extractor.go, internal/harness/antigravity/executor.go`

### [seed-815aa635] Used the exported `delivery.RankEntries` and `delivery.SelectWithinBudget` directly instead of a non-existent method on `delivery.Engine` for ranking entries.
`decision · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-95b3ba69] Staged and committed archive movement of `.memory/claims/` to maintain clean git status.
`decision · anchors: pr-05b-dogfood-pilot/wo4-b5-evaluation, pr-05-v2-foundation/wo1-close-pr05b, .memory/claims, .memory/archive/v1-claims, docs/dogfood-evaluation-01.md, docs/design-v2-distillation-retrieval.md, docs/experiments/SPEC-CHANGES.md, internal/harness/antigravity/executor.go`

### [seed-a3027bbf] Handled both camelCase (`artifactDirectoryPath`) and snake_case (`artifact_directory_path`) in `StopPayload.Normalize()`.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, internal/core/evidence/artifacts_test.go, internal/harness/antigravity/hooks.go, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-db3299bd] Used `config.DeriveModuleScopes` to determine scopes of touched files.
`decision · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-e061d96d] Used atomic temp-file write and rename in `CopyArtifacts` to avoid partial reads or corrupt writes.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, internal/core/evidence/artifacts_test.go, internal/harness/antigravity/hooks.go, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-f1b5b2e2] Added a workaround extraction method in `processReconcile` to lookup the real underlying session ULID via string manipulation of the job's idempotency key since the extracted candidates did not reliably capture it back from the extraction payload arrays.
`decision · anchors: internal/core/tick/pipeline.go, internal/core/pipeline/extract/extractor.go, internal/harness/antigravity/executor.go`

### [seed-f61bac8b] In `snapshot` command, `resolveBrainDir` accepts both conversation IDs and direct directory paths (extracting the basename as conversation ID), making local testing and explicit path inputs seamless.
`decision · anchors: internal/core/config/layout.go, internal/core/evidence/artifacts.go, internal/core/evidence/artifacts_test.go, internal/harness/antigravity/hooks.go, internal/harness/antigravity/hooks_test.go, internal/core/tick/sweep.go, internal/core/tick/sweep_test.go, cmd/repo-memory/cmd_snapshot.go, cmd/repo-memory/cmd_snapshot_test.go, cmd/repo-memory/main.go`

### [seed-fec5210b] Configured `cobra` command stubs with `cobra.ArbitraryArgs` to accept CLI parameters without failing flag/arg validation during early integration.
`decision · anchors: go.mod, go.sum, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go`

## Open issues

### [seed-3d68e661] A pre-existing flaky test `TestSessionAttributor_Integration` was found in `attribution_test.go`. It has been left untouched since it is outside the WO boundary box.
`open-issue · anchors: internal/harness/antigravity/ledger_store.go, internal/harness/antigravity/compaction.go, internal/harness/antigravity/preinvocation.go, compaction_test.go, ledger_store_test.go, preinvocation_test.go`

### [seed-411b878c] Tests `TestFastPath_NonEnrolledLatency`, `TestProbe_Disabled_FastPath`, and `TestSessionAttributor_Integration` fail out-of-the-box (pre-existing issue where PreToolUse fastpath outputs multiple lines). Surfaced for the Principal Architect.
`open-issue · anchors: internal/harness/antigravity/preinvocation.go, cmd/repo-memory/cmd_hook.go, v2_spike_channels.go, preinvocation_test.go, cmd/repo-memory/enrolment_test.go`

### [seed-9387a988] Usage analytics and pipeline LLM costs are not being written to the database; they are currently discarded using a dummy `&jobs.Cost{}` object on completion for every stage.
`open-issue · anchors: internal/core/tick/pipeline.go, internal/core/pipeline/extract/extractor.go, internal/harness/antigravity/executor.go`

### [seed-f124f548] `internal/harness/antigravity/attribution_test.go:483` (`TestSessionAttributor_Integration` from PR-02b WO5) checks `fi.ModTime()` of newly created test transcript files against a hardcoded frozen past timestamp `2026-10-02`, which fails when tests run with current system clock. Should be updated with `os.Chtimes` or dynamic relative time.
`open-issue · anchors: internal/harness/antigravity/pretooluse.go, cmd/repo-memory/cmd_hook.go, internal/harness/antigravity/pretooluse_test.go, cmd/repo-memory/hook_test.go`

### [seed-f5f24ef7] The `agy` CLI's `stream-json` input handler behavior isn't documented externally and ignores standard events like "prompt" or "request", which may block pipeline integration until the expected event name is documented.
`open-issue · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, fake_test.go, internal/harness/fake/recorded.go`
