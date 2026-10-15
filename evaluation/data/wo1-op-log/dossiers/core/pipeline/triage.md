# core/pipeline/triage

## Ruled out, failure signatures and gotchas

### [seed-53056064] When using `harness.StepKindUser`, the human messages will be parsed as verbatim.
`ruled-out · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`

### [seed-dba1dab0] The `ep.Header.ID` doesn't exist, it is `EpisodeID`.
`ruled-out · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`

## Decisions

### [seed-a3984f4d] Allowed `skip` to only evaluate if `keep` is false in rule engine, so keep takes precedence and we don't accidentally skip-all on an already kept episode.
`decision · anchors: internal/core/pipeline/triage/rules.go, internal/core/pipeline/triage/windows.go, internal/core/pipeline/triage/synopsis.go, internal/core/pipeline/triage/triage.go, internal/core/pipeline/triage/rules_test.go, internal/core/pipeline/triage/windows_test.go, internal/core/pipeline/triage/synopsis_test.go`
