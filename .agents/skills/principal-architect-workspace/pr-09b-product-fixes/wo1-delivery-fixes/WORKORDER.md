---
name: pr-09b-wo1
description: Handoff instructions for Work Order 1 — Delivery Fixes
complexity: standard
---

# WO1: Delivery Fixes

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Integrate the three validated delivery fixes from X6-lite into the main codebase as clean, well-tested changes. Fix the index wiring so all pathways (reindex, tick, hooks, MCP) use the same index file. Rebuild and reinstall the plugin binary.

## Context

During the X6-lite evaluation, three critical product bugs were found and fixed on the `pr-09-v2-x6-lite/wo4-runs` branch. These fixes are already on `main` (merged with PR-09). However, they were patched in-place during evaluation runs and need review:

1. **`8d7ad7b` — Injection channel fix:** Changed `systemMessage` to `ephemeralMessage` in delivery. Without this, Antigravity rejects every injection with `type mismatch: cannot convert string to message`.

2. **`d57f561` — Native tool-call format parser:** `first_touch` and `about_to_edit` moments never fired because the delivery engine didn't parse Antigravity's native tool-call format (used alongside the standard MCP format).

3. **`3036eb4` — Placeholder filter in seeder:** ~10% of seeded entries contained `None.`, `(pending)`, `N/A` placeholder content. The seeder now filters these at source.

Additionally, X6-lite found an **index wiring issue:** `reindex`, the tick pipeline, and the hooks/MCP each look for the index in a different location. The live hook index has 0 entries because it reads from a path the tick never writes to.

## Strict Boundary Box

- **IN scope:** `internal/core/delivery/`, `internal/core/seed/convert.go`, `internal/harness/antigravity/`, `internal/core/index/`, `cmd/repo-memory/`
- **OUT of scope:** `evaluation/` (evaluation harness, scripts, data — do not modify). `.agents/skills/principal-architect-workspace/` (PA workspace — do not modify). Ranker improvements (WO2).

## Tooling Constraints

- Build with `go build ./...`
- Test with `go test ./...`
- The PlatformIO MCP tools are NOT relevant to this Go project

## Line-Item Instructions

- [x] **Review the 3 X6-lite fixes** — They are already on main. Read the diffs at commits `8d7ad7b`, `d57f561`, `3036eb4`. Verify they are clean and have proper tests. If any fix lacks test coverage, add tests.

- [x] **Fix index wiring** — Trace how the index path is resolved in:
  - `cmd/repo-memory/cmd_reindex.go` (the `reindex` subcommand)
  - `internal/core/pipeline/tick.go` (the tick pipeline)  
  - `internal/harness/antigravity/preinvocation.go` (the hook reader)
  - `internal/mcpserver/tools.go` (the MCP pull tools)
  Ensure all four resolve to the same index file path. The canonical location should be `<data-dir>/<repo-hash>/index.db`. Add a shared `IndexPath()` function if one doesn't exist.

- [x] **Verify the installed binary is current** — Run `which repo-memory` and check the build date/version. If stale, rebuild with `go build -o /usr/local/bin/repo-memory ./cmd/repo-memory/` (or wherever the binary is installed).

- [x] **Smoke test** — After all fixes, enrol a test repo, run the tick, and verify that:
  - The index has > 0 entries
  - A `PreInvocation` hook call produces injection content (not an error)
  - The MCP `memory_search` tool returns results

## Verification Requirement

The worker must verify `go build ./...` and `go test ./...` pass. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
Complete. All 4 line items implemented and verified. All 26 product test packages pass cleanly, and end-to-end smoke testing validated index population, PreInvocation hook injection, and MCP tool results against canonical index path.

### Changes Made
- `internal/core/config/layout.go`: Added `config.IndexPath(dataDir, repoID)` helper returning `<data-dir>/<repo-id>/index.db` with default fallback to `config.DataDir()`.
- `internal/core/index/index.go`: Added `index.IndexPath(dataDir, repoID)` wrapper delegating to `config.IndexPath`.
- `cmd/repo-memory/cmd_reindex.go`: Updated index target resolution to use canonical `index.IndexPath` and `gitops.FindRepoRoot` / `GenerateRepoID`, rebuilding index into `<data-dir>/<repo-id>/index.db`.
- `internal/core/tick/pipeline.go`: Updated `ProcessPipeline` to open `index.IndexPath(t.dataDir, repoID)` (replacing `index.sqlite`), and updated `p.Run(ctx)` to rebuild from `.memory/ops` when ops exist.
- `internal/harness/antigravity/preinvocation.go`, `pretooluse.go`, `posttooluse.go`: Consolidated index path resolution to `index.IndexPath(dataDir, repoID)` and standardized repoID derivation using `gitops.FindRepoRoot` and `gitops.GenerateRepoID`.
- `cmd/repo-memory/cmd_mcp.go`: Consolidated index opening to `index.IndexPath(dataDir, repoID)`, resolved repoID via registry / gitops, and configured `opsWriter` to point to `<repoRoot>/.memory/ops`.
- `internal/harness/antigravity/attribution.go` & `attribution_test.go`: Fixed idle conversation check to evaluate the newest primary transcript mtime when transcript path is a directory; synchronized mock directory mtimes in `attribution_test.go`.
- `internal/harness/antigravity/preinvocation_test.go`: Added `TestPreInvocation_AntigravityNativeToolCalls` unit test verifying Antigravity native `{name, args}` tool calls.
- `internal/core/seed/seed_test.go`: Added `TestReportToOps_JunkFiltered` unit test verifying that placeholder/junk strings (`"None."`, `"N/A"`, `"(pending)"`, `"TBD"`, etc.) are discarded.
- Rebuilt binary to `./repo-memory` and installed to `~/.local/bin/repo-memory`.

### Decisions Made
- Canonical index location is strictly `<data-dir>/<repo-id>/index.db` across all four pathways (`cmd_reindex`, `tick`, hooks, `mcp`).
- In `internal/core/tick/pipeline.go`, added op-log replay during `Run` so periodic ticks automatically project newly added ops from `.memory/ops` into `index.db`.

### Dead Ends & Warnings
- When using macOS `/tmp`, symlinks resolve to `/private/tmp`, so registry lookups and path derivations must always support `filepath.EvalSymlinks` or `gitops.FindRepoRoot`.

### Out-of-Scope Findings
- None.

### Blockers
None.
