---
name: pr-13-v2-consolidation-wo2
description: Handoff instructions for Work Order 2 (Five-Tier Scope Classification, Dossier Routing, Exact-Duplicate Merging & Broken-Anchor Detection)
complexity: standard
---

# pr-13-v2-consolidation / WO2 — Five-Tier Scope Classification, Dossier Routing, Exact-Duplicate Merging & Broken-Anchor Detection

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not create branches other than the active branch (`pr-13-v2-consolidation/wo2-routing-dedup-and-anchor-checks`), and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md` and `.agents/skills/principal-architect-workspace/pr-13-v2-consolidation/PROJECT.md` (which records human approval of this 6-WO decomposition). You are authorized to implement and commit this Work Order autonomously within the Strict Boundary Box below.

## Objective
Create `internal/core/consolidate/route.go` and `internal/core/consolidate/route_test.go` implementing Stage 2 Steps 2–3: **Five-Tier State Scope Classification (`ScopeTier`, `ClassifyTier` — `VD17`, `SPEC.md` §5.5, `design-v2` §6 step 7)**, **Deterministic Dossier Routing (`RouteItem` — `SPEC.md` §6.5 step 2, `design-v2` §6 step 2)**, **Deterministic Exact-Duplicate Merging (`MergeExactDuplicates` — `SPEC.md` §6.5 step 3, `design-v2` §6 step 3)**, and **Existing Dossier Anchor Verification (`CheckDossierAnchors` — `SPEC.md` §6.5 step 3, `design-v2` §6 step 4)**.

## Strict Boundary Box
- **Allowed to create/modify:**
  - `internal/core/consolidate/route.go`
  - `internal/core/consolidate/route_test.go`
  - `.agents/skills/principal-architect-workspace/pr-13-v2-consolidation/wo2-routing-dedup-and-anchor-checks/WORKORDER.md` (Completion Report)
- **Forbidden:**
  - Do NOT modify `filter.go` unless adding a tiny helper needed by `route.go`.
  - Do NOT create `llm.go`, `record.go`, `publish.go`, or `tick/` files — those belong to WO3–WO6.
  - Do NOT import `internal/harness/antigravity` or any adapter package from `internal/core/consolidate`.
  - NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute `/usr/local/google/...` paths) in any code, comment, or commit message.
  - All git commits must use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.

## Tooling Constraints
- Use standard `read_file`, `write_file`, `edit_file`, and `run_command` tools.
- Run `go test -v -race ./internal/core/consolidate/...` and `go vet ./...` to verify.

## Line-Item Instructions

- [ ] **1. `internal/core/consolidate/route.go` — Five-Tier State Scope Classification (`ScopeTier`, `ClassifyTier`)**
  - Define `ScopeTier`:
    ```go
    type ScopeTier string

    const (
        TierTemp    ScopeTier = "temp"
        TierSession ScopeTier = "session"
        TierUser    ScopeTier = "user"
        TierBranch  ScopeTier = "branch"
        TierRepo    ScopeTier = "repo"
    )
    ```
  - Implement `StripTierPrefix(rawScope string) (ScopeTier, string)`:
    - Inspects `strings.TrimSpace(rawScope)` case-insensitively for `temp:`, `session:`, `user:`, `branch:`, or `repo:` prefixes.
    - If found, returns `(tier, strings.TrimSpace(rest))`.
    - Otherwise returns `("", strings.TrimSpace(rawScope))`.
  - Implement `IsDefaultBranch(branch string) bool`:
    - Returns `true` if `branch == ""` or `strings.EqualFold(branch, "main")` or `strings.EqualFold(branch, "master")` or `strings.EqualFold(branch, "HEAD")`.
  - Implement `ClassifyTier(note *inbox.Note, item inbox.NoteItem, repoRoot string, isBranchMerged func(branch string) bool) ScopeTier`:
    1. **Explicit prefix on `item.Scope`:**
       - Call `prefixTier, _ := StripTierPrefix(item.Scope)`.
       - If `prefixTier == TierUser || prefixTier == TierBranch || prefixTier == TierRepo || prefixTier == TierSession || prefixTier == TierTemp`, return `prefixTier`.
    2. **Personal/Workstation heuristic (`TierUser`):**
       - If `strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Scope)), "user-dossiers")` or `strings.EqualFold(strings.TrimSpace(item.Scope), "user")`, return `TierUser`.
    3. **Unmerged Branch heuristic (`TierBranch`):**
       - When `note != nil && !IsDefaultBranch(note.Branch)`:
         - Check if the branch is already merged into the default branch (`isBranchMerged != nil && isBranchMerged(note.Branch)`).
         - If NOT merged:
           - Check whether `item` has file anchors (`StripAnchorSymbol(a) != ""`) that existed on the feature branch at harvest time (`item.AnchorChecks` or `note.AnchorChecks` has `Exists == true`) OR currently do NOT exist in `repoRoot` (`repoRoot != ""` and `inbox.CheckAnchors(repoRoot, item.Anchors)` shows **all** file anchors have `Exists == false` on the default branch checkout).
           - Specifically: if `len(fileAnchors) > 0` and all of `item`'s file anchors are missing in `repoRoot` (when `repoRoot != ""` represents the default branch / current checkout) OR if `repoRoot == ""` and `!IsDefaultBranch(note.Branch)` with branch-only anchor checks (`Exists == false`), classify as `TierBranch`.
    4. **Default (`TierRepo`):**
       - Return `TierRepo`.

- [ ] **2. `internal/core/consolidate/route.go` — Deterministic Dossier Routing (`RoutedItem`, `RouteItem`, `RouteItems`)**
  - Define `RoutedItem`:
    ```go
    type RoutedItem struct {
        NoteID         string               `json:"note_id"`
        NoteSource     inbox.NoteSource     `json:"note_source"`
        ConversationID string               `json:"conversation_id,omitempty"`
        Branch         string               `json:"branch,omitempty"`
        HeadCommit     string               `json:"head_commit,omitempty"`
        CreatedAt      time.Time            `json:"created_at"`
        Tier           ScopeTier            `json:"tier"`
        DossierScope   string               `json:"dossier_scope"`
        DossierKind    dossiers.DossierKind `json:"dossier_kind"`
        DerivedNew     bool                 `json:"derived_new,omitempty"`
        Item           inbox.NoteItem       `json:"item"`
    }
    ```
  - Implement `RouteItem(note *inbox.Note, item inbox.NoteItem, resolver *config.ScopeResolver, repoRoot string, isBranchMerged func(branch string) bool) RoutedItem`:
    1. Compute `tier := ClassifyTier(note, item, repoRoot, isBranchMerged)`.
    2. Strip any tier prefix from `item.Scope` (`_, explicitScope := StripTierPrefix(item.Scope)`). Normalize slashes and trim leading/trailing `/`.
    3. Resolve `dossierScope` and `dossierKind` and `derivedNew`:
       - **Case A — Explicit scope on `item.Scope` (`explicitScope != ""`):**
         - Resolve aliases if `resolver != nil` (`resolver.ResolveScope([]string{explicitScope}, []string{explicitScope})` if it matches an alias, or `explicitScope` directly; also expose or check `resolver.ScopeKind(dossierScope)`).
         - If `dossierKind == ""`, infer from scope prefix (`topics/` -> `DossierKindTopic`, `external/` or `externals/` -> `DossierKindExternal`, `procedures/` -> `DossierKindProcedure`, else `DossierKindModule`).
       - **Case B — Anchored item (`len(item.Anchors) > 0`):**
         - Extract keywords from `item.Title`, `item.Why`, `item.Do`, and `item.Commands`.
         - If `resolver != nil`: call `matched := resolver.ResolveScope(item.Anchors, nil)` (anchor-first matching).
         - If `matched != ""`:
           - `dossierScope = matched`
           - `dossierKind = resolver.ScopeKind(matched)` (default `dossiers.DossierKindModule`).
         - Else (no existing/declared module scope matched the anchors):
           - Call `derived := config.DeriveModuleScopes(item.Anchors)`.
           - If `len(derived) > 0`:
             - `dossierScope = derived[0].Name`
             - `dossierKind = dossiers.DossierKindModule`
             - `derivedNew = true`
       - **Case C — Unanchored item (or still empty `dossierScope`):**
         - Extract keywords from `item.Title`, `item.Why`, `item.Do`, and `item.Commands`.
         - If `resolver != nil`: call `matched := resolver.ResolveScope(nil, keywords)`.
         - If `matched != ""`:
           - `dossierScope = matched`
           - `dossierKind = resolver.ScopeKind(matched)`
         - Else:
           - If `item.Type == dossiers.EntryTypeRecipe || item.Type == dossiers.EntryTypeProcedure`:
             - `dossierScope = "procedures/general"`, `dossierKind = dossiers.DossierKindProcedure`
           - Else if `item.Type == dossiers.EntryTypeExternalBehaviour || item.Type == dossiers.EntryTypeGotcha`:
             - `dossierScope = "external/general"`, `dossierKind = dossiers.DossierKindExternal`
           - Else:
             - `dossierScope = "general"`, `dossierKind = dossiers.DossierKindTopic`
    4. Populate `RoutedItem` with `NoteID`, `NoteSource`, `ConversationID`, `Branch`, `HeadCommit`, `CreatedAt` from `note` (if `note != nil`), `Tier: tier`, `DossierScope: dossierScope`, `DossierKind: dossierKind`, `DerivedNew: derivedNew`, and `Item: item` (with `item.Scope = dossierScope`).
  - Implement `RouteItems(note *inbox.Note, items []inbox.NoteItem, resolver *config.ScopeResolver, repoRoot string, isBranchMerged func(branch string) bool) []RoutedItem`.

- [ ] **3. `internal/core/consolidate/route.go` — Deterministic Exact-Duplicate Merging (`MergeExactDuplicates`)**
  - Define `DedupResult`:
    ```go
    type DedupResult struct {
        Remaining []RoutedItem
        AmendOps  []ops.Op
    }
    ```
  - Implement `CanonicalEntryTypeGroup(t dossiers.EntryType) dossiers.EntryType`:
    - Maps alias-equivalent types to their canonical counterpart so exact duplicates across aliases merge cleanly:
      - `EntryTypeConvention` -> `EntryTypeContract`
      - `EntryTypeProcedure` -> `EntryTypeRecipe`
      - `EntryTypeExternalBehaviour` -> `EntryTypeGotcha`
      - All other types return `t`.
  - Implement `MergeExactDuplicates(routed []RoutedItem, dossiersMap map[string]*dossiers.Dossier, now time.Time) DedupResult`:
    - If `now.IsZero()`, set `now = time.Now().UTC()`.
    - First, deduplicate candidate items against **existing active entries** in `dossiersMap`:
      - For each `ri` in `routed`:
        - Look up target dossier `d := dossiersMap[ri.DossierScope]` (only for `ri.Tier == TierRepo` or matching dossier map).
        - Check if any active entry `e` (`!e.Retired && e.SupersededBy == ""`) in `d` is an exact duplicate of `ri.Item`:
          - `CanonicalEntryTypeGroup(ri.Item.Type) == CanonicalEntryTypeGroup(e.Type)`
          - `NormalizeTitle(ri.Item.Title) != ""` && `NormalizeTitle(ri.Item.Title) == NormalizeTitle(ExtractEntryTitle(e.Text))`
          - `AnchorsOverlap(ri.Item.Anchors, e.Anchors)` is `true`.
        - If a matching existing entry `e` is found:
          - Build the merged, deduplicated evidence slice starting from `e.Evidence` and appending `ri.NoteID` (if non-empty) and `ri.Item.Evidence` (if non-empty and not equal to `ri.NoteID`).
          - Also merge any new anchors from `ri.Item.Anchors` into `e.Anchors` (deduplicated).
          - Note: if multiple candidate items in `routed` match the same `e.ID`, coalesce them into a single `ops.OpAmend` for `e.ID` (or update the in-flight `OpAmend` for `e.ID`)!
          - Construct `ops.Op`:
            ```go
            ops.Op{
                Type:         ops.OpAmend,
                EntryID:      e.ID,
                DossierScope: ri.DossierScope,
                Timestamp:    now.UTC(),
                Source:       ops.SourceConsolidation,
                SessionID:    ri.ConversationID,
                Fields: map[string]any{
                    "evidence": mergedEvidence,
                    "anchors":  mergedAnchors,
                },
            }
            ```
            (If `!IsVerifiedStamp(e.Verified) && IsVerifiedStamp(ri.Item.Verified)`, also set `"verified": ri.Item.Verified` in `Fields`.)
    - Second, among candidate items that did NOT match an existing dossier entry, merge exact duplicates **within the candidate batch itself**:
      - If two `RoutedItem`s in the same `(Tier, DossierScope)` have the same `CanonicalEntryTypeGroup`, the same non-empty `NormalizeTitle(item.Title)`, and `AnchorsOverlap(a.Item.Anchors, b.Item.Anchors)`:
        - Keep the first `RoutedItem` and merge the second item's `NoteID` / `Evidence` (comma-separated in `Item.Evidence`), `Anchors`, and `Verified` stamp into the first `RoutedItem`.
    - Return `DedupResult{Remaining, AmendOps}`.

- [ ] **4. `internal/core/consolidate/route.go` — Check Existing Dossier Anchors (`CheckDossierAnchors`)**
  - Define `BrokenAnchorReport`:
    ```go
    type BrokenAnchorReport struct {
        EntryID        string   `json:"entry_id"`
        DossierScope   string   `json:"dossier_scope"`
        EntryType      dossiers.EntryType `json:"entry_type"`
        Title          string   `json:"title"`
        MissingAnchors []string `json:"missing_anchors"`
        ValidAnchors   []string `json:"valid_anchors"`
        AllMissing     bool     `json:"all_missing"`
    }
    ```
  - Implement `CheckDossierAnchors(repoRoot string, d *dossiers.Dossier) []BrokenAnchorReport`:
    - If `repoRoot == ""` or `d == nil`, return `nil`.
    - For each active entry `e` in `d.Entries` (`!e.Retired && e.SupersededBy == ""`):
      - If `len(e.Anchors) == 0`, continue.
      - Check each anchor in `e.Anchors`:
        - If `StripAnchorSymbol(anchor) == ""` (a pure symbol anchor without a file path), treat it as valid (`validAnchors`).
        - Otherwise run `inbox.CheckAnchors(repoRoot, []string{anchor})` (and if the anchor has a non-numeric `#symbol` or `:symbol` suffix and the file exists, also verify the symbol exists in the file, matching `HasVerifiedStaticAnchor`).
        - Partition `e.Anchors` into `MissingAnchors` and `ValidAnchors`.
      - If `len(MissingAnchors) > 0`:
        - Append `BrokenAnchorReport{EntryID: e.ID, DossierScope: d.Scope, EntryType: e.Type, Title: ExtractEntryTitle(e.Text), MissingAnchors: missing, ValidAnchors: valid, AllMissing: len(valid) == 0}`.

- [ ] **5. `internal/core/consolidate/route_test.go` — Unit Tests**
  - Create `internal/core/consolidate/route_test.go` testing:
    1. `TestClassifyTier`:
       - Explicit `user:workflow`, `branch:core/ops`, `repo:core/ops` prefixes;
       - Unmerged feature branch `feat/new-pkg` where anchored file `internal/newpkg/foo.go` does not exist in `repoRoot` (`main`) -> `TierBranch`;
       - Merged feature branch (`isBranchMerged` returns `true`) or existing anchor on `main` -> `TierRepo`.
    2. `TestRouteItem`:
       - Explicit scope with alias resolution (`old/scope` -> `new/scope`);
       - Matched existing module scope in `ScopeResolver`;
       - Unmatched file anchor rolled up via `config.DeriveModuleScopes` (`DerivedNew == true`);
       - Unanchored item routed by keywords to declared `topic`, `external`, and `procedure` scopes in `ScopeConfig`, plus fallback when no keywords match.
    3. `TestMergeExactDuplicates`:
       - Candidate item matching an existing entry (including alias-equivalent type `convention` matching `contract`) generates an `ops.OpAmend` that preserves existing `e.Evidence` and appends the new `note.ID`, removing the duplicate from `Remaining`;
       - Multiple duplicate candidates in the same batch matching the same existing entry coalesce into one `OpAmend`;
       - Two identical new candidates in the same batch (not in `dossiersMap`) coalesce into a single `RoutedItem` in `Remaining`.
    4. `TestCheckDossierAnchors`:
       - Detects entries with partially missing anchors (`AllMissing == false`) and 100% missing anchors (`AllMissing == true`), and ignores entries whose anchors all exist in `repoRoot`.

## Verification Requirement
- Run `go test -v -race ./internal/core/consolidate/...` and `go vet ./...` and verify 100% pass.
- Commit your changes on `pr-13-v2-consolidation/wo2-routing-dedup-and-anchor-checks` using `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **When finished:** Send a message back to the Principal Architect confirming validation, or append your Completion Report to this `WORKORDER.md` and commit it once approved.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
