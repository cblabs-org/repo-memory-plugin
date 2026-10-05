# PR-06b-v2-plugin-split: Daemon + Workspace Bundle Architecture

> **Programme:** repo-memory
> **PR:** pr-06b-v2-plugin-split
> **Base branch:** `main` (PR-05-v2-foundation merged 2026-10-04)
> **Status:** ✅ Complete (4/4 WOs)
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Human Approval

> **Approved:** yes (2026-10-05, user requested interstitial PR for architecture refactor)

## Objective

Split the single global `repo-memory` plugin into a **daemon** (sidecar-only, zero session overhead) and a **workspace bundle** (hooks, MCP, rules, skills loaded per-repo via `.agents/plugins.json`). This eliminates hook subprocess spawning, MCP schema loading, and rule injection in every non-enrolled workspace.

### Shared conventions with bug-fixer-loop-plugin

This refactor harmonises with the [pattern refinements](../../../../docs/antigravity-plugin-architecture.md) and the [shared team conventions](pattern-refinements-reference.md). The applicable shared patterns are:

| # | Refinement | Applies? |
|---|---|---|
| §2 | Workspace cache for hook performance | ✅ Yes — defence-in-depth, hooks only fire in enrolled repos now |
| §3 | `enroll`/`unenroll` naming | ✅ Yes — rename `init`→`enroll`, `leave`→`unenroll` |
| §7 | Phase 0 spike checks (#7, #8) | ✅ Yes — validated in WO1 |
| §1 | Sidecar folders as registry | ❌ No — repo-memory has single shared tick sidecar |
| §4 | Dispatcher scripts in workspace plugin | ❌ No — tick binary IS the logic |
| §5 | Per-repo sidecars with `agent_permissions` | ❌ No — tick is a pure OS binary |
| §6 | Internal cron via tick_receiver | ❌ No — single tick process |

## Design & SPEC Sections

- **Architecture doc:** [`docs/antigravity-plugin-architecture.md`](../../../../docs/antigravity-plugin-architecture.md)
- **v2 Design rev 3:** §2.3 (Mechanics — plugin loading), §12 (Validation)
- **SPEC:** §4.6 (Enrolment), §6.2 (Tick), §7.6 (Hook dispatch)

## Dependencies & Gate Preconditions

- **Base branch:** `main` (PR-05-v2-foundation merged)
- **Dependencies:** X1 and X9 findings from PR-06-v2-spike (plugin enablement bug, bimodal hook contracts)
- **Gate:** Must complete before PR-06 WO4 (X8 lean agent) resumes — agent discovery depends on the split architecture
- **Does NOT block:** PR-06 WO3 (X5 channels) — independent, can run in parallel

### Key Decisions (already made)

1. **Enrolment-only, not apply-all.** Memory is heavy on the harness. Workspace bundle loads only in enrolled repos.
2. **`.agents/plugins.json` is gitignored.** Machine-specific paths. Recreated by `repo-memory enroll`. A doctor check provides instructions for new clones.
3. **`agent_permissions` removed from tick sidecar.** Tick is a pure OS subprocess. `agent_permissions` caused the plugin to default to disabled (X1 finding).
4. **Command rename:** `init`→`enroll`, `leave`→`unenroll` (shared convention §3).

## Directory Layout (Target)

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
        ├── mcp_config.json              # repo-memory MCP tools
        ├── rules/
        │   └── AGENTS.md
        └── skills/
            └── memory/
                └── SKILL.md
```

Per-repo `.agents/plugins.json` (gitignored, **absolute paths required** — `~` not expanded):
```json
{
  "entries": [
    { "path": "/Users/<user>/.gemini/config/plugins/repo-memory-daemon/workspace-plugin" }
  ]
}
```

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [`wo1-validate-assumptions`](wo1-validate-assumptions/WORKORDER.md) | Validate the two platform assumptions the split depends on: (a) workspace-plugin/ nested inside a global plugin is NOT loaded globally; (b) `.agents/plugins.json` `path` entries support `~` expansion. Also verify extra files in sidecar dirs are ignored by the loader. Document results in `docs/experiments/phase0-plugin-split.md`. | standard | ☑ Complete |
| 2 | [`wo2-install-restructure`](wo2-install-restructure/WORKORDER.md) | Restructure `cmd_install.go` and `packaging/antigravity/` to create the daemon + workspace-plugin directory layout. Daemon gets: `plugin.json` (`name: repo-memory-daemon`), `config.json` (`enabled: true`), `sidecars/tick/sidecar.json` (NO `agent_permissions`). Workspace-plugin gets: `plugin.json` (`name: repo-memory`), `hooks.json`, `mcp_config.json`. Update `ResolveAntigravityPluginDir` to return the daemon path. Add `ResolveWorkspacePluginDir`. Update `ensurePluginEnabled` for `repo-memory-daemon`. Add migration from old `repo-memory/` layout. | standard | ☑ Complete |
| 3 | [`wo3-enrollment-flow`](wo3-enrollment-flow/WORKORDER.md) | Rename `init`→`enroll`, `leave`→`unenroll` (keep old names as hidden aliases). `enroll` writes `.agents/plugins.json` pointing to the workspace bundle, and adds `plugins.json` to `.agents/.gitignore` (or creates it). `unenroll` removes the entry. `enable`/`disable` toggle the `.agents/plugins.json` entry (already exist as commands, need updating). Update `cmd_doctor.go` to check workspace plugin reference is valid and provide install instructions for new clones. | standard | ☑ Complete |
| 4 | [`wo4-tests-and-migration`](wo4-tests-and-migration/WORKORDER.md) | Update all test fixtures and references to the old plugin path. Add integration test: `install` → `enroll` → verify hooks fire → `unenroll` → verify hooks don't fire. Run full test suite. Migrate the live installed plugin on the user's machine: script or command that moves `repo-memory/` to `repo-memory-daemon/` layout. Verify with `go build`, `go vet`, `go test ./...`. | standard | ☑ Complete |

## Exit Criteria

- The daemon plugin (`repo-memory-daemon/`) contains only `plugin.json`, `config.json` and `sidecars/tick/sidecar.json` (no `agent_permissions`)
- The workspace plugin (`workspace-plugin/repo-memory/`) contains `plugin.json`, `hooks.json`, `mcp_config.json`
- Workspace plugin is NOT loaded in non-enrolled repos (verified by Phase 0 check)
- `repo-memory enroll` writes `.agents/plugins.json`; `repo-memory unenroll` removes it
- `.agents/plugins.json` is gitignored
- `repo-memory doctor` detects missing plugin and provides install instructions
- `init`/`leave` still work as hidden aliases
- `go build ./...`, `go vet ./...`, `go test ./...` all pass
- Migration from old layout documented and tested
- Live installed plugin migrated successfully on the dev machine

## Human Tasks

- Verify the Phase 0 check results (WO1)
- Test `enroll`/`unenroll` on the live dev machine after WO3
- Approve the migration of the live plugin (WO4)

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| **`~` not expanded by Antigravity in `.agents/plugins.json`** | WO1 validates this first; fallback is absolute path with per-machine generation | WO1 |
| **Workspace plugin loaded globally despite nesting** | WO1 validates; the loader documented as non-recursive | WO1 |
| **Live plugin migration breaks capture** | Migration is reversible; old directory backed up before removal | WO4 |
| **Test fixtures miss old paths** | `grep -r` for old path in WO4 | WO4 |

## Session Log

- 2026-10-05 14:15 — PR-06b-v2-plugin-split created as interstitial PR for architecture refactor. Based on X1/X9 findings and the split architecture design doc.
- 2026-10-05 14:48 — WO1 complete. All 3 Phase 0 checks pass. Key finding: `~` NOT expanded in `.agents/plugins.json` — must use absolute paths.
- 2026-10-05 15:00 — WO2 complete. Install restructured: daemon (sidecar only) + workspace-plugin (hooks/MCP). `agent_permissions` removed from sidecar. Legacy migration with backup.
- 2026-10-05 15:16 — WO3 complete. `init`→`enroll`, `leave`→`unenroll` (aliases preserved). `.agents/plugins.json` managed by enroll/unenroll/enable/disable. Doctor checks added.
- 2026-10-05 17:58 — WO4 complete. Integration tests added (enroll/unenroll flow, third-party preservation, migration). Live plugin migrated. Finding: IDE keeps plugin paths in memory — old directory should remain until IDE restart.
