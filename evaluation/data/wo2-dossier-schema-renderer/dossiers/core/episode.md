# core/episode

## Ruled out, failure signatures and gotchas

### [seed-285fb189] `git log -g` (reflog walker) does not permit `--reverse` flag in git (`fatal: options '--reverse' and '--walk-reflogs' cannot be used together`). Reversed the candidate slice in Go instead of passing `--reverse` to git reflog.
`ruled-out · anchors: internal/core/episode/commit_linkage.go, step.Commit, header.Commits, header.HeadEnd, step.Commit, Header.UnmarshalJSON, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

## Decisions

### [seed-3972d803] Reversing raw git log and reflog output (which lists newest commits first) ensures that commits occurring within the same second preserve their chronological parent/creation ordering.
`decision · anchors: internal/core/episode/commit_linkage.go, step.Commit, header.Commits, header.HeadEnd, step.Commit, Header.UnmarshalJSON, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

### [seed-9e198370] Edit steps (`StepKindEdit`) in the episode are automatically matched to commits via `git diff-tree --name-only -r --root <hash>` where possible.
`decision · anchors: internal/core/episode/commit_linkage.go, step.Commit, header.Commits, header.HeadEnd, step.Commit, Header.UnmarshalJSON, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`

### [seed-e0a980b6] A configurable `timeMargin` (default 60 seconds) handles timestamp granularity rounding and minor clock skew between transcript logs and git committer timestamps without false positives, because commits still require transcript command confirmation.
`decision · anchors: internal/core/episode/commit_linkage.go, step.Commit, header.Commits, header.HeadEnd, step.Commit, Header.UnmarshalJSON, internal/core/episode/store.go, internal/core/episode/store.go, internal/core/episode/commit_linkage_test.go, internal/core/episode/store_test.go`
