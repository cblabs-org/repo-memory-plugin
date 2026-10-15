---
name: pr-11-v2-doc-seeding-and-lifecycles-wo2
description: Enforce SPEC.md §5.4 and design-v2 §7.3 Optional Section Resolution ({section?}), render kind and source_doc metadata, and make handedit detection deterministic across v2.2 entry prefixes
complexity: standard
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md`. This `WORKORDER.md` belongs to parent project `pr-11-v2-doc-seeding-and-lifecycles`, whose 5-WO decomposition is explicitly approved by the human in `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md`. You are authorized to implement and commit directly on the current branch (`pr-11-v2-doc-seeding-and-lifecycles/wo2-optional-section-rendering`).
>
> **External GitHub Hygiene:** Never include any Google-internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or local `/usr/local/google/...` paths) in any code, comments, tests, or commit messages. Use git author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Objective
Update `internal/core/dossiers/renderer.go` and `internal/core/dossiers/handedit.go` to implement `SPEC.md` §5.4 and `docs/design-v2-distillation-retrieval.md` §7.3 Optional Section Resolution (`{section?}`), render `kind: <semantic|procedural|episodic>` and `source_doc: <path>` in the backtick metadata line, group `v2.2` entry types under their canonical headings, and ensure deterministic prefix inference in `DetectHandEdits`.

## Strict Boundary Box
- **Allowed to modify:**
  - `internal/core/dossiers/renderer.go`
  - `internal/core/dossiers/handedit.go`
  - `internal/core/dossiers/dossiers_test.go`
- **Do NOT touch:**
  - `internal/core/seed/` (WO3)
  - `internal/core/tick/` (WO4)
  - `.memory/` (WO5)

## Tooling Constraints
- Run tests with `go test -race ./internal/core/dossiers/...`.
- Commit with `git -c user.name="matthewmcneill" -c user.email="97467074+matthewmcneill@users.noreply.github.com" commit ...`.

## Line-Item Instructions

- [x] `internal/core/dossiers/renderer.go` — Update section groupings, `{section?}` resolution, and entry metadata rendering:
  1. Update the section `groups` in `renderDossierInternal`:
     - `{"Invariants & contracts", []EntryType{EntryTypeInvariant, EntryTypeContract, EntryTypeConvention}}`
     - `{"Ruled out, failure signatures and gotchas", []EntryType{EntryTypeRuledOut, EntryTypeFailureSignature, EntryTypeGotcha, EntryTypeExternalBehaviour}}`
     - `{"Decisions", []EntryType{EntryTypeDecision}}`
     - `{"Procedures / Recipes", []EntryType{EntryTypeProcedure, EntryTypeRecipe}}`
     - `{"Interfaces and dependencies", []EntryType{EntryTypeInterface}}`
     - `{"Open issues", []EntryType{EntryTypeOpenIssue}}`
  2. Enforce `{section?}` Optional Section Resolution: only emit `## <group.title>` when `len(groupEntries) > 0` (any section with 0 entries is omitted completely). Ensure blank line separation between rendered entries and sections is clean (use `strings.TrimSpace(b.String()) + "\n"` at the end).
  3. In `renderEntry(b *strings.Builder, e Entry)`:
     - Right after `meta = append(meta, string(e.Type))`, append `"kind: " + string(e.EffectiveKind())` (e.g., `` `invariant · kind: semantic · anchors: ...` ``).
     - After `anchors` (if `len(e.Anchors) > 0`), if `e.SourceDocPath != ""` append `"source_doc: " + e.SourceDocPath`.
     - Keep `verified` and `evidence` in `meta` as before.

- [x] `internal/core/dossiers/handedit.go` — Make prefix-to-EntryType inference deterministic and support all `v2.2` prefixes:
  1. Add a deterministic helper `InferEntryTypeFromID(id string) EntryType` using an ordered slice/switch (never iterating over `EntryTypePrefixes` map, which has non-deterministic iteration order when aliases share prefixes like `con-` or `got-`):
     - `"inv-"` -> `EntryTypeInvariant`
     - `"con-"` -> `EntryTypeContract`
     - `"rul-"` -> `EntryTypeRuledOut`
     - `"fail-"`, `"err-"` -> `EntryTypeFailureSignature`
     - `"got-"` -> `EntryTypeGotcha`
     - `"dec-"` -> `EntryTypeDecision`
     - `"rec-"` -> `EntryTypeRecipe`
     - `"proc-"` -> `EntryTypeProcedure`
     - `"iface-"` -> `EntryTypeInterface`
     - `"issue-"`, `"iss-"` -> `EntryTypeOpenIssue`
     - `"docseed-"` -> `EntryTypeInvariant` (default if not specified)
     - default -> `EntryTypeInvariant`
  2. Use `InferEntryTypeFromID(id)` in `DetectHandEdits`.
  3. Ensure `parseMarkdownEntries` cleanly skips the backtick metadata line (which always starts and ends with `` ` `` and contains `" · "`).

- [x] `internal/core/dossiers/dossiers_test.go` — Add comprehensive unit tests for WO2:
  1. Verify `RenderDossier` renders `## Invariants & contracts` for `invariant`, `contract`, and `convention` entries, and `## Procedures / Recipes` for `procedure` and `recipe` entries, while omitting any empty `{section?}` headings.
  2. Verify `RenderDossier` includes `kind: semantic`, `kind: procedural`, and `kind: episodic` in the backtick metadata line of rendered entries, and includes `source_doc: docs/guide.md` when `SourceDocPath` is set.
  3. Verify `DetectHandEdits` round-trips cleanly (`0` ops when unmodified, `amend` op when body text is edited, and deterministic `add` op with the expected `entry_type` for `con-`, `rec-`, `err-`, `iss-`, `got-` prefixes).

## Verification Requirement
Run:
```bash
go test -race ./internal/core/dossiers/...
```
All tests must pass with zero race warnings. Commit your changes to `pr-11-v2-doc-seeding-and-lifecycles/wo2-optional-section-rendering`, then **STOP and send a message back to the Principal Architect** confirming validation. Once approved, append your Completion Report below and commit it.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---
## Completion Report
- **Status:** Complete
- **Implementation Commit:** `16d0be3`
- **Summary:**
  - Updated `internal/core/dossiers/renderer.go` to group `invariant`/`contract`/`convention` under `## Invariants & contracts`, `ruled-out`/`failure-signature`/`gotcha`/`external-behaviour` under `## Ruled out, failure signatures and gotchas`, and `procedure`/`recipe` under `## Procedures / Recipes`, omitting empty `{section?}` headings and rendering `kind: <semantic|procedural|episodic>` and `source_doc: <path>` in entry metadata lines.
  - Added deterministic `InferEntryTypeFromID(id string) EntryType` in `internal/core/dossiers/handedit.go` and wired it into `DetectHandEdits`.
  - Added unit tests `TestRenderDossier_OptionalSectionsAndMetadata` and `TestDetectHandEdits_RoundTripAndDeterministicPrefixes` in `internal/core/dossiers/dossiers_test.go` and verified `go test -race ./internal/core/dossiers/...` passes with 0 race warnings.
- **Out-of-Boundary Issues Discovered:** None.
