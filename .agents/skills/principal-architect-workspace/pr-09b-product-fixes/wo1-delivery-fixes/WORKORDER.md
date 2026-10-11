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

- [ ] **Review the 3 X6-lite fixes** — They are already on main. Read the diffs at commits `8d7ad7b`, `d57f561`, `3036eb4`. Verify they are clean and have proper tests. If any fix lacks test coverage, add tests.

- [ ] **Fix index wiring** — Trace how the index path is resolved in:
  - `cmd/repo-memory/cmd_reindex.go` (the `reindex` subcommand)
  - `internal/core/pipeline/tick.go` (the tick pipeline)  
  - `internal/harness/antigravity/preinvocation.go` (the hook reader)
  - `internal/mcpserver/tools.go` (the MCP pull tools)
  Ensure all four resolve to the same index file path. The canonical location should be `<data-dir>/<repo-hash>/index.db`. Add a shared `IndexPath()` function if one doesn't exist.

- [ ] **Verify the installed binary is current** — Run `which repo-memory` and check the build date/version. If stale, rebuild with `go build -o /usr/local/bin/repo-memory ./cmd/repo-memory/` (or wherever the binary is installed).

- [ ] **Smoke test** — After all fixes, enrol a test repo, run the tick, and verify that:
  - The index has > 0 entries
  - A `PreInvocation` hook call produces injection content (not an error)
  - The MCP `memory_search` tool returns results

## Verification Requirement

The worker must verify `go build ./...` and `go test ./...` pass. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->
