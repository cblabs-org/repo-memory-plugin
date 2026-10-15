# core/episode

## Ruled out, failure signatures and gotchas

### [seed-285fb189] `git log -g` (reflog walker) does not permit `--reverse` flag in git (`fatal: options '--reverse' and '--walk-reflogs' cannot be used together`). Reversed the candidate slice in Go instead of passing `--reverse` to git reflog.
`ruled-out · kind: episodic · anchors: internal/core/episode/commit_linkage.go, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

## Decisions

### [seed-396eaa3e] Implemented atomic rename on write within the target directory to prevent partial files or reader corruption.
`decision · kind: episodic · anchors: internal/core/episode/store.go, internal/core/episode/store_test.go, go.mod`

### [seed-3972d803] Reversing raw git log and reflog output (which lists newest commits first) ensures that commits occurring within the same second preserve their chronological parent/creation ordering.
`decision · kind: episodic · anchors: internal/core/episode/commit_linkage.go, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

### [seed-586677a0] Implemented streaming chunk-based line reader `readLineBytes` with `bufio.Reader` to ensure arbitrarily long JSON lines do not fail with buffer overflow.
`decision · kind: episodic · anchors: internal/core/episode/store.go, internal/core/episode/store_test.go, go.mod`

### [seed-7fc83a86] Automated gap detection on write if gaps have not been explicitly populated.
`decision · kind: episodic · anchors: internal/core/episode/store.go, internal/core/episode/store_test.go, go.mod`

### [seed-9e198370] Edit steps (`StepKindEdit`) in the episode are automatically matched to commits via `git diff-tree --name-only -r --root <hash>` where possible.
`decision · kind: episodic · anchors: internal/core/episode/commit_linkage.go, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

### [seed-e07cd549] Selected pure-Go `github.com/klauspost/compress/zstd` per SPEC §18.4 (Episode compression: Pure-Go zstd, zero cgo).
`decision · kind: episodic · anchors: internal/core/episode/store.go, internal/core/episode/store_test.go, go.mod`

### [seed-e0a980b6] A configurable `timeMargin` (default 60 seconds) handles timestamp granularity rounding and minor clock skew between transcript logs and git committer timestamps without false positives, because commits still require transcript command confirmation.
`decision · kind: episodic · anchors: internal/core/episode/commit_linkage.go, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

### [seed-f57f5c43] Selected `github.com/oklog/ulid/v2` for generating standardized `ep-<ulid>` episode IDs.
`decision · kind: episodic · anchors: internal/core/episode/store.go, internal/core/episode/store_test.go, go.mod`
