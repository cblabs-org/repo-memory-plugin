# PR-02a: Transcript → Episode

> **Programme:** repo-memory
> **PR:** pr-02a-transcript-episode
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Turn an Antigravity conversation's transcript file set into a compacted, scrubbed, attributed episode. This is pure transformation, testable entirely on fixtures before any runtime capture exists.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-transcript-parser` | `pr-02a-transcript-episode/wo1-transcript-parser` | ☑ Complete | architect | a28bf92a |
| 2 | `wo2-compaction` | `pr-02a-transcript-episode/wo2-compaction` | ☑ Complete | architect | 2d445f95 |
| 3 | `wo3-episode-store` | `pr-02a-transcript-episode/wo3-episode-store` | ☑ Complete | standard | 75b7ca8e |
| 4 | `wo4-commit-linkage` | `pr-02a-transcript-episode/wo4-commit-linkage` | ☑ Complete | standard | 1a09193a |
| 5 | `wo5-metrics-skeleton` | `pr-02a-transcript-episode/wo5-metrics-skeleton` | ☐ Pending | standard | — |

## Exit Criteria
- Fixture conversations produce episodes with no unresolved `truncated_fields`; the chunk-only fallback yields byte-identical results to the monolithic file
- Compaction ratio ≥ 4× measured on full transcripts; §4.4.1 sizes re-measured
- Zero secrets in a scan of 50 fixture episodes
- Core tests pass on the `fake` adapter; `internal/core` imports no concrete adapter

## Session Log

### WO1: wo1-transcript-parser
- **Outcome:** Complete.
- **Key Decisions:** Used `bufio.Scanner` with a generous 50MB buffer on top of an `io.MultiReader` to ensure lexical chunks straddling JSON boundaries can be parsed correctly.
- **Findings:** Found that `cat`'s EOF behavior on newlines broke chunking, which the worker mitigated with strict byte continuity via `printf`.

### WO2: wo2-compaction
- **Outcome:** Complete. Target >= 4x compaction achieved.
- **Key Decisions:** Collapsing duplicate identical log lines for device/test outputs ensures major compaction without losing structure. Bounded command excerpt truncation retains failing markers or the last 5-10 lines of output for non-zero exit codes.
- **Findings:** Stripped actual content text from exposure markers keeping only reference IDs. Secret scrubber replaces AWS keys, UUIDs, Tokens with `«secret:type»`.

### WO3: wo3-episode-store
- **Outcome:** Complete.
- **Key Decisions:** Used pure-Go zstd compression (`klauspost/compress/zstd`) and atomic writes (tmp + rename). Implemented `StepGap` for handling unwritten internal dispatch steps.
- **Findings:** Successfully added support for flexible timestamp parsing (RFC3339/RFC3339Nano, null).

### WO4: wo4-commit-linkage
- **Outcome:** Complete.
- **Key Decisions:** Implemented `CommitLinker` with pluggable `GitRunner` interface (defaulting to system git execution via `ExecGitRunner`). Resolves edit steps (`StepKindEdit`) to their touched commit hashes.
- **Findings:** Correctly handles candidate commits against transcript commit commands (direct hash, `-m`, sequence matching, etc).
