---
name: pr-01-go-skeleton-wo4
description: Handoff instructions for Work Order 4 - Subcommands & Install
complexity: standard
---

# WORKORDER: WO4 - Subcommands & Install

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Wire all cobra subcommands completely. Implement the `install --harness antigravity` lifecycle, generating `hooks.json`, `mcp_config.json`, and `sidecar.json` with correct absolute binary paths. Implement `uninstall`. Ensure idempotency. Ensure the sidecar is NEVER auto-enabled.

## Strict Boundary Box
- DO NOT implement the execution logic of `hook`, `tick`, `mcp`, `search`, etc. beyond their basic entry point and flags setup. Just parse arguments and print placeholders.
- DO NOT implement Claude Code installation yet. ONLY target Antigravity plugin generation.

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.

## Line-Item Instructions
- [x] Wire the cobra commands in `cmd/repo-memory/` for all specified commands (`hook`, `tick` (alias `run`), `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor`). Define appropriate flags (e.g., `--harness antigravity` for install/uninstall).
- [x] Create Antigravity install templates in `packaging/antigravity/` (using `go:embed` or string templates):
  - `hooks.json`: map `PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop` to `<bin> hook antigravity <event>`.
  - `mcp_config.json`: stdio MCP server pointing to `<bin> mcp`.
  - `sidecar.json`: `builtin: schedule`, `* * * * *`, `restart_policy: "always"`, `display_name`, `description`, `agent_permissions` (with appropriate grants, e.g., `access_grants` and `workspace_uris` so the sidecar can start report conversations).
- [x] Implement `install` logic in `cmd/repo-memory/cmd_install.go`:
  - Determine absolute path to the current executable (`os.Executable()`).
  - Resolve the Antigravity plugin directory (`~/.gemini/config/plugins/repo-memory/` or platform equivalent, resolving the user's home dir).
  - Create necessary directories (`sidecars/tick/`, `rules/`, `skills/memory/`).
  - Generate the JSON files using the templates (replacing `<bin>` with the absolute path) and write them to the plugin directory. Make this idempotent (upgrade in place without failing).
  - **CRITICAL:** Do not auto-enable the sidecar. Print instructions to stdout telling the user to enable it in the Antigravity Automations dashboard.
- [x] Implement `uninstall` logic in `cmd/repo-memory/cmd_uninstall.go`:
  - Removes the generated plugin files and the `repo-memory` plugin directory (except data if `--purge` is not given; but here just remove the plugin config dir).
- [x] Write unit tests for the installation template generation and logic.

## Verification Requirement
- Verify `make build` compiles correctly.
- Verify `go test ./...` passes.
- **CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `cmd/repo-memory/main.go`: Wired all 10 subcommands to `rootCmd`.
- `cmd/repo-memory/cmd_install.go`: Implemented `install` subcommand with `--harness` flag (default `antigravity`), executable resolution, Antigravity plugin directory resolution, directory scaffolding (`sidecars/tick/`, `rules/`, `skills/memory/`), idempotent JSON file writes, and automations dashboard guidance.
- `cmd/repo-memory/cmd_uninstall.go`: Implemented `uninstall` subcommand with `--harness` and `--purge` flags, cleanly removing the plugin directory and optionally purging the data directory.
- `cmd/repo-memory/cmd_hook.go`, `cmd_tick.go`, `cmd_mcp.go`, `cmd_search.go`, `cmd_report.go`, `cmd_status.go`, `cmd_reindex.go`, `cmd_doctor.go`: Created modular entry points and argument parsing for all subcommands.
- `cmd/repo-memory/install_test.go`: Added comprehensive unit tests covering installation layout, idempotency, JSON validity, binary path replacement, uninstall with and without purge, directory resolution, and flag validation.
- `packaging/antigravity/hooks.json`: Defined Antigravity lifecycle hook mapping for `PreInvocation`, `PreToolUse`, `PostToolUse`, and `Stop`.
- `packaging/antigravity/mcp_config.json`: Defined stdio MCP server config pointing to `<bin> mcp`.
- `packaging/antigravity/sidecar.json`: Defined scheduled sidecar configuration (`builtin: schedule`, `* * * * *`, `restart_policy: always`, `agent_permissions`).
- `packaging/antigravity/plugin.json`: Defined manifest for repo-memory plugin.
- `packaging/antigravity/templates.go`: Implemented template rendering using `go:embed` and JSON-safe Windows path escaping.
- `packaging/antigravity/templates_test.go`: Added unit tests verifying valid JSON generation, correct event mapping, tool matchers, sidecar permissions, and path escaping.

### Decisions Made
- Windows path escaping in `RenderTemplate`: Windows absolute paths contain backslashes (`\`), which can produce invalid JSON if raw string replaced. Escaping backslashes before template substitution guarantees valid JSON across all platforms.
- Configurable plugin directory resolution via `REPO_MEMORY_PLUGIN_DIR` and `ANTIGRAVITY_CONFIG_DIR`: Allows hermetic unit testing without modifying the user's real `~/.gemini/config/plugins` directory.

### Dead Ends & Warnings
- None.

### Out-of-Scope Findings
- None.

### Blockers
(none)
