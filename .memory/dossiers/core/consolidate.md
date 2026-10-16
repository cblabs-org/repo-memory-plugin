# core/consolidate

## Ruled out, failure signatures and gotchas

### [seed-782f6c28] Ensure `SYSTEM_SDK` or `[repo-memory ...]` prefixed `StepKindUser` steps are always excluded in `HasExplicitUserConfirmation`, as Antigravity `PreInvocation` `userMessage` pushes are logged as `USER_INPUT` with `source: SYSTEM_SDK`.
`ruled-out · kind: episodic · anchors: internal/core/consolidate/filter.go, internal/core/consolidate/filter_test.go`

### [seed-bbe3d661] In `extractRoutingKeywords`, preserve full multi-word command strings (e.g. `"git tag"`, `"make release"`) before splitting into tokens so multi-word `DeclaredScope.Commands` match with full weight (`+20`).
`ruled-out · kind: episodic · anchors: internal/core/consolidate/route.go, internal/core/consolidate/route_test.go`

## Decisions

### [seed-1ac2897d] In `HasVerifiedStaticAnchor`, non-numeric `#symbol` or `:symbol` suffixes are verified against the file contents on disk when `repoRoot` is provided, preventing hallucinated symbols on real files from passing static anchor corroboration.
`decision · kind: episodic · anchors: internal/core/consolidate/filter.go, internal/core/consolidate/filter_test.go`

### [seed-1b7ecca5] Exported `NormalizeTitle`, `ExtractEntryTitle`, `ExtractEntryDo`, `StripAnchorSymbol`, `AnchorsOverlap`, and `FlattenActiveEntries` so downstream consolidation stages (`WO2` routing/exact-duplicate merge and `WO4` op validation) share identical normalization semantics.
`decision · kind: episodic · anchors: internal/core/consolidate/filter.go, internal/core/consolidate/filter_test.go`

### [seed-2ca4cd40] When `MergeExactDuplicates` generates an `ops.OpAmend` for an existing entry, `op.Fields["evidence"]` and `op.Fields["anchors"]` are populated with the merged deduplicated slices of the existing entry's values plus the new candidate's values, matching `dossiers.Projector`'s replacement semantics on `applyAmend`.
`decision · kind: episodic · anchors: internal/core/consolidate/route.go, internal/core/consolidate/route_test.go`

### [seed-734cff19] Invoked `scrub.Scrub("", opScrubCtx)` once upfront on every op so malformed Layer 3 `.memory/scrub.yaml` configs fail closed even on ops without optional text fields.
`decision · kind: episodic · anchors: internal/core/consolidate/record.go, internal/core/consolidate/record_test.go`

### [seed-7e3d56c1] Multiple duplicate candidates in the same batch targeting the same existing `EntryID` are coalesced into a single `OpAmend` per `EntryID`.
`decision · kind: episodic · anchors: internal/core/consolidate/route.go, internal/core/consolidate/route_test.go`
