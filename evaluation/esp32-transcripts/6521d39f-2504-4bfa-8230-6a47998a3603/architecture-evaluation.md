# Evaluation: Split Global Daemon + Per-Workspace Plugin Architecture

## Verdict: ✅ Architecturally Sound — The Right Design

The proposed pattern correctly exploits three documented Antigravity mechanisms:
1. Global plugins with bundled sidecars (lifecycle tied to plugin)
2. `plugins.json` entries for per-workspace plugin loading
3. Global plugin discovery does not recurse into unknown subdirectories

This eliminates the entire class of problems we've been fighting: hacked hooks, hardcoded paths, per-repo sidecar proliferation, and the `agent_permissions` disabled-by-default trap.

---

## Point-by-Point Validation

### 1. `plugins.json` Loads Plugins from Arbitrary Paths ✅

From [json_configs.md](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/json_configs.md#L42-L44):
> `entries` (array of objects): A list of path entries to scan for customizations of this type. Each entry directory is scanned **one level deep**.

And from [plugins.md](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/plugins.md#L62-L68):
> Plugins can be discovered automatically if placed in standard customization roots, or they can be **explicitly registered using `plugins.json`**.

So `<repo>/.agents/plugins.json` with:
```json
{ "entries": [{ "path": "~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin" }] }
```
will scan `workspace-plugin/` one level deep, discover `bug-fixer-loop/` (has `plugin.json`), and load it **only for that workspace**. ✅

### 2. Global Plugin Loader Does Not Recurse ✅

From [plugins.md](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/plugins.md#L12-L13):
> A plugin must be contained within a **subdirectory** of a `plugins/` folder in a customization root.

The loader scans one level deep under `~/.gemini/config/plugins/`, finding `bug-fixer-daemon/`. Inside that plugin, it looks for known files: `plugin.json`, `hooks.json`, `sidecars/`, `skills/`, `rules/`, `mcp_config.json`. An unknown subdirectory like `workspace-plugin/` is **ignored**. ✅

### 3. No Sidecar `agent_permissions` = Plugin Defaults to Enabled ✅

Verified empirically:

| Plugin | Has `agent_permissions` in sidecar? | Default state |
|--------|:---:|:---:|
| `datacloud_telemetry` | No sidecars at all | **Enabled** |
| `repo-memory` | Sidecar, but NO `agent_permissions` | **Enabled** |
| `bug-fixer-loop-plugin` (current) | Sidecar WITH `agent_permissions` | **Disabled** |

The daemon plugin ships sidecars without `agent_permissions` → starts enabled → sidecars start automatically. ✅

### 4. Plugin Name Collision Prevention ✅

From [plugin/SKILL.md](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/plugin/SKILL.md#L229-L232):
> `name` — the label users see, and the global unique key — two plugins sharing a `name` collide and one is dropped.

Using `"bug-fixer-daemon"` for the global sidecar plugin and `"bug-fixer-loop"` for the workspace plugin avoids collision. ✅

### 5. Zero Overhead in Unenrolled Repos ✅

Without `.agents/plugins.json` referencing the workspace plugin, unenrolled repos:
- ❌ Do NOT load bug-fixer hooks (no subprocess spawning per tool call)
- ❌ Do NOT load bug-fixer skills (no context window overhead)
- ❌ Do NOT load bug-fixer rules
- ❌ Do NOT load bug-fixer MCP servers

Only the daemon's sidecars run globally — and those are cron-based OS subprocesses that check a registry and exit immediately if no enrolled repos have work. ✅

**This is a major improvement.** Currently `repo-memory` fires `PreInvocation`, `PreToolUse` (on `run_command|write_to_file|replace_file_content|call_mcp_tool`), `PostToolUse`, and `Stop` hooks in **every workspace** — including repos that don't use repo-memory. The proposed architecture eliminates that overhead.

### 6. Single-Point Upgrades ✅

Updating files in `~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin/bug-fixer-loop/` immediately updates ALL enrolled workspaces (they reference it via `plugins.json` path). No per-repo re-install. ✅

### 7. Clean Uninstall ✅

Everything under `~/.gemini/config/plugins/bug-fixer-daemon/`. Removing that directory (or `agy plugin uninstall bug-fixer-daemon`) removes the daemon, sidecars, AND the workspace plugin bundle. ✅

---

## Concrete Layout for Bug-Fixer-Loop

```
~/.gemini/config/plugins/bug-fixer-daemon/
├── plugin.json                          # { "name": "bug-fixer-daemon", "displayName": "Bug Fixer Background Service" }
│
├── sidecars/
│   ├── engineering-loop/
│   │   └── sidecar.json                 # schedule, ${PLUGIN_ROOT}/scripts/engineering_dispatcher.py
│   ├── triage-loop/
│   │   └── sidecar.json                 # schedule, ${PLUGIN_ROOT}/scripts/triage_dispatcher.py
│   ├── pr-review-loop/
│   │   └── sidecar.json                 # schedule, ${PLUGIN_ROOT}/scripts/architect_dispatcher.py
│   └── trajectory-loop/
│       └── sidecar.json                 # schedule, ${PLUGIN_ROOT}/scripts/trajectory_dispatcher.py
│
├── scripts/                             # dispatcher scripts (pure OS subprocesses)
│   ├── engineering_dispatcher.py
│   ├── triage_dispatcher.py
│   ├── architect_dispatcher.py
│   ├── trajectory_dispatcher.py
│   ├── liveness.py
│   ├── bugfixer_api.py
│   └── safe_ingest.py
│
└── workspace-plugin/                    # NOT loaded globally (global loader ignores this)
    └── bug-fixer-loop/
        ├── plugin.json                  # { "name": "bug-fixer-loop", "displayName": "Bug Fixer Loop" }
        ├── hooks.json                   # PreToolUse gates (relative paths → resolved against this dir)
        ├── rules/
        │   └── AGENTS.md
        ├── skills/
        │   ├── bug-fixer/SKILL.md
        │   ├── bug-fixer-engineering/SKILL.md
        │   ├── bug-fixer-triage/SKILL.md
        │   ├── bug-fixer-trajectory-review/SKILL.md
        │   ├── architect-review/SKILL.md
        │   └── test-quality-audit/SKILL.md
        ├── agents/
        │   └── leak-auditor.md
        └── scripts/                     # hook scripts (run by hooks.json)
            ├── pre_tool_gate.py
            └── github_url_gate.py
```

Per enrolled repo:
```
<repo>/.agents/
└── plugins.json                         # { "entries": [{ "path": "~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin" }] }
```

Registry (for dispatchers to iterate):
```
${PLUGIN_DATA}/
├── registry.json                        # [{ "repo": "owner/repo", "repo_dir": "/path", "project_id": "..." }, ...]
└── repos/
    ├── esp32-app-foundation/
    │   └── config.json                  # maintainer, test_command, labels, cron overrides, etc.
    └── bug-fixer-loop-plugin/
        └── config.json
```

---

## Practical Considerations for Bug-Fixer

### A. Multi-Repo Dispatchers

**Change required:** Each dispatcher goes from "handle one repo" to "iterate enrolled repos."

```python
# Current (single-repo):
config = load_config(os.environ["BUG_FIXER_CONFIG"])
check_and_dispatch(config)

# New (multi-repo):
registry = load_registry()  # from ${PLUGIN_DATA}/registry.json
for repo_entry in registry:
    config = load_repo_config(repo_entry)
    check_and_dispatch(config)
```

The core dispatch logic stays the same — just wrapped in a loop. **~30–50 lines changed per dispatcher.**

### B. Per-Repo Cron Schedules

**Problem:** Currently each repo can have different cron schedules (engineering every 10 min for repo A, every 30 min for repo B). With one sidecar per loop type, there's one cron expression.

**Solution:** The sidecar runs on the **fastest** configured schedule (e.g., `*/10 * * * *`). The dispatcher checks each repo's configured interval internally:

```python
for repo in registry:
    config = load_repo_config(repo)
    last_run = get_last_run_time(repo["repo"])
    interval = config.get("engineering_interval_minutes", 10)
    if minutes_since(last_run) < interval:
        continue  # not time yet for this repo
    check_and_dispatch(config)
    record_run_time(repo["repo"])
```

### C. `agentapi` Workspace Scoping (The Key Question)

**Problem:** Without `agent_permissions.workspace_uris` on the sidecar, how do spawned `agentapi new-conversation` calls get the right workspace?

**Solution:** The dispatcher sets `ANTIGRAVITY_PROJECT_ID` per-repo before calling `agentapi`:

```python
for repo in registry:
    os.environ["ANTIGRAVITY_PROJECT_ID"] = repo["project_id"]
    # agentapi inherits this env var
    subprocess.run(["agentapi", "new-conversation", "--model=...", prompt])
```

The `project_id` is captured during `configure.sh` (per-repo activation) when the Antigravity session is active, and stored in the registry.

> [!WARNING]
> **Needs testing:** We need to verify that `ANTIGRAVITY_PROJECT_ID` alone provides sufficient workspace scoping for the spawned conversations, or whether `workspace_uris` via `agent_permissions` is also required. If it is, we may need the sidecar to set a per-conversation override. This is the one open question.

### D. Hook Scripts Workspace Check

The workspace plugin's hooks only fire in enrolled workspaces (because `plugins.json` controls loading). But as defense-in-depth:

```python
# pre_tool_gate.py — fast check at top
input_data = json.loads(sys.stdin.read())
workspace = (input_data.get("workspacePaths") or [None])[0]
if not is_enrolled(workspace):
    print(json.dumps({"decision": "allow"}))
    sys.exit(0)
```

### E. Hook Script Path Resolution

The workspace plugin's `hooks.json` lives at:
```
~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin/bug-fixer-loop/hooks.json
```

Hook commands execute with **cwd = directory containing hooks.json**. So:
```json
{ "command": "python3 scripts/pre_tool_gate.py" }
```
resolves to `workspace-plugin/bug-fixer-loop/scripts/pre_tool_gate.py`. ✅

Note: The hook scripts (`pre_tool_gate.py`) and the dispatcher scripts (`engineering_dispatcher.py`) can live in **different** `scripts/` directories — one at the daemon level (for dispatchers), one at the workspace-plugin level (for hooks). Or they could share if we use absolute paths via `${PLUGIN_ROOT}`.

---

## Phased Implementation Plan

### Phase 1: Foundation (PR1)
Create the global daemon plugin structure and workspace plugin bundle. No dispatcher refactoring yet.

- Create `~/.gemini/config/plugins/bug-fixer-daemon/plugin.json`
- Create `workspace-plugin/bug-fixer-loop/` with hooks.json, skills, rules, agents, hook scripts
- Update `install.sh` → `install-plugin.sh` (one-time global install)
- Create `configure.sh` → per-repo activation (writes `.agents/plugins.json` + registry)
- Keep standalone sidecars temporarily (4 × N pattern persists for now)
- **Result:** Hooks work properly via plugin system. Sidecars still standalone.

### Phase 2: Sidecar Migration (PR2)
Move sidecars into the daemon plugin bundle and refactor dispatchers for multi-repo.

- Create `sidecars/` in daemon plugin with `${PLUGIN_ROOT}` paths
- Refactor dispatchers to iterate `registry.json`
- Add per-repo interval checking
- Remove standalone sidecar installation from `install.sh`
- Test `ANTIGRAVITY_PROJECT_ID` scoping for spawned conversations
- **Result:** 4 sidecars total. Lifecycle tied to daemon plugin.

### Phase 3: Cleanup (PR3)
Remove all legacy artifacts.

- Delete `hooks.json.template`
- Remove hook injection code from old `install.sh`
- Clean up injected hooks from consumer repos' `.agents/hooks.json`
- Remove git submodule from consumer repos
- Remove standalone sidecar stubs from `~/.gemini/config/sidecars/`
- Remove `bug-fixer-loop-workspace/` from consumer repos
- Update README and docs
- **Result:** Clean architecture, single directory tree.

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|:---:|-----------|
| `plugins.json` doesn't load workspace plugins correctly | Low | Documented mechanism; test before Phase 1 merge |
| Global loader recurses into `workspace-plugin/` | Very Low | Documented one-level scan; verify with GetAllPlugins RPC |
| `ANTIGRAVITY_PROJECT_ID` insufficient for conversation scoping | Medium | Test in Phase 2; fallback: pass workspace path via agentapi if supported |
| Per-repo cron interval logic has edge cases | Low | Unit-testable; existing dispatch logic unchanged |
| Hook scripts can't find config from workspace plugin cwd | Low | Use `${PLUGIN_ROOT}` env var or registry path |
