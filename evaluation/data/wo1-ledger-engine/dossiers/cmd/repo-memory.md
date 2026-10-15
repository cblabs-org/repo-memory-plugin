# cmd/repo-memory

## Ruled out, failure signatures and gotchas

### [seed-2d9c650e] **Antigravity In-Memory Plugin Path Caching**: When Antigravity starts, it caches global plugin root directories in memory. Any hook runner invocation sets the working directory to that cached path. Renaming the legacy directory live caused all subsequent tool executions to fail with `chdir ...: no such file or directory`. Future migration tools should copy or maintain a compatibility symlink/shim until the IDE process restarts.
`ruled-out · anchors: cmd/repo-memory/cmd_enroll_test.go, .agents/plugins.json, .agents/.gitignore, .agents/plugins.json, .agents/.gitignore, plugins.json, plugin.json, config.json, sidecars/tick/sidecar.json, plugin.json, hooks.json, mcp_config.json, .agents/plugins.json, plugins.json, .agents/.gitignore`

### [seed-42169b28] **Subprocesses in fast path**: Any invocation of `git` or external binaries in the fast path introduces 5-20 ms of overhead. Using pure `os.Stat` is essential to maintaining the sub-millisecond path.
`ruled-out · anchors: .gitignore, cmd/repo-memory, .memory/.gitignore, internal/core/config/layout.go, internal/core/config/marker.go, .memory/.enrolled, .memory/.enrolled, internal/core/config/registry.go, cmd/repo-memory/cmd_init.go, cmd/repo-memory/cmd_leave.go, .memory/.enrolled, .memory/.enrolled, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/cmd_install.go, hooks.json, internal/core/config/log_rotate.go, internal/core/config/marker_test.go, internal/core/config/log_rotate_test.go, cmd/repo-memory/fastpath_test.go, enrolment_test.go, hook_test.go, install_test.go, probe_test.go`

### [seed-7a5f2cce] **PreToolUse implicit deny**: In Antigravity, returning `{}` or empty output for a `PreToolUse` hook causes Antigravity to fail-closed and block all user/agent tool execution. Non-enrolled and fallback responses for `PreToolUse` MUST always return `{"decision":"allow"}`.
`ruled-out · anchors: .gitignore, cmd/repo-memory, .memory/.gitignore, internal/core/config/layout.go, internal/core/config/marker.go, .memory/.enrolled, .memory/.enrolled, internal/core/config/registry.go, cmd/repo-memory/cmd_init.go, cmd/repo-memory/cmd_leave.go, .memory/.enrolled, .memory/.enrolled, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/cmd_install.go, hooks.json, internal/core/config/log_rotate.go, internal/core/config/marker_test.go, internal/core/config/log_rotate_test.go, cmd/repo-memory/fastpath_test.go, enrolment_test.go, hook_test.go, install_test.go, probe_test.go`

## Decisions

### [seed-069a4cc4] **Read-back validation on install**: `cmd_install.go` checks generated content and reads back the written `hooks.json` file to guarantee it contains non-empty hook configurations before considering installation successful.
`decision · anchors: .gitignore, cmd/repo-memory, .memory/.gitignore, internal/core/config/layout.go, internal/core/config/marker.go, .memory/.enrolled, .memory/.enrolled, internal/core/config/registry.go, cmd/repo-memory/cmd_init.go, cmd/repo-memory/cmd_leave.go, .memory/.enrolled, .memory/.enrolled, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/cmd_install.go, hooks.json, internal/core/config/log_rotate.go, internal/core/config/marker_test.go, internal/core/config/log_rotate_test.go, cmd/repo-memory/fastpath_test.go, enrolment_test.go, hook_test.go, install_test.go, probe_test.go`

### [seed-5653e267] **Fail-open default for PreToolUse**: When non-enrolled or on error/kill switch, `PreToolUse` explicitly returns `{"decision":"allow"}\n` to ensure Antigravity never denies agent tool execution in non-enrolled workspaces.
`decision · anchors: .gitignore, cmd/repo-memory, .memory/.gitignore, internal/core/config/layout.go, internal/core/config/marker.go, .memory/.enrolled, .memory/.enrolled, internal/core/config/registry.go, cmd/repo-memory/cmd_init.go, cmd/repo-memory/cmd_leave.go, .memory/.enrolled, .memory/.enrolled, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/cmd_install.go, hooks.json, internal/core/config/log_rotate.go, internal/core/config/marker_test.go, internal/core/config/log_rotate_test.go, cmd/repo-memory/fastpath_test.go, enrolment_test.go, hook_test.go, install_test.go, probe_test.go`

### [seed-6c7b0946] Leveraged `go-version-file: 'go.mod'` in `actions/setup-go@v5` across both workflows to maintain version consistency with `go.mod`.
`decision · anchors: cmd/repo-memory/main.go, .github/workflows/ci.yml, .github/workflows/release.yml, softprops/action-gh-release@v2`

### [seed-9ba8da0d] Maintained a temporary directory placeholder at `~/.gemini/config/plugins/repo-memory` for the active session: Antigravity's in-memory hook runner keeps the initial plugin path loaded in memory and sets `cmd.Dir` to it when executing hooks. Keeping this path available until IDE restart prevents `chdir: no such file or directory` errors during active session tool runs.
`decision · anchors: cmd/repo-memory/cmd_enroll_test.go, .agents/plugins.json, .agents/.gitignore, .agents/plugins.json, .agents/.gitignore, plugins.json, plugin.json, config.json, sidecars/tick/sidecar.json, plugin.json, hooks.json, mcp_config.json, .agents/plugins.json, plugins.json, .agents/.gitignore`

### [seed-c92fa785] **Zero-byte filesystem marker**: `.memory/.enrolled` was chosen over registry queries or SQLite lookups for fast path resolution, reducing non-enrolled hook check latency to ~0.013 ms (13.4 µs in benchmark), well below the 5 ms target.
`decision · anchors: .gitignore, cmd/repo-memory, .memory/.gitignore, internal/core/config/layout.go, internal/core/config/marker.go, .memory/.enrolled, .memory/.enrolled, internal/core/config/registry.go, cmd/repo-memory/cmd_init.go, cmd/repo-memory/cmd_leave.go, .memory/.enrolled, .memory/.enrolled, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/cmd_install.go, hooks.json, internal/core/config/log_rotate.go, internal/core/config/marker_test.go, internal/core/config/log_rotate_test.go, cmd/repo-memory/fastpath_test.go, enrolment_test.go, hook_test.go, install_test.go, probe_test.go`

### [seed-e8c7bff7] Used `softprops/action-gh-release@v2` with `permissions: contents: write` and `env: GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` for robust Node 20 GitHub release asset management.
`decision · anchors: cmd/repo-memory/main.go, .github/workflows/ci.yml, .github/workflows/release.yml, softprops/action-gh-release@v2`

## Open issues

### [seed-059e4c29] `internal/harness/antigravity/attribution_test.go`: `TestSessionAttributor_Integration` failed due to a hardcoded test clock timestamp (`2026-10-02`) being older than the system clock date (`2026-10-05`). New temp files written to disk during test execution have a ModTime newer than the artificial test `now`, causing the negative duration to fail `< 15 * time.Minute`. Because this is pre-existing code outside WO4's strict boundary box, it was documented rather than modified.
`open-issue · anchors: cmd/repo-memory/cmd_enroll_test.go, .agents/plugins.json, .agents/.gitignore, .agents/plugins.json, .agents/.gitignore, plugins.json, plugin.json, config.json, sidecars/tick/sidecar.json, plugin.json, hooks.json, mcp_config.json, .agents/plugins.json, plugins.json, .agents/.gitignore`

### [seed-48936155] Work order originally mentioned creating `EpisodeRecord` entries directly, but `harness: "import"` refers to the JSONL store `episode.Episode` header schema. I implemented the latter (saving to JSONL store via `episode.NewStore`), which behaves seamlessly with downstream components like FTS index population.
`open-issue · anchors: cmd/repo-memory/cmd_import_pa.go, PROJECT.md, WORKORDER.md, context_bridge.md, cmd/repo-memory/cmd_import_pa_test.go, cmd/repo-memory/main.go`
