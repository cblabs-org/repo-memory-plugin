# core

## Ruled out, failure signatures and gotchas

### [seed-ce39aaca] (none)
`ruled-out · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views/`

## Decisions

### [seed-4212517c] Designed `Generator.Generate()` to filter claims internally by type (e.g. failure signatures vs decisions) and output the respective files so it handles missing/empty collections safely.
`decision · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views/`

## Open issues

### [seed-6823189b] There are pre-existing upstream test failures in `cmd/repo-memory/cmd_search_test.go`, `internal/core/jobs/store_test.go`, and `internal/harness/antigravity/attribution_test.go`. The PA should investigate these broken tests under a separate work order.
`open-issue · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views/`

### [seed-8e40ba94] `cmd/repo-memory` seems to be ignored by `.gitignore` under the rule `repo-memory`, which could cause git to ignore source files in that directory.
`open-issue · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views/`
