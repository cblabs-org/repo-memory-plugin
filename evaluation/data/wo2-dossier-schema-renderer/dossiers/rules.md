# rules

## Ruled out, failure signatures and gotchas

### [seed-c39c50a7] None.
`ruled-out · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, sidecars/tick/, rules/, skills/memory/, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd_tick.go, cmd_mcp.go, cmd_search.go, cmd_report.go, cmd_status.go, cmd_reindex.go, cmd_doctor.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/plugin.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`

## Decisions

### [seed-664ccba0] Windows path escaping in `RenderTemplate`: Windows absolute paths contain backslashes (`\`), which can produce invalid JSON if raw string replaced. Escaping backslashes before template substitution guarantees valid JSON across all platforms.
`decision · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, sidecars/tick/, rules/, skills/memory/, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd_tick.go, cmd_mcp.go, cmd_search.go, cmd_report.go, cmd_status.go, cmd_reindex.go, cmd_doctor.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/plugin.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`

### [seed-73d6dc53] Configurable plugin directory resolution via `REPO_MEMORY_PLUGIN_DIR` and `ANTIGRAVITY_CONFIG_DIR`: Allows hermetic unit testing without modifying the user's real `~/.gemini/config/plugins` directory.
`decision · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, sidecars/tick/, rules/, skills/memory/, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd_tick.go, cmd_mcp.go, cmd_search.go, cmd_report.go, cmd_status.go, cmd_reindex.go, cmd_doctor.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/plugin.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`

## Open issues

### [seed-49756102] None.
`open-issue · anchors: cmd/repo-memory/main.go, cmd/repo-memory/cmd_install.go, sidecars/tick/, rules/, skills/memory/, cmd/repo-memory/cmd_uninstall.go, cmd/repo-memory/cmd_hook.go, cmd_tick.go, cmd_mcp.go, cmd_search.go, cmd_report.go, cmd_status.go, cmd_reindex.go, cmd_doctor.go, cmd/repo-memory/install_test.go, packaging/antigravity/hooks.json, packaging/antigravity/mcp_config.json, packaging/antigravity/sidecar.json, packaging/antigravity/plugin.json, packaging/antigravity/templates.go, packaging/antigravity/templates_test.go`
