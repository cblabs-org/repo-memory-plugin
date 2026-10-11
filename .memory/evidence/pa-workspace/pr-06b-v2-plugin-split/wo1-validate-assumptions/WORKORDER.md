---
name: pr-06b-v2-plugin-split-wo1
description: Validate platform assumptions for the split plugin architecture
complexity: standard
---

# WO1: Validate Platform Assumptions

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Validate the three platform assumptions the split daemon + workspace bundle architecture depends on. If any fail, document the failure and the fallback. Write results to `docs/experiments/phase0-plugin-split.md`.

## Strict Boundary Box
- **DO NOT** modify any production code — this is validation only
- **DO NOT** modify `cmd/repo-memory/`, `internal/`, or `packaging/`
- **Create only:** `docs/experiments/phase0-plugin-split.md` and a temporary test directory under `/tmp/` for the validation
- Clean up any temp files when done

## Context

### What we're validating

The split architecture doc ([`docs/antigravity-plugin-architecture.md`](../../../../docs/antigravity-plugin-architecture.md)) assumes:

1. **Nested plugin dirs are not loaded globally.** A subdirectory `workspace-plugin/repo-memory/` inside `~/.gemini/config/plugins/repo-memory-daemon/` should NOT be loaded as a global plugin. The plugin loader is non-recursive.

2. **`.agents/plugins.json` supports `~` expansion.** When a repo has `.agents/plugins.json` with `{"entries": [{"path": "~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin"}]}`, Antigravity should expand `~` and load the nested plugin for that workspace only.

3. **Extra files in sidecar dirs are ignored.** A `config.json` file alongside `sidecar.json` inside `sidecars/tick/` should not break the sidecar loader. (Phase 0 check #3 from the shared conventions.)

### Where to find documentation

- The Antigravity plugin loading behaviour is documented in the AGY plugin/customisation skills and docs
- The `agy` CLI can list loaded plugins: `agy plugin list` or similar
- The `.agents/plugins.json` format is documented in the AGY json_configs reference

## Line-Item Instructions

### Check 1: Nested plugin dir isolation

- [x] Create a temporary test plugin structure that mimics the split layout:
  ```
  /tmp/repo-memory-split-test/
  ├── plugin.json              # { "name": "test-daemon" }
  ├── sidecars/
  │   └── tick/
  │       └── sidecar.json     # minimal: { "builtin": "schedule", "args": ["0 0 31 2 *", "echo", "never"] }
  └── workspace-plugin/
      └── test-workspace/
          ├── plugin.json      # { "name": "test-workspace-bundle" }
          └── hooks.json       # { "Stop": { "commands": [{ "command": ["echo", "test-hook-fired"] }] } }
  ```
- [x] Symlink or copy the test plugin to `~/.gemini/config/plugins/test-split-daemon/`
- [x] Check if `test-workspace-bundle` appears as a loaded plugin (check `~/.gemini/config/config.json` plugins section, or use `agy` commands)
- [x] If it IS loaded globally → the assumption fails. Document the fallback (separate directories instead of nesting).
- [x] Clean up: remove `~/.gemini/config/plugins/test-split-daemon/`

### Check 2: `~` expansion in `.agents/plugins.json`

- [x] In this repo's `.agents/` directory, create a temporary `plugins.json`:
  ```json
  {
    "entries": [
      { "path": "~/.gemini/config/plugins/test-split-daemon/workspace-plugin" }
    ]
  }
  ```
  (Use the test plugin from Check 1 — re-install it for this check)
- [x] Start a conversation in this repo and check if the `test-workspace-bundle` hooks are loaded
- [x] If `~` is NOT expanded → test with absolute path as fallback
- [x] Document whether `~`, `$HOME`, or only absolute paths work
- [x] Clean up: remove `.agents/plugins.json` and the test plugin

### Check 3: Extra files in sidecar dirs

- [x] Add a `config.json` file alongside the test sidecar:
  ```
  test-split-daemon/sidecars/tick/config.json   # { "enrolled_repos": [] }
  ```
- [x] Verify the sidecar still appears in the dashboard / is loadable
- [x] Document whether extra files cause errors
- [x] Clean up

### Write the report

- [x] Create `docs/experiments/phase0-plugin-split.md` with:
  - Date, branch, methodology
  - Results table: assumption × result × evidence
  - For any failed assumption: the fallback approach
  - Conclusion: can we proceed with the split architecture?

## Verification Requirement
All three checks documented with clear pass/fail. Report committed. No production code modified. `go build ./...` still passes (nothing changed).

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
Complete

### Changes Made
- Created [`docs/experiments/phase0-plugin-split.md`](docs/experiments/phase0-plugin-split.md) documenting the methodology, empirical results, binary disassembly analysis, and architecture recommendations across all three checks.
- Empirically validated Check 1 (nested plugin directory isolation) using a temporary daemon with schedule sidecar and nested workspace bundle.
- Empirically validated Check 2 (`~` / `$HOME` expansion vs absolute/relative paths in `.agents/plugins.json`) using an active test hook harness and CLI prompts.
- Empirically validated Check 3 (extra configuration files in sidecar directories).
- Cleaned up all temporary test files, configuration modifications, and runtime sidecar directories.

### Decisions Made
- **Target layout confirmed:** `workspace-plugin/repo-memory` can safely be nested under `repo-memory-daemon/` without global discovery collision.
- **Absolute paths required:** `repo-memory enroll` (WO4) must resolve and write absolute paths to `.agents/plugins.json` because Antigravity's loader does not expand `~` or `$HOME`.
- **Sidecar directory configuration supported:** Storing `config.json` in `sidecars/tick/config.json` causes zero supervisor or validation issues.

### Dead Ends & Warnings
- **Warning on `.agents/plugins.json`:** Never write `~` or `$HOME` in `.agents/plugins.json`. Antigravity's Go customization loader does not evaluate shell expansions, resulting in silent load failures.

### Out-of-Scope Findings
- In `internal/harness/antigravity/attribution_test.go`, `TestSessionAttributor_Integration` fails when executed beyond the hardcoded test timestamp (`now := time.Date(2026, 10, 2, ...)`), because dynamically created transcript files on disk have modification times newer than `now` (`now.Sub(fi.ModTime())` is negative, triggering the idle cutoff check).

### Blockers
None. WO1 assumptions are fully validated and documented. WO2 and WO3 can proceed immediately.
