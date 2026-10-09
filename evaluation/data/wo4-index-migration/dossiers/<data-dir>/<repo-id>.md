# <data-dir>/<repo-id>

## Decisions

### [seed-01e79d6d] Symlinks & Subdirectory Enrolment: Registered paths are indexed both as raw paths and evaluated symlink paths (e.g. `/var` vs `/private/var` on macOS). `FindRegistryEntry` also supports subdirectories within enrolled repositories so hooks invoked from subfolders match correctly.
`decision · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, registry.json, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_init.go, GEMINI.md, registry.json, cmd/repo-memory/cmd_leave.go, registry.json, <data-dir>/<repo-id>/, .memory/, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

### [seed-4367d3cc] Origin URL Canonicalization: Normalized SSH (`git@host:owner/repo.git`), schemes (`ssh://`, `git://`), trailing slashes, and `.git` suffixes to `https://host/owner/repo` so all clone protocols produce matching Repo IDs.
`decision · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, registry.json, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_init.go, GEMINI.md, registry.json, cmd/repo-memory/cmd_leave.go, registry.json, <data-dir>/<repo-id>/, .memory/, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`
