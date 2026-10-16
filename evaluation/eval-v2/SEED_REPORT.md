# Seed Index Build Report (v2.2)

## Overview

This report documents the build and validation of the `v2.2` seeded memory corpus and SQLite FTS5 index for the `esp32-app-foundation` repository (`pr-10-eval-v2/wo3-index-build`, upgraded for `v2.2` in `pr-14-v2-micro-probes-and-pilot/wo1-esp32-doc-seeding-and-linux-harness`). The seed artifacts power evaluation conditions **c** (`repo-memory` push only) and **d** (both PA workspace and `repo-memory`).

The seed artifacts are stored in:
- `evaluation/eval-v2/seed/ops/` — Seeded memory op batch files (JSON format partitioned by month, combining historical PA workorder ops and deterministic documentation/skill ops).
- `evaluation/eval-v2/seed/dossiers/` — 54 projected subsystem and module markdown dossiers (`modules/Application`, `modules/DataManager`, `modules/WebServer`, `modules/U8G2Display`, `modules/MCPServer`, `modules/EventBus`, `modules/ConfigManager`, etc.).
- `evaluation/eval-v2/seed/ATLAS.md` — Projected repository memory atlas indexing all 54 dossier scopes.
- `evaluation/eval-v2/seed/index.db` — SQLite database with FTS5 search index (`entries`, `entries_fts`, `entry_anchors`, `error_patterns`, `aliases`, `entry_history`).

The build process is automated via the repeatable script:
- `evaluation/eval-v2/scripts/build_index.sh`

---

## Seeding Metrics (v2.2 Combined Corpus)

| Metric | Count | Details |
| :--- | :--- | :--- |
| **PA Projects Scanned** | 105 | Directories under `.agents/skills/principal-architect-workspace/` |
| **Total Workorders Scanned** | 435 | Total `WORKORDER.md` files at baseline cutoff (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) |
| **Workorders with Completion Reports** | 118 | Contain `## Completion Report` header |
| **Workorders Seeded (`source: seed`)** | 79 | Contain valid, non-placeholder Decisions, Dead Ends, or Findings |
| **Workorder Entries Seeded (`source: seed`)** | 256 | Historical PA workorder entries after `IsJunkText` placeholder filtering |
| **Curated Docs & Skills Scanned** | 41 | `docs/**/*.md` and `.agents/skills/**/SKILL.md` (excluding `principal-architect-workspace`) |
| **Curated Docs & Skills Seeded (`source: doc-seed`)** | 40 | 40 seeded, 1 skipped (`docs/archive/embedded-best-practices-assessment.md`) |
| **Doc-Seed Entries Seeded (`source: doc-seed`)** | 1,027 | Extracted architectural invariants, contracts, recipes, failure signatures, and gotchas |
| **Op Batches Written** | 80 | 79 PA workorder batch files + 1 `doc-seed` batch file in `evaluation/eval-v2/seed/ops/` |
| **Projected Dossier Scopes** | 54 | Rendered `.md` dossiers in `evaluation/eval-v2/seed/dossiers/` + `ATLAS.md` |
| **Total Indexed Entries** | **1,283** | Rows in SQLite `entries` and `entries_fts` tables (`256` `seed` + `1,027` `doc-seed`) |
| **Total Entry Anchors** | **3,190** | Rows in SQLite `entry_anchors` table mapping entries to verified repository paths |

### Breakdown by Source

| Source | Count | Share | Origin |
| :--- | :--- | :--- | :--- |
| **`doc-seed`** | 1,027 | 80.0% | `repo-memory seed-docs` across `docs/developer/**/*.md`, `docs/*.md`, and `.agents/skills/**/SKILL.md` |
| **`seed`** | 256 | 20.0% | `repo-memory seed` across 79 completed Principal Architect `WORKORDER.md` reports |
| **Total** | **1,283** | **100.0%** | |

### Breakdown by Cognitive Lifecycle Kind (`v2.2`)

| Cognitive Kind | Count | Share | Lifecycle Semantics |
| :--- | :--- | :--- | :--- |
| **`semantic`** | 957 | 74.6% | Durable architectural invariants (`767`), contracts (`125`), open issues (`42`), and gotchas (`23`) |
| **`episodic`** | 214 | 16.7% | Historical design decisions (`157`) and ruled-out dead ends (`57`) from PA workorders |
| **`procedural`** | 112 | 8.7% | Executable build/test recipes (`73`) and failure signatures (`39`) |
| **Total** | **1,283** | **100.0%** | |

### Breakdown by Entry Type

| Entry Type | Source | Cognitive Kind | Count | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Invariants** (`invariant`) | `doc-seed` | `semantic` | 767 | Mandatory framework rules, memory/threading invariants, and registration constraints |
| **Decisions** (`decision`) | `seed` | `episodic` | 157 | Architectural and implementation choices made during PA workorders |
| **Contracts** (`contract`) | `doc-seed` | `semantic` | 125 | Pure virtual C++ interface contracts (`IDataSource`, `IWebContributor`, `IMCPContributor`, `IDisplayBoard`, `FetchContext`) |
| **Recipes** (`recipe`) | `doc-seed` | `procedural` | 73 | Build, test (`pio test -e native`), and CLI verification workflows |
| **Dead Ends** (`ruled-out`) | `seed` | `episodic` | 57 | Negative knowledge, debunked claims, and failed implementation paths |
| **Findings** (`open-issue`) | `seed` | `semantic` | 42 | Discovered bugs, inconsistencies, and follow-up warnings |
| **Failure Signatures** (`failure-signature`) | `doc-seed` | `procedural` | 39 | Panic, watchdog timeout, HTTP error, and spinlock failure patterns |
| **Gotchas** (`gotcha`) | `doc-seed` | `semantic` | 23 | Lifetime traps (`EventBus` synchronous payload lifetime, `sdkconfig` caching, bare SVG geometry) |
| **Total** | | | **1,283** | |

---

## Projected Dossier Scopes (`seed/dossiers/` & `ATLAS.md`)

Running `repo-memory ops gc` projects 54 clean subsystem dossiers and `ATLAS.md` with zero polluted scopes (`ScopesRepaired=0` on `doc-seed` ops, and 74 legacy `pr-*` workorder scopes repaired to canonical module/root scopes). Top subsystem scopes include:

| Dossier Scope | Total Entries | `seed` (PA) | `doc-seed` (Docs/Skills) | Primary Anchored Files |
| :--- | :--- | :--- | :--- | :--- |
| `modules/Application` | 187 | 2 | 185 | `modules/Application/Application.hpp`, `modules/Application/Application.cpp` |
| `root` | 134 | 72 | 62 | `platformio.ini`, `sdkconfig.defaults`, root build configuration |
| `modules/DataManager` | 110 | 0 | 110 | `modules/DataManager/DataManager.hpp`, `include/interfaces/IDataSource.hpp`, `include/types/FetchContext.hpp` |
| `include/interfaces` | 64 | 4 | 60 | `include/interfaces/*.hpp`, `include/AppFoundation.h` |
| `modules/WebServer` | 60 | 0 | 60 | `modules/WebServer/WebServer.hpp`, `include/interfaces/IWebContributor.hpp`, `include/interfaces/ITabRegistry.hpp` |
| `modules/ConfigManager` | 57 | 0 | 57 | `modules/ConfigManager/ConfigManager.hpp`, `include/interfaces/IConfigurable.hpp` |
| `scripts` | 44 | 7 | 37 | `scripts/*.py`, `scripts/*.sh` |
| `test/test-unit/test-bleManager` | 41 | 41 | 0 | `test/test-unit/test_bleManager/*` |
| `modules/BLEManager` | 37 | 0 | 37 | `modules/BLEManager/BLEManager.hpp`, `include/interfaces/IBLEServiceProvider.hpp` |
| `lib/DeviceCrypto` | 37 | 0 | 37 | `lib/DeviceCrypto/DeviceCrypto.hpp` |
| `hardware` | 36 | 2 | 34 | `hardware/*/board.h` |
| `docs/developer` | 34 | 24 | 10 | `docs/developer/*.md` |
| `modules/EventBus` | 33 | 0 | 33 | `modules/EventBus/EventBus.hpp` |
| `modules/U8G2Display` | 32 | 3 | 29 | `modules/U8G2Display/U8G2Display.hpp`, `include/interfaces/IDisplayBoard.hpp` |
| `sample-app` | 32 | 6 | 26 | `sample-app/src/*` |
| `modules/OTAUpdateManager` | 31 | 0 | 31 | `modules/OTAUpdateManager/OTAUpdateManager.hpp`, `include/interfaces/IOTAVerifier.hpp` |
| `modules/MCPServer` | 27 | 0 | 27 | `modules/MCPServer/MCPServer.hpp`, `include/interfaces/IMCPContributor.hpp` |
| `lib/DeepStackExecutor` | 24 | 0 | 24 | `lib/DeepStackExecutor/DeepStackExecutor.hpp` |
| `modules/LVGLDisplay` | 23 | 8 | 15 | `modules/LVGLDisplay/LVGLDisplay.hpp`, `include/interfaces/ILVGLHardwareAdapter.hpp` |
| `modules/PreconditionEngine` | 21 | 0 | 21 | `modules/PreconditionEngine/PreconditionEngine.hpp`, `include/interfaces/IPreconditionContributor.hpp` |
| `modules/AuthManager` | 20 | 0 | 20 | `modules/AuthManager/AuthManager.hpp` |

---

## Index & FTS5 Verification

The SQLite index file `evaluation/eval-v2/seed/index.db` was validated against schema integrity, scope queries, and full-text search (FTS5) queries.

### Schema Validation
The database contains all `v2.2` tables:
- `entries` (1,283 rows)
- `entries_fts` (1,283 rows indexed via FTS5)
- `entry_anchors` (3,190 rows mapping entries to repository file paths)
- `error_patterns`, `aliases`, `entry_history`

### FTS5 Search & Subsystem Query Verification

| Query Term | FTS5 Matches | Scope Entries (`dossier_scope`) | Sample Extracted Entry |
| :--- | :--- | :--- | :--- |
| **`DataManager`** | 119 | 110 (`modules/DataManager`) | `[contract]` *(modules/DataManager)* **Three-Layer Bounded Transport**: `DataManager` bounds the transport via `setConnectionTimeout`, temporarily unsubscribes from the TWDT... |
| **`U8G2Display`** | 38 | 32 (`modules/U8G2Display`) | `[open-issue]` *(root)* **U8G2Display has the same gap**: `modules/U8G2Display/ScreensaverController.hpp` is missing `getPolicy()` entirely, and `U8G2DisplayModule`... |
| **`WebServer`** | 60 | 60 (`modules/WebServer`) | `[invariant]` *(modules/WebServer)* Auth enforcement for `/api/` paths is handled centrally — your handlers don't need to check authentication. |
| **`MCPServer`** | 33 | 27 (`modules/MCPServer`) | `[invariant]` *(modules/MCPServer)* During `MCPServer::begin()`, built-in tools are registered first via `mcpBuiltinTools::registerAll()`, then contributor tools from `IMCPContributor`... |
| **`EventBus`** | 55 | 33 (`modules/EventBus`) | `[gotcha]` *(modules/EventBus)* **Strict Synchronous Lifetime:** Payloads passed via the EventBus (`void* payload`) are only valid for the duration of the synchronous callback execution... |
| **`IModule`** | 0 | 64 (`include/interfaces`) | *(Negative control — `esp32-app-foundation` uses pure virtual role interfaces `ITickable`, `IDataSource`, `IWebContributor`, `IMCPContributor`, `IDisplayBoard` under `include/interfaces` rather than a monolithic `IModule` base class)* |
| **`Http`** | 35 | 60 (`modules/WebServer`) | `[invariant]` *(modules/AuthManager)* Looks for an `Authorization: Bearer <token>` HTTP header... |
| **`display`** | 69 | 55 (`U8G2Display` + `LVGLDisplay`) | `[invariant]` *(modules/Application)* **Override the RUNNING transition** — your display module loads your app screen instead of the default `_diagScreen`... |
| **`portal`** | 62 | 19 (`web/portal`) | `[invariant]` *(modules/AuthManager)* **Bearer token** — `Authorization: Bearer <token>` header. The token is the stored password hash, retrievable via `GET /api/auth/mcp-token`... |
| **`OTA`** | 50 | 31 (`modules/OTAUpdateManager`) | `[invariant]` *(modules/Application)* **Learned URL** — stored in OTA module config (`learnedManifestUrl` in `/mod/ota.json`), updated from the `manifestUrl` field... |
| **`WiFi`** | 54 | 8 (`modules/WifiManager`) | `[invariant]` *(modules/AuthManager)* `managedBy`: Identifies the subsystem that registered the secret (e.g. `"WiFi"`, `"AuthManager"`). |
| **`LVGL`** | 36 | 23 (`modules/LVGLDisplay`) | `[invariant]` *(modules/Application)* Prepare an XBM array (U8G2) or `lv_image_dsc_t` (LVGL) for the boot splash logo in `PROGMEM`. |
| **`display OR portal OR OTA`** | 166 | — | Compound domain search verifying multi-term OR query matching across `seed` and `doc-seed` entries |

---

## Hermetic Build Workflow

To ensure compliance with the Strict Boundary Box (which forbids modifying the evaluation templates or master `template/`), `build_index.sh` uses a hermetic 5-stage pipeline:

1. Builds the latest `repo-memory` binary into `~/.local/bin/repo-memory`.
2. Runs `repo-memory seed --repo "${TEMPLATE_DIR}" --pa-dir "${PA_DIR}" --ops-dir "${OUT_DIR}/ops" --before 45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa` to extract historical PA workorder ops.
3. Runs `repo-memory seed-docs --repo "${TEMPLATE_DIR}" --ops-dir "${OUT_DIR}/ops"` to extract anchored rules, contracts, gotchas, failure signatures, and recipes from `docs/` and `.agents/skills/`.
4. Runs `repo-memory ops gc --repo "${TEMPLATE_DIR}" --ops-dir "${OUT_DIR}/ops" --dossiers-dir "${OUT_DIR}"` to sanitize scopes/anchors and project `evaluation/eval-v2/seed/dossiers/` and `evaluation/eval-v2/seed/ATLAS.md`.
5. Stages an ephemeral Git repository in `/tmp/seed_work.XXXXXX`, runs `repo-memory reindex` with `REPO_MEMORY_DATA_DIR=/tmp/seed_data.XXXXXX`, checkpoints the SQLite WAL log (`PRAGMA wal_checkpoint(TRUNCATE)`), and copies `index.db` to `evaluation/eval-v2/seed/index.db`.

This hermetic workflow leaves `evaluation/eval-v2/template` and `tpl-{a,b,c,d}` completely untouched while producing a self-contained, reproducible `v2.2` seed corpus and index.
