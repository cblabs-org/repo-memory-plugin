# X6-lite Protocol

### Decisions Made
- Selected WOs: 8 work orders from PR-07 (5) and PR-08 (3): wo1-op-log, wo2-dossier-schema-renderer, wo3-scopes-aliases, wo4-index-migration, wo6-seeding, wo1-ledger-engine, wo4-gates, wo6-pull-tools.
- Negative control: wo1-op-log included as a calibration point (low prior-knowledge dependency; all conditions expected to perform similarly).
- Struggle candidates: wo6-seeding (novel time-cut + v1 dead ends) and wo4-gates (X9 hook contract, deny-once lifecycle).
- Threshold: condition (c) dossiers must strictly beat condition (a) no-memory on aggregate exit test pass rate. PA comparison (b) reported but not gating.
- Seeds: 2 per WO/condition (42, 137), 3rd tiebreaker (256) on disagreement.
- Budget: ≤72 runs, ≤7M tokens, ~4.5–7 hours across 1–2 days.
- Exit test paths corrected per actual WORKORDER.md contents: wo4-gates uses `./internal/harness/antigravity/...`, wo6-pull-tools uses `./internal/mcpserver/...`.
- Protocol approved by user on 2026-10-09.

### Dead Ends & Warnings
- PR-06b/wo2 and wo3 have unfilled completion report templates despite code being completed — excluded from selection.
- PR-08/wo3 (ephemeral-and-errors) also has pending template — excluded.
- Several WOs document pre-existing test failures (TestSessionAttributor_Integration) outside their boundary boxes — these must be excluded from pass/fail scoring during X6-lite runs.
- Early-PR candidates (pr-02a/wo4 commit-linkage, pr-05a/wo1 executors, pr-05b/wo1 subsystem-bootstrap) were considered as alternatives with concrete dead-end discrimination. Not selected because late-programme WOs have richer dossiers and tighter dependency chains, but documented as viable substitutions if needed.
