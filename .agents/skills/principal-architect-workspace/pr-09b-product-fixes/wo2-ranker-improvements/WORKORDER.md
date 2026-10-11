---
name: pr-09b-wo2
description: Handoff instructions for Work Order 2 — Ranker Improvements
complexity: standard
---

# WO2: Ranker Improvements

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Fix the ranker so it produces relevant, non-duplicate, threshold-filtered results. The current ranker ignores bm25 relevance scores from FTS search, produces 35-60% duplicate injections across moments, and swallows FTS errors. These are product bugs found during the X6-lite evaluation that must be fixed before evaluation v2 can meaningfully test repo-memory's value.

## Context

The X6-lite verdict (`docs/evaluations/x6-lite-verdict.md` §5) identified these ranker issues:
- **bm25 ignored:** `RankEntries()` in `internal/core/delivery/ranker.go` uses anchor match, entry type, verification status, and evidence count — but NOT the bm25 relevance score from FTS. The FTS search in `engine.go` caps results to preserve bm25 ranking, but that ranking is then discarded when `RankEntries()` re-sorts by type.
- **~35-60% duplicate injections:** The same entry is injected at task-start, then again at dossier-entry, then again at anchor-touch. No per-session deduplication.
- **FTS errors swallowed:** Two X6-lite runs got no task-start injection at all because FTS errors were silently caught and returned as empty results.
- **No relevance threshold:** Any entry within the token budget gets injected, even entries with zero anchor match and low type score.

Additionally, the `about_to_edit` moment never fires because the model decides to edit and issues the edit tool call in the same turn, so there's no PreToolUse window to inject between "I'll edit file X" and the actual edit. This is a design limitation — document it and consider alternatives, but don't rewrite the moment system.

## Strict Boundary Box

- **IN scope:** `internal/core/delivery/ranker.go`, `internal/core/delivery/engine.go`, `internal/core/delivery/delivery_test.go`, `internal/core/delivery/formatter.go`, `internal/harness/antigravity/preinvocation.go` (for dedup tracking)
- **OUT of scope:** `evaluation/` (do not modify), `cmd/` (do not modify), `.agents/skills/principal-architect-workspace/` (do not modify), index building, MCP tools

## Tooling Constraints

- Build with `go build ./...`
- Test with `go test ./cmd/... ./internal/... ./packaging/...` (exclude `evaluation/oracle/` — those are X6-lite fixtures that fail on main)

## Line-Item Instructions

- [ ] **Incorporate bm25 relevance into ranking.** Add a `FTSScore float64` field to `RankedEntry`. When entries come from FTS search (via `TaskStartEntries`), pass the bm25 score through. In `RankEntries()`, add a weighted bm25 component to the score calculation (e.g., `score += int(ftsScore * 100)`). The bm25 score should be a significant factor — currently type ranking (5-80 points) dominates, but a highly relevant FTS match should outrank a low-relevance high-type entry.

- [ ] **Per-session deduplication.** Add an `InjectedSet` (a `map[string]bool` keyed by entry ID) to the `Engine` or to the delivery context. Before injecting an entry, check if it was already injected in this session. Skip duplicates. The set persists across moments within a session but resets per session. This should be tracked in the ledger or in memory on the `Engine` struct.

- [ ] **Surface FTS errors.** In `engine.go`, when FTS search returns an error, do NOT return an empty result silently. Either:
  - Return the error to the caller (preferred — let the hook decide whether to inject an error note), OR
  - Log the error and include a single `[repo-memory] search error: <msg>` entry so the agent knows memory is degraded.

- [ ] **Add a relevance threshold.** In `SelectWithinBudget()` (or in `RankEntries()`), filter out entries below a minimum score threshold. Entries with no anchor match and a low type score (e.g., score < 50) should not be injected. The threshold should be configurable but have a sensible default.

- [ ] **Document the `about_to_edit` limitation.** Add a comment in `internal/harness/antigravity/pretooluse.go` explaining why `about_to_edit` rarely fires: the model typically decides and edits in the same turn, so PreToolUse for the edit tool call is the first signal. By then the decision is already made and a "you're about to edit X" injection is too late. Note: a future approach might use `PostToolUse` on file-read as a proxy for "interested in this file."

- [ ] **Add tests for all changes:**
  - Test that entries with high bm25 scores outrank entries with higher type scores but low bm25
  - Test that duplicate entries are filtered across moments
  - Test that FTS errors are surfaced (not swallowed)
  - Test that below-threshold entries are filtered

## Verification Requirement

The worker must verify `go build ./...` and `go test ./cmd/... ./internal/... ./packaging/...` pass. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->
