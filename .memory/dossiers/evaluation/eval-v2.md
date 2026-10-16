# evaluation/eval-v2

## Ruled out, failure signatures and gotchas

### [seed-8a69f0f1] In `v2.1`, single-turn `EphemeralMessage` delivery combined with session-long `ledger.InjectedSet` deduplication left `0` entries in context by Invocation $\ge 2$; switching persistent moments to `UserMessage` (`PR-10b`) resolved this completely.
`ruled-out · kind: episodic · anchors: evaluation/eval-v2/SEED_REPORT.md, docs/evaluations/v2-2-read-side-micro-probe.md, docs/evaluations/v2-2-write-side-micro-probe.md, docs/evaluations/v2-2-micro-probe-verdict.md`

## Decisions

### [seed-5303378e] Retained natural templates without artificial "read the docs" prompts so Stage 2 evaluates real-world push (`repo-memory v2.2`) vs. pull (`PA` workspace and static docs), where historical voluntary pull rates across 77 sessions were `1%–8%`.
`decision · kind: episodic · anchors: evaluation/eval-v2/SEED_REPORT.md, docs/evaluations/v2-2-read-side-micro-probe.md, docs/evaluations/v2-2-write-side-micro-probe.md, docs/evaluations/v2-2-micro-probe-verdict.md`

### [seed-c1cad236] Upgraded the evaluation index from PA-only (`256` entries) to the `v2.2` combined `seed` + `seed-docs` corpus (`1,283` entries) so all 6 target `esp32-app-foundation` subsystems have file-anchored semantic, episodic, and procedural coverage.
`decision · kind: episodic · anchors: evaluation/eval-v2/SEED_REPORT.md, docs/evaluations/v2-2-read-side-micro-probe.md, docs/evaluations/v2-2-write-side-micro-probe.md, docs/evaluations/v2-2-micro-probe-verdict.md`
