# Implementation Plan v2: Plugin Architecture Migration

> **Revision:** Addresses all 7 blocking issues, 5 spike additions, factual errors, and
> smaller points from the review of v1.

## Goal

Migrate from the hacked architecture (injected hooks, 4×N standalone sidecars,
relative-path config resolution) to the split daemon + workspace plugin pattern
(1 global daemon, per-repo sidecars, per-workspace activation via `plugins.json`).

**Base branch:** `main` (at `50f406a`)
**Project name:** `plugin-architecture`
**Branch format:** `plugin-architecture/wo[N]-[topic]`

---

## Resolved Questions

- **Q1: Command naming** — ✅ `configure.sh` → `enroll.sh`, add `unenroll.sh`.
- **Q2: Config merge** — ✅ Single merged config per repo at `sidecars/<repo>/config.json`.
- **Q3: Naming** — ✅ `bug-fixer-loop-workspace` → `bug-fixer-workspace`.

---

## Blocking Issues Addressed

### Issue 1: Move, Don't Copy — ✅ Fixed

WO1 uses `git mv` to relocate scripts, skills, rules, agents into `daemon/workspace-plugin/bug-fixer/`.
Test imports update in the same WO. No duplicate source of truth.

### Issue 2: Config Loader Regression — ✅ Fixed

WO2 now:
- Targets the correct `load_config()` at line 225 (not `_load_config`)
- **Preserves** `normalise_config()`, `_loaded_from` stamping, fail-closed on bad paths
- **Extends** the candidate search list (adds sidecars scan) rather than replacing
- Uses `Path.is_relative_to()` not `startswith()`
- Derives daemon path from `__file__`, not hardcoded `~/.gemini/config/...`
- Documents multi-root strategy (first matching workspacePaths entry)
- Keeps correct `docs/plugin-architecture.md` default (not `design_specification.md`)

### Issue 3: Gate A Trust Anchors — ✅ Fixed

WO2 extends `_derive_workspace_paths()` and `HOST_SECURITY_SCRIPTS` to protect:
- `.agents/plugins.json`
- `sidecars/<repo>/config.json` and `sidecars/<repo>/sidecar.json`
- All scripts listed in `HOST_SECURITY_SCRIPTS` (adds `integrity_monitor.py`)
- No workspace cache (dropped per review)

### Issue 4: All Legacy Path Resolutions — ✅ Fixed

WO2 covers **all 10 scripts** that have path resolution, not just `pre_tool_gate.py`:

| Script | Bug | Fix |
|--------|-----|-----|
| `pre_tool_gate.py` | 7 refs to `bug-fixer-loop-workspace` | Update `load_config`, `_derive_workspace_paths`, `_repo_root`, `main()` |
| `safe_ingest.py` | Own `load_config()` + `normalise_config()` duplicates | Import from `pre_tool_gate` or shared module |
| `safe_post.py` | **Nested-vs-sibling bug** L37: `PLUGIN_DIR / ".agents/plugins/..."` | Fix to derive from `BUG_FIXER_CONFIG` or `__file__` correctly |
| `integrity_monitor.py` | **Nested-vs-sibling bug** L36, L71 | Fix to derive from `BUG_FIXER_CONFIG` or `__file__` correctly |
| `injection_scanner.py` | **Nested-vs-sibling bug** L214-216 | Fix to derive from `BUG_FIXER_CONFIG` or `__file__` correctly |
| `bugfixer_api.py` | `_find_workspace_dir()` correct sibling but hardcodes `bug-fixer-loop-workspace` | Update workspace name constant |
| `github_url_gate.py` | Delegates to `pre_tool_gate.load_config` — no own resolution | No change needed |
| `shell_analysis.py` | No path resolution at all | No change needed |
| `liveness.py` | Only `__file__`-based `safe_post.py` reference | No change needed |
| 4 dispatchers | Config via `BUG_FIXER_CONFIG` or `PLUGIN_DIR / "../bug-fixer-loop-workspace/"` | Update workspace name, SCRIPT_DIR/SKILL_PATH |

### Issue 5: Runtime State Migration — ✅ Fixed

State location: `sidecars/<repo>/state/` (inside the sidecar dir, which is the per-repo data root).
WO4 (`enroll.sh`) migrates existing state from `.agents/plugins/bug-fixer-loop-workspace/` if present.
5 skills updated to reference new `STATE_DIR`.

**State files to migrate:**
- `claims/`, `dispatches/`, `tmp/` (directories)
- `cooldown.json`, `triage_cooldown.json`
- `failure_count`, `triage_failure_count`
- `pr_completions.jsonl`
- `injection_cache.json`
- `.integrity_snapshot.json`
- `issue_dispatch_failures.json`

### Issue 6: Ordered Cutover — ✅ Fixed

New WO5 is a dedicated cutover step:
1. Install and enroll (new sidecars created but not yet approved)
2. Confirm new hooks fire via `plugins.json`
3. Remove old standalone sidecars from `~/.gemini/config/sidecars/*--<repo>/`
4. Remove injected hooks from `.agents/hooks.json`
5. Approve new sidecars in dashboard

### Issue 7: Reinstall Safety — ✅ Fixed

`install.sh` uses targeted file copy (not `cp -r daemon/`) that skips `sidecars/`:
```bash
# Install only daemon-owned files, never touch per-repo data
rsync -a --exclude='sidecars/' daemon/ "$INSTALL_DIR/"
```
`uninstall.sh` warns about per-repo data loss and requires `--force` to remove `sidecars/`.

---

## WO0 Spike Additions

| # | Check | Why |
|---|-------|-----|
| 1–8 | Original 8 checks from v1 | Unchanged |
| **9** | **Do workspace-plugin hooks fire inside sidecar-started conversations?** | If not, dispatcher conversations run with no gates |
| **10** | **Is hook cwd the directory containing hooks.json?** | Relative `scripts/...` commands depend on this |
| **11** | **Is `${PLUGIN_ROOT}` substituted in skill/rule text, or only in sidecar args?** | Skills use `${PLUGIN_DIR}` |
| **12** | **Does `plugins.json` expand `~`? Does its path point at `workspace-plugin/` or `workspace-plugin/bug-fixer/`?** | Determines the correct entry format |
| **13** | **Does `agy plugin enable` exist as a CLI command?** | WO4 assumes it does |

---

## Proposed Changes

### Dependency Graph

```mermaid
flowchart LR
    WO0["WO0: Spike<br/>(interactive)"]
    WO1["WO1: git-mv<br/>+ Daemon Scaffold"]
    WO2["WO2: Config Loader<br/>+ Gate A Anchors"]
    WO3["WO3: tick.py +<br/>Dispatchers"]
    WO4["WO4: install.sh +<br/>enroll.sh"]
    WO5["WO5: Ordered<br/>Cutover"]
    WO6["WO6: Legacy Cleanup<br/>+ Docs + Tests"]

    WO0 --> WO1
    WO1 --> WO2
    WO2 --> WO3
    WO3 --> WO4
    WO4 --> WO5
    WO5 --> WO6

    style WO0 fill:#f90,color:#000
    style WO1 fill:#69f,color:#fff
    style WO2 fill:#c33,color:#fff
    style WO3 fill:#69f,color:#fff
    style WO4 fill:#69f,color:#fff
    style WO5 fill:#4a4,color:#fff
    style WO6 fill:#4a4,color:#fff
```

WO2 is red — highest risk WO (Gate A + config loader changes).
All WOs strictly serialised.

---

### WO0 — Phase 0 Spike (`interactive`)

**Complexity:** `interactive` — bridge prompt to user, not spawned.

13 checks with a throwaway `bug-fixer-spike` plugin. No code changes.
Results determine: daemon-plugin layout vs standalone-sidecars fallback.

---

### WO1 — git-mv + Daemon Scaffold (`standard`)

**Base:** `main`
**Branch:** `plugin-architecture/wo1-daemon-scaffold`

**Objective:** Move (not copy) all source files into the daemon directory layout.
Create the workspace plugin structure. Update test imports.

#### Step 1: Create daemon directory structure
```
daemon/
├── plugin.json               # name: "bug-fixer-daemon", no "disabled"
├── scripts/
│   └── tick.py               # scaffold (full impl in WO3)
└── workspace-plugin/
    └── bug-fixer/
        ├── plugin.json       # name: "bug-fixer"
        └── hooks.json        # 4 hook groups, relative paths
```

#### Step 2: git mv source files
```bash
git mv scripts/    daemon/workspace-plugin/bug-fixer/scripts/
git mv skills/     daemon/workspace-plugin/bug-fixer/skills/
git mv rules/      daemon/workspace-plugin/bug-fixer/rules/
git mv agents/     daemon/workspace-plugin/bug-fixer/agents/
```

#### Step 3: Update test imports
```python
# tests/*.py — update all imports from:
from scripts.pre_tool_gate import ...
# to:
from daemon.workspace_plugin.bug_fixer.scripts.pre_tool_gate import ...
```

> [!IMPORTANT]
> This changes every test file's import paths. The WO must also update
> `pyproject.toml` if it declares package paths, and verify `python3 -m pytest`
> still discovers and runs all tests.

#### Step 4: Create hooks.json (4 hook groups)
All 4 hooks from current `.agents/hooks.json` on main, with relative paths:
- `bug-fixer-file-gate` (PreToolUse: write_to_file|replace_file_content → `scripts/pre_tool_gate.py`)
- `bug-fixer-command-gate` (PreToolUse: run_command → `scripts/pre_tool_gate.py`, PostToolUse: run_command → `scripts/integrity_monitor.py --post`)
- `bug-fixer-integrity-snapshot` (PreToolUse: run_command → `scripts/integrity_monitor.py --pre`)
- `bug-fixer-url-gate` (PreToolUse: read_url_content → `scripts/github_url_gate.py`)

**Files moved:** ~30 (entire scripts/, skills/, rules/, agents/ trees)
**Files created:** 4 (daemon/plugin.json, daemon/workspace-plugin/bug-fixer/plugin.json, hooks.json, tick.py scaffold)
**Files modified:** ~12 test files (import paths)

---

### WO2 — Config Loader + Gate A Trust Anchors (`architect`)

**Base:** `plugin-architecture/wo1-daemon-scaffold`
**Branch:** `plugin-architecture/wo2-hook-config-resolution`

**Objective:** Update config resolution in ALL 10 scripts that have path resolution.
Extend Gate A to protect new trust anchors. Preserve all existing contracts.

> [!CAUTION]
> This is the highest-risk WO. Changes to `load_config()`, `_derive_workspace_paths()`,
> and `SCANNER_PROTECTED_FILES` affect every hook invocation. Test thoroughly.

#### [MODIFY] `daemon/workspace-plugin/bug-fixer/scripts/pre_tool_gate.py`

**7 changes, all preserving existing contracts:**

**(a) `load_config()` (line 225)** — extend candidate list, don't replace:
```python
def load_config(config_path: str | Path | None = None) -> dict[str, Any]:
    """Loads plugin config. Search order:
    1. Explicit config_path arg (fail-closed: FileNotFoundError)
    2. BUG_FIXER_CONFIG env var (fail-closed: FileNotFoundError)
    3. __file__-based sibling: daemon-dir/sidecars/*/config.json (match workspacePaths)
    4. Legacy sibling: plugin_dir.parent / "bug-fixer-workspace" / "config.json"
    5. cwd-based: .agents/plugins/bug-fixer-workspace/config.json
    6. Defaults (fail-open)
    """
    target_path: Path | None = None
    if config_path:
        target_path = Path(config_path).expanduser().resolve()
        if not target_path.is_file():
            raise FileNotFoundError(f"Configuration file not found at: {target_path}")
    elif os.environ.get("BUG_FIXER_CONFIG"):
        env_path = Path(os.environ["BUG_FIXER_CONFIG"]).expanduser().resolve()
        if not env_path.is_file():
            raise FileNotFoundError(f"BUG_FIXER_CONFIG not found: {env_path}")
        target_path = env_path
    else:
        script_dir = Path(__file__).resolve().parent
        daemon_dir = _find_daemon_dir(script_dir)
        workspace_paths = _get_workspace_paths()  # from stdin or env

        # NEW: scan sidecars/ for a config matching this workspace
        if daemon_dir and workspace_paths:
            sidecars_dir = daemon_dir / "sidecars"
            if sidecars_dir.is_dir():
                for repo_dir in sidecars_dir.iterdir():
                    cfg = repo_dir / "config.json"
                    if cfg.is_file():
                        try:
                            data = json.loads(cfg.read_text())
                            cfg_repo_dir = Path(data.get("repo_dir", "")).resolve()
                            for wp in workspace_paths:
                                if Path(wp).resolve().is_relative_to(cfg_repo_dir):
                                    target_path = cfg
                                    break
                        except (json.JSONDecodeError, OSError):
                            continue
                    if target_path:
                        break

        # LEGACY fallbacks (kept for dev/symlink installs)
        if not target_path:
            plugin_dir = script_dir.parent
            for cand in [
                plugin_dir.parent / "bug-fixer-workspace" / "config.json",
                Path.cwd() / ".agents" / "plugins" / "bug-fixer-workspace" / "config.json",
            ]:
                if cand.is_file():
                    target_path = cand
                    break

    if target_path is None or not target_path.is_file():
        return normalise_config({
            "spec_file_path": "docs/plugin-architecture.md",
            "protected_files": ["docs/plugin-architecture.md"],
            "leak_patterns": [],
            "scan_exclude_paths": ["tests/"],
        })

    try:
        with open(target_path, "r", encoding="utf-8") as f:
            data = json.load(f)
            if not isinstance(data, dict):
                raise ValueError(f"Config at {target_path} must be a JSON object")
            data["_loaded_from"] = str(target_path)       # PRESERVED
            return normalise_config(data)                  # PRESERVED
    except json.JSONDecodeError as exc:
        raise ValueError(f"Invalid JSON: {target_path}: {exc}") from exc
```

**(b) `_find_daemon_dir()`** — new helper, derived from `__file__`:
```python
def _find_daemon_dir(script_dir: Path) -> Path | None:
    """Walk up from script_dir to find the daemon plugin root (contains plugin.json)."""
    for ancestor in [script_dir.parent, script_dir.parent.parent,
                     script_dir.parent.parent.parent]:
        if (ancestor / "plugin.json").is_file():
            return ancestor
    return None
```

**(c) `_derive_workspace_paths()` (line 163)** — extend, don't hardcode:
```python
def _derive_workspace_paths(loaded_config_path: Path | None = None) -> frozenset[str]:
    protected = set(_STATIC_PROTECTED)

    if loaded_config_path:
        ws_dir = loaded_config_path.resolve().parent
    else:
        ws_dir = Path.cwd() / ".agents" / "plugins" / "bug-fixer-workspace"

    # Protect the config and injection cache wherever they are
    protected.add(str(ws_dir / "config.json"))
    protected.add(str(ws_dir / "injection_cache.json"))

    # Relative paths for shell command matching
    rel_ws = None
    try:
        rel_ws = ws_dir.relative_to(Path.cwd())
    except ValueError:
        pass
    if rel_ws:
        protected.add(str(rel_ws / "config.json"))
        protected.add(str(rel_ws / "injection_cache.json"))
    # Legacy relative path (for commands using the old name)
    protected.add(".agents/plugins/bug-fixer-workspace/config.json")
    protected.add(".agents/plugins/bug-fixer-workspace/injection_cache.json")

    # NEW: protect sidecar config and plugins.json
    protected.add(".agents/plugins.json")
    if loaded_config_path:
        sidecar_dir = loaded_config_path.resolve().parent
        protected.add(str(sidecar_dir / "sidecar.json"))

    return frozenset(protected)
```

**(d) `HOST_SECURITY_SCRIPTS` (line 142)** — add `integrity_monitor.py`:
```python
HOST_SECURITY_SCRIPTS = frozenset({
    "pre_tool_gate.py",
    "safe_ingest.py",
    "safe_post.py",
    "injection_scanner.py",
    "shell_analysis.py",
    "github_url_gate.py",
    "integrity_monitor.py",       # NEW
})
```

**(e) `_repo_root()` (line 2096)** — update workspace name check:
```python
if ws_dir.name in ("bug-fixer-workspace", "bug-fixer-loop-workspace", "state") \
        and ws_dir.parent.parent.name == ".agents":
    return str(ws_dir.parent.parent.parent)
# NEW: sidecar layout — config in sidecars/<repo>/config.json
if ws_dir.parent.name == "sidecars" and (ws_dir.parent.parent / "plugin.json").exists():
    repo_dir = config.get("repo_dir")
    if repo_dir:
        return repo_dir
```

**(f) `main()` (line 2830)** — update `bug-fixer-loop-workspace` string check.

**(g) Multi-root strategy:** `_get_workspace_paths()` reads `workspacePaths` from stdin JSON.
If multiple paths match sidecars, first match wins. Document in code comment.

#### [MODIFY] All other scripts with path resolution

For `safe_ingest.py`, `safe_post.py`, `integrity_monitor.py`, `injection_scanner.py`:
- Fix sibling-vs-nested bugs (3 scripts have wrong `PLUGIN_DIR / ".agents/..."`)
- Update workspace name from `bug-fixer-loop-workspace` → `bug-fixer-workspace`
- Add `BUG_FIXER_CONFIG`-first resolution where missing
- For `safe_ingest.py`: import `load_config` and `normalise_config` from `pre_tool_gate` instead of duplicating

For `bugfixer_api.py`:
- Update `_WORKSPACE_NAME = "bug-fixer-workspace"`

For 4 dispatchers:
- Update `_config_relative` path from `"../bug-fixer-loop-workspace/"` → `"../bug-fixer-workspace/"`
- Update `SKILL_PATH` and `SAFE_INGEST` to use new directory layout

#### [MODIFY] 5 skills
Update all `STATE_DIR` and `PLUGIN_DIR` references:
- `bug-fixer-engineering/SKILL.md`
- `bug-fixer-triage/SKILL.md`
- `architect-review/SKILL.md`
- `bug-fixer-trajectory-review/SKILL.md`
- `bug-fixer/SKILL.md`

#### [MODIFY] `rules/github-reads.md`
Update `python3 scripts/safe_ingest.py` reference to use correct relative path from hook cwd.

#### [NEW] Tests for `load_config()` search path logic
- Test explicit path → loads correctly, stamps `_loaded_from`
- Test `BUG_FIXER_CONFIG` → loads correctly
- Test bad explicit path → `FileNotFoundError` (fail-closed)
- Test bad `BUG_FIXER_CONFIG` → `FileNotFoundError` (fail-closed)
- Test sidecars scan match → correct config loaded
- Test no match → defaults with correct spec_file_path
- Test `normalise_config()` preserves v1 and v2 (existing tests, extend)

#### [NEW] Tests for Gate A trust anchors
- Test `.agents/plugins.json` write denied
- Test `sidecars/<repo>/config.json` write denied
- Test `sidecars/<repo>/sidecar.json` write denied
- Test `integrity_monitor.py` write denied (now in HOST_SECURITY_SCRIPTS)

**Files modified:** ~16 (10 scripts + 5 skills + 1 rule)
**Files created:** test additions (~100 lines)

---

### WO3 — tick.py + tick_receiver.py + Dispatcher Adaptation (`standard`)

**Base:** `plugin-architecture/wo2-hook-config-resolution`
**Branch:** `plugin-architecture/wo3-tick-dispatchers`

**Objective:** Implement the tick/dispatch pipeline. `tick.py` is a minimal router.
`tick_receiver.py` manages internal intervals with persistent last-run times and per-loop locks.
All dispatchers read config from `BUG_FIXER_CONFIG` env var (set in sidecar.json at enroll time).

#### [MODIFY] `daemon/scripts/tick.py`
```python
#!/usr/bin/env python3
"""Heartbeat router: find the workspace plugin and invoke tick_receiver."""
import os, subprocess, sys
from pathlib import Path

DAEMON_DIR = Path(__file__).resolve().parent.parent
TICK_RECEIVER = DAEMON_DIR / "workspace-plugin" / "bug-fixer" / "scripts" / "tick_receiver.py"

config_path = os.environ.get("BUG_FIXER_CONFIG", "")
if not config_path or not Path(config_path).is_file():
    sys.exit(0)

result = subprocess.run(
    [sys.executable, str(TICK_RECEIVER)],
    env=os.environ,  # BUG_FIXER_CONFIG already in env from sidecar
)
sys.exit(result.returncode)
```

#### [NEW] `daemon/workspace-plugin/bug-fixer/scripts/tick_receiver.py`
**Not ~45 lines.** Full implementation with:
- Persistent last-run times in `state/<loop>.last` (mtime-based)
- Missed-tick policy: run immediately if overdue, skip if still running
- Per-loop file lock (`state/<loop>.lock`) to prevent overlapping runs
- Configurable intervals from config.json (`triage_cron`, `engineering_cron`, etc.)
- Subprocess exit code logging (not discarded)
- No cron parser needed — use interval-in-minutes from config, compare against mtime

```python
LOOPS = {
    "triage":      {"script": "triage_dispatcher.py",     "config_key": "triage_interval_min",      "default": 60},
    "engineering": {"script": "engineering_dispatcher.py", "config_key": "engineering_interval_min",  "default": 10},
    "architect":   {"script": "architect_dispatcher.py",   "config_key": "architect_interval_min",    "default": 15},
    "trajectory":  {"script": "trajectory_dispatcher.py",  "config_key": "trajectory_interval_min",   "default": 1440},
}
```

#### [MODIFY] Dispatchers: config resolution simplification
All 4 dispatchers already read `BUG_FIXER_CONFIG` first. The sidecar template sets it.
Only change: update fallback path from `../bug-fixer-loop-workspace/` → `../bug-fixer-workspace/`
and update `STATE_DIR` derivation to use `config.get("state_dir")` or derive from config path.

#### [MODIFY] Dispatchers: STATE_DIR
```python
# All dispatchers — replace inline STATE_DIR derivation:
STATE_DIR = Path(os.environ.get("BUG_FIXER_STATE_DIR", "")) or CONFIG_FILE.parent / "state"
```

**Files created:** 1 (`tick_receiver.py`, ~120 lines)
**Files modified:** 5 (tick.py, 4 dispatchers)

---

### WO4 — install.sh + enroll.sh Rewrite (`architect`)

**Base:** `plugin-architecture/wo3-tick-dispatchers`
**Branch:** `plugin-architecture/wo4-install-enroll`

**Objective:** Rewrite install/enroll lifecycle commands.

#### [MODIFY] `scripts/install.sh`
- Targeted rsync (excludes `sidecars/`) to `~/.gemini/config/plugins/bug-fixer-daemon/`
- Enable plugin via config.json or `agy plugin enable` (if exists per WO0 check 13)
- Idempotent — safe to re-run for upgrades

```bash
INSTALL_DIR="$HOME/.gemini/config/plugins/bug-fixer-daemon"
mkdir -p "$INSTALL_DIR"
# Install daemon-owned files only, never touch per-repo sidecars or state
rsync -a --exclude='sidecars/' \
         --delete-excluded \
         daemon/ "$INSTALL_DIR/"
```

#### [NEW] `scripts/enroll.sh` (replaces `configure.sh`)
Carries over the interactive wizard but writes to new locations:
- Sidecar: `$INSTALL_DIR/sidecars/<repo-slug>/sidecar.json`
- Config: `$INSTALL_DIR/sidecars/<repo-slug>/config.json` (merged v2 schema, includes `repo_dir`)
- State: `$INSTALL_DIR/sidecars/<repo-slug>/state/` (empty dir, populated at runtime)
- Consumer repo: `<repo>/.agents/plugins.json` with entry pointing to workspace plugin
- Migrates existing state from `.agents/plugins/bug-fixer-loop-workspace/` if found
- Sets `BUG_FIXER_CONFIG` in sidecar.json env to the absolute config path

```json
{
  "builtin": "schedule",
  "args": ["* * * * *", "<install_dir>/scripts/tick.py"],
  "restart_policy": "always",
  "display_name": "Bug Fixer (<repo>)",
  "agent_permissions": {
    "workspace_uris": ["file://<repo_dir>"]
  },
  "env": {
    "BUG_FIXER_CONFIG": "<install_dir>/sidecars/<repo-slug>/config.json",
    "BUG_FIXER_STATE_DIR": "<install_dir>/sidecars/<repo-slug>/state",
    "BUG_FIXER_REPO": "<owner/repo>",
    "ANTIGRAVITY_PROJECT_ID": "<project_id>",
    "PATH": "<path>"
  }
}
```

> [!IMPORTANT]
> `repo_dir` is a **required, validated key** in the merged v2 config.
> `enroll.sh` must validate it exists and is a directory.

#### [NEW] `scripts/unenroll.sh`
- Removes `$INSTALL_DIR/sidecars/<repo-slug>/`
- Removes `.agents/plugins.json` entry from consumer repo
- Warns about in-flight claims/dispatches

#### [DELETE] `scripts/configure.sh`

**Files renamed:** 1 (`configure.sh` → `enroll.sh`)
**Files modified:** 1 (`install.sh`)
**Files created:** 1 (`unenroll.sh`)

---

### WO5 — Ordered Cutover (`interactive`)

**Complexity:** `interactive` — the user drives the cutover.

**Objective:** Migrate both enrolled repos (esp32-app-foundation and bug-fixer-loop-plugin)
from the old architecture to the new one, with zero duplicate-fire window.

> [!CAUTION]
> This WO touches a separate repository (esp32-app-foundation). It cannot be a
> stacked branch — changes there must be committed separately by the user.

#### Cutover sequence per repo:

**Step 1: Install + enroll**
```bash
cd /path/to/bug-fixer-loop-plugin
./scripts/install.sh
./scripts/enroll.sh -r matthewmcneill/esp32-app-foundation -d /path/to/esp32-app-foundation -m matthewmcneill
```

**Step 2: Verify new hooks fire**
- Open workspace in Antigravity
- Confirm hooks load from `plugins.json` → workspace plugin
- Test a `write_to_file` on a protected file → should be denied

**Step 3: Remove old standalone sidecars**
```bash
# Remove the old per-loop sidecars from ~/.gemini/config/sidecars/
rm -rf ~/.gemini/config/sidecars/bug-fixer-*--esp32-app-foundation/
```

**Step 4: Remove injected hooks from consumer repo**
- esp32-app-foundation `.agents/hooks.json`: remove `bug-fixer-file-gate`, `bug-fixer-command-gate`, `bug-fixer-integrity-snapshot`, `bug-fixer-url-gate` (keep `flash-tag-gate`, `pio-cli-gate`, `pio-reset-gate`)
- bug-fixer-loop-plugin `.agents/hooks.json`: remove all bug-fixer hooks (file is gitignored)

**Step 5: Remove git submodule** from esp32-app-foundation
```bash
cd /path/to/esp32-app-foundation
git submodule deinit .agents/plugins/bug-fixer-loop-plugin
git rm .agents/plugins/bug-fixer-loop-plugin
rm -rf .agents/plugins/bug-fixer-loop-workspace/
```

**Step 6: Approve new sidecars** in Automations dashboard

**Step 7: Remove `bug-fixer-AGENTS.md`** from `.agents/rules/` (now in workspace plugin's `rules/`)

---

### WO6 — Legacy Cleanup + Docs + Tests (`standard`)

**Base:** `plugin-architecture/wo4-install-enroll` (WO5 is interactive, doesn't create a branch)
**Branch:** `plugin-architecture/wo6-cleanup-docs-tests`

**Objective:** Delete legacy files, update documentation, clean root clutter.

#### [DELETE] Legacy files
- `hooks.json.template` (replaced by `daemon/workspace-plugin/bug-fixer/hooks.json`)
- `sidecars/` directory (all 4 sidecar templates — replaced by `sidecar.json.template` in daemon)
- Root `plugin.json` (replaced by two `plugin.json` files in `daemon/`)
- Root `config.json` (per-repo config now in `sidecars/<repo>/config.json`)

#### [DELETE] Root clutter
- `pr_52.json`
- `update_pr.sh`, `update_pr2.sh`
- `approve_pr.sh`
- `test-repo/`, `test-remote/`
- `matthewmcneill_bug-fixer-loop-plugin_triage_preview.json`

#### [MODIFY] `scripts/uninstall.sh`
Rewrite for new layout:
- Remove `~/.gemini/config/plugins/bug-fixer-daemon/` (warn + require `--force` if `sidecars/` non-empty)
- Remove stale entries from `~/.gemini/config/config.json` sidecars section
- Print instructions to remove `.agents/plugins.json` entries from consumer repos

#### [MODIFY] `docs/plugin-architecture.md`
Update to reflect split-daemon pattern, new directory layout, enroll/unenroll commands.

#### [MODIFY] `README.md`
Update installation and usage instructions.

#### [MODIFY] Test files
- Verify all tests pass with new import paths
- Add missing `load_config()` tests if not done in WO2
- Fix 2 pre-existing test failures (import hygiene, git-push gate) if in scope

**Files deleted:** ~10
**Files modified:** ~5

---

## Verification Plan

### Automated Tests

```bash
# After each WO:
python3 -m unittest discover -s tests -v

# After WO1 (import path changes):
python3 -c "from daemon.workspace_plugin.bug_fixer.scripts.pre_tool_gate import load_config; print('OK')"

# After WO2 (config loader + Gate A):
python3 -m pytest tests/test_pre_tool_gate.py -v -k "config or protected or trust"
```

### Manual Verification

**After WO2:** Verify Gate A denies writes to:
- `.agents/plugins.json`
- `sidecars/<repo>/config.json`
- `integrity_monitor.py`

**After WO4:** Run `install.sh` then `enroll.sh`, verify:
- `~/.gemini/config/plugins/bug-fixer-daemon/` created correctly
- `sidecars/<repo>/config.json` has merged v2 schema with `repo_dir`
- State migrated from old workspace if present
- Re-running `install.sh` does NOT wipe sidecars/

**After WO5:** In Antigravity:
- Hooks fire via `plugins.json` (not injected `.agents/hooks.json`)
- No duplicate hook firing
- Sidecar fires, dispatcher runs, creates PR
- Old standalone sidecars are gone
