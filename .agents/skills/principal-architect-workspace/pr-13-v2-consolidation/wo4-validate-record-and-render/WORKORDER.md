---
name: pr-13-v2-consolidation-wo4
description: Handoff instructions for Work Order 4 (Deterministic Op Validation, Scrubber Gate, Op-Log Append & Dossier/Atlas Re-Render)
complexity: standard
---

# WORKORDER: wo4-validate-record-and-render

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Step 6 of the Stage 2 Warm Batch Consolidation pipeline (`SPEC.md` §6.4 step 6; `docs/design-v2-distillation-retrieval.md` §6 step 6; `docs/addendum-a-provenance-and-hardening.md` §5 `A4`) in `internal/core/consolidate/record.go` and `internal/core/consolidate/record_test.go`:
1. State-aware deterministic op validation (`ValidateAndPrepareOps`) verifying allowed op types, existing entry IDs, required fields, `≤ 6`-line entry length caps, and typed ULID entry ID assignment.
2. 4-layer secret and blob scrubber gate (`scrub.Scrub` with `Boundary: "op"`) on all op string fields, quarantining offending ops via `scrub.QuarantinePayload`.
3. Atomic ULID `ops.OpBatch` writing (`ops.NewWriter`), deterministic dossier/history/atlas re-projection (`dossiers.NewProjector`, `dossiers.NewWriter`), `dossiers.LintDossier` cap checks, and marking processed inbox notes `Consolidated = true` (`inboxStore.MarkConsolidated`).

## Context & Architectural Why
- **The LLM Proposes; Go Validates (`design-v2` §5.6 & §6 step 6):** Even if the consolidator LLM hallucinates an `amend`/`retire`/`supersede`/`move` against a non-existent `entry_id`, omits required fields, exceeds the 6-line entry cap, or leaks a secret into an op field, Go deterministically validates and scrubs every individual `ops.Op` before it can reach the append-only op log.
- **Deterministic Projection (`SPEC.md` §5.1–§5.2):** `.memory/ops/<yyyy-mm>/<ulid>.json` is the source of truth; `.memory/dossiers/*.md`, `.history.md`, and `ATLAS.md` are deterministically projected from the op log and linted via `dossiers.LintDossier`.

## Strict Boundary Box
- **Allowed Files to Create/Modify:**
  - `internal/core/consolidate/record.go`
  - `internal/core/consolidate/record_test.go`
- **Forbidden Actions:**
  - Do NOT modify any files outside `internal/core/consolidate/record.go` and `internal/core/consolidate/record_test.go`.
  - Do NOT change git branches or rebase.
  - `internal/core/consolidate` MUST NOT import `internal/harness/antigravity`.
  - **External GitHub Hygiene:** NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or `/usr/local/google/...`) in code, comments, or commit messages.
  - Use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"` when committing.

## Tooling Constraints
- Verify with:
  - `go test -v -race ./internal/core/consolidate/...`
  - `go vet ./internal/core/consolidate/...`
  - `GOOS=windows go build ./...`

## Line-Item Instructions

- [ ] **1. Implement `ValidateAndPrepareOps` in `internal/core/consolidate/record.go`:**
  - Define `OpValidationFailure`:
    ```go
    type OpValidationFailure struct {
    	Index  int    `json:"index"`
    	Op     ops.Op `json:"op"`
    	Reason string `json:"reason"`
    }
    ```
  - Implement `ValidateAndPrepareOps(candidateOps []ops.Op, existing map[string]*dossiers.Dossier, scrubCtx scrub.Context, now time.Time) (validOps []ops.Op, rejected []OpValidationFailure)`:
    - Normalize `now` to UTC (`time.Now().UTC()` if zero).
    - Build a live state tracker of active entry IDs per dossier scope (`activeByScope map[string]map[string]bool` and `allKnownIDs map[string]bool`) seeded from `existing` (entries where `!e.Retired && e.SupersededBy == ""`).
    - For each `op` at index `i`:
      - Deep-copy `op.Fields` so mutations do not alias caller maps.
      - Default `op.Timestamp = now` if zero, and default `op.Source = ops.SourceConsolidation` if empty (or if not one of `ops.SourceConsolidation`, `ops.SourcePASeed`, `ops.SourceHandEdit`, `ops.SourceDocSeed`).
      - For `ops.OpAdd`:
        - Ensure `type` and `entry_type` are both populated in `op.Fields` if either is present.
        - If `strings.TrimSpace(op.EntryID) == ""`, generate one via `ops.NewEntryIDWithTime(entryPrefixForType(dossiers.EntryType(entryType)), now.Add(time.Duration(i)*time.Millisecond))`.
        - Ensure `op.Fields["purpose"]` is set if `op.DossierScope` does not exist in `existing` or has an empty `Purpose` (default to `fmt.Sprintf("Curated repository memory for %s.", op.DossierScope)` so `dossiers.LintDossier` never warns about a missing purpose section on newly created dossiers).
      - For `ops.OpSupersede`:
        - Ensure `type` and `entry_type` are both populated in `op.Fields` (inheriting the superseded entry's `Type` from `existing` if omitted by the LLM).
        - Ensure `text` and `new_text` are both populated in `op.Fields` if either is present.
        - If `strings.TrimSpace(stringField(op.Fields, "new_entry_id")) == ""`, generate one via `ops.NewEntryIDWithTime(entryPrefixForType(dossiers.EntryType(entryType)), now.Add(time.Duration(i)*time.Millisecond))`.
      - For `ops.OpMove`:
        - Ensure both `"target_scope"` and `"to_dossier"` are populated if either is set.
      - For `ops.OpSplit`:
        - Ensure both `"new_scopes"` and `"new_dossiers"` are populated if either is set.
      - **Enforce `≤ 6` lines per entry text cap:**
        - For `OpAdd`, `OpAmend`, and `OpSupersede`, inspect `op.Fields["text"]` (and `"new_text"`). Strip empty lines if `len(lines) > 6`; if the text still has `> 6` lines, reject the op with `Reason: "entry text exceeds 6-line cap"`.
      - **State-Aware Existence Check:**
        - `OpAdd`: reject if `allKnownIDs[op.EntryID]` is already true (`"duplicate entry_id"`).
        - `OpAmend`, `OpSupersede`, `OpRetire`, `OpMove`: reject if `op.EntryID` is not active in `activeByScope[op.DossierScope]` (`"target entry_id does not exist or is not active in dossier scope"`).
        - `OpSplit`: reject if `len(activeByScope[op.DossierScope]) == 0` (`"source dossier scope has no active entries to split"`) or if any entry ID key in `entry_moves` is not active in `activeByScope[op.DossierScope]`.
      - **Schema Validation:**
        - Call `ops.ValidateOp(op)`. If it returns an error, reject the op with `err.Error()`.
      - **4-Layer Scrubber Gate (`scrub.Scrub`):**
        - Prepare `opScrubCtx := scrubCtx` (default `Boundary: "op"` if empty), cloning `AllowedIDs` and adding `op.EntryID`, `stringField(op.Fields, "new_entry_id")`, and any `evidence` string IDs so legitimate entry/note IDs are preserved.
        - Scrub string fields in `op.Fields` (`"text"`, `"new_text"`, `"title"`, `"purpose"`, `"reason"`, `"closes_when"`) via `scrub.Scrub(val, opScrubCtx)`.
        - If `scrub.Scrub` returns a non-nil error, quarantine the JSON-marshaled op via `scrub.QuarantinePayload(opScrubCtx.DataDir, opScrubCtx.RepoID, "op", scrubErr.Error(), rawBytes)` (when `DataDir != ""` and `RepoID != ""`) and reject the op with `fmt.Sprintf("scrubber rejected op: %v", scrubErr)`.
      - **Update Live State Tracker on Success:**
        - `OpAdd`: mark `op.EntryID` active in `activeByScope[op.DossierScope]` and `allKnownIDs[op.EntryID] = true`.
        - `OpSupersede`: delete `op.EntryID` from `activeByScope[op.DossierScope]`; mark `new_entry_id` active in `activeByScope[op.DossierScope]` and `allKnownIDs[newID] = true`.
        - `OpRetire`: delete `op.EntryID` from `activeByScope[op.DossierScope]`.
        - `OpMove`: delete `op.EntryID` from `activeByScope[op.DossierScope]` and mark it active in `activeByScope[targetScope]`.
        - `OpSplit`: move each `entryID -> dstScope` in `entry_moves` from `activeByScope[op.DossierScope]` to `activeByScope[dstScope]`.
      - Append `op` to `validOps`.

- [ ] **2. Implement `RecordAndRenderDirs` and `RecordAndRenderTier` in `internal/core/consolidate/record.go`:**
  - Define `RecordAndRenderResult`:
    ```go
    type RecordAndRenderResult struct {
    	Batch        *ops.OpBatch                 `json:"batch,omitempty"`
    	Dossiers     map[string]*dossiers.Dossier `json:"dossiers"`
    	LintWarnings map[string][]string          `json:"lint_warnings,omitempty"`
    	MarkedNotes  []string                     `json:"marked_notes,omitempty"`
    }
    ```
  - Implement `RecordAndRenderDirs(ctx context.Context, opsDir string, renderBaseDir string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error)`:
    - Normalize `now` to UTC.
    - If `len(validOps) > 0`:
      - Create `batch := ops.NewBatchWithTime(validOps, now)`.
      - Write `batch` via `ops.NewWriter(opsDir).Write(batch)`.
    - Re-project dossiers from `opsDir`:
      - Ensure `opsDir` exists (`os.MkdirAll(opsDir, 0o755)`).
      - Read and project via `dossiers.NewProjector().ProjectFromReader(ops.NewReader(opsDir))`.
      - For each projected dossier `d`: if `strings.TrimSpace(d.Purpose) == ""` and `len(d.Entries) > 0`, set `d.Purpose = fmt.Sprintf("Curated repository memory for %s.", d.Scope)`.
      - Write `.md`, `.history.md`, and `ATLAS.md` via `dossiers.NewWriter(renderBaseDir).WriteAll(projected)`.
      - Run `dossiers.LintDossier(d)` on each dossier with `len(d.Entries) > 0` and record any non-empty warnings in `res.LintWarnings[d.Scope]`.
    - If `inboxStore != nil` and `len(consolidatedNoteIDs) > 0`:
      - Deduplicate `consolidatedNoteIDs` (preserving order), call `inboxStore.MarkConsolidated(id)` for each non-empty ID, and record marked IDs in `res.MarkedNotes`.
    - Return `res, nil`.
  - Implement convenience wrapper `RecordAndRenderTier(ctx context.Context, tierBaseDir string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error)` which calls `RecordAndRenderDirs(ctx, filepath.Join(tierBaseDir, "ops"), tierBaseDir, validOps, inboxStore, consolidatedNoteIDs, now)`.

- [ ] **3. Write Comprehensive Unit Tests in `internal/core/consolidate/record_test.go`:**
  - `TestValidateAndPrepareOps_StateAwareAndLifecycle`:
    - Verifies `OpAdd` auto-generates typed entry IDs and allows subsequent `OpAmend` in the same batch to reference that newly added entry.
    - Verifies `OpAmend`, `OpSupersede`, `OpRetire`, `OpMove`, and `OpSplit` against non-existent or already-retired `EntryID`s are rejected.
    - Verifies `> 6` line entry text is rejected.
  - `TestValidateAndPrepareOps_ScrubberRedactionAndQuarantine`:
    - Verifies that an inline secret (e.g. `AKIAIOSFODNN7EXAMPLE` or `ghp_...`) inside `op.Fields["text"]` is redacted by `scrub.Scrub` while preserving entry IDs and note ULIDs.
    - Verifies that a malformed `.memory/scrub.yaml` (e.g. containing a forbidden `allow:` key) causes `scrub.Scrub` to fail closed, quarantining the op in `<dataDir>/<repoID>/quarantine/` and rejecting the op.
  - `TestRecordAndRenderTier_EndToEndProjectionAndMarkConsolidated`:
    - Writes 2 inbox notes via `inbox.NewStore`, validates a batch of `OpAdd`, `OpSupersede`, and `OpRetire` ops, calls `RecordAndRenderTier`, and verifies:
      1. The `.json` batch file exists under `<tierBaseDir>/ops/<yyyy-mm>/<batch-id>.json`.
      2. `<tierBaseDir>/dossiers/<scope>.md`, `<tierBaseDir>/dossiers/<scope>.history.md`, and `<tierBaseDir>/ATLAS.md` are rendered on disk.
      3. `len(res.LintWarnings) == 0`.
      4. `inboxStore.ListUnconsolidated()` returns `0` notes (`Consolidated == true` on both notes).

## Verification Requirement
Run all tests (`go test -v -race ./internal/core/consolidate/...`), `go vet ./internal/core/consolidate/...`, and `GOOS=windows go build ./...`. Commit your changes on `pr-13-v2-consolidation/wo4-validate-record-and-render`, then **STOP and send a message back to the Principal Architect**.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/consolidate/record.go` — Implemented `OpValidationFailure`, `ValidateAndPrepareOps`, `RecordAndRenderResult`, `RecordAndRenderDirs`, and `RecordAndRenderTier` covering state-aware op existence checks, 6-line entry text cap enforcement, schema validation (`ops.ValidateOp`), 4-layer secret/blob scrubbing (`scrub.Scrub` + `scrub.QuarantinePayload`), ULID `ops.OpBatch` persistence, dossier/history/atlas projection, `dossiers.LintDossier` cap checks, and `inboxStore.MarkConsolidated`.
- `internal/core/consolidate/record_test.go` — Added unit tests `TestValidateAndPrepareOps_StateAwareAndLifecycle`, `TestValidateAndPrepareOps_ScrubberRedactionAndQuarantine`, and `TestRecordAndRenderTier_EndToEndProjectionAndMarkConsolidated`.

### Decisions Made
- Invoked `scrub.Scrub("", opScrubCtx)` once upfront on every op so malformed Layer 3 `.memory/scrub.yaml` configs fail closed even on ops without optional text fields.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
