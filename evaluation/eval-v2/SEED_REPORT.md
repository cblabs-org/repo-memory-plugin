# Seed Index Build Report

## Overview

This report documents the build and validation of the seeded memory index for the `esp32-app-foundation` repository, created as part of Work Order 3 (`pr-10-eval-v2/wo3-index-build`). The seed index powers evaluation conditions **c** (repo-memory push only) and **d** (both PA workspace and repo-memory).

The seed artifacts are stored in:
- `evaluation/eval-v2/seed/ops/` — Seeded memory op batch files (JSON format partitioned by month).
- `evaluation/eval-v2/seed/index.db` — SQLite database with FTS5 search index.

The build process is automated via the repeatable script:
- `evaluation/eval-v2/scripts/build_index.sh`

---

## Seeding Metrics

| Metric | Count | Details |
| :--- | :--- | :--- |
| **PA Projects Scanned** | 105 | Directories under `.agents/skills/principal-architect-workspace/` |
| **Total Workorders Scanned** | 435 | Total `WORKORDER.md` files |
| **Workorders with Completion Reports** | 118 | Contain `## Completion Report` header |
| **Workorders Seeded** | 79 | Contain valid, non-placeholder Decisions, Dead Ends, or Findings |
| **Workorders Uncompleted / Pending** | 317 | No Completion Report section present |
| **Workorders with Empty / Template Reports** | 39 | Completion Report only contained template stubs or `(none)` |
| **Workorders Failing to Parse** | 0 | All scanned workorders parsed cleanly without parse errors |
| **Op Batches Written** | 79 | JSON batch files in `evaluation/eval-v2/seed/ops/2026-10/` |
| **Total Memory Ops Generated** | 260 | Extracted knowledge entries |
| **Total Indexed Entries** | 260 | Rows in SQLite `entries` and `entries_fts` tables |
| **Total Entry Anchors** | 2,206 | Rows in SQLite `entry_anchors` table |

### Breakdown by Entry Type

| Entry Type | Op Type | Count | Description |
| :--- | :--- | :--- | :--- |
| **Decisions** | `decision` | 157 | Architectural and implementation choices made during workorders |
| **Dead Ends** | `ruled-out` | 58 | Negative knowledge, debunked claims, and failed implementation paths |
| **Findings** | `open-issue` | 45 | Discovered bugs, inconsistencies, and follow-up warnings |
| **Total** | | **260** | |

---

## Workorder Parsing Analysis

- **435 workorders total**: The workspace includes 105 project directories representing various iterations and work orders across the repository history.
- **317 pending/uncompleted workorders**: These workorders represent template specifications, drafts, or workorders that did not include a completion report.
- **118 workorders with Completion Reports**:
  - **79 workorders** yielded non-empty, actionable memory ops (260 ops total).
  - **17 workorders** had completion report templates that were unpopulated (e.g. `<!-- Written by the worker at task completion -->` or `_To be filled in on completion._`).
  - **22 workorders** had completion reports with `(none)` or only status/changes made without decisions, dead ends, or findings.
- **0 parse errors**: The `repo-memory seed` parser handled all markdown formatting without crashes or unhandled exceptions.

---

## Index Verification

The SQLite index file `evaluation/eval-v2/seed/index.db` was validated against entry counts and full-text search (FTS5) queries.

### Schema Validation
The database contains the expected tables:
- `entries` (260 rows)
- `entries_fts` (260 rows indexed via FTS5)
- `entry_anchors` (2,206 rows mapping entries to file paths)
- `error_patterns`, `aliases`, `entry_history`

### FTS Search Verification

| Term | Match Count | Sample Extracted Entry |
| :--- | :--- | :--- |
| **`display`** | 13 | `[decision]` *(docs)* Relative link repairs in archived doc: When moving `lvgl-display-design.md` into `docs/archive/`, adjusted relative links... |
| **`portal`** | 19 | `[decision]` *(include/portal-assets)* Used an inner `PreconditionWebContributor` in `Application` rather than hardcoding routes in `WebServerManager`... |
| **`OTA`** | 11 | `[decision]` *(test/test-unit/test-lvglDisplay)* Mapped straight from the export, not the WO3 table. The human's SLS tweaks renamed several Error/OTA globals into the display controller... |
| **`WiFi`** | 13 | `[decision]` *(test/test-unit/test-lvglDisplay)* `gateHelperScreen` returns nullptr for `GateNames::WIFI_STATION` so the module's Tier-2 (`wifiWizardScreen`, Setup group) handles setup... |
| **`LVGL`** | 17 | `[decision]` *(docs)* Relative link repairs in archived doc: When moving `lvgl-display-design.md` into `docs/archive/`, adjusted relative links... |
| **`display OR portal OR OTA`** | 38 | Compound domain search verifying multi-term OR query matching |

---

## Hermetic Build Workflow

To ensure compliance with the Strict Boundary Box (which forbids modifying the evaluation templates or master `template/`), `build_index.sh` uses a hermetic staging approach:

1. Runs `repo-memory seed` to generate ops into `evaluation/eval-v2/seed/ops/`.
2. Creates an ephemeral Git repository in `/tmp/seed_work.XXXXXX` and links/copies `ops/` into `.memory/ops/`.
3. Runs `repo-memory reindex` with `REPO_MEMORY_DATA_DIR=/tmp/seed_data.XXXXXX`.
4. Checkpoints the SQLite WAL log (`PRAGMA wal_checkpoint(TRUNCATE)`) and copies the completed `index.db` to `evaluation/eval-v2/seed/index.db`.
5. Cleans up all temporary directories via an `EXIT` trap.
6. Verifies row counts and FTS query matches.

This hermetic workflow leaves `evaluation/eval-v2/template` and `tpl-{a,b,c,d}` completely untouched while producing a self-contained, reproducible seed index.
