---
name: pr-13-v2-consolidation-wo1
description: Handoff instructions for Work Order 1 (Deterministic P13 Echo Filter & VD18 Pre-Consolidation Corroboration Gate)
complexity: standard
---

# pr-13-v2-consolidation / WO1 — Deterministic P13 Echo Filter & Pre-Consolidation Corroboration Gate

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not create branches other than the active branch (`pr-13-v2-consolidation/wo1-echo-filter-and-corroboration-gate`), and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md` and `.agents/skills/principal-architect-workspace/pr-13-v2-consolidation/PROJECT.md` (which records human approval of this 6-WO decomposition). You are authorized to implement and commit this Work Order autonomously within the Strict Boundary Box below.

## Objective
Create package `consolidate` (`internal/core/consolidate/filter.go` and `internal/core/consolidate/filter_test.go`) implementing Stage 2 Step 1: the deterministic **P13 Echo Filter (`FilterEchoes`)** (`SPEC.md` §6.5 step 1, `design-v2` §6 step 1, Addendum A §11.2) and the **Pre-Consolidation Corroboration Gate (`EnforceCorroborationGate`)** (`VD18`, `SPEC.md` §6.4 step 3, `design-v2` §6 step 1).

## Strict Boundary Box
- **Allowed to create/modify:**
  - `internal/core/consolidate/filter.go`
  - `internal/core/consolidate/filter_test.go`
  - `.agents/skills/principal-architect-workspace/pr-13-v2-consolidation/wo1-echo-filter-and-corroboration-gate/WORKORDER.md` (Completion Report)
- **Forbidden:**
  - Do NOT create or modify routing, LLM consolidation, op-log writing, worktree publishing, or tick orchestration files (`route.go`, `llm.go`, `record.go`, `publish.go`, `tick/`) — those belong to WO2–WO6.
  - Do NOT import `internal/harness/antigravity` or any adapter package from `internal/core/consolidate` (importing `internal/harness` for `harness.Step` and `harness.StepKind` is allowed, matching `internal/core/episode`).
  - NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute `/usr/local/google/...` paths) in any code, comment, or commit message.
  - All git commits must use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.

## Tooling Constraints
- Use standard `read_file`, `write_file`, `edit_file`, and `run_command` tools.
- Run `go test -race ./internal/core/consolidate/...` and `go vet ./...` to verify.

## Line-Item Instructions

- [ ] **1. `internal/core/consolidate/filter.go` — Helper Utilities for Entry Text, Anchors & Build/Test Steps**
  - Create package `consolidate` in `internal/core/consolidate/filter.go`.
  - Implement text and anchor normalization helpers (exported or package-shared for WO2–WO4):
    - `NormalizeTitle(s string) string`: lowercases, strips markdown heading prefixes (`###`, `##`, `#`), strips bracketed entry ID tags like `[inv-01J...]`, replaces punctuation (`-`, `_`, `:`, `.`, `,`, `/`, `` ` ``, `'`, `"`, `(`, `)`) with spaces, and collapses whitespace into single spaces.
    - `ExtractEntryTitle(entryText string) string`: extracts the title from the first line of `entry.Text` (stripping `### [<id>]` or `[<id>]` prefix if present; if single-line `Why:`/`Do:`, takes the first non-empty line before `Why:`/`Do:`).
    - `ExtractEntryDo(entryText string) string`: extracts the `Do:` line content from `entry.Text` if present.
    - `StripAnchorSymbol(anchor string) string`: strips `file:` prefix and `#symbol` or `:line` / `:symbol` suffix, normalizes slashes (`\ -> /`), and strips leading `./` or `/`.
    - `AnchorsOverlap(a, b []string) bool`: returns `true` if `a` and `b` share at least one normalized file path (`StripAnchorSymbol`) or raw symbol anchor, or if one anchor path is a directory prefix of the other. Also returns `true` if both `len(a) == 0 && len(b) == 0`.
    - `FlattenActiveEntries(dossiersMap map[string]*dossiers.Dossier) []dossiers.Entry`: collects all non-retired, non-superseded entries (`!e.Retired && e.SupersededBy == ""`) across all dossiers in deterministic sorted order (by scope, then entry ID).

- [ ] **2. `internal/core/consolidate/filter.go` — P13 Echo Filter (`FilterEchoes`)**
  - Define `EchoFilterResult`:
    ```go
    type EchoFilterResult struct {
        Kept                 []inbox.NoteItem
        Suppressed           []inbox.NoteItem
        Reconfirmed          []inbox.NoteItem
        EchoSuppressionCount int
    }
    ```
  - Implement `IsUnknownExposureNote(note *inbox.Note) bool`:
    - Per Addendum A §11.2, returns `true` when `note != nil && len(note.ExposureList) == 0` AND (`strings.HasPrefix(strings.ToLower(string(note.Source)), "capture:")` || `strings.EqualFold(string(note.Source), "external")` || `strings.EqualFold(string(note.Source), "unknown")` || `strings.Contains(strings.ToLower(note.DedupKey), "exposure:unknown")`).
  - Implement `FilterEchoes(note *inbox.Note, activeEntries []dossiers.Entry, ep *episode.Episode) EchoFilterResult`:
    - If `note == nil || len(note.Items) == 0`, return an empty `EchoFilterResult{}`.
    - Build `entryByID := make(map[string]dossiers.Entry)` for all active entries (`!e.Retired && e.SupersededBy == ""`).
    - Build the set of exposed entries for `note`:
      - `exposedIDs := make(map[string]bool)` and `exposedEntries := make([]dossiers.Entry, 0)`
      - For each `rec` in `note.ExposureList`:
        - `id := strings.TrimSpace(rec.EntryID)`
        - If `id != ""`: mark `exposedIDs[id] = true`; if `e, ok := entryByID[id]; ok`, append `e` to `exposedEntries`.
      - If `IsUnknownExposureNote(note)` is true: treat **all** active entries in `activeEntries` as exposed (`exposedEntries = activeEntries`, and mark all their `ID`s in `exposedIDs`).
    - For each `item` in `note.Items`:
      - Check whether `item` echoes an exposed entry (`matchedEntry *dossiers.Entry`, `isEcho bool`):
        1. **Explicit EntryID match:** If any `expID` in `exposedIDs` appears (case-insensitive word/token/substring match) in `item.Title`, `item.Evidence`, `item.Why`, or `item.Do`:
           - `isEcho = true`
           - If `e, ok := entryByID[expID]; ok`, `matchedEntry = &e`; else `matchedEntry = &dossiers.Entry{ID: expID, Anchors: item.Anchors}`.
        2. **Semantic Title/Do + Anchor overlap match:** Otherwise, loop over `e` in `exposedEntries`:
           - Let `normItemTitle := NormalizeTitle(item.Title)` and `normEntryTitle := NormalizeTitle(ExtractEntryTitle(e.Text))` (and `normEntryText := NormalizeTitle(e.Text)`).
           - Let `normItemDo := NormalizeTitle(item.Do)` and `normEntryDo := NormalizeTitle(ExtractEntryDo(e.Text))`.
           - Title/Do matches if:
             - (`normItemTitle != ""` && (`normItemTitle == normEntryTitle` || (len(normItemTitle) >= 12 && strings.Contains(normEntryText, normItemTitle)) || (len(normEntryTitle) >= 12 && strings.Contains(normItemTitle, normEntryTitle))))
             - OR (`normItemDo != ""` && `normEntryDo != ""` && `normItemDo == normEntryDo`).
           - If Title/Do matches AND `AnchorsOverlap(item.Anchors, e.Anchors)`:
             - `isEcho = true`, `matchedEntry = &e`, break.
      - If `!isEcho`:
        - Append `item` to `res.Kept`.
      - If `isEcho`:
        - Determine candidate anchors to check: combine `matchedEntry.Anchors` and `item.Anchors` (deduplicated).
        - Check `IsEchoReconfirmed(ep, combinedAnchors)`:
          - Returns `true` if and only if `ep != nil`, `len(combinedAnchors) > 0`, and `HasPassingBuildOrTestAfterEdit(ep, combinedAnchors)` is `true`!
        - If `IsEchoReconfirmed` is `true`:
          - If `!IsVerifiedStamp(item.Verified)`:
            - Set `item.Verified = formatVerifiedStamp(note, ep, "reconfirmed")`.
          - Append `item` to `res.Kept` and `res.Reconfirmed`.
        - Else (`!IsEchoReconfirmed`):
          - Append `item` to `res.Suppressed` and increment `res.EchoSuppressionCount`.
  - Implement `HasPassingBuildOrTestAfterEdit(ep *episode.Episode, anchors []string) bool`:
    - If `ep == nil || len(ep.Steps) == 0 || len(anchors) == 0`, return `false`.
    - Normalize `anchors` via `StripAnchorSymbol` into non-empty file/dir paths. If none are file/dir paths, return `false`.
    - Find the first step in `ep.Steps` (at slice index `editPos`, step index `editStepIdx`) where `IsEditToAnchoredFile(step, anchorPaths)` is `true`:
      - `step.Kind == harness.StepKindEdit` (or `step.Action == "edit"` / `"write"`) AND `step.File` (normalized via `StripAnchorSymbol`, or if `step.File` is an absolute path ending with `"/" + anchorPath`) matches an `anchorPath` (exact match or `strings.HasPrefix(stepFile, anchorPath+"/")`).
    - If no such edit step exists, return `false`.
    - Scan subsequent steps (`pos > editPos` or `step.Index > editStepIdx`) for any step where `IsPassingBuildOrTestStep(step)` is `true`:
      - Case 1: `step.Kind == harness.StepKindTest` and `(step.ExitCode != nil && *step.ExitCode == 0) || (step.ExitCode == nil && step.Failed == 0 && (step.Passed > 0 || isPassOutcome(step.Outcome)))`.
      - Case 2: `step.Kind == harness.StepKindCommand` and `IsBuildOrTestCommand(step.Command)` is `true` and `(step.ExitCode != nil && *step.ExitCode == 0 || (step.ExitCode == nil && isPassOutcome(step.Outcome)))`.
    - Return `true` if found, `false` otherwise.
  - Implement `IsBuildOrTestCommand(cmd string) bool`:
    - Checks case-insensitively whether the command invokes a build, test, lint, or verification tool (e.g., `go test`, `go build`, `go vet`, `pytest`, `cargo test`, `cargo build`, `cargo check`, `npm test`, `npm run build`, `pnpm test`, `yarn test`, `make`, `cmake`, ` ninja`, `idf.py build`, `idf.py test`, `pio run`, `pio test`, `gradle`, `mvn test`, `ctest`, `jest`, `vitest`).

- [ ] **3. `internal/core/consolidate/filter.go` — Pre-Consolidation Corroboration Gate (`EnforceCorroborationGate` — `VD18`)**
  - Define `CorroborationResult`:
    ```go
    type CorroborationResult struct {
        Items                       []inbox.NoteItem
        Downgraded                  []inbox.NoteItem
        CorroborationDowngradeCount int
    }
    ```
  - Implement `RequiresCorroboration(item inbox.NoteItem) bool`:
    - Returns `true` if `item.Type` is one of:
      - `dossiers.EntryTypeInvariant` (`invariant`)
      - `dossiers.EntryTypeContract` (`contract`)
      - `dossiers.EntryTypeConvention` (`convention`)
      - `dossiers.EntryTypeRecipe` (`recipe`)
      - `dossiers.EntryTypeProcedure` (`procedure`)
    - Or if the item requests an edit gate (`strings.Contains(strings.ToLower(item.Do), "gate: edit")` || `strings.Contains(strings.ToLower(item.Why), "gate: edit")` || `strings.Contains(strings.ToLower(item.Verified), "gate: edit")`).
  - Implement `IsVerifiedStamp(v string) bool`:
    - Let `s := strings.TrimSpace(strings.ToLower(v))`.
    - If `s == ""` || `s == "null"` || `s == "nil"` || `s == "unverified"` || `s == "none"` || `s == "false"` || `s == "no"` || `s == "pending"` || `s == "hypothesis"`, return `false`.
    - Return `true`.
  - Implement `EnforceCorroborationGate(note *inbox.Note, items []inbox.NoteItem, ep *episode.Episode, repoRoot string) CorroborationResult`:
    - For each `item` in `items`:
      - If `!RequiresCorroboration(item)`:
        - Append `item` unchanged to `res.Items` and continue.
      - If `note != nil && (note.Source == inbox.SourceDocSeed || note.Source == inbox.SourcePA)`:
        - If `!IsVerifiedStamp(item.Verified)`:
          - Set `item.Verified = formatVerifiedStamp(note, ep, string(note.Source))`
        - Append `item` to `res.Items` and continue.
      - If `IsVerifiedStamp(item.Verified)`:
        - Append `item` unchanged to `res.Items` and continue.
      - Otherwise (`verified` is empty / `"null"` / `"unverified"`), evaluate the 3 `VD18` corroboration checks:
        1. **Check 1 — Passing build/test step after edit in `ep` (or passing command for anchorless recipe):**
           - `corroboratedByTest := HasPassingBuildOrTestAfterEdit(ep, item.Anchors)`
           - If `!corroboratedByTest && (item.Type == dossiers.EntryTypeRecipe || item.Type == dossiers.EntryTypeProcedure) && len(item.Commands) > 0`:
             - Check if `ep` has a passing command step matching any command prefix in `item.Commands`.
           - If `corroboratedByTest`:
             - Set `item.Verified = formatVerifiedStamp(note, ep, "test-pass")`
             - Append `item` to `res.Items` and continue.
        2. **Check 2 — Verified static code anchor present in `HEAD` / `repoRoot`:**
           - Check `HasVerifiedStaticAnchor(note, item, repoRoot)`:
             - Inspect `item.AnchorChecks`, then `note.AnchorChecks` (matching `item.Anchors`), and if `repoRoot != ""` and no positive check is recorded yet, run `inbox.CheckAnchors(repoRoot, item.Anchors)`.
             - If an anchor in `item.Anchors` has `Exists == true`:
               - If the anchor also specifies a `#symbol` or `:symbol` (non-numeric suffix) and `repoRoot != ""`, read the file at `filepath.Join(repoRoot, StripAnchorSymbol(anchor))` and verify `strings.Contains(fileContent, symbol)` (if the file is a regular file on disk).
               - Return `true`.
           - If `HasVerifiedStaticAnchor(note, item, repoRoot)` is `true`:
             - Set `item.Verified = formatVerifiedStamp(note, ep, "anchor-verified")`
             - Append `item` to `res.Items` and continue.
        3. **Check 3 — Explicit human `USER_INPUT` confirmation in `ep`:**
           - Check `HasExplicitUserConfirmation(ep, item)`:
             - Requires `ep != nil` and a step in `ep.Steps` with `step.Kind == harness.StepKindUser`, ` !strings.EqualFold(step.Author, "SYSTEM_SDK")`, and `!strings.HasPrefix(strings.TrimSpace(step.Text), "[repo-memory")` where:
               - Either `step.Index` is cited in `item.StepCitations`, OR
               - `step.Text` contains a confirmation phrase (`"confirm"`, `"approved"`, `"yes"`, `"correct"`, `"must"`, `"always"`, `"never"`, `"invariant"`, `"rule"`, `"contract"`) AND shares at least one significant token ($\ge 4$ chars) or anchor path with `item.Title` / `item.Do` / `item.Anchors`.
           - If `HasExplicitUserConfirmation(ep, item)` is `true`:
             - Set `item.Verified = formatVerifiedStamp(note, ep, "user-confirmed")`
             - Append `item` to `res.Items` and continue.
      - **Downgrade uncorroborated rule to `open-issue`:**
        - Set `item.Type = dossiers.EntryTypeOpenIssue`.
        - Set `item.Verified = "unverified"`.
        - If `strings.TrimSpace(item.ClosesWhen) == ""`:
          - If `len(item.Anchors) > 0 && StripAnchorSymbol(item.Anchors[0]) != ""`:
            - `item.ClosesWhen = "path_exists:" + StripAnchorSymbol(item.Anchors[0])`
          - Else if `len(item.Commands) > 0 && strings.TrimSpace(item.Commands[0]) != ""`:
            - `item.ClosesWhen = "command_passes:" + strings.TrimSpace(item.Commands[0])`
          - Else:
            - `item.ClosesWhen = "outcome:build_or_test_pass"`
        - Append `item` to `res.Items` and `res.Downgraded`, and increment `res.CorroborationDowngradeCount`.

- [ ] **4. `internal/core/consolidate/filter_test.go` — Comprehensive Unit Tests**
  - Create `internal/core/consolidate/filter_test.go` covering:
    1. `TestFilterEchoes_DropsUneditedEcho`: an item repeating an entry in `note.ExposureList` (by ID or by matching title + overlapping anchor) when the episode did NOT edit the anchored file is dropped (`EchoSuppressionCount == 1`, `len(Kept) == 0`).
    2. `TestFilterEchoes_DropsEditedEchoWithoutPassingTest`: an item repeating an exposed entry where the episode edited the anchored file (`StepKindEdit`) but the subsequent test failed (`exit != 0` or `Failed > 0`) or never ran is dropped (`EchoSuppressionCount == 1`).
    3. `TestFilterEchoes_RetainsReconfirmedEchoAfterEditAndPassingTest`: an item repeating an exposed entry where the episode edited an anchored file at step 5 and passed `go test ./...` (`StepKindTest` or `StepKindCommand` with `exit == 0`) at step 8 is retained in `Kept` and `Reconfirmed` (`EchoSuppressionCount == 0`, `Verified` populated).
    4. `TestFilterEchoes_UnknownExposureExternalCapture`: when `note.ExposureList` is empty and `note.Source` is `"capture:external"` (Addendum A §11.2), repeating an active entry without edit+test is suppressed as an echo.
    5. `TestEnforceCorroborationGate_DowngradesUncorroboratedRules`: uncorroborated `invariant`, `contract`, `convention`, `recipe`, and `gate: edit` items with `Verified: ""` or `"null"` (no passing test after edit, no existing `HEAD` anchor, no user confirmation) are downgraded to `dossiers.EntryTypeOpenIssue` with deterministic `ClosesWhen` (`path_exists:...` or `command_passes:...` or `outcome:build_or_test_pass`) and `CorroborationDowngradeCount` incremented. Non-rule types (`decision`, `ruled-out`, `gotcha`, `failure-signature`, `open-issue`) pass through without downgrade.
    6. `TestEnforceCorroborationGate_CorroboratedByTestAnchorOrUser`:
       - Subtest A: `invariant` with `Verified: "null"` corroborated by `StepKindEdit` + subsequent passing `StepKindTest` in `ep` remains `invariant` with non-empty `Verified`.
       - Subtest B: `contract` with `Verified: ""` corroborated by a verified static file/symbol anchor existing in `repoRoot` (`t.TempDir()`) remains `contract` with non-empty `Verified`.
       - Subtest C: `invariant` with `Verified: ""` corroborated by explicit `StepKindUser` confirmation (`Author: "USER_EXPLICIT"`) in `ep` remains `invariant` with non-empty `Verified`; verify that a hook-injected `SYSTEM_SDK` step (`[repo-memory ...]`) does NOT count as human confirmation!

## Verification Requirement
- Run `go test -v -race ./internal/core/consolidate/...` and `go vet ./...` and verify 100% pass.
- Run `go test ./internal/core/...` to ensure package boundary tests (`boundary_test.go`) pass.
- Commit your changes on `pr-13-v2-consolidation/wo1-echo-filter-and-corroboration-gate` using `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **When finished:** Send a message back to the Principal Architect confirming validation, or append your Completion Report to this `WORKORDER.md` and commit it once approved.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
