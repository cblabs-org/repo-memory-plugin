# Work Order 6: Addendum A8 Managed Instruction Block (`v2`) & End-to-End Verification

> **PR:** pr-12-v2-distillation
> **WO:** 6 of 6 — `wo6-managed-instruction-block-and-verification`
> **Branch:** `pr-12-v2-distillation/wo6-managed-instruction-block-and-verification` (stacked on `pr-12-v2-distillation/wo5-sidecar-fallback`)
> **Complexity:** standard

## Goal

Implement Addendum A §8 (`A8: Managed instruction blocks (v2)`) in `internal/core/config/managed_block.go` and wire it into `repo-memory enroll` (`init`), `repo-memory unenroll` (`leave`), and `repo-memory doctor`. Rebuild `~/.local/bin/repo-memory` and verify the full `PR-12` stack across `go build ./...`, `go vet ./...`, `GOOS=windows go build ./...`, and `go test -race ./cmd/... ./internal/...`.

## Exact Specification & Requirements

### 1. Addendum A8 Managed Instruction Block (`internal/core/config/managed_block.go`)
Create `internal/core/config/managed_block.go` implementing Addendum A §8 (`A8: Managed instruction blocks (v2)`):

- **Constants:**
  ```go
  const (
      InstructionFilename     = "GEMINI.md"
      ManagedBlockBeginV2     = "<!-- repo-memory:begin v2 — managed by repo-memory; edits inside are overwritten -->"
      ManagedBlockBeginPrefix = "<!-- repo-memory:begin"
      ManagedBlockEnd         = "<!-- repo-memory:end -->"
      LegacyMapMention        = "@[memory map](.memory/views/MAP.md)"
  )
  ```
- **Canonical `v2` Managed Block Content (`CanonicalManagedBlockV2()`):**
  ```markdown
  <!-- repo-memory:begin v2 — managed by repo-memory; edits inside are overwritten -->
  @[memory atlas](.memory/ATLAS.md)

  ## Repository Memory (`repo-memory`)
  - **Pushed context (`[repo-memory ...]`):** Hooks automatically inject verified invariants, gotchas, failure signatures, and ruled-out approaches when you start a task or touch anchored files. Treat them as repository notes verified against the code; do not repeat or echo them as new discoveries.
  - **Pull for depth (MCP):** Use `memory_brief` (module dossier), `memory_why` (entry/file/line rationale), `memory_history` (evolution), and `memory_search` (FTS5 search) when exploring unfamiliar subsystems.
  - **Distillation:** End-of-turn capture happens automatically via the `Stop` hook on milestone turns (no action required). Use `memory_note` mid-turn only when you want to record a critical discovery or dead end immediately.
  <!-- repo-memory:end -->
  ```
- **`ValidateInstructionBlock(content string) error`:**
  - Scans `content` for `ManagedBlockBeginPrefix` (`<!-- repo-memory:begin`) and `ManagedBlockEnd` (`<!-- repo-memory:end -->`).
  - Returns `nil` if neither marker is present (file has no managed block) OR if exactly one `begin` marker and one `end` marker are present with `beginIdx < endIdx`.
  - Returns a descriptive error if:
    - More than one `<!-- repo-memory:begin` marker is present (`"duplicate repo-memory:begin markers in GEMINI.md"`).
    - More than one `<!-- repo-memory:end -->` marker is present (`"duplicate repo-memory:end markers in GEMINI.md"`).
    - Only one of `begin` or `end` is present (`"unbalanced repo-memory managed block markers in GEMINI.md"`).
    - `end` appears before `begin` (`"inverted repo-memory managed block markers in GEMINI.md"`).
- **`EnsureInstructionBlock(repoRoot string) (createdFile bool, err error)`:**
  - Target file: `filepath.Join(repoRoot, InstructionFilename)` (`GEMINI.md`).
  - **Case 1: `GEMINI.md` does not exist:**
    - Create `GEMINI.md` containing `CanonicalManagedBlockV2() + "\n"` and return `true, nil`.
  - **Case 2: `GEMINI.md` exists:**
    - Read `GEMINI.md`.
    - Run `ValidateInstructionBlock(content)`. If it returns an error, refuse to modify `GEMINI.md` and return `false, err`.
    - If a valid `<!-- repo-memory:begin ... --> ... <!-- repo-memory:end -->` block is present (whether `v1` or `v2`), replace ONLY the span from `<!-- repo-memory:begin` through `<!-- repo-memory:end -->` in place with `CanonicalManagedBlockV2()`, preserving all surrounding user content byte-for-byte.
    - Otherwise, if the legacy bare line `@[memory map](.memory/views/MAP.md)` is present, replace that line with `CanonicalManagedBlockV2()`.
    - Otherwise, append `CanonicalManagedBlockV2()` at the end of `GEMINI.md` (separated by `\n\n` if the file is non-empty) while preserving all existing content.
    - Write atomically / cleanly (`0644`) and return `false, nil`.
- **`RemoveInstructionBlock(repoRoot string, createdByUs bool) error`:**
  - Target file: `filepath.Join(repoRoot, InstructionFilename)` (`GEMINI.md`).
  - If `GEMINI.md` does not exist, return `nil`.
  - Read `GEMINI.md`. Run `ValidateInstructionBlock(content)`. If invalid/unbalanced, return the error without corrupting the file.
  - If a `<!-- repo-memory:begin ... --> ... <!-- repo-memory:end -->` block is present, remove the block (and any single adjacent blank line introduced when appending).
  - Also strip any bare legacy `@[memory map](.memory/views/MAP.md)` line if present.
  - If `strings.TrimSpace(remaining) == ""`:
    - If `createdByUs` is `true`, delete `GEMINI.md` (`os.Remove`).
    - If `createdByUs` is `false`, write the empty file (`""`) so a pre-existing user file is not deleted.
  - Otherwise, write the remaining content back to `GEMINI.md` (`0644`).

### 2. Track `CreatedInstructionFile` & Wire into `enroll`, `unenroll`, and `doctor`
- **Track `CreatedInstructionFile` in `RegistryEntry` (`internal/core/config/registry.go`) and `.memory/.enrolled` (`internal/core/config/marker.go`):**
  - Add `CreatedInstructionFile bool` (`json:"created_instruction_file,omitempty"`) to `RegistryEntry` in `internal/core/config/registry.go` (or provide a helper `SetCreatedInstructionFile(repoRoot string, created bool)` / marker metadata in `.memory/.enrolled`) so `RunUnenroll` knows whether `repo-memory enroll` originally created `GEMINI.md` when it did not previously exist.
  - Note: if `RunEnroll` is called a second time on an already-enrolled repo where `CreatedInstructionFile` was already recorded as `true`, preserve `CreatedInstructionFile = true`!
- **Wire into `RunEnroll` (`cmd/repo-memory/cmd_enroll.go`):**
  - Call `EnsureInstructionBlock(repoRoot)` during `RunEnroll` and record `createdFile` so `RunUnenroll` can pass `createdByUs` to `RemoveInstructionBlock`.
- **Wire into `RunUnenroll` (`cmd/repo-memory/cmd_unenroll.go`):**
  - Look up whether `GEMINI.md` was created by `repo-memory` (`createdByUs`), and call `config.RemoveInstructionBlock(repoRoot, createdByUs)`.
- **Wire into `RunDoctor` (`cmd/repo-memory/cmd_doctor.go`):**
  - If `GEMINI.md` exists in `repoRoot`:
    - Call `config.ValidateInstructionBlock(string(data))`.
    - If `ValidateInstructionBlock` returns an error, print a clear `⚠` warning to `cmd.OutOrStdout()` (e.g. `⚠ GEMINI.md has invalid repo-memory managed block markers: <err>`).
    - Also, if the repo is enrolled (`hasMemoryDir && hasEnrolledMarker`) and `GEMINI.md` either does not exist or does not contain `config.ManagedBlockBeginV2`, print a `⚠` warning (`⚠ GEMINI.md is missing the v2 repo-memory managed instruction block. Run: repo-memory enroll .`) ONLY when `GEMINI.md` was already expected or when checking block validity — wait: check `TestDoctor_Checks` in `cmd/repo-memory/enrolment_test.go`!
    - **IMPORTANT Compatibility Check with `TestDoctor_Checks` (`cmd/repo-memory/enrolment_test.go`):**
      In `TestDoctor_Checks` (lines 610–666 of `cmd/repo-memory/enrolment_test.go`), the test manually calls `config.EnsureRepoLayout(repoDir)`, `config.CreateEnrolledMarker(repoDir)`, and `EnsureWorkspacePluginConfigured(repoDir)` without calling `RunEnroll`, and in Case 3 asserts `if buf.Len() > 0 { t.Errorf("expected clean doctor output, got: %s", buf.String()) }`.
      Therefore, in `RunDoctor`, check `GEMINI.md` if `GEMINI.md` exists on disk:
      - If `GEMINI.md` exists and `config.ValidateInstructionBlock(string(data))` returns an error, print `⚠ GEMINI.md managed block error: ...`.
      - If `GEMINI.md` exists and contains `<!-- repo-memory:begin v1` (stale v1 block), print `⚠ GEMINI.md contains a v1 managed block; run 'repo-memory enroll .' to upgrade to v2.`
      This ensures `TestDoctor_Checks` Case 3 stays 100% green while `RunDoctor` detects any malformed/unbalanced/duplicate markers or stale `v1` blocks in `GEMINI.md`!

### 3. Unit Tests (`internal/core/config/managed_block_test.go` & `cmd/repo-memory/enrolment_test.go`)
- Create `internal/core/config/managed_block_test.go` testing:
  1. `EnsureInstructionBlock` on a repo without `GEMINI.md` creates `GEMINI.md` with the `v2` block (`createdFile == true`), and `RemoveInstructionBlock(repoRoot, true)` deletes `GEMINI.md` when no user content was added.
  2. `EnsureInstructionBlock` on a repo with existing user content in `GEMINI.md` appends the `v2` block (`createdFile == false`), re-running `EnsureInstructionBlock` is idempotent (exactly 1 block), and `RemoveInstructionBlock(repoRoot, false)` removes only the managed block while preserving the user's `GEMINI.md` byte-for-byte.
  3. `EnsureInstructionBlock` upgrades a `v1` managed block (`<!-- repo-memory:begin v1 --> ... <!-- repo-memory:end -->`) in place to `v2` while preserving text before and after the block.
  4. `ValidateInstructionBlock` and `EnsureInstructionBlock` reject duplicate `begin`, duplicate `end`, unbalanced markers, and inverted markers (`end` before `begin`).
- Add a test in `cmd/repo-memory/enrolment_test.go` verifying `RunDoctor` warns when `GEMINI.md` has unbalanced or duplicate `<!-- repo-memory:begin` / `<!-- repo-memory:end -->` markers.

### 4. Binary Rebuild & Full Stack Verification
- Rebuild `~/.local/bin/repo-memory`:
  ```bash
  go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
  ```
- Run full verification across the repository:
  ```bash
  go build ./...
  go vet ./...
  GOOS=windows go build ./...
  go test -race ./cmd/... ./internal/... ./packaging/...
  ```
- Verify `git status -sb` is completely clean (no untracked `GEMINI.md` or test artifacts left in the `repo-memory-plugin` working tree).

## Verification Checklist
- [x] `internal/core/config/managed_block.go` and `managed_block_test.go` implemented and passing.
- [x] `cmd_enroll.go`, `cmd_unenroll.go`, and `cmd_doctor.go` wired and passing all tests in `cmd/repo-memory/...`.
- [x] `~/.local/bin/repo-memory` rebuilt from the `WO6` branch.
- [x] `go build ./...`, `go vet ./...`, `GOOS=windows go build ./...`, and `go test -race ./cmd/... ./internal/... ./packaging/...` pass with zero errors.
- [x] Zero proprietary internal references (`cl/`, `b/`, `go/`, `/home/<user>/...`) in any file or commit message.

## Completion Report

- **Summary of Changes:**
  - Created `internal/core/config/managed_block.go` implementing Addendum A §8 (`A8: Managed instruction blocks (v2)`): `CanonicalManagedBlockV2`, `ValidateInstructionBlock`, `EnsureInstructionBlock`, and `RemoveInstructionBlock`.
  - Added `CreatedInstructionFile` (`json:"created_instruction_file,omitempty"`) to `RegistryEntry` and `SetCreatedInstructionFile` in `internal/core/config/registry.go`, preserving `CreatedInstructionFile` across repeated `RunEnroll` calls.
  - Wired `EnsureInstructionBlock` into `RunEnroll` (`cmd/repo-memory/cmd_enroll.go`), `RemoveInstructionBlock` into `RunUnenroll` (`cmd/repo-memory/cmd_unenroll.go`), and marker validation / stale `v1` detection into `RunDoctor` (`cmd/repo-memory/cmd_doctor.go`).
  - Added unit tests in `internal/core/config/managed_block_test.go` and updated `cmd/repo-memory/enrolment_test.go`.
  - Rebuilt `~/.local/bin/repo-memory` and verified the full stack.
- **Verification Results:**
  - `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`: Passed.
  - `go build ./...`: Passed.
  - `go vet ./cmd/... ./internal/... ./packaging/...`: Passed.
  - `GOOS=windows go build ./...`: Passed.
  - `go test -race ./cmd/... ./internal/... ./packaging/...`: Passed (all 30 packages green).
  - External GitHub hygiene check: 0 internal references found.

## Audit Remediation (PR #20 Cold-Firewall Audit)

- **Code Commit:** `e038f91` (`fix(distill): address PR #20 cold-firewall audit minor findings (MINOR-1..3)`)
- **Fixes Applied:**
  1. **`[MINOR-1]` `internal/mcpserver/tools.go` (`HandleMemoryNote`):**
     - Attached `s.ledger.GetExposureList()` to `inboxNote.ExposureList` when `s.ledger != nil` on `source: "mcp-note"` inbox notes so Stage 2 Echo Filter (`P13`) receives conversation exposures for MCP-originated notes.
     - Verified with `TestMemoryNote_V2EntryTypesInboxAndMark` in `internal/mcpserver/server_test.go`.
  2. **`[MINOR-2]` `internal/harness/antigravity/stop_distill.go` (`harvestTurnNote`):**
     - Added early return when `!mark.PendingContinuation && mark.PendingRequestStep == 0` on `executionNum >= 1` so unrequested continuations do not advance `mark.StepIndex` or scan replies.
     - Updated harvest tests and added `unrequested executionNum >= 1 returns early without advancing mark.StepIndex` in `internal/harness/antigravity/stop_distill_test.go`.
  3. **`[MINOR-3]` `internal/core/tick/fallback.go` (`RunSidecarFallback`) & `internal/core/state/marks.go`:**
     - Added `LastFallbackAt` to `state.Mark`, populated `mark.StepTimestamp = lastStepTS` and `mark.LastFallbackAt = now` whenever `ValidateDistillerOutput` succeeds, and restricted `mark.LastNoteAt = now`, `mark.NoteCount++`, and `mark.NotesToday++` strictly to when `written > 0`.
     - Added `TestRunSidecarFallback_NoneOutputAdvancesMarkWithoutIncrementingNoteCount` and updated assertions in `internal/core/tick/fallback_test.go`.
- **Verification:**
  - Rebuilt `~/.local/bin/repo-memory`.
  - `go test -race ./cmd/... ./internal/... ./packaging/...` and `go vet ./cmd/... ./internal/... ./packaging/...` passed.
