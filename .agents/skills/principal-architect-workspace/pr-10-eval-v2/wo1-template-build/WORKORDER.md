---
name: pr-10-eval-v2-wo1
description: Handoff instructions for Work Order 1 — Build the esp32 evaluation template
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Build a build-ready, isolated, reproducible evaluation template from the esp32-app-foundation working tree, per §9 of the [evaluation proposal](../../../../docs/evaluations/memory-value-evaluation-proposal.md). The template will be used to spawn identical per-run copies for the 2×2 factorial evaluation.

## Context

The esp32-app-foundation repo is at `~/Documents/Projects/esp32-app-foundation`. It is a ~85K line C/C++ PlatformIO project. **A fresh clone does not build** — it needs gitignored generated files that exist in the working tree. We copy the working tree (not clone) to preserve these files.

Key generated files that must be preserved (gitignored but required for build):
- Root `CMakeLists.txt` and `dependencies.lock`
- `include/publicKey.hpp`, `include/portal_assets/`
- `modules/LVGLDisplay/ui/`
- `lib/Redactor/CMakeLists.txt`
- `modules/ButtonModule/idf_component.yml`
- `managed_components/` (566MB)

PlatformIO environments: `esp32dev`, `esp32s3nano`, `crowpanel-128-rotary`, `native`.

## Strict Boundary Box

- Do NOT modify any source code in the esp32 repo or repo-memory-plugin.
- Do NOT push, create PRs, or interact with GitHub.
- Do NOT run any hardware builds (no `esp32dev`, `esp32s3nano`, `crowpanel-128-rotary`). Only `native` tests.
- Do NOT delete or modify the original esp32 working tree.
- Work exclusively under `~/Documents/Projects/repo-memory-plugin/evaluation/eval-v2/` (create this directory).

## Tooling Constraints

Use standard shell commands (`rsync`, `git`, `pio`). The PlatformIO MCP server is available for build verification if needed, but raw CLI is fine for this work order since we're working on a copy outside the main project.

## Line-Item Instructions

- [ ] **Create the eval-v2 directory structure:**
  ```
  evaluation/eval-v2/
  ├── template/          # The master template
  ├── tpl-a/             # Condition (a): Nothing
  ├── tpl-b/             # Condition (b): PA only
  ├── tpl-c/             # Condition (c): repo-memory push
  ├── tpl-d/             # Condition (d): Both
  └── scripts/           # Verification and run scripts
  ```

- [ ] **Copy the esp32 working tree to `template/`** using rsync with these exclusions:
  ```bash
  rsync -a \
    --exclude='private_key.pem' \
    --exclude='.pio/build/' \
    --exclude='.agents/plugins.json' \
    --exclude='.memory/.enrolled' \
    --exclude='.memory/' \
    --exclude='.vscode/' \
    --exclude='.agents/tmp/' \
    --exclude='release/' \
    --exclude='releases/' \
    --exclude='.DS_Store' \
    ~/Documents/Projects/esp32-app-foundation/ \
    evaluation/eval-v2/template/
  ```
  Keep `.pio/libdeps` and packages (they're needed for the build). Exclude `.pio/build/` because its `CMakeCache.txt` files embed absolute paths.

- [ ] **Verify the template builds:**
  1. `cd evaluation/eval-v2/template/ && pio test -e native` — native tests must pass.
  2. If the build fails, diagnose and fix **only** the template copy (not the original repo). Document any fixes needed in the completion report. If fixes are required, capture them as a `fix_template.sh` script.

- [ ] **Squash the history:**
  ```bash
  cd evaluation/eval-v2/template/
  rm -rf .git
  git init
  git add -A
  git commit -m "baseline"
  ```
  The `.gitignore` keeps generated build files on disk but untracked. There is no history to mine and no remote.

- [ ] **Create condition templates** by copying `template/` to each `tpl-X/` directory:

  **tpl-a (Nothing):** PA off, repo-memory off.
  ```bash
  cp -c -R template/ tpl-a/
  cd tpl-a/
  rm -rf .agents/skills/principal-architect-workspace/
  # Create a PA-skill disable override:
  mkdir -p .agents/overrides/
  echo "principal-architect: disabled" > .agents/overrides/disabled-skills.txt
  git add -A && git commit -m "condition-a: no memory"
  ```

  **tpl-b (PA only):** PA on (as-is), repo-memory off.
  ```bash
  cp -c -R template/ tpl-b/
  cd tpl-b/
  git add -A && git commit -m "condition-b: PA only"
  ```
  (PA workspace is already present from the rsync; no changes needed.)

  **tpl-c (repo-memory push):** PA off, repo-memory on (hooks push from seeded index).
  ```bash
  cp -c -R template/ tpl-c/
  cd tpl-c/
  rm -rf .agents/skills/principal-architect-workspace/
  mkdir -p .agents/overrides/
  echo "principal-architect: disabled" > .agents/overrides/disabled-skills.txt
  # repo-memory enrolment happens at run-time (WO3 builds the index, per-run script handles enrolment)
  git add -A && git commit -m "condition-c: repo-memory push"
  ```

  **tpl-d (Both):** PA on, repo-memory on.
  ```bash
  cp -c -R template/ tpl-d/
  cd tpl-d/
  git add -A && git commit -m "condition-d: both"
  ```

  Note: tpl-c3 (advertised pull) is a variant of tpl-c created at run time by adjusting the hooks config. No separate template needed.

- [ ] **Create `evaluation/eval-v2/scripts/verify_template.sh`:**
  A script that:
  1. Takes a template directory as argument
  2. Runs `pio test -e native` in it
  3. Reports pass/fail
  4. Computes and prints a sha256 manifest of key files (platformio.ini, src/, modules/, include/, test/)

- [ ] **Create `evaluation/eval-v2/MANIFEST.md`:**
  Record the template creation date, the esp32 HEAD commit hash, the exclusion list used, the sha256 of `platformio.ini`, and the native test results.

## Verification Requirement

1. `pio test -e native` passes in `template/`
2. Each `tpl-X/` has a clean `git status` (one or two commits on `main`)
3. `tpl-a/` and `tpl-c/` have no `.agents/skills/principal-architect-workspace/`
4. `tpl-b/` and `tpl-d/` retain the PA workspace
5. The `verify_template.sh` script runs and reports pass for `template/`

**CRITICAL:** Before committing any files to the repo-memory-plugin repo, ask the user if they are happy with the changes.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

### Status
complete

### Changes Made
- `evaluation/eval-v2/template/` — Created master evaluation template cloned from `~/Documents/Projects/esp32-app-foundation` (commit `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) with exclusions applied (`private_key.pem`, `.pio/build/`, `.agents/plugins.json`, `.memory/`, `.vscode/`, `.agents/tmp/`, `release/`, `releases/`, `.DS_Store`). History squashed to a clean single commit `baseline` on `main`.
- `evaluation/eval-v2/tpl-a/` — Condition (a) (Nothing): PA workspace removed, `.agents/overrides/disabled-skills.txt` installed (`principal-architect: disabled`), committed as `condition-a: no memory`.
- `evaluation/eval-v2/tpl-b/` — Condition (b) (PA only): PA workspace retained as-is, repo-memory off, committed as `condition-b: PA only`.
- `evaluation/eval-v2/tpl-c/` — Condition (c) (repo-memory push): PA workspace removed, `.agents/overrides/disabled-skills.txt` installed, committed as `condition-c: repo-memory push`.
- `evaluation/eval-v2/tpl-d/` — Condition (d) (Both): PA workspace retained as-is, repo-memory on, committed as `condition-d: both`.
- `evaluation/eval-v2/scripts/verify_template.sh` — Executable verification script that tests `pio test -e native`, reports pass/fail, and calculates a SHA-256 manifest of key files (`platformio.ini`, `src/`, `modules/`, `include/`, `test/`).
- `evaluation/eval-v2/scripts/exclude.txt` — Exclusion list used during rsync.
- `evaluation/eval-v2/MANIFEST.md` — Manifest detailing template creation date, source repo HEAD commit, exclusions list, `platformio.ini` SHA-256 (`d4e548eb511ebcb2a1769d96028da61acc3b7431f6038a884fbcf0455c1e8855`), condition configurations, and native test results summary.

### Verification Results
1. `pio test -e native` in `template/`: 989 test cases succeeded, 0 failed (00:01:19 duration).
2. Clean `git status` confirmed across `template`, `tpl-a`, `tpl-b`, `tpl-c`, and `tpl-d`.
3. Verified absence of `.agents/skills/principal-architect-workspace/` in `tpl-a` and `tpl-c`.
4. Verified retention of `.agents/skills/principal-architect-workspace/` in `tpl-b` and `tpl-d`.
5. Executed `verify_template.sh evaluation/eval-v2/template`: Reported `PASSED` (exit code 0).

### Decisions Made
- Used APFS copy-on-write cloning (`cp -c -R`) to create condition templates instantly with zero redundant disk allocation.
- Retained `.pio/libdeps` and packages in the template so evaluations do not require external network access or dependency downloads.
- Stored `exclude.txt` in `evaluation/eval-v2/scripts/` to keep root evaluation directory clean and structured.

### Dead Ends & Warnings
- Sockets created by git fsmonitor (`fsmonitor--daemon.ipc`) in the source repo produced an rsync warning, which was resolved as `.git` was discarded and squashed to a clean `baseline` commit as specified.

### Out-of-Scope Findings
(none)

### Blockers
(none)

