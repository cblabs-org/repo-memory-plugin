# packaging/antigravity

## Decisions

### [seed-664ccba0] Windows path escaping in `RenderTemplate`: Windows absolute paths contain backslashes (`\`), which can produce invalid JSON if raw string replaced. Escaping backslashes before template substitution guarantees valid JSON across all platforms.
`decision · kind: episodic · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`

### [seed-73d6dc53] Configurable plugin directory resolution via `REPO_MEMORY_PLUGIN_DIR` and `ANTIGRAVITY_CONFIG_DIR`: Allows hermetic unit testing without modifying the user's real `~/.gemini/config/plugins` directory.
`decision · kind: episodic · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`

### [seed-f2c2c14b] Kept cron expression strictly aligned with `*/1 * * * *` per workorder instructions and updated corresponding test expectations.
`decision · kind: episodic · anchors: packaging/antigravity/sidecar.json, packaging/antigravity/templates_test.go, cmd/repo-memory/install_test.go`
