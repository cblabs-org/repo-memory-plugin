# core/pipeline/triage

## Ruled out, failure signatures and gotchas

### [seed-53056064] When using `harness.StepKindUser`, the human messages will be parsed as verbatim.
`ruled-out · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`

### [seed-dba1dab0] The `ep.Header.ID` doesn't exist, it is `EpisodeID`.
`ruled-out · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`

## Decisions

### [seed-77325143] Converted `TestBackwardCompatibility_EntitiesAndClaims` in `internal/core/index/v2_test.go` to `TestBackwardCompatibility_Entities` to allow all 11 index tests to pass without referencing `claims`.
`decision · anchors: internal/core/pipeline/triage, internal/core/pipeline/extract, internal/core/pipeline/reconcile, internal/core/pipeline/verify, internal/core/claims, internal/core/events, internal/core/views, cmd/repo-memory/cmd_views.go, internal/core/tick/pipeline.go, claims/store, cmd/repo-memory/cmd_reindex.go, claims/store, internal/core/index/index.go, internal/core/index/rebuild.go, internal/core/pipeline/publish/local.go, local_test.go, internal/core/index/v2_test.go, internal/core/tick/pipeline_test.go`

### [seed-a3984f4d] Allowed `skip` to only evaluate if `keep` is false in rule engine, so keep takes precedence and we don't accidentally skip-all on an already kept episode.
`decision · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`

### [seed-b4f1cf3d] Replaced `claims.Claim` with a local `publish.Item` in `internal/core/pipeline/publish/local.go` rather than deleting `publish`, ensuring that the local publish pipeline and tests remained intact and cleanly decoupled from the deleted v1 `claims` package.
`decision · anchors: internal/core/pipeline/triage, internal/core/pipeline/extract, internal/core/pipeline/reconcile, internal/core/pipeline/verify, internal/core/claims, internal/core/events, internal/core/views, cmd/repo-memory/cmd_views.go, internal/core/tick/pipeline.go, claims/store, cmd/repo-memory/cmd_reindex.go, claims/store, internal/core/index/index.go, internal/core/index/rebuild.go, internal/core/pipeline/publish/local.go, local_test.go, internal/core/index/v2_test.go, internal/core/tick/pipeline_test.go`
