# Plugin Architecture Migration — Final Status

## PR Staged ✅

**[PR #56](https://github.com/matthewmcneill/bug-fixer-loop-plugin/pull/56)** — `feat/plugin-architecture-migration`

Branch: `feat/plugin-architecture-migration` → `main`

---

## Work Order Completion

| WO | Description | Status | Commit | Tests |
|----|-------------|--------|--------|-------|
| WO0 | Spike — validate Antigravity plugin system | ✅ Complete | (no commit — research only) | N/A |
| WO1 | Daemon scaffold + `git mv` files | ✅ Complete | `10d6a87` | 201 pass |
| WO2a | `load_config()` + Gate A rewrite | ✅ Complete | `87c938b` | 215 pass (+14 new) |
| WO2b | Other scripts config/path fixes | ✅ Complete | `5bd7ad4` | 226 pass (+11 new) |
| WO3 | `tick_receiver.py` + dispatcher fixes | ✅ Complete | `54ed1ba` | 236 pass (+10 new) |
| WO4 | `install.sh` + `enroll.sh`/`unenroll.sh` | ✅ Complete | `ab63547` | 236 pass |
| WO5 | Interactive cutover | ⏸️ **Deferred** | — | — |
| WO6 | Legacy cleanup + docs + tests | ✅ Complete | `1fe9ea9` | 236 pass, 0 errors |

**Final test suite:** 240 tests — 236 pass, 3 skipped, 1 known failure (`injection_scanner.py:82`)

---

## What You Need To Do

### 1. Review PR #56
The code changes are ready. Review at your pace.

### 2. Run WO5 Cutover (when ready)
See [wo5-cutover-bridge.md](file:///Users/mcneillm/.gemini/antigravity/brain/6521d39f-2504-4bfa-8230-6a47998a3603/wo5-cutover-bridge.md) for the step-by-step runbook:

1. Remove old standalone sidecars
2. `./scripts/install.sh`
3. `./scripts/enroll.sh -r <owner/repo> -d <repo-dir> --migrate-state`
4. **Verify hooks fire via `plugins.json`** (Check 9 — critical)
5. Remove injected hooks from consumer repos
6. Remove git submodule from esp32-app-foundation
7. Approve new sidecars in Automations dashboard

### 3. Restore hooks
After cutover, the hooks will load from the workspace plugin via `plugins.json`. The `.agents/hooks.json.bak` can be deleted.

---

## Key Architecture Decisions Applied

| Decision | Source |
|----------|--------|
| `plugins.json` uses `"entries"` key, not `"plugins"` | WO0 spike |
| Hook scripts derive paths from `__file__`, not `PLUGIN_ROOT` | WO0 spike (MCP-only) |
| `_find_daemon_dir()` matches `name == "bug-fixer-daemon"` in manifest | User review v3 |
| `Popen` with `pass_fds` + `start_new_session` for lock inheritance | User review v3 |
| Prefix protection with trailing `/` and `state/` exception | User review v3.1 |
| Stdin guard covers all non-hook modes | User review v3.1 |
| `Path("")` truthiness bug avoided with explicit `if _env:` checks | User review v3 |
