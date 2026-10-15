# docs/experiments

## Ruled out, failure signatures and gotchas

### [seed-8675082c] **Warning on `.agents/plugins.json`:** Never write `~` or `$HOME` in `.agents/plugins.json`. Antigravity's Go customization loader does not evaluate shell expansions, resulting in silent load failures.
`ruled-out · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-a45619cf] **Warning:** Token measurement is impossible by parsing transcripts right now.
`ruled-out · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

### [seed-d472d172] **Dead End:** `PreInvocation` context delivery via `systemMessage` and `userMessage`. We must pivot to an alternative for PR-08.
`ruled-out · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

## Decisions

### [seed-26fc4334] **Target layout confirmed:** `workspace-plugin/repo-memory` can safely be nested under `repo-memory-daemon/` without global discovery collision.
`decision · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-6ed4ed4d] **Absolute paths required:** `repo-memory enroll` (WO4) must resolve and write absolute paths to `.agents/plugins.json` because Antigravity's loader does not expand `~` or `$HOME`.
`decision · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-86292314] Chose to omit token tracking directly from the X3-quota document's "results" other than noting its absence, since the platform simply doesn't expose it there.
`decision · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

### [seed-bcdfacd4] **Sidecar directory configuration supported:** Storing `config.json` in `sidecars/tick/config.json` causes zero supervisor or validation issues.
`decision · anchors: docs/experiments/phase0-plugin-split.md`

## Open issues

### [seed-8389c92c] In `internal/harness/antigravity/attribution_test.go`, `TestSessionAttributor_Integration` fails when executed beyond the hardcoded test timestamp (`now := time.Date(2026, 10, 2, ...)`), because dynamically created transcript files on disk have modification times newer than `now` (`now.Sub(fi.ModTime())` is negative, triggering the idle cutoff check).
`open-issue · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-c7598b0b] `agy -p /usage` acts as an undocumented, zero-token CLI subcommand (though the interactive TUI `/usage` behaves differently). We must invoke it via `-p` and JSON output to reliably parse the model buckets.
`open-issue · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`
