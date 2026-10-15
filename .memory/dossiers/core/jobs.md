# core/jobs

## Decisions

### [seed-aab4c02e] Used a decorator pattern (`usageTracker`) around `harness.Housekeeper` in `pipeline.go` so token usage is captured directly without changing the housekeeper interface or stage constructor signatures.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-b330b872] Supported both object and array layouts in `ReadGenMetadata` to accommodate varied Antigravity internal metadata formats, auto-aggregating totals if top-level counts are zero.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-cfca4c77] Preserved hierarchical groups in `QuotaStatus.Groups` while also flattening buckets into `QuotaStatus.Buckets` with helper methods (`GetBucket`, `RemainingFraction`) for convenient access by spike experiments.
`decision · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

## Open issues

### [seed-4ff705e0] `cmd/repo-memory/cmd_search_test.go` fails during full repository tests due to global directory manipulation (`os.Chdir(tempDir)`).
`open-issue · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`

### [seed-fa730066] `internal/harness/antigravity/attribution_test.go` failed in full suite execution due to an idle-time check flake.
`open-issue · anchors: internal/core/jobs/cost_test.go, internal/core/jobs/store_test.go, internal/core/tick/pipeline_test.go, cmd/repo-memory/cmd_usage.go, cmd/repo-memory/main.go, cmd/repo-memory/cmd_usage_test.go`
