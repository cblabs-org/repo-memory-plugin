---
name: repo-memory-admin
description: >
  Manage repo-memory across repositories: enroll/unenroll workspaces, bootstrap
  Day-1 dossiers from docs and Principal Architect history, rebuild the FTS5/BM25
  index, check status and health (doctor), consolidate notes, graduate skills,
  and garden dossiers. Use when the user mentions enrolling, unenrolling, adding
  repo-memory to a repo, seeding docs, checking repo-memory status, or
  troubleshooting repo-memory.
---

# Repo-Memory Management Skill (`repo-memory-admin`)

## REPO_MEMORY_BIN

```
<bin>
```

All commands below use: `<bin> <subcommand> [args]`

---

## Read-Only & Diagnostic Commands (run directly, report output)

| Command | Example | Purpose |
|---|---|---|
| **status** | `<bin> status [--repo <dir>] [--json]` | Enrolment status, freshness, active dossiers/entries, graduated skills |
| **doctor** | `<bin> doctor [<dir>] [--repo <dir>]` | Verify `.memory/.enrolled`, `.agents/plugins.json`, `GEMINI.md` v2 block, and skill lint |
| **report** | `<bin> report [--days 7] [--json]` | Multi-tier telemetry and token usage summary |
| **search** | `<bin> search "<query>"` | FTS5/BM25 query across active entries, unconsolidated inbox notes, and evidence |

---

## Enrollment & Day-1 Bootstrap Procedure

Whenever the user asks to enroll a repository via `/repo-memory-admin` without explicitly specifying all flags on the command line (e.g., `/repo-memory-admin enroll this repo please`), **always inspect the repository first and use `ask_question` to confirm their enrollment options before running `<bin> enroll`**.

### Step 1 — Read-only repository inspection

Determine the target repository root (defaults to the current workspace root if the user says *"enroll this repo"*) and inspect its current documentation and Principal Architect state:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
ls -d "$REPO_ROOT/.memory" \
  "$REPO_ROOT/.agents/plugins/principal-architect-workspace" \
  "$REPO_ROOT/.agents/skills/principal-architect-workspace" \
  "$REPO_ROOT/.agents/skills" \
  "$REPO_ROOT/docs" 2>/dev/null || true
ls "$REPO_ROOT"/*.md "$REPO_ROOT"/docs/*.md "$REPO_ROOT"/docs/**/*.md 2>/dev/null | head -n 25 || true
```

Note what was detected:
- **Principal Architect workspace present**: whether `.agents/plugins/principal-architect-workspace` (v0.1.0+) or `.agents/skills/principal-architect-workspace` (legacy v1) exists.
- **Candidate documentation**: whether top-level design docs (`SPEC.md`, `ARCHITECTURE.md`) or `docs/**/*.md` files exist outside the default `docs/developer/**/*.md` and `.agents/skills/**/SKILL.md` globs.

### Step 2 — Interactive Option Confirmation (`ask_question`)

Use `ask_question` to confirm the user's preferred configuration before executing `<bin> enroll`:

1. **Day-1 Bootstrap & Seeding (`--no-seed`, `--no-seed-pa`, `--no-seed-docs`)**:
   - `(Recommended) Seed both repository docs/skills and Principal Architect history` *(or `(Recommended) Seed repository docs and skills` if no PA workspace is present)*
   - `Seed repository docs and skills only (--no-seed-pa)`
   - `Seed Principal Architect history only (--no-seed-docs)`
   - `Start with an empty memory bank (--no-seed)`
2. **Additional Documentation Globs (`--doc-seed`)** *(include when top-level or non-default docs like `SPEC.md` or `docs/*.md` are detected outside `docs/developer/**/*.md` and `.agents/skills/**/SKILL.md`)*:
   - `(Recommended) Include detected architecture/design docs (<list detected e.g. SPEC.md, docs/**/*.md>)`
   - `Default globs only (docs/developer/**/*.md and .agents/skills/**/SKILL.md)`
3. **End-of-Turn Distillation Mode (`--turn-notes`)**:
   - `(Recommended) Warm-cache Stop hook + background tick (on) — automatic end-of-turn harvest at milestone turns + zero-token CHECKPOINT/brain harvest`
   - `Background tick & CHECKPOINT only (fallback-only) — zero synchronous Stop hook continuations during interactive turns`
   - `Manual only (off) — only record explicit memory_note MCP calls and seeded docs`
4. **Principal Architect Memory Mode Handoff (`--pa-memory-mode`)** *(include when `.agents/{plugins,skills}/principal-architect-workspace` is detected)*:
   - `(Recommended) Switch Principal Architect to external mode (--pa-memory-mode external) — repo-memory owns codebase memory; PA uses Succinct Write-Back Mode`
   - `Keep Principal Architect's current memory_mode unchanged (--pa-memory-mode keep)`
5. **Coverage Gap Delivery (`--push-gaps`)**:
   - `(Recommended) Pull-only coverage gaps — surface open questions and undocumented files in memory_brief and .memory/QUESTIONS.md`
   - `Push coverage gap hints in context (--push-gaps)`

### Step 3 — Execute enrollment & verify health

Run `<bin> enroll` with the confirmed flags, then verify with `doctor` and `status`:

```bash
<bin> enroll "$REPO_ROOT" \
  [--turn-notes <on|fallback-only|off>] \
  [--doc-seed "<glob>"] \
  [--pa-memory-mode <external|keep>] \
  [--no-seed | --no-seed-pa | --no-seed-docs] \
  [--push-gaps]

<bin> doctor "$REPO_ROOT"
<bin> status --repo "$REPO_ROOT"
```

What `enroll` does automatically:
- Creates `.memory/` (`config.yaml`, `scrub.yaml`, `dossiers/`, `ops/`, `local/`), applies any `--turn-notes`, `--doc-seed`, and `--push-gaps` overrides to `.memory/config.yaml`, and creates `.memory/.enrolled`
- Inserts or upgrades the `<!-- repo-memory:begin v2 ... -->` managed block in `GEMINI.md`
- Writes `.agents/plugins.json` pointing to `workspace-plugin/repo-memory` and adds `.agents/plugins.json` to `.git/info/exclude`
- Registers the repository and its git worktrees in `<data-dir>/registry.json`
- **Principal Architect (PA) handoff & migration**: Unless `--pa-memory-mode keep` or `--no-seed-pa` is set, switches `repo_config.memory_mode` in `<pa-workspace>/.state/state.db` to `external`, seeds PA `WORKORDER.md` completion blocks and `PROJECT.md` Part III architectural invariants into `.memory/ops/`, and imports PA artifacts (`MEMORY.md`, `MODULE_INDEX.md`, `PROJECT_INDEX.md`, `PROJECT.md`, `WORKORDER.md`, `context_bridge.md`) as episodes
- **Day-1 doc seeding, projection & indexing**: Seeds curated docs (`docs/developer/**/*.md`, `.agents/skills/**/SKILL.md`, plus any `--doc-seed` globs), projects `.memory/dossiers/` + `ATLAS.md`, refreshes `GEMINI.md`'s Atlas table, and builds `<data-dir>/<repo-id>/index.db`

### Step 4 — Post-enrollment summary

Report the `repo-id`, active dossier count, and active entry count to the user, and note:
1. `.memory/` (`config.yaml`, `scrub.yaml`, `ATLAS.md`, `QUESTIONS.md`, `dossiers/`, `ops/`) is tracked in git so memory compounds across the team; `.agents/plugins.json` and `.memory/.enrolled` remain git-excluded.
2. Hooks and `repo-memory` MCP pull tools (`memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status`) are active immediately.
3. Ensure `"Repo Memory Tick"` (`repo-memory-daemon/tick`) is enabled in `[Open Automations Dashboard](sidecar://dashboard)` for background consolidation and doc freshness ("dreaming").

---

## Other Management Operations

| Operation | Command | Notes |
|---|---|---|
| **Seed PA history** | `<bin> seed --repo <dir> [--dry-run]` | Manually re-scans `.agents/{plugins,skills}/principal-architect-workspace` (`WORKORDER.md` + `PROJECT.md`) |
| **Import PA episodes** | `<bin> import-pa --repo <dir>` | Manually imports PA markdown files (`MEMORY.md`, `PROJECT.md`, `WORKORDER.md`, etc.) as searchable episodes |
| **Refresh doc seeds** | `<bin> seed-docs --repo <dir> [--dry-run]` | Re-distills changed docs/skills (`source: doc-seed`) |
| **Backfill history** | `<bin> backfill --repo <dir> [--transcripts] [--docs] [--git] [--dry-run]` | Imports historical transcripts/PA artifacts, docs, and git reverts |
| **Consolidate inbox** | `<bin> consolidate --repo <dir> [--force] [--dry-run]` | Runs Stage 2 batch consolidation over pending inbox notes |
| **Garden dossiers** | `<bin> garden --repo <dir> [--dry-run]` | Retires broken anchors, merges duplicates, splits oversized dossiers |
| **Graduate skills** | `<bin> promote-skills --repo <dir> [--dry-run]` | Graduates validated procedural recipes (`convCount >= 2`) to `.agents/skills/rm-*` |
| **Replay op-log** | `<bin> reproject --repo <dir> [--dry-run]` | Regenerates dossiers, `ATLAS.md`, and `QUESTIONS.md` from `.memory/ops/` |
| **Rebuild FTS5 index** | `<bin> reindex --repo <dir>` | Rebuilds `<data-dir>/<repo-id>/index.db` from `.memory/ops/` |
| **Clean op-log** | `<bin> ops gc --repo <dir> [--dry-run]` | Prunes junk/broken-anchor ops and re-renders dossiers |
| **Pause / Resume** | `<bin> disable [--all]` / `<bin> enable` | Temporarily pauses or resumes hooks without unenrolling |
| **Unenroll** | `<bin> unenroll [<dir>] [--purge]` | Removes `.agents/plugins.json` entry, `.enrolled`, and `GEMINI.md` block, and restores PA `memory_mode` to `builtin` (`--purge` deletes private `<data-dir>/<repo-id>/`) |

---

## Troubleshooting

1. **Hooks or MCP tools not firing**: Run `<bin> doctor .` and `<bin> status`. If `.agents/plugins.json` or `.memory/.enrolled` is missing, re-run `<bin> enroll .`.
2. **Empty search results after seeding**: Run `<bin> reindex --repo .` to rebuild `<data-dir>/<repo-id>/index.db` from `.memory/ops/`.
3. **`GEMINI.md` managed block error**: Ensure `<!-- repo-memory:begin ... -->` and `<!-- repo-memory:end -->` markers are balanced in `GEMINI.md`, then re-run `<bin> enroll .`.
4. **Inbox notes not consolidating automatically**: Check `[Open Automations Dashboard](sidecar://dashboard)` to verify `Repo Memory Tick` is enabled, or run `<bin> consolidate --repo . --force`.
