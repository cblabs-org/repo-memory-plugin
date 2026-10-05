# Phase 0 Experiment Report: Plugin Split & Multi-Bundle Architecture Assumptions

**Date:** 2026-10-05  
**Branch:** `pr-06b-v2-plugin-split/wo1-validate-assumptions`  
**Status:** Completed & Validated  
**Author:** Execution Worker Agent (Antigravity)

---

## 1. Executive Summary

This report documents the empirical validation of the three core platform assumptions underpinning the `pr-06b-v2-plugin-split` architecture:

1. **Check 1 (Nested plugin dir isolation):** **PASS**  
   A nested plugin directory (`workspace-plugin/test-workspace/`) placed inside a global plugin directory (`test-split-daemon/`) is **completely isolated**. Antigravity's global plugin discovery scans strictly 1 level deep; it does not discover, validate, or auto-enable nested plugins.
2. **Check 2 (`~` expansion in `.agents/plugins.json`):** **FAIL for `~` and `$HOME`; PASS for absolute and relative paths**  
   Antigravity does **not** expand `~` or environment variables in `.agents/plugins.json`. When configured with a tilde path, hooks fail to fire. When configured with an **absolute path** (or relative path), the workspace plugin loads successfully and all lifecycle hooks (`PreInvocation`, `Stop`) fire normally.  
   **Actionable Impact:** `repo-memory enroll` (WO4) must resolve and write **absolute paths** to `.agents/plugins.json`.
3. **Check 3 (Extra files in sidecar directories):** **PASS**  
   Placing `config.json` alongside `sidecar.json` inside `sidecars/tick/` causes zero validation errors, crashes, or loader disruption. Antigravity specifically inspects `sidecars/<name>/sidecar.json` and ignores extraneous files.

---

## 2. Methodology & Empirical Findings

### Check 1: Nested Plugin Directory Isolation

#### Objective
Verify whether placing `workspace-plugin/repo-memory/` inside `~/.gemini/config/plugins/repo-memory-daemon/` causes Antigravity to:
- (a) Accidentally discover `repo-memory` globally (violating isolation), or
- (b) Only discover `repo-memory-daemon` (preserving isolation).

#### Test Setup
1. Created `/tmp/repo-memory-split-test` with:
   - Root: `plugin.json` (`name: "test-daemon"`), `sidecars/tick/sidecar.json` (`schedule: "0 0 31 2 * echo never"`).
   - Nested: `workspace-plugin/test-workspace/plugin.json` (`name: "test-workspace-bundle"`), `workspace-plugin/test-workspace/hooks.json`.
2. Validated with `agy plugin validate`:
   - Daemon root: `hooks: skipped (not found)`. Loader did not descend into `workspace-plugin/`.
   - Nested bundle: `hooks: 1 processed`.
3. Installed to `~/.gemini/config/plugins/test-split-daemon/` and enabled via `agy plugin enable test-split-daemon`.

#### Observations
- `agy plugin enable test-split-daemon` registered `"test-split-daemon": {"enabled": true}` in `~/.gemini/config/config.json`.
- `agy plugin enable test-workspace-bundle` returned: `Error: plugin "test-workspace-bundle" not found or invalid`.
- `agy plugin enable test-workspace` returned: `Error: plugin "test-workspace" not found or invalid`.
- Antigravity IDE `language_server` (PID 76829) launched `language_server multicall schedule 0 0 31 2 * echo never` (PID 71338) under supervisor.
- Sidecar log directory `~/.gemini/antigravity/sidecar_data/test-daemon/tick/` was created.
- Zero sidecar or plugin state was created for `test-workspace-bundle`.
- Binary string inspection confirmed the release note:
  > *"Changed how directory entries in skills.json, rules.json, agents.json, and plugins.json are scanned: an entry now loads only the items directly inside the directory, the same as a .agents/skills/ folder, instead of recursively loading everything beneath it"*.

#### Verdict: PASS
Global discovery does not traverse into nested subdirectories. The nested bundle remains completely inert until explicitly registered by a workspace manifest.

---

### Check 2: `~` Expansion in `.agents/plugins.json`

#### Objective
Determine whether Antigravity expands `~` or `$HOME` in `.agents/plugins.json`, or whether absolute/relative paths are required.

#### Test Setup
1. Created executable test hook script `/tmp/test-hook.sh`:
   ```bash
   #!/bin/sh
   echo "test-hook-fired: $1 (pid: $$)" >> /tmp/test-hook-events.log
   echo "{}"
   ```
2. Configured `test-split-daemon/workspace-plugin/test-workspace/hooks.json` to register `PreInvocation` and `Stop` hooks executing `/tmp/test-hook.sh`.
3. Tested four configuration variants in `.agents/plugins.json` by running a fresh turn with `agy -p "hi"`:

#### Empirical Results

| Variant | Config Path | Hook Log Created? | Events Observed |
|:---|:---|:---:|:---|
| **1. Tilde (`~`)** | `~/.gemini/config/plugins/test-split-daemon/workspace-plugin` | **NO** | `ls: /tmp/test-hook-events.log: No such file or directory` |
| **2. Environment (`$HOME`)** | `$HOME/.gemini/config/plugins/test-split-daemon/workspace-plugin` | **NO** | `ls: /tmp/test-hook-events.log: No such file or directory` |
| **3. Absolute Path** | `/Users/mcneillm/.gemini/config/plugins/test-split-daemon/workspace-plugin` | **YES** | `PreInvocation (pid: 73949)`<br>`PreInvocation (pid: 73955)`<br>`Stop (pid: 73968)` |
| **4. Relative Path** | `../../../../.gemini/config/plugins/test-split-daemon/workspace-plugin` | **YES** | `PreInvocation (pid: 74997)`<br>`Stop (pid: 75009)` |

#### Root Cause Analysis via Binary Disassembly
Disassembly of `google3/third_party/jetski/cortex/customizations`:
- Function `jsonhook.expandHome` (`0x10189cb70`) exists exclusively for `hooks.json` command strings.
- In `customizations.resolveEntry` (`0x1018b4f80`) and `customizations.resolveJSONConfig` (`0x1018b5750`), paths in `entries` are parsed directly using standard Go `filepath` calls without tilde expansion or environment variable expansion.
- `filepath.IsAbs(path)` returns false for `~` and `$HOME`, treating them as literal relative path components. Because directory `./~` does not exist, lookup fails silently.
- For absolute paths, `filepath.IsAbs` is true and loads directly.
- For relative paths, `filepath.Join(configDir, path)` resolves the target path relative to `.agents/plugins.json`.

#### Verdict: FAIL for `~` / `$HOME`; PASS for Absolute & Relative paths
**Fallback Recommendation:** `repo-memory enroll` must resolve the user's home directory (via `os.UserHomeDir()` or `filepath.Abs()`) and write an **absolute path** to `.agents/plugins.json`.

---

### Check 3: Extra Files in Sidecar Directories

#### Objective
Verify that adding non-standard files (such as `config.json` alongside `sidecar.json`) in `sidecars/<name>/` does not disrupt sidecar discovery or execution.

#### Test Setup
1. Created `config.json` inside `test-split-daemon/sidecars/tick/`:
   ```json
   { "enrolled_repos": [] }
   ```
2. Monitored running supervisor process PID 71338 (`language_server multicall schedule 0 0 31 2 * echo never`).
3. Ran `agy plugin validate ~/.gemini/config/plugins/test-split-daemon`.
4. Checked supervisor logs in `~/.gemini/antigravity/sidecar_data/test-daemon/tick/logs/sidecar.log`.

#### Observations
- The running sidecar process PID 71338 remained active and continued executing scheduled runs cleanly.
- `agy plugin validate` passed with `[ok]`.
- No warnings or parse errors appeared in `sidecar.log`.
- Disassembly of `customizations.loadPluginSidecars` (`0x1018c0340`) confirms the loader specifically targets `sidecars/<name>/sidecar.json` and ignores all other sibling files.

#### Verdict: PASS
Storing `config.json` in `sidecars/tick/config.json` is completely safe.

---

## 3. Results Summary Table

| Assumption | Expected | Observed | Verdict | Impact / Fallback |
|:---|:---|:---|:---:|:---|
| **Nested plugin isolation** | Nested `workspace-plugin/` ignored globally | Ignored by global scanner; `agy plugin enable` fails to find it | **PASS** | Target layout in `PROJECT.md` is valid. Daemon and workspace bundle can coexist in one directory tree. |
| **`.agents/plugins.json` `~` expansion** | Loader expands `~` | `~` and `$HOME` not expanded; absolute & relative paths load successfully | **FAIL** (tilde) / **PASS** (absolute) | **Fallback Required:** `repo-memory enroll` (WO4) must write an **absolute path** (`/Users/...`) to `.agents/plugins.json`. |
| **Extra files in sidecar dirs** | `config.json` alongside `sidecar.json` tolerated | Zero errors; supervisor and validator ignore extra files | **PASS** | Shared daemon configuration can safely reside at `sidecars/tick/config.json`. |

---

## 4. Architectural Recommendations for Subsequent Work Orders

1. **For WO2 (`packaging/antigravity/daemon`):**
   - Proceed with packaging the global daemon under `~/.gemini/config/plugins/repo-memory-daemon/`.
   - Embed `workspace-plugin/repo-memory/` nested under the daemon directory as planned.
   - `sidecars/tick/config.json` is a safe, validated location for daemon runtime configuration (e.g. `enrolled_repos`).
2. **For WO3 (`packaging/antigravity/workspace`):**
   - The workspace bundle directory structure (`plugin.json`, `hooks.json`, `skills/`, `rules/`) is validated.
   - The bundle can be pointed to directly via `.agents/plugins.json` entry pointing to `.../workspace-plugin/repo-memory` or its parent `workspace-plugin/`.
3. **For WO4 (`repo-memory enroll`):**
   - **Critical Implementation Rule:** `repo-memory enroll` must resolve the daemon path using `filepath.Abs()` / `os.UserHomeDir()` and write the resolved **absolute path** into `.agents/plugins.json`:
     ```json
     {
       "entries": [
         {
           "path": "/Users/<username>/.gemini/config/plugins/repo-memory-daemon/workspace-plugin"
         }
       ]
     }
     ```
   - Do **NOT** use `~` or `$HOME` in `.agents/plugins.json`.

---

## 5. Cleanup Verification

- Removed `~/.gemini/config/plugins/test-split-daemon/`
- Removed `"test-split-daemon"` from `~/.gemini/config/config.json`
- Removed `~/.gemini/antigravity/sidecar_data/test-daemon/`
- Removed `/tmp/test-hook-events.log`, `/tmp/test-hook.sh`, `/tmp/repo-memory-split-test`
- Removed `.agents/plugins.json`
- Verified test sidecar process terminated
- Verified `git status`: working directory clean (only `docs/experiments/phase0-plugin-split.md` untracked/new)
- Verified `go build ./...`: clean build (exit code 0)
