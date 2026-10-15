# core

## Ruled out, failure signatures and gotchas

### [seed-ce39aaca] (none)
`ruled-out · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views`

## Decisions

### [seed-01e79d6d] Symlinks & Subdirectory Enrolment: Registered paths are indexed both as raw paths and evaluated symlink paths (e.g. `/var` vs `/private/var` on macOS). `FindRegistryEntry` also supports subdirectories within enrolled repositories so hooks invoked from subfolders match correctly.
`decision · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, registry.json, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_init.go, GEMINI.md, registry.json, cmd/repo-memory/cmd_leave.go, registry.json, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

### [seed-4212517c] Designed `Generator.Generate()` to filter claims internally by type (e.g. failure signatures vs decisions) and output the respective files so it handles missing/empty collections safely.
`decision · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views`

### [seed-4367d3cc] Origin URL Canonicalization: Normalized SSH (`git@host:owner/repo.git`), schemes (`ssh://`, `git://`), trailing slashes, and `.git` suffixes to `https://host/owner/repo` so all clone protocols produce matching Repo IDs.
`decision · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, registry.json, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_init.go, GEMINI.md, registry.json, cmd/repo-memory/cmd_leave.go, registry.json, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

## Open issues

### [seed-6823189b] There are pre-existing upstream test failures in `cmd/repo-memory/cmd_search_test.go`, `internal/core/jobs/store_test.go`, and `internal/harness/antigravity/attribution_test.go`. The PA should investigate these broken tests under a separate work order.
`open-issue · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views`

### [seed-8e40ba94] `cmd/repo-memory` seems to be ignored by `.gitignore` under the rule `repo-memory`, which could cause git to ignore source files in that directory.
`open-issue · anchors: internal/core/views/generator.go, MAP.md, SIGNATURES.md, DECISIONS.md, cmd/repo-memory/cmd_views.go, .memory/views`
