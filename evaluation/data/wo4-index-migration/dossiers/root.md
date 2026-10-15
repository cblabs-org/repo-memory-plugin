# root

## Ruled out, failure signatures and gotchas

### [seed-4ac0c7aa] When using SQLite in-memory databases (`:memory:`), `PRAGMA journal_mode = WAL;` returns `"memory"`. The store gracefully handles this while ensuring file-backed databases always run in `wal` mode.
`ruled-out · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-5e480c6a] Encountered a few test failures because tests originally asserted behaviors that were explicitly removed (like `GEMINI.md` modification and `.agents/.gitignore` creation). Fixed tests by removing those legacy checks and adapting to the new `.git/info/exclude` approach.
`ruled-out · anchors: config.json, cmd_install.go, .agents/plugins.json, cmd_enroll.go, .git/info/exclude, .agents/.gitignore, cmd_doctor.go, cmd_uninstall.go, .agents/plugins.json, GEMINI.md, PROGRAM-PLAN.md, PROJECT.md, docs/antigravity-plugin-architecture.md, cmd_enroll_test.go, install_test.go, enrolment_test.go`

## Decisions

### [seed-21182716] Two-tier lease reclamation: `ReclaimExpiredLeases` atomically handles both recoverable jobs (reset to `queued` with cleared lease) and crash-looping jobs (transitioned to `dead` once `maxAttempts` is reached).
`decision · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-364ac945] Chose simple heuristic checks for `classifyAuthor` based on keyword analysis of transcript text.
`decision · anchors: internal/core/pipeline/compact/compact.go, compact_test.go`

### [seed-49190a58] Idempotency deduplication: Used unique partial index `WHERE idempotency_key IS NOT NULL AND idempotency_key != ''` so that non-keyed jobs can be enqueued without conflict, while keyed jobs are strictly deduplicated and return existing jobs on duplicate insertion.
`decision · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-4a4f6899] Used `context.Context` in the `Registry` interface for standard cancellation and timeout handling.
`decision · anchors: entity.go, relation.go, registry.go`

### [seed-6cc75a12] Used `os.UserHomeDir()` instead of `config.DataDir()` directly for creating the backup path in `cmd_install.go` to exactly match the specific `~/.gemini/antigravity/...` target without relying on environment variable variations, while passing the mocked `$HOME` in tests to intercept and test properly.
`decision · anchors: config.json, cmd_install.go, .agents/plugins.json, cmd_enroll.go, .git/info/exclude, .agents/.gitignore, cmd_doctor.go, cmd_uninstall.go, .agents/plugins.json, GEMINI.md, PROGRAM-PLAN.md, PROJECT.md, docs/antigravity-plugin-architecture.md, cmd_enroll_test.go, install_test.go, enrolment_test.go`

### [seed-767715a6] Implemented minimalistic structs matching the exact requirements from §5.2.
`decision · anchors: entity.go, relation.go, registry.go`

### [seed-7cc85282] Bounded command excerpt truncation retains failing markers or the last 5-10 lines of output for contexts yielding a non-zero exit code.
`decision · anchors: internal/core/pipeline/compact/compact.go, compact_test.go`

### [seed-8206d969] Collapsing duplicate identical log lines for device/test outputs ensures major compaction without losing structure.
`decision · anchors: internal/core/pipeline/compact/compact.go, compact_test.go`

### [seed-aab4c02e] Used a decorator pattern (`usageTracker`) around `harness.Housekeeper` in `pipeline.go` so token usage is captured directly without changing the housekeeper interface or stage constructor signatures.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, gen_metadata.go, quota.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-b330b872] Supported both object and array layouts in `ReadGenMetadata` to accommodate varied Antigravity internal metadata formats, auto-aggregating totals if top-level counts are zero.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, gen_metadata.go, quota.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-b9d9f70f] Pure-Go SQLite: Selected `modernc.org/sqlite` per SPEC §18.4 to maintain a cgo-free single binary.
`decision · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-cfca4c77] Preserved hierarchical groups in `QuotaStatus.Groups` while also flattening buckets into `QuotaStatus.Buckets` with helper methods (`GetBucket`, `RemainingFraction`) for convenient access by spike experiments.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, gen_metadata.go, quota.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

## Open issues

### [seed-4ff705e0] `cmd/repo-memory/cmd_search_test.go` fails during full repository tests due to global directory manipulation (`os.Chdir(tempDir)`).
`open-issue · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, gen_metadata.go, quota.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-fa730066] `internal/harness/antigravity/attribution_test.go` failed in full suite execution due to an idle-time check flake.
`open-issue · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, gen_metadata.go, quota.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`
