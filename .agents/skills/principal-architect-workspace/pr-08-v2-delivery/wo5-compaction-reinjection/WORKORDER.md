---
name: pr-08-v2-delivery-wo5
description: Handoff instructions for Work Order 5 — Compaction Detection & Re-injection
complexity: architect
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Implement compaction detection and re-injection — the mechanism that keeps injected memory entries alive when Antigravity compacts long sessions. This is the **X11 experiment** confirmed in production code.

When Antigravity compacts a conversation, all previously injected `systemMessage` entries are lost. This WO detects that compaction happened and re-injects a consolidated set of still-relevant entries.

# Context

## X11 Spike Findings (CRITICAL)
- Injected `systemMessage` steps do NOT survive compaction
- Transcripts contain literal `"CHECKPOINT"` lines separating compacted chunks
- `Step.IsCheckpoint` is set by `ParseTranscript()` when a CHECKPOINT line is detected
- Recovery requires:
  1. Tracking injected entries in a session ledger (step index + content hash)
  2. Scanning for CHECKPOINT lines during PreInvocation
  3. Re-injecting up to 1k tokens of active entries
  4. Clearing deduplication sets so subsequent reads push entries again

## Design §8.4 (Compaction & Rewind)
- **Ledger:** Session mark file records all injections (entry IDs, channel, step index, content hash)
- **Detection:** On every `PreInvocation`, read transcript backwards. Compaction = `CHECKPOINT` step after an injection's step. Rewind = transcript shorter or step timestamp changed.
- **Re-injection:** Inject ONE consolidated `systemMessage` of still-relevant entries (≤1k tokens), omitting tags surviving checkpoint text
- **Re-arming:** Clear first-entry and first-touch dedup sets so subsequent reads push entries again

## Existing Code
- `internal/harness/antigravity/transcript.go` — `ParseTranscript()` returns `[]Step` with `IsCheckpoint bool`
- `internal/harness/antigravity/preinvocation.go` — `HandlePreInvocation()` creates a fresh ledger each call. Needs ledger persistence.
- `internal/core/delivery/ledger.go` — `Ledger`, `InjectionRecord`, `FindInjectionAfterStep()`, `RearmDedup()`
- `internal/core/delivery/engine.go` — For re-ranking entries after compaction
- `internal/core/delivery/formatter.go` — `FormatEntries()`, `ContentHash()`

# Strict Boundary Box

**DO NOT:**
- Modify `internal/core/delivery/` (engine, ranker, formatter — frozen)
- Modify `internal/core/ops/`, `internal/core/dossiers/`, `internal/core/index/`
- Modify the `PostToolUse` or `PreToolUse` handlers
- Create CLI subcommands

**DO:**
- Create `internal/harness/antigravity/compaction.go` — compaction detection logic
- Create `internal/harness/antigravity/ledger_store.go` — ledger persistence (file-based)
- Modify `internal/harness/antigravity/preinvocation.go` — integrate compaction detection and ledger persistence
- Create tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/harness/antigravity/... -v` to run tests
- Use `go vet ./...` for static analysis

# Line-Item Instructions

## 1. Ledger Store (`internal/harness/antigravity/ledger_store.go`)

- [ ] The ledger must persist across PreInvocation calls within a conversation to:
  - Track what was injected and at what step index
  - Track which dossiers/anchors have been seen (dedup)
  - Track gate acknowledgements
- [ ] Storage: JSON file at `<dataDir>/<repoID>/ledgers/<conversationID>.json`
- [ ] Define `SaveLedger(dataDir, repoID string, ledger *delivery.Ledger) error`
- [ ] Define `LoadLedger(dataDir, repoID, conversationID string) (*delivery.Ledger, error)` — returns a new ledger if file doesn't exist
- [ ] Use `os.MkdirAll` for the ledger directory
- [ ] JSON marshal/unmarshal the `delivery.Ledger` struct

## 2. Compaction Detector (`internal/harness/antigravity/compaction.go`)

- [ ] Define:
  ```go
  type CompactionResult struct {
      Detected           bool
      CheckpointStepIdx  int      // step index where CHECKPOINT was found
      LostInjections     []delivery.InjectionRecord  // injections after the checkpoint
  }
  ```
- [ ] `DetectCompaction(steps []Step, ledger *delivery.Ledger) CompactionResult`
  - Scan `steps` for any `IsCheckpoint == true`
  - If found, find the LAST checkpoint step index
  - Check if any ledger injections occurred BEFORE that checkpoint (they are now lost)
  - Return the lost injections and the checkpoint index
- [ ] `DetectRewind(steps []Step, lastKnownStepCount int) bool`
  - Rewind = current step count < last known count
  - (For future use; not yet wired)

## 3. Re-injection Logic (`internal/harness/antigravity/compaction.go`)

- [ ] `BuildReinjection(ctx context.Context, engine *delivery.Engine, lost []delivery.InjectionRecord, checkpointText string, budget int) (*InjectStep, error)`
  - For each lost injection, check if the entry is still active (not retired)
  - Re-rank the entries using the engine
  - Select within the re-injection budget (1000 tokens)
  - Format as a consolidated `systemMessage`
  - Skip entries whose `[repo-memory <id>]` tag appears in `checkpointText` (survived compaction)
  - Return the InjectStep, or nil if nothing to re-inject

## 4. Integrate into PreInvocation (`internal/harness/antigravity/preinvocation.go`)

- [ ] After the existing injection logic (line ~237), add compaction detection:
  1. Load the persisted ledger: `LoadLedger(dataDir, repoID, conversationID)`
  2. If loaded ledger has injections AND transcript has steps:
     - Call `DetectCompaction(steps, ledger)`
     - If compaction detected:
       - Call `BuildReinjection()` with the lost injections
       - If re-injection step returned, prepend it to `injectSteps`
       - Call `ledger.RearmDedup()` to re-arm first-entry/first-touch
  3. Merge the current session's injections into the loaded ledger
  4. Save the updated ledger: `SaveLedger(dataDir, repoID, ledger)`
- [ ] Update the fresh-ledger creation to use `LoadLedger` instead of `delivery.NewLedger()`

## 5. Tests

- [ ] `internal/harness/antigravity/compaction_test.go`:
  - **No checkpoint:** `DetectCompaction` returns `Detected: false`
  - **Checkpoint present, no prior injections:** returns `Detected: false` (nothing lost)
  - **Checkpoint present, prior injections lost:** returns correct `LostInjections`
  - **Multiple checkpoints:** uses the LAST one
  - **Rewind detection:** step count decreased

- [ ] `internal/harness/antigravity/ledger_store_test.go`:
  - Save/Load round-trip: injections, seen dossiers, seen anchors preserved
  - Load non-existent: returns fresh ledger
  - Concurrent-safe: two saves don't corrupt

- [ ] Integration test in `preinvocation_test.go`:
  - Set up enrolled workspace with seeded entries
  - Simulate compaction by creating transcript with CHECKPOINT
  - Verify re-injection occurs with consolidated systemMessage

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/harness/antigravity/... -v` — all tests pass
3. Run `go vet ./...` — must pass

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below.

---

## Completion Report (written by the worker at task completion)

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
