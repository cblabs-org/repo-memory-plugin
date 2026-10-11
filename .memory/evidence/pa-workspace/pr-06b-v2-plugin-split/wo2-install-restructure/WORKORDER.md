---
name: pr-06b-v2-plugin-split-wo2
description: Restructure install command for daemon + workspace-plugin layout
complexity: standard
---

# WO2: Install Restructure

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Restructure `cmd_install.go` and `packaging/antigravity/` to produce the split daemon + workspace-plugin directory layout. The daemon contains only the sidecar (no `agent_permissions`). The workspace bundle contains hooks, MCP, rules, and skills.

## Strict Boundary Box
- **Modify:** `cmd/repo-memory/cmd_install.go`, `cmd/repo-memory/cmd_uninstall.go`, `packaging/antigravity/`
- **DO NOT** modify `cmd_init.go`, `cmd_leave.go`, `cmd_enable.go`, `cmd_disable.go` — those are WO3
- **DO NOT** modify hook handlers, pipeline, evidence, telemetry, or episode code
- **DO NOT** modify the v2 spike prototype code

## Context

### Phase 0 results
Read `docs/experiments/phase0-plugin-split.md` first — it documents whether `~` expansion and nested plugin isolation work. Adapt the implementation based on those findings.

### Target directory layout
```
~/.gemini/config/plugins/repo-memory-daemon/
├── plugin.json                          # { "name": "repo-memory-daemon" }
├── config.json                          # { "enabled": true }
├── sidecars/
│   └── tick/
│       └── sidecar.json                 # Global 1-min tick (NO agent_permissions)
└── workspace-plugin/
    └── repo-memory/
        ├── plugin.json                  # { "name": "repo-memory" }
        ├── hooks.json                   # PreInvocation, PreToolUse, PostToolUse, Stop
        └── mcp_config.json              # repo-memory MCP tools
```

### Current architecture (what changes)
- `ResolveAntigravityPluginDir()` returns `~/.gemini/config/plugins/repo-memory/` → change to `repo-memory-daemon/`
- `RunInstall()` writes everything flat → split into daemon files and workspace-plugin files
- `packaging/antigravity/plugin.json` has `name: repo-memory` → need two: `repo-memory-daemon` and `repo-memory`
- `packaging/antigravity/sidecar.json` has `agent_permissions` → REMOVE (X1 finding: causes plugin to default disabled; tick is pure OS binary)
- `packaging/antigravity/templates.go` has single set of embeds → need daemon and workspace sets

## Line-Item Instructions

### Templates

- [ ] **Create `packaging/antigravity/daemon_plugin.json`:**
  ```json
  { "name": "repo-memory-daemon", "version": "0.1.0", "description": "Repository-level memory daemon (sidecar only)" }
  ```

- [ ] **Rename existing `plugin.json` to `workspace_plugin.json`** (keeps `name: repo-memory`)

- [ ] **Fix `sidecar.json`:** Remove the entire `agent_permissions` block. The tick binary is a pure OS subprocess — it never spawns agent conversations. `agent_permissions` caused the plugin to default to disabled (X1 finding, commit `ea91e0a`).

- [ ] **Update `templates.go`:**
  - Add `//go:embed daemon_plugin.json` → `DaemonPluginJSONTemplate`
  - Rename `PluginJSONTemplate` → `WorkspacePluginJSONTemplate` (embed `workspace_plugin.json`)
  - Add `GenerateDaemonPluginJSON()` and rename `GeneratePluginJSON()` → `GenerateWorkspacePluginJSON()`
  - Add `GenerateConfigJSON()` that returns `{ "enabled": true }`

- [ ] **Update `templates_test.go`** for the new function names.

### Install command

- [ ] **Update `ResolveAntigravityPluginDir()`:**
  - Return `~/.gemini/config/plugins/repo-memory-daemon` (not `repo-memory`)
  - Keep the env var overrides (`REPO_MEMORY_PLUGIN_DIR`, `ANTIGRAVITY_PLUGIN_DIR`, `ANTIGRAVITY_CONFIG_DIR`)

- [ ] **Add `ResolveWorkspacePluginDir(daemonDir string) string`:**
  - Returns `filepath.Join(daemonDir, "workspace-plugin", "repo-memory")`

- [ ] **Restructure `RunInstall(pluginDir, binPath string) error`:**
  - Create daemon directory hierarchy: `pluginDir/`, `pluginDir/sidecars/tick/`
  - Create workspace directory hierarchy: `pluginDir/workspace-plugin/repo-memory/`
  - Write daemon files: `plugin.json` (daemon), `config.json` (enabled), `sidecars/tick/sidecar.json`
  - Write workspace files: `plugin.json` (workspace), `hooks.json`, `mcp_config.json`
  - Keep the hooks.json safety validation (read-back + parse)

- [ ] **Update install command output messages:**
  - `"Successfully installed repo-memory daemon plugin to %s"` for the daemon
  - `"Workspace plugin bundle at %s"` for the workspace-plugin dir
  - Explain that repos must be enrolled with `repo-memory enroll` (not just `init`)

### Uninstall command

- [ ] **Update `cmd_uninstall.go`:** Point at the daemon directory (`repo-memory-daemon/`), which deletes both daemon and workspace-plugin in one `rm -rf`.

### Migration

- [ ] **Add migration logic to `RunInstall`:** If the old `repo-memory/` directory exists alongside the new `repo-memory-daemon/`:
  - Print a warning: `"Migrating from legacy plugin layout (repo-memory/ → repo-memory-daemon/)"`
  - Back up old dir: `repo-memory/ → repo-memory.bak-YYYYMMDD/`
  - Continue with fresh install to `repo-memory-daemon/`
  - Print: `"Legacy plugin backed up to %s. Remove after verifying the new layout."`

## Verification Requirement
- `go build ./...` passes
- `go vet ./...` passes
- `go test ./packaging/antigravity/...` passes
- `go test ./cmd/repo-memory/...` passes
- The installed directory structure matches the target layout

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(pending)
