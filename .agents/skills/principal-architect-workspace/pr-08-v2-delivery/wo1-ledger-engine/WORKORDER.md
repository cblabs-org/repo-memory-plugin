---
name: pr-08-v2-delivery-wo1
description: Handoff instructions for Work Order 1 — Ledger & Delivery Engine
complexity: architect
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the delivery engine core in `internal/core/delivery/`. This package is the heart of the v2 delivery system: it tracks what has been injected (the ledger), decides what to inject next (the ranking engine), enforces token budgets, verifies anchors, and records the exposure list for the downstream echo filter (PR-11).

This WO does NOT build the hook handlers or injection code — those come in WO2-WO5. This WO builds the **engine** that those handlers call.

# Context

## v2 Design References
- **§8.1 (Principles):** Precision over recall. Anchor verification before push. Persistent vs. ephemeral channels. Reference, not command. No fluff.
- **§8.2 (Channels & Budgets):** `systemMessage` (persistent, ≤500/150 tokens), `ephemeralMessage` (1 invocation, ≤200 tokens), `PreToolUse` deny (rare). Budget enforcement per channel.
- **§8.3 (Moments & Trigger Matrix):** Task start, first-entry-into-dossier, first-touch-of-anchor, about-to-edit, error, gate. Each moment has a channel, token budget, and deduplication rule.
- **§8.4 (Compaction & Rewind):** Ledger records step index and content hash per injection. Tags: `[repo-memory <id>]`.
- **§8.5 (Scale):** Constant-time anchor lookups via SQLite index.

## Spike Findings (Critical)
- **X5:** `systemMessage` must be a nested object `{"systemMessage": {"systemMessage": "...", "metadata": {}}}`. `ephemeralMessage` is a plain string. Gate on `invocationNum == 0` or ledger state to prevent duplicate pushes across invocations within a turn.
- **X9:** `PreToolUse` is fail-closed — must always return `{"decision": "allow"}` on success or error. All other hooks fail-open.

## Dependencies (PR-07, already built)
- `internal/core/dossiers/` — `Entry` (8 types: invariant, ruled-out, failure-signature, gotcha, decision, procedure, interface, open-issue), `Projector`, `EntryType`, `EntryTypePrefixes`
- `internal/core/dossiers/projector.go` — `typeOrder` ranking: invariant(1) > ruled-out(2) > failure-signature(3) > gotcha(4) > decision(5) > procedure(6) > interface(7) > open-issue(8)
- `internal/core/index/` — `LookupByAnchor`, `LookupByAnchorExact`, `ListEntriesByScope`, `LookupErrorPatterns`, `SearchEntries`, `ResolveAlias`
- `internal/core/ops/` — `Op`, `OpBatch`, `Source` constants
- `internal/core/config/` — `ScopeResolver`

# Strict Boundary Box

**DO NOT:**
- Modify any hook handler code (`internal/harness/antigravity/`)
- Modify the MCP server (`internal/mcpserver/`)
- Modify the tick pipeline (`internal/core/tick/`)
- Modify the index, dossiers, ops, or config packages
- Create any CLI subcommands
- Import or depend on `internal/harness/antigravity/` — the delivery engine must be harness-agnostic

**DO:**
- Create only files within `internal/core/delivery/`
- Read from `internal/core/dossiers/`, `internal/core/index/`, `internal/core/ops/` types (import them)
- Write pure Go with comprehensive tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/core/delivery/...` to run tests
- Use `go vet ./internal/core/delivery/...` for static analysis

# Line-Item Instructions

## 1. Package Setup
- [ ] Create `internal/core/delivery/doc.go` — Package documentation:
  ```
  // Package delivery implements the v2 delivery engine: the injection ledger,
  // ranking engine, budget enforcement, anchor verification, and exposure tracking.
  // It is harness-agnostic; hook handlers in internal/harness/ call this engine.
  ```

## 2. Channel and Moment Types (`internal/core/delivery/types.go`)
- [ ] Define `Channel` type (string enum):
  - `ChannelSystem` = `"system"` — persistent `systemMessage`
  - `ChannelEphemeral` = `"ephemeral"` — single-invocation `ephemeralMessage`
  - `ChannelDeny` = `"deny"` — `PreToolUse` deny-once gate
- [ ] Define `Moment` type (string enum):
  - `MomentTaskStart` = `"task_start"` — first invocation (≤500 tokens)
  - `MomentFirstEntry` = `"first_entry"` — first entry into a dossier scope (≤150 tokens)
  - `MomentFirstTouch` = `"first_touch"` — first touch of a file/symbol anchor (≤150 tokens)
  - `MomentAboutToEdit` = `"about_to_edit"` — pointer before edit (≤50 tokens ephemeral)
  - `MomentError` = `"error"` — error pattern match (≤200 tokens ephemeral)
  - `MomentGate` = `"gate"` — deny-once for gated entries (≤200 tokens deny)
- [ ] Define `Budget` map: `map[Moment]int` with the token limits above
- [ ] Define `DefaultBudgets()` function returning the budget map

## 3. Injection Record (`internal/core/delivery/ledger.go`)
- [ ] Define `InjectionRecord` struct:
  ```go
  type InjectionRecord struct {
      EntryID     string    // e.g., "inv-01JXXXXXX"
      Channel     Channel
      Moment      Moment
      StepIndex   int       // transcript step where injected
      ContentHash string    // SHA-256 of the injected text
      Tag         string    // "[repo-memory <id>]" marker
      Timestamp   time.Time
  }
  ```
- [ ] Define `Ledger` struct — per-conversation state:
  ```go
  type Ledger struct {
      ConversationID   string
      Injections       []InjectionRecord
      SeenDossiers     map[string]bool   // scope → already pushed first-entry
      SeenAnchors      map[string]bool   // anchor → already pushed first-touch
      AcknowledgedGates map[string]bool  // entryID → gate acknowledged
      ExposureList     []ExposureRecord  // entries pushed or pulled
  }
  ```
- [ ] Define `ExposureRecord` struct:
  ```go
  type ExposureRecord struct {
      EntryID   string
      Channel   Channel    // or "pull" for MCP pulls
      Moment    Moment     // empty for pulls
      Timestamp time.Time
  }
  ```
- [ ] `NewLedger(conversationID string) *Ledger` — constructor
- [ ] `(*Ledger) RecordInjection(rec InjectionRecord)` — append to injections and exposure list
- [ ] `(*Ledger) RecordPull(entryID string)` — append to exposure list with channel="pull"
- [ ] `(*Ledger) HasSeenDossier(scope string) bool`
- [ ] `(*Ledger) MarkDossierSeen(scope string)`
- [ ] `(*Ledger) HasSeenAnchor(anchor string) bool`
- [ ] `(*Ledger) MarkAnchorSeen(anchor string)`
- [ ] `(*Ledger) IsGateAcknowledged(entryID string) bool`
- [ ] `(*Ledger) AcknowledgeGate(entryID string)`
- [ ] `(*Ledger) RearmDedup()` — clear SeenDossiers and SeenAnchors (called after compaction detection)
- [ ] `(*Ledger) GetExposureList() []ExposureRecord`
- [ ] `(*Ledger) FindInjectionAfterStep(stepIndex int) []InjectionRecord` — for compaction detection: find injections that occurred at or after a given step

## 4. Anchor Verifier (`internal/core/delivery/anchor.go`)
- [ ] `VerifyAnchor(anchor string, workspaceRoot string) bool` — check file/directory existence on disk. For file anchors: `os.Stat`. For symbol anchors (containing `:`): check the file portion exists.
- [ ] `VerifyAnchors(anchors []string, workspaceRoot string) (valid []string, suspect []string)` — partition anchors into verified and suspect

## 5. Ranking Engine (`internal/core/delivery/ranker.go`)
- [ ] Define `RankedEntry` struct:
  ```go
  type RankedEntry struct {
      Entry       dossiers.Entry
      Score       int       // composite ranking score
      AnchorMatch string    // "exact", "directory", "scope", or ""
  }
  ```
- [ ] Define ranking score computation following v2 §8.3:
  1. **Anchor match** (highest priority):
     - Exact file/symbol match: +1000
     - Directory match: +500
     - Scope-only (no specific anchor): +100
  2. **Entry type** (per `typeOrder` from projector):
     - `invariant`: +80
     - `failure-signature`: +60
     - `ruled-out`: +50
     - `gotcha`: +40
     - `decision`: +30
     - `procedure`: +20
     - `interface`: +10
     - `open-issue`: +5
  3. **Verification status:**
     - `verified != ""`: +200
  4. **Evidence count:**
     - `+len(Evidence) * 10` (capped at +50)
- [ ] `RankEntries(entries []dossiers.Entry, targetAnchors []string) []RankedEntry` — score and sort descending
- [ ] `SelectWithinBudget(ranked []RankedEntry, tokenBudget int) []RankedEntry` — greedily select entries until the token budget is exhausted. Estimate tokens as `len(entry.Text) / 4` (conservative approximation).

## 6. Content Formatter (`internal/core/delivery/formatter.go`)
- [ ] `FormatEntry(entry dossiers.Entry) string` — format a single entry for injection:
  ```
  [repo-memory <id>] <type>: <text>
  ```
  Example: `[repo-memory inv-01J...] invariant: Never import harness packages from core.`
- [ ] `FormatEntries(entries []RankedEntry) string` — format multiple entries, one per line, with a header:
  ```
  --- repo-memory context ---
  [repo-memory <id1>] invariant: ...
  [repo-memory <id2>] failure-signature: ...
  ---
  ```
- [ ] `FormatGateDeny(entry dossiers.Entry) string` — format deny message:
  ```
  [REPO-MEMORY <id>] <text>. Re-issue the edit after checking this.
  ```
- [ ] `ContentHash(text string) string` — SHA-256 hex of the formatted text (for ledger tracking)

## 7. Delivery Engine (`internal/core/delivery/engine.go`)
- [ ] Define `Engine` struct:
  ```go
  type Engine struct {
      index         *index.Index       // for anchor/scope/error lookups
      projector     *dossiers.Projector // for entry data (or pass entries directly)
      scopeResolver *config.ScopeResolver
      workspaceRoot string
      budgets       map[Moment]int
  }
  ```
- [ ] `NewEngine(idx *index.Index, workspaceRoot string, opts ...EngineOption) *Engine`
- [ ] `EngineOption` functional options: `WithBudgets(map[Moment]int)`, `WithScopeResolver(*config.ScopeResolver)`
- [ ] `(*Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error)` — query index by prompt terms and diff paths, rank, select within `MomentTaskStart` budget. Verify anchors.
- [ ] `(*Engine) DossierEntries(ctx context.Context, scope string) ([]RankedEntry, error)` — entries for first-entry-into-dossier moment, within `MomentFirstEntry` budget. Only invariants, failure-signatures, and ruled-out entries.
- [ ] `(*Engine) AnchorEntries(ctx context.Context, anchor string) ([]RankedEntry, error)` — entries for first-touch-of-anchor moment, within `MomentFirstTouch` budget. Verify the anchor first.
- [ ] `(*Engine) ErrorEntries(ctx context.Context, commandPrefix string, output string) ([]RankedEntry, error)` — match error patterns via `index.LookupErrorPatterns`, rank within `MomentError` budget.
- [ ] `(*Engine) GateEntries(ctx context.Context, targetFile string) ([]RankedEntry, error)` — find entries with `gate: edit` tag anchored to the target file. Only unacknowledged entries.
- [ ] All query methods must:
  - Skip entries where `Retired == true`
  - Skip entries whose anchors are all suspect (verified none)
  - Respect the per-moment token budget
  - Return entries sorted by ranking score descending

## 8. Tests (`internal/core/delivery/delivery_test.go`)
- [ ] **Ledger tests:**
  - Create a ledger, record injections, verify exposure list populated
  - Test dedup: `HasSeenDossier`/`MarkDossierSeen` round-trip
  - Test `RearmDedup` clears seen sets
  - Test `FindInjectionAfterStep` returns correct subset
  - Test gate acknowledgement
- [ ] **Anchor verifier tests:**
  - Create temp files, verify `VerifyAnchor` returns true
  - Non-existent file returns false
  - Symbol anchor (`file.go:FuncName`) checks file portion
  - Directory anchor verified
- [ ] **Ranker tests:**
  - Create entries of different types, anchors, verification status, evidence counts
  - Verify ranking order: exact-anchor invariant with evidence > directory-match decision without evidence
  - Verify `SelectWithinBudget` truncates correctly
- [ ] **Formatter tests:**
  - `FormatEntry` produces correct tag format
  - `FormatEntries` produces correct header/footer
  - `FormatGateDeny` produces correct deny message
  - `ContentHash` is deterministic
- [ ] **Engine integration tests:**
  - Set up a temp SQLite index with known entries and anchors
  - Test `TaskStartEntries` returns ranked entries matching prompt terms
  - Test `DossierEntries` returns only invariant/failure-sig/ruled-out types
  - Test `AnchorEntries` skips suspect anchors
  - Test `ErrorEntries` matches known error patterns
  - Test budget enforcement: large entry set truncated to budget

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/core/delivery/...` — all tests must pass
3. Run `go vet ./internal/core/delivery/...` — must pass

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->

### Status
(pending)

### Changes Made
- (pending)

### Decisions Made
- (pending)

### Dead Ends & Warnings
- (pending)

### Out-of-Scope Findings
- (pending)

### Blockers
- (pending)
