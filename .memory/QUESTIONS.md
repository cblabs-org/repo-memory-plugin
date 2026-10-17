# Memory Elicitation & Open Questions

Generated automatically from repository memory coverage, open issues, and staleness checks.

## 1. Unresolved Open Issues

- **[seed-059e4c29]** (`cmd/repo-memory`): `internal/harness/antigravity/attribution_test.go`: `TestSessionAttributor_Integration` failed due to a hardcoded test clock timestamp (`2026-10-02`) being older than the system clock date (`2026-10-05`). New temp files written to disk during test execution have a ModTime newer than the artificial test `now`, causing the negative duration to fail `< 15 * time.Minute`. Because this is pre-existing code outside WO4's strict boundary box, it was documented rather than modified.
  - Anchors: `cmd/repo-memory/cmd_enroll_test.go`, `.agents/.gitignore`, `.agents/.gitignore`, `.agents/.gitignore`
- **[seed-1a602f9b]** (`core/config`): These tests were failing outside of the modified `config` boundary box and require follow-up.
  - Anchors: `internal/core/config/config.go`, `.memory/config.yaml`, `internal/core/config/config_test.go`, `internal/core/config/subsystem.go`, `internal/core/config/subsystem_test.go`
- **[seed-29440817]** (`core`): Legacy v1 oracle test suites under `evaluation/oracle/wo1-op-log`, `wo2-dossier-schema-renderer`, `wo4-index-migration`, `wo1-ledger-engine`, and `wo4-gates` reference removed `internal/core/claims` or unexported package symbols when running unscoped `go test ./...` across `evaluation/oracle/...`; all production and test packages under `./cmd/...`, `./internal/...`, and `./packaging/...` pass 100%.
  - Anchors: `internal/core/inbox/free_harvest.go`, `internal/core/tick/pipeline.go`, `internal/core/inbox/free_harvest_test.go`, `internal/core/tick/pipeline_test.go`
- **[seed-3d68e661]** (`harness/antigravity`): A pre-existing flaky test `TestSessionAttributor_Integration` was found in `attribution_test.go`. It has been left untouched since it is outside the WO boundary box.
  - Anchors: `internal/harness/antigravity/ledger_store.go`, `internal/harness/antigravity/compaction.go`, `internal/harness/antigravity/preinvocation.go`
- **[seed-411b878c]** (`harness/antigravity`): Tests `TestFastPath_NonEnrolledLatency`, `TestProbe_Disabled_FastPath`, and `TestSessionAttributor_Integration` fail out-of-the-box (pre-existing issue where PreToolUse fastpath outputs multiple lines). Surfaced for the Principal Architect.
  - Anchors: `internal/harness/antigravity/preinvocation.go`, `cmd/repo-memory/cmd_hook.go`, `cmd/repo-memory/enrolment_test.go`
- **[seed-48936155]** (`cmd/repo-memory`): Work order originally mentioned creating `EpisodeRecord` entries directly, but `harness: "import"` refers to the JSONL store `episode.Episode` header schema. I implemented the latter (saving to JSONL store via `episode.NewStore`), which behaves seamlessly with downstream components like FTS index population.
  - Anchors: `cmd/repo-memory/cmd_import_pa.go`, `cmd/repo-memory/cmd_import_pa_test.go`, `cmd/repo-memory/main.go`
- **[seed-4ff705e0]** (`core/jobs`): `cmd/repo-memory/cmd_search_test.go` fails during full repository tests due to global directory manipulation (`os.Chdir(tempDir)`).
  - Anchors: `internal/core/jobs/cost_test.go`, `internal/core/jobs/store_test.go`, `internal/core/tick/pipeline_test.go`, `cmd/repo-memory/cmd_usage.go`, `cmd/repo-memory/main.go`, `cmd/repo-memory/cmd_usage_test.go`
- **[seed-56df9f6f]** (`core/config`): `cmd/repo-memory` tests: `TestSearchCommand` fails locally with `unable to open database file (14)`.
  - Anchors: `internal/core/config/config.go`, `.memory/config.yaml`, `internal/core/config/config_test.go`, `internal/core/config/subsystem.go`, `internal/core/config/subsystem_test.go`
- **[seed-6823189b]** (`root`): There are pre-existing upstream test failures in `cmd/repo-memory/cmd_search_test.go`, `internal/core/jobs/store_test.go`, and `internal/harness/antigravity/attribution_test.go`. The PA should investigate these broken tests under a separate work order.
- **[seed-6d43540c]** (`mcpserver`): `internal/harness/antigravity/attribution_test.go` (`TestSessionAttributor_Integration`) failed due to an idle time assumption (`expected c1 to be idle (modified 1h ago)`). This is an existing timing test failure outside the `internal/mcpserver` boundary box.
  - Anchors: `internal/mcpserver/server.go`, `internal/mcpserver/tools.go`, `internal/mcpserver/tools_test.go`
- **[seed-8da94911]** (`packaging/antigravity/agents`): `gen_metadata` and `usage` metrics were not visible in the local transcript logs.
  - Anchors: `packaging/antigravity/agents/repo-memory-distiller/agent.md`
- **[seed-8e40ba94]** (`root`): `cmd/repo-memory` seems to be ignored by `.gitignore` under the rule `repo-memory`, which could cause git to ignore source files in that directory.
- **[seed-9387a988]** (`core/tick`): Usage analytics and pipeline LLM costs are not being written to the database; they are currently discarded using a dummy `&jobs.Cost{}` object on completion for every stage.
  - Anchors: `internal/core/tick/pipeline.go`, `internal/harness/antigravity/executor.go`
- **[seed-985afa4a]** (`core/pipeline`): `index.go` needs a way to insert individual claims incrementally. Currently, it lacks an `InsertClaim` method. This needs to be built at the application layer or added to the `Index` type natively.
  - Anchors: `internal/core/pipeline/publish/local.go`, `internal/core/pipeline/publish/local_test.go`
- **[seed-b4369427]** (`core/config`): `internal/harness/antigravity` tests: `TestSessionAttributor_Integration` fails locally with `expected c1 to be idle (modified 1h ago)`.
  - Anchors: `internal/core/config/config.go`, `.memory/config.yaml`, `internal/core/config/config_test.go`, `internal/core/config/subsystem.go`, `internal/core/config/subsystem_test.go`
- **[seed-b634fade]** (`harness/antigravity`): None (both findings discovered during WO5 verification were incorporated into WO5 with Principal Architect approval).
  - Anchors: `internal/harness/antigravity/preinvocation.go`, `internal/harness/antigravity/compaction.go`, `internal/harness/antigravity/preinvocation_test.go`
- **[seed-bd7c4a12]** (`core/config`): `internal/core/jobs/store_test.go`: Line 308 checks `if err != nil` after `AcquireNext`, which returns `ErrNoJobAvailable` on empty queues. Should check `errors.Is(err, jobs.ErrNoJobAvailable)`.
  - Anchors: `internal/core/config/layout.go`, `internal/core/episode/store.go`, `internal/core/evidence/checkpoint.go`, `internal/core/evidence/parser.go`, `internal/core/evidence/builder.go`, `internal/core/evidence/checkpoint_test.go`, `internal/core/tick/pipeline.go`, `internal/core/tick/pipeline_test.go`
- **[seed-c309f6be]** (`core/tick`): Transcript parsing and compaction are deferred to Release 2 (R2.1) per SPEC §18.6 rule 11.
  - Anchors: `internal/core/tick/sweep.go`, `internal/core/tick/tick.go`, `internal/harness/claudecode/copier.go`, `cmd/repo-memory/cmd_tick.go`, `internal/harness/claudecode/copier_test.go`
- **[seed-c91eb6fc]** (`core/delivery`): `typeOrder` in the `dossiers/projector.go` is unexported, so it had to be re-declared in the `ranker.go` file.
  - Anchors: `internal/core/delivery/doc.go`, `internal/core/delivery/types.go`, `internal/core/delivery/ledger.go`, `internal/core/delivery/anchor.go`, `internal/core/delivery/ranker.go`, `internal/core/delivery/formatter.go`, `internal/core/delivery/engine.go`, `internal/core/delivery/delivery_test.go`
- **[seed-f124f548]** (`harness/antigravity`): `internal/harness/antigravity/attribution_test.go:483` (`TestSessionAttributor_Integration` from PR-02b WO5) checks `fi.ModTime()` of newly created test transcript files against a hardcoded frozen past timestamp `2026-10-02`, which fails when tests run with current system clock. Should be updated with `os.Chtimes` or dynamic relative time.
  - Anchors: `internal/harness/antigravity/pretooluse.go`, `cmd/repo-memory/cmd_hook.go`, `internal/harness/antigravity/pretooluse_test.go`, `cmd/repo-memory/hook_test.go`
- **[seed-f5f24ef7]** (`harness`): The `agy` CLI's `stream-json` input handler behavior isn't documented externally and ignores standard events like "prompt" or "request", which may block pipeline integration until the expected event name is documented.
  - Anchors: `internal/harness/interfaces.go`, `internal/harness/antigravity/executor.go`, `internal/core/tick/sweep.go`, `internal/harness/antigravity/attribution.go`, `internal/harness/fake/fake.go`, `internal/harness/fake/recorded.go`
- **[seed-fa730066]** (`core/jobs`): `internal/harness/antigravity/attribution_test.go` failed in full suite execution due to an idle-time check flake.
  - Anchors: `internal/core/jobs/cost_test.go`, `internal/core/jobs/store_test.go`, `internal/core/tick/pipeline_test.go`, `cmd/repo-memory/cmd_usage.go`, `cmd/repo-memory/main.go`, `cmd/repo-memory/cmd_usage_test.go`

## 2. Suspect / Stale Entries Needing Verification

_No suspect or stale entries._

## 3. Uncovered & Low-Coverage Scopes

- **Scope `capture-runtime`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `cli`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `core-infrastructure`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `data-model`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `delivery`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `git-operations`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `harness-adapters`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `knowledge-pipeline`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `metrics`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `packaging`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Scope `transcript-episode`**: coverage score `0.00` (0 active, 0 suspect, 0 open issues)
- **Unmapped directory `scripts`**: 0 anchored memory entries

