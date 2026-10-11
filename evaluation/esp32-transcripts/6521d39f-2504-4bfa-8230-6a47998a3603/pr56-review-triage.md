# PR #56 Review Triage

## Blocking Items

### 1. Skills look for config in wrong place
**Issue:** Skills use `${BUG_FIXER_STATE_DIR}/config.json` but config is in `sidecars/<repo>/`, one level above `state/`.
**Fix:** Add `BUG_FIXER_CONFIG` to sidecar env. Update skill SKILL.md to use `BUG_FIXER_CONFIG` instead of `STATE_DIR/config.json`. The sidecar.json already has both env vars — but the skills need updating.

### 2. `enroll.sh` is a stub that throws away real config
**Issues:**
- Hardcoded template drops `maintainer`, `trusted_users`, `scanning`, `required_labels`, etc.
- `test_command` hardcoded to `pio test -e native`
- `--migrate-state` copies config into `state/` instead of using it as the sidecar config
- Missing `PATH` and `ANTIGRAVITY_PROJECT_ID` in sidecar env
- Key mismatch: `trajectory_review_interval_min` vs `trajectory_interval_min`
- Sidecar args shape unconfirmed

**Fix:** Restore `configure.sh` config-building logic into `enroll.sh`: read existing config if present, validate, merge. Copy migrated `config.json` to sidecar root (not `state/`). Add `PATH` to sidecar env. Fix the key name.

### 3. `safe_ingest` loads wrong repo's config
**Issue:** Sidecar scan returns first match regardless of current repo. `require=True` not called.
**Fix:** Match sidecar by comparing `repo_dir` in each sidecar's `config.json` against the current workspace path. Re-add `require=True` where needed.

### 4. Gate sidecar match is wrong way round
**Issue:** `r_path.is_relative_to(Path(wp))` checks if repo is under workspace — should be the opposite. Paths not resolved.
**Fix:** Swap to `Path(wp).resolve().is_relative_to(Path(repo_dir).resolve())` — workspace should be under (or equal to) the repo.

### 5. `integrity_monitor` mostly blind in interactive sessions
**Issue:** No `BUG_FIXER_CONFIG` in hook env. Doesn't watch sidecar config, plugins.json, hooks.json. Snapshot shared across sessions/repos.
**Fix:** Read `workspacePaths` from stdin to find the matching sidecar config. Scope snapshot by repo slug. Add sidecar files to watch list.

### 6. `install.sh` contains comment block just to pass a test
**Issue:** Commented-out model lines exist only for `test_install_script_contains_default_models`.
**Fix:** Delete both the test and the comment. Models are now in `enroll.sh`.

---

## Should Fix

### 7. Relative file paths denied
**Fix:** Resolve `TargetFile` against workspace path from stdin, not hook cwd.

### 8. Invalid JSON returns exit code 1
**Fix:** Return exit 0 with deny decision on all error paths.

### 9. `safe_ingest` path wrong in rules/gate messages
**Fix:** Use absolute installed path in `REASON` and `rules/github-reads.md`.

### 10. `install.sh` claims enabled when it isn't
**Fix:** Check `agy plugin enable` return code. Remove `|| true`. Remove `"disabled": true` from `daemon/plugin.json`.

### 11. `plugins.json` entry may point one level too high
**Fix:** Verify — it points at `workspace-plugin/` which is scanned one level deep, finding `bug-fixer/` with `plugin.json`. This is correct per `json_configs.md`.

### 12. `tick_receiver` never records exit status
**Fix:** Remove dead `exit_file` variable or implement the feature.

### 13. Shell variable pasting in heredocs
**Fix:** Pass values as Python arguments to `json.dump` instead of interpolating in heredocs.

### 14. PR body links to missing file
**Fix:** Update PR description.
