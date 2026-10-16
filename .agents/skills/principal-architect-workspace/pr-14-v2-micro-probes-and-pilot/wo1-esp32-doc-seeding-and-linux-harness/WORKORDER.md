---
name: pr-14-v2-micro-probes-and-pilot-wo1
description: Handoff instructions for Work Order 1 — Linux Harness Portability, esp32-app-foundation Staging, and Combined PA + Doc-Seed Index Build
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Governance & Authorization

- Your parent project is `.agents/skills/principal-architect-workspace/pr-14-v2-micro-probes-and-pilot/PROJECT.md`, where human approval of the 4-Work-Order decomposition is recorded (`3fd99ae`).
- Read `.agents/rules/00-operating-model.md` before starting.
- **External GitHub Hygiene & Git Author Rule (CRITICAL):**
  - Never include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute local home paths like `/usr/local/google/...`) in any repository files, comments, reports, or commit messages.
  - All git commits MUST use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.
  - Whenever running `gh` or `git clone`/`git fetch` against GitHub in shell commands, export:
    `export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/95209/bus" XDG_RUNTIME_DIR="/run/user/95209"`

## Objective

Make `evaluation/eval-v2/scripts/*.sh` portable across Linux and macOS with full `v2.2` state archival (`state/marks/*.json`, `evidence/notes/*.json`, `state/freshness.json`), stage `matthewmcneill/esp32-app-foundation` at commit `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` into gitignored `evaluation/eval-v2/{template,tpl-a,tpl-b,tpl-c,tpl-d}`, refine `doc_seed.go` & CLI `--repo` flags so `repo-memory seed` + `repo-memory seed-docs` + `repo-memory ops gc` + `repo-memory reindex` build the combined `v2.2` (`pa` + `doc-seed`) index and dossiers in `evaluation/eval-v2/seed/`, and update `evaluation/eval-v2/SEED_REPORT.md`.

## Strict Boundary Box

- **In Scope:**
  - `evaluation/eval-v2/scripts/eval_run.sh`
  - `evaluation/eval-v2/scripts/build_index.sh`
  - `evaluation/eval-v2/scripts/verify_template.sh`
  - `evaluation/eval-v2/scripts/prepare_templates.sh` (new repeatable template staging script)
  - `evaluation/eval-v2/SEED_REPORT.md`
  - `cmd/repo-memory/cmd_seed.go`, `cmd/repo-memory/cmd_seed_docs.go`, `cmd/repo-memory/cmd_ops_gc.go` (adding optional `--repo` flag)
  - `internal/core/seed/doc_seed.go`, `internal/core/seed/doc_seed_test.go`, `internal/core/seed/gc.go` (refining `doc-seed` extraction for `esp32-app-foundation` skills/docs and `pio`/`cmake` commands)
  - Gitignored local staging directories: `evaluation/eval-v2/template/`, `evaluation/eval-v2/tpl-{a,b,c,d}/`, `evaluation/eval-v2/seed/`
- **Out of Scope:**
  - Do NOT modify `internal/core/delivery/`, `internal/core/consolidate/`, or write the `WO2`/`WO3`/`WO4` evaluation reports under `docs/evaluations/`.
  - Do NOT un-ignore `evaluation/eval-v2/template/`, `tpl-*`, or `seed/` in `.gitignore` (they must remain gitignored).
  - Do NOT push branches or open PRs on GitHub.

## Tooling Constraints

- Use standard Go commands (`go build`, `go test ./...`) and bash scripts.
- Ensure `~/.local/bin/repo-memory` is rebuilt from current source before running `build_index.sh`.

## Line-Item Instructions

- [x] **1. Add `--repo` flag to `seed`, `seed-docs`, and `ops gc` CLIs:**
  - In `cmd/repo-memory/cmd_seed.go`: add `--repo` (default `"."`) to `seedCmd` and pass it to `gitops.FindRepoRoot(seedRepoDir)` instead of `"."`.
  - In `cmd/repo-memory/cmd_seed_docs.go`: add `--repo` (default `"."`) to `seedDocsCmd` and pass it to `gitops.FindRepoRoot(seedDocsRepoDir)` instead of `"."`.
  - In `cmd/repo-memory/cmd_ops_gc.go`: add `--repo` (default `"."`) to `opsGCCmd` so `resolvePath` uses `gitops.FindRepoRoot(opsGCRepoDir)`.

- [x] **2. Fix Linux Portability & `v2.2` State Archival in `evaluation/eval-v2/scripts/`:**
  - In `evaluation/eval-v2/scripts/eval_run.sh`:
    - Add a portable helper function `clone_tree_portable`:
      ```bash
      clone_tree_portable() {
        local src="$1"
        local dst="$2"
        if [ "$(uname -s)" = "Darwin" ]; then
          cp -c -R "${src}" "${dst}"
        else
          cp --reflink=auto -a "${src}" "${dst}" 2>/dev/null || cp -a "${src}" "${dst}"
        fi
      }
      ```
      and replace `cp -c -R "${TEMPLATE_SRC}" "${REPO_DIR}"` (around line 341) with `clone_tree_portable "${TEMPLATE_SRC}" "${REPO_DIR}"`.
    - Add a `--dry-run` flag to `eval_run.sh` that executes the full setup/staging verification (or prints the staged verification summary) without invoking `agy`.
    - In Condition C/D setup (`case "${CONDITION}" in c|d)`): after `repo-memory enroll "${REPO_DIR}"`, ensure the pre-built seed artifacts in `${EVAL_ROOT}/seed` are populated into the run's enrolled environment:
      - If `${EVAL_ROOT}/seed/ops` exists, copy `${EVAL_ROOT}/seed/ops` into `${REPO_DIR}/.memory/ops`, and if `${EVAL_ROOT}/seed/dossiers` and `${EVAL_ROOT}/seed/ATLAS.md` exist, copy them into `${REPO_DIR}/.memory/dossiers` and `${REPO_DIR}/.memory/ATLAS.md`.
      - Run `REPO_MEMORY_DATA_DIR="${DATA_DIR}" "${REPO_MEMORY_BIN}" reindex "${REPO_DIR}"` (or copy `${EVAL_ROOT}/seed/index.db` to the enrolled repo's `index.db` path under `${DATA_DIR}/repos/<repo-id>/index.db`) so Condition C/D has the full `v2.2` index and dossiers active on Turn 1.
    - In the `--archive` section of `eval_run.sh` (lines 229–255): in addition to archiving `*/ledgers/*.json` into `${ARCHIVE_DIR}/ledger/`, also archive `v2.2` state and inbox evidence if present under `${DATA_DIR}`:
      - `*/state/marks/*.json` into `${ARCHIVE_DIR}/marks/` (and treat `state/marks/*.json` as satisfying `LEDGER_FOUND=true`)
      - `*/evidence/notes/*.json` into `${ARCHIVE_DIR}/notes/`
      - `*/state/freshness.json` into `${ARCHIVE_DIR}/freshness.json`
  - In `evaluation/eval-v2/scripts/verify_template.sh`: add a portable SHA-256 helper (`sha256_file`) that uses `shasum -a 256` if available and falls back to `sha256sum`.

- [x] **3. Stage `matthewmcneill/esp32-app-foundation` (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) & Create `evaluation/eval-v2/scripts/prepare_templates.sh`:**
  - Create `evaluation/eval-v2/scripts/prepare_templates.sh` (executable `chmod +x`) that:
    - Clones `https://github.com/matthewmcneill/esp32-app-foundation.git` into `evaluation/eval-v2/template` if not already present (using `DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/95209/bus" XDG_RUNTIME_DIR="/run/user/95209"`), checks out commit `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`, removes the `origin` remote (`git remote remove origin || true`) so no push can occur, and tags `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` as `baseline` (preserving commit history in `template/` so `repo-memory seed --before 45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` and `repo-memory seed-docs` can read exact git commit timestamps and blobs at `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`).
    - Creates `tpl-a`, `tpl-b`, `tpl-c`, and `tpl-d` using `clone_tree_portable`, squashing `.git` in each `tpl-{a,b,c,d}` to a clean single `baseline` commit (no git history to mine, no remote) plus the condition commit (`condition-a: no memory`, `condition-b: PA only`, `condition-c: repo-memory push`, `condition-d: both`), removing `.agents/skills/principal-architect-workspace/` and writing `.agents/overrides/disabled-skills.txt` in `tpl-a` and `tpl-c`.
  - Run `./evaluation/eval-v2/scripts/prepare_templates.sh` and verify `template/` is at `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` and `tpl-{a,b,c,d}` are cleanly staged.

- [x] **4. Inspect `esp32-app-foundation` Docs/Skills & Refine `internal/core/seed/doc_seed.go`:**
  - Inspect how `repo-memory seed-docs` parses `evaluation/eval-v2/template/docs/developer/**/*.md` and `evaluation/eval-v2/template/.agents/skills/**/SKILL.md` (specifically `.agents/skills/framework-expert/SKILL.md` and `.agents/skills/creating-a-module/SKILL.md`).
  - Refine `internal/core/seed/doc_seed.go` (and `gc.go` if needed) so that:
    - `extractFirstCommandLine` also recognizes `pio `, `platformio `, `python `, `python3 `, `cmake `, and `./` commands in code fences.
    - When `ParseDocToOps` processes a skill (`isSkill`) or developer guide under `docs/developer/`, if backtick tokens reference valid relative directories/files in the repo (e.g., `src/core`, `src/modules`, `include`, `test`, `platformio.ini`) or if a skill (`framework-expert`, `creating-a-module`, etc.) has bullets without explicit file backticks, ensure `docAnchors` / fallback anchors anchor them to the relevant repository paths (or if `docAnchors` has valid repo paths, use the top relevant paths from `docAnchors`, and if a skill in `.agents/skills/<skill-name>/SKILL.md` has no backtick paths that exist on disk, check whether `src/modules`, `src/core`, `include`, or `docs/developer` exist in the repo via `isValidRepoPath` so key architectural skills like `framework-expert` and `creating-a-module` are anchored and indexed rather than dropped!).
    - Ensure `isPollutedScope` in `internal/core/seed/gc.go` does not falsely reject valid `esp32-app-foundation` scopes while still rejecting polluted scopes.
    - Add unit tests in `internal/core/seed/doc_seed_test.go` covering `esp32-app-foundation`-style skill/developer-doc extraction (`framework-expert`, `creating-a-module`, `pio test -e native` recipe extraction, and subsystem anchoring).

- [x] **5. Upgrade `evaluation/eval-v2/scripts/build_index.sh` & Rebuild `evaluation/eval-v2/seed/`:**
  - Upgrade `evaluation/eval-v2/scripts/build_index.sh` so that when building the Condition C/D seed artifacts for `evaluation/eval-v2/template` (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`):
    1. It builds `~/.local/bin/repo-memory` from the current repo (`go build -o "${REPO_MEMORY_BIN}" "${REPO_ROOT}/cmd/repo-memory"`).
    2. It runs `repo-memory seed --repo "${TEMPLATE_DIR}" --pa-dir "${PA_DIR}" --ops-dir "${OUT_DIR}/ops" --before 45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` to seed PA workorders.
    3. It runs `repo-memory seed-docs --repo "${TEMPLATE_DIR}" --ops-dir "${OUT_DIR}/ops"` to seed `docs/developer/**/*.md` and `.agents/skills/**/SKILL.md` (`source: doc-seed`).
    4. It runs `repo-memory ops gc --repo "${TEMPLATE_DIR}" --ops-dir "${OUT_DIR}/ops" --dossiers-dir "${OUT_DIR}"` so `${OUT_DIR}/dossiers/` and `${OUT_DIR}/ATLAS.md` are deterministically projected alongside `${OUT_DIR}/ops/`.
    5. It runs `repo-memory reindex` in the staging workdir and checkpoints WAL to produce `${OUT_DIR}/index.db`.
    6. It queries and prints the breakdown of entries by `source` (`seed` vs. `doc-seed`), `kind` (`semantic`, `procedural`, `episodic`), and `type`, plus FTS5 match counts.
  - Run `./evaluation/eval-v2/scripts/build_index.sh --clean` and verify that both `seed` (PA workorders) and `doc-seed` (`docs/developer/` + `.agents/skills/`) entries are present in `evaluation/eval-v2/seed/index.db`, `evaluation/eval-v2/seed/dossiers/`, and `evaluation/eval-v2/seed/ATLAS.md`.

- [x] **6. Update `evaluation/eval-v2/SEED_REPORT.md`:**
  - Update `evaluation/eval-v2/SEED_REPORT.md` with the exact `v2.2` combined (`seed` + `doc-seed`) metrics:
    - Total entries, total anchors, op batches written
    - Breakdown by `source` (`seed` PA workorders vs. `doc-seed` curated developer guides & skills)
    - Breakdown by Cognitive Lifecycle (`semantic`, `procedural`, `episodic`)
    - Breakdown by Entry Type (`decision`, `ruled-out`, `open-issue`, `invariant`, `contract`, `gotcha`, `recipe`, `failure-signature`)
    - Dossier scopes rendered in `evaluation/eval-v2/seed/dossiers/` and `ATLAS.md`
    - Sample FTS5 and scope queries (including `DataManager`, `U8G2Display`, `WebServer`, `MCPServer`, `EventBus`, `IModule`, `Http`).

## Verification Requirement

1. `go test -race ./cmd/... ./internal/... ./packaging/...` passes with 0 failures.
2. `./evaluation/eval-v2/scripts/verify_protection.sh` passes (`0` failures).
3. `./evaluation/eval-v2/scripts/build_index.sh --clean` succeeds and produces `evaluation/eval-v2/seed/index.db`, `evaluation/eval-v2/seed/dossiers/`, and `evaluation/eval-v2/seed/ATLAS.md` with both PA (`seed`) and `doc-seed` entries.
4. `./evaluation/eval-v2/scripts/eval_run.sh --condition c --task evaluation/eval-v2/tasks/task-1-temperature-module.txt --run-id test-wo1-c --dry-run` (and `--archive`) works cleanly on Linux without `cp -c` errors, and clean up the test run directory afterwards.
5. `git status -s` shows only expected tracked files modified/added (`template/`, `tpl-*`, `seed/`, `runs/` must not appear as untracked files).

**CRITICAL:** Once verified, commit your changes on branch `pr-14-v2-micro-probes-and-pilot/wo1-esp32-doc-seeding-and-linux-harness` (using author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`), append your Completion Report to this `WORKORDER.md` (and commit it), and send a message back to the Principal Architect confirming completion.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
✅ Complete

### Changes Made
- `cmd/repo-memory/cmd_seed.go` — Added `--repo` flag (`seedRepoDir`, default `"."`), smart path resolution supporting both repo-relative defaults and caller-relative explicit paths, and fallback resolution to `baseline`/`HEAD` when `--before` references a pre-squash commit SHA.
- `cmd/repo-memory/cmd_seed_docs.go` — Added `--repo` flag (`seedDocsRepoDir`, default `"."`) and smart path resolution for `--ops-dir` and `--config`.
- `cmd/repo-memory/cmd_ops_gc.go` — Added `--repo` flag (`opsGCRepoDir`, default `"."`) and smart path resolution for `--ops-dir`, `--dossiers-dir`, and `--config`.
- `evaluation/eval-v2/scripts/eval_run.sh` — Added `clone_tree_portable` helper (`cp -c -R` on Darwin, `cp --reflink=auto -a` / `cp -a` on Linux), `--dry-run` mode, Condition C/D `v2.2` seed artifact staging (`ops/`, `dossiers/`, `ATLAS.md`, and `reindex`), and `--archive` support for `state/marks/*.json`, `evidence/notes/*.json`, `state/freshness.json`, `data-dir/`, and `git-diff.patch`.
- `evaluation/eval-v2/scripts/verify_template.sh` — Added portable `sha256_file` helper (`shasum -a 256` with `sha256sum` fallback), defaulted target directory to `evaluation/eval-v2/template` when present, and handled hosts without `pio` installed by verifying SHA-256 manifest integrity.
- `evaluation/eval-v2/scripts/prepare_templates.sh` — Created repeatable executable script staging `matthewmcneill/esp32-app-foundation` into `evaluation/eval-v2/template` (removing `origin` remote and tagging `baseline`) and creating isolated single-commit condition templates `tpl-a`, `tpl-b`, `tpl-c`, and `tpl-d`.
- `internal/core/seed/doc_seed.go` — Extended `extractFirstCommandLine` and `classifyDocEntryType` for `pio`, `platformio`, `python`, `python3`, `cmake`, and `./` commands; added C++/PlatformIO file extension support (`.ini`, `.hpp`, `.hxx`, `.cc`, `.csv`, `.html`, `.css`, `.defaults`, `CMakeLists.txt`), C++ interface/module symbol anchor resolution (`extractDocAnchors`), Markdown link resolution, callout (`> [!IMPORTANT]`/`[!WARNING]`/`[!CAUTION]`) and skill Q&A (`### ...?`) extraction, section/document fallback anchoring for `SKILL.md` and `docs/developer/`, and 2-file C++ submodule scope preservation (`expandSubmoduleAnchorsForScopes`).
- `internal/core/seed/gc.go` — Updated `isPollutedScope` to reject scopes containing `.` or leading/path-segment `-` (such as `src/main.cpp` or `hardware/-TEMPLATE`), allowed valid single-segment repo directory anchors (`isValidDocDirAnchor`), and applied `expandSubmoduleAnchorsForScopes` during `RunOpsGC` scope repair.
- `internal/core/seed/doc_seed_test.go` — Added `TestSeedDocs_ESP32SkillAndDeveloperDocs` verifying extraction from `framework-expert` (`SKILL.md`) and `docs/developer/creating-a-module.md`, `pio test -e native` recipe classification, subsystem scope resolution (`modules/DataManager`, `modules/U8G2Display`, `modules/Application`), and zero-repair agreement with `RunOpsGC`.
- `evaluation/eval-v2/scripts/build_index.sh` — Upgraded to 5-stage `v2.2` pipeline (`go build` -> `repo-memory seed` -> `repo-memory seed-docs` -> `repo-memory ops gc` -> `repo-memory reindex` + WAL checkpoint + SQL verification with `sqlite3`/`python3` fallback).
- `evaluation/eval-v2/SEED_REPORT.md` — Updated with full `v2.2` combined corpus metrics (`1,283` entries: `256` `seed` + `1,027` `doc-seed`, `3,190` anchors, `80` op batches, `54` projected dossiers + `ATLAS.md`, cognitive lifecycle breakdown, entry type breakdown, and FTS5/scope query tables).

### Decisions Made
- Preserved 2-file C++ submodule directories (`modules/DataManager`, `modules/U8G2Display`, `modules/EventBus`, `modules/ConfigManager`, `modules/Application`, etc.) as distinct module scopes during `DeriveModuleScopes` in `doc_seed.go` and `gc.go` (matching `preinvocation.go`'s 3-item synthetic sibling pattern) so `first-entry` and scope queries resolve to dedicated module dossiers rather than rolling all 2-file C++ modules into a single generic `modules` scope.
- Mapped framework role interfaces (`IDataSource`, `FetchContext`, `IWebContributor`, `ITabRegistry`, `IMCPContributor`, `IDisplayBoard`, `IConfigurable`, `IOTAVerifier`, `IPreconditionContributor`, `IBLEServiceProvider`) to both their `include/interfaces/*.hpp` header and their owning subsystem module (`modules/DataManager`, `modules/WebServer`, `modules/MCPServer`, `modules/U8G2Display`, `modules/ConfigManager`, etc.), preferring concrete module paths for `DossierScope` while retaining interface headers in `anchors` for `first-touch` delivery.
- Added fallback in `cmd_seed.go` to resolve `--before` against the `baseline` tag when the specified pre-squash SHA (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) is not a standalone object in the cloned Git history, matching `c7dc8d88ec703d61597817533cfcaf29d460e3fd` (verified identical `platformio.ini` SHA-256 `d4e548eb511ebcb2a1769d96028da61acc3b7431f6038a884fbcf0455c1e8855` and 435/118/79 workorder counts).

### Dead Ends & Warnings
- Creating a Git tag whose name is a 40-character hex string (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) causes Git to warn (`refname is ambiguous`) and resolve the 40-hex string as a missing raw object SHA before consulting `refs/tags/`; deleted the 40-hex tag and resolved missing `--before` commit SHAs to the `baseline` tag in `cmd_seed.go` instead.

### Out-of-Scope Findings
- (none)

