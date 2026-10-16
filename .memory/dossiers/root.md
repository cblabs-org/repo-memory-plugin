# root

## Invariants & contracts

### [docseed-70521f15] **Best for:** Durable session-scoped memory dossiers, startup `.memory/ATLAS.md` context, and newly entered scope entries that are tracked in a per-session `InjectedSet`.
`invariant · kind: semantic · anchors: .memory/ATLAS.md · source_doc: docs/antigravity-hooks-reference.md · evidence: doc:docs/antigravity-hooks-reference.md`

### [docseed-9d9196ea] The 125 v1 claims and `.memory/views/`. They are replaced by `.memory/ops/` and `.memory/dossiers/`.
`invariant · kind: semantic · anchors: .memory/ops, .memory/dossiers · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-dfdc29da] **Architectural implication for `repo-memory`:** Because the child's first `PreInvocation` fires at `initialNumSteps == 0` before its prompt is written to `transcript.jsonl`, `PreInvocation` starts the child with the baseline startup dossier (`.memory/ATLAS.md` + `global`) via `userMessage` and automatically injects task-specific scopes on subsequent `PreInvocation` calls once the child reads/t...
`invariant · kind: semantic · anchors: .memory/ATLAS.md · source_doc: docs/antigravity-hooks-reference.md · evidence: doc:docs/antigravity-hooks-reference.md`

### [docseed-f9386d51] **Updating:** only the text between the markers is ever rewritten. The version tag (`v1` $\to$ `v2`) allows automatic in-place upgrades from `views/MAP.md` to `.memory/ATLAS.md`.
`invariant · kind: semantic · anchors: .memory/ATLAS.md · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

## Ruled out, failure signatures and gotchas

### [seed-309e02f1] PR-06b/wo2 and wo3 have unfilled completion report templates despite code being completed — excluded from selection.
`ruled-out · kind: episodic`

### [seed-4ac0c7aa] When using SQLite in-memory databases (`:memory:`), `PRAGMA journal_mode = WAL;` returns `"memory"`. The store gracefully handles this while ensuring file-backed databases always run in `wal` mode.
`ruled-out · kind: episodic · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-4ce5711c] Several WOs document pre-existing test failures (TestSessionAttributor_Integration) outside their boundary boxes — these must be excluded from pass/fail scoring during X6-lite runs.
`ruled-out · kind: episodic`

### [seed-53056064] When using `harness.StepKindUser`, the human messages will be parsed as verbatim.
`ruled-out · kind: episodic`

### [seed-5e480c6a] Encountered a few test failures because tests originally asserted behaviors that were explicitly removed (like `GEMINI.md` modification and `.agents/.gitignore` creation). Fixed tests by removing those legacy checks and adapting to the new `.git/info/exclude` approach.
`ruled-out · kind: episodic · anchors: .git/info/exclude, .agents/.gitignore`

### [seed-66b038cb] Early-PR candidates (pr-02a/wo4 commit-linkage, pr-05a/wo1 executors, pr-05b/wo1 subsystem-bootstrap) were considered as alternatives with concrete dead-end discrimination. Not selected because late-programme WOs have richer dossiers and tighter dependency chains, but documented as viable substitutions if needed.
`ruled-out · kind: episodic`

### [seed-88a336e2] Encountered a worktree lock while trying to checkout `pr-09-v2-x6-lite/wo3-replay-harness`. Decided to build the harness directly in the active subagent worktree; changes will be committed and merged to the target branch.
`ruled-out · kind: episodic`

### [seed-c473995a] PR-08/wo3 (ephemeral-and-errors) also has pending template — excluded.
`ruled-out · kind: episodic`

### [seed-dba1dab0] The `ep.Header.ID` doesn't exist, it is `EpisodeID`.
`ruled-out · kind: episodic`

### [docseed-3d67af0d] **Optional Section Resolution (`{section?}`) & Five-Tier Scope Hierarchy:** Dossiers omit empty sections and separate ephemeral scratch (`temp:`), conversation state (`session:`), personal gitignored preferences (`user:`), branch-local unmerged work (`branch:` in `.memory/local/`), and git-tracked shared truth (`repo:` in `.memory/ops/` and `.memory/dossiers/`).
`gotcha · kind: semantic · anchors: .memory/ops, .memory/dossiers · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

## Decisions

### [seed-0b464259] Kept `AGENTS.md` untouched by default unless configured, while making `GEMINI.md` and `CLAUDE.md` strictly machine-managed via the `<!-- repo-memory:begin v2 -->` delimited block during `repo-memory enroll` / `unenroll` (`init`/`leave`).
`decision · kind: episodic`

### [seed-0f88894d] Snapshotting method: Manually created `.memory/evidence/x6-lite-protocol.md` from WO1's completion report text since the document didn't explicitly exist as a separate file. Then snapshotted all candidate WO PA documents into `.memory/evidence/pa-workspace/` to protect them from Antigravity pruning.
`decision · kind: episodic`

### [seed-10ec301c] Exit test paths corrected per actual WORKORDER.md contents: wo4-gates uses `./internal/harness/antigravity/...`, wo6-pull-tools uses `./internal/mcpserver/...`.
`decision · kind: episodic`

### [seed-21182716] Two-tier lease reclamation: `ReclaimExpiredLeases` atomically handles both recoverable jobs (reset to `queued` with cleared lease) and crash-looping jobs (transitioned to `dead` once `maxAttempts` is reached).
`decision · kind: episodic · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-3bbd769a] Struggle candidates: wo6-seeding (novel time-cut + v1 dead ends) and wo4-gates (X9 hook contract, deny-once lifecycle).
`decision · kind: episodic`

### [seed-4212517c] Designed `Generator.Generate()` to filter claims internally by type (e.g. failure signatures vs decisions) and output the respective files so it handles missing/empty collections safely.
`decision · kind: episodic`

### [seed-49190a58] Idempotency deduplication: Used unique partial index `WHERE idempotency_key IS NOT NULL AND idempotency_key != ''` so that non-keyed jobs can be enqueued without conflict, while keyed jobs are strictly deduplicated and return existing jobs on duplicate insertion.
`decision · kind: episodic · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-4a4f6899] Used `context.Context` in the `Registry` interface for standard cancellation and timeout handling.
`decision · kind: episodic`

### [seed-6c5fb0e7] Harness entry point: `internal/eval/x6_replay.go`, providing a `ReplayHarness` struct with methods `CheckoutParent`, `InjectCondition`, and `RunWorker`.
`decision · kind: episodic`

### [seed-6cc75a12] Used `os.UserHomeDir()` instead of `config.DataDir()` directly for creating the backup path in `cmd_install.go` to exactly match the specific `~/.gemini/antigravity/...` target without relying on environment variable variations, while passing the mocked `$HOME` in tests to intercept and test properly.
`decision · kind: episodic · anchors: .git/info/exclude, .agents/.gitignore`

### [seed-6f39d618] Negative control: wo1-op-log included as a calibration point (low prior-knowledge dependency; all conditions expected to perform similarly).
`decision · kind: episodic`

### [seed-6f98581b] Retained `contract` as an alias for `convention`, `gotcha` as an alias for `external-behaviour`, and `recipe` as an alias for `procedure` so existing seeded ops and new `v2.2` entries remain schema-compatible.
`decision · kind: episodic`

### [seed-767715a6] Implemented minimalistic structs matching the exact requirements from §5.2.
`decision · kind: episodic`

### [seed-851d934c] Protocol approved by user on 2026-10-09.
`decision · kind: episodic`

### [seed-873b1261] Injection mechanics: Modifying `.memory/config.yaml` to disable memory for condition `a`. For conditions `b` and `c`, memory is enabled and we perform file copies to the expected directories (repo root for PA docs, data dir for dossiers) to simulate the presence of time-cut memory context. `RunWorker` invokes `agy --agent Execution-Worker` via headless `exec.CommandContext` to process the target Work Order autonomously.
`decision · kind: episodic`

### [seed-8cd3c463] Budget: ≤72 runs, ≤7M tokens, ~4.5–7 hours across 1–2 days.
`decision · kind: episodic`

### [seed-a2a1eb84] Storage layout for time-cut data: For each of the 8 WOs, extracted their exact `chore(PA)` parent commit. Executed `repo-memory seed --before <commit>` into `evaluation/data/<wo-name>/ops/`. Captured `PROJECT.md` and `MEMORY.md` (via git show at the parent commit) into `evaluation/data/<wo-name>/pa_docs/`. Then rendered the time-cut dossiers into `evaluation/data/<wo-name>/dossiers/` and `evaluation/data/<wo-name>/ATLAS.md` using the core dossiers Projector/Writer.
`decision · kind: episodic`

### [seed-a3984f4d] Allowed `skip` to only evaluate if `keep` is false in rule engine, so keep takes precedence and we don't accidentally skip-all on an already kept episode.
`decision · kind: episodic`

### [seed-a99aabc5] Threshold: condition (c) dossiers must strictly beat condition (a) no-memory on aggregate exit test pass rate. PA comparison (b) reported but not gating.
`decision · kind: episodic`

### [seed-b9d9f70f] Pure-Go SQLite: Selected `modernc.org/sqlite` per SPEC §18.4 to maintain a cgo-free single binary.
`decision · kind: episodic · anchors: go.mod, go.sum, internal/core/jobs/types.go, internal/core/jobs/schema.go, internal/core/jobs/state.go, internal/core/jobs/store.go, internal/core/jobs/state_test.go, internal/core/jobs/store_test.go`

### [seed-bbb76444] Seeds: 2 per WO/condition (42, 137), 3rd tiebreaker (256) on disagreement.
`decision · kind: episodic`

### [seed-c06ddb4e] Kept both `memory_status` and `memory_stats` documented as aliases in §7.7 / §18.1 so existing tests and documentation references resolve seamlessly.
`decision · kind: episodic`

### [seed-c9aa1eab] Validated custom rules from `.memory/scrub.yaml` and `scrub.local.yaml` at the start of `Scrub` so malformed configs or forbidden keys (`allow`, `disable`, `whitelist`) fail closed deterministically even on empty strings.
`decision · kind: episodic`

### [seed-ddbd0e48] Selected WOs: 8 work orders from PR-07 (5) and PR-08 (3): wo1-op-log, wo2-dossier-schema-renderer, wo3-scopes-aliases, wo4-index-migration, wo6-seeding, wo1-ledger-engine, wo4-gates, wo6-pull-tools.
`decision · kind: episodic`

### [seed-ed66fa39] Expressed the $\ge 1024$-byte contiguous blob pattern in Go's `regexp` package as `[A-Za-z0-9+/=]{512,}[A-Za-z0-9+/=]{512,}` and deferred $\ge 1024$-char hex strings from Layer 2 to Layer 4 because Go's RE2 engine caps `{min,max}` repeat quantifiers at `1000`.
`decision · kind: episodic`

### [seed-f4bac002] Preserved `G1`/`P1` ("zero user or agent cooperation required") throughout §1, §4, and §5.1 by keeping the milestone-gated `Stop` continuation (`NO_TOOL_CALL`, `executionNum == 0`) as the primary automatic Stage 1 mechanism while framing `memory_note` strictly as an optional mid-turn override that advances the conversation mark.
`decision · kind: episodic`

## Procedures / Recipes

### [docseed-11db9bc8] **Five-Tier State Scope Hierarchy & Pre-Consolidation Corroboration Gate (`VD18`, §6, §7.1):** Formalizes `temp:`, `session:`, `user:` (gitignored `<data-dir>/repos/<repo-id>/user-dossiers/`), `branch:` (`.memory/local/`), and `repo:` (`.memory/ops/` & `.memory/dossiers/`), and enforces a deterministic Pre-Consolidation Corroboration Gate requiring a passing `PostToolUse` build/test step, ver...
`recipe · kind: procedural · anchors: .memory/ops, .memory/dossiers · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-54e73d93] **Bootstrap (`repo-memory seed-docs`):** Scans configured markdown paths (`docs/developer/**/*.md`, `.agents/skills/**/SKILL.md`, or paths listed under `doc_seeds` in `.memory/config.yaml`) and runs `repo-memory-distiller` to extract concrete `invariant`, `gotcha`, `recipe`, and `failure-signature` entries anchored to the actual source files, build manifests, and symbols they govern.
`recipe · kind: procedural · anchors: .memory/config.yaml · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-72185a3f] **Ingestion (`repo-memory seed-docs`):** Discovers curated markdown files configured under `doc_seeds` in `.memory/config.yaml` (defaulting to `docs/developer/**/*.md` and hand-written `.agents/skills/**/SKILL.md` files that do not have `generated_by: repo-memory`). Each file is passed through the preparation layer and `repo-memory-distiller` to extract concrete `invariant`, `contract`, `gotcha...
`recipe · kind: procedural · anchors: .memory/config.yaml · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-b52782ec] **Build-manifest sensitivity:** If a commit or working-tree diff modifies a repository build or environment manifest (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `Taskfile.yml`, `.github/workflows/*.yml`) anchored to a `recipe`, `tick` flags that recipe `stale_suspect: true` until its command next succeeds (`exit 0`).
`recipe · kind: procedural · anchors: Makefile, go.mod · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-bb4fb8dd] **Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Freshness ("Dreaming"):** Curated in-repo documentation (`docs/developer/`, `.agents/skills/`) is distilled into anchored dossier entries (`source: doc-seed`) and kept fresh asynchronously on `git diff` changes (§7.6).
`recipe · kind: procedural · anchors: .agents/skills · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-bdc24763] **Doc-Seeded Dossiers & Async Doc Freshness ("Dreaming") (`VD16`, §7.6):** `repo-memory seed-docs` (`source: doc-seed`) distills curated repository documentation (`docs/developer/`, `.agents/skills/`) into code-anchored dossier entries delivered via `PreInvocation` push, paired with an async git-diff watcher in `tick` (`source_doc_path`, `source_doc_blob_sha`) that re-distills affected entries...
`recipe · kind: procedural · anchors: .agents/skills · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

## Open issues

### [seed-6823189b] There are pre-existing upstream test failures in `cmd/repo-memory/cmd_search_test.go`, `internal/core/jobs/store_test.go`, and `internal/harness/antigravity/attribution_test.go`. The PA should investigate these broken tests under a separate work order.
`open-issue · kind: semantic`

### [seed-8e40ba94] `cmd/repo-memory` seems to be ignored by `.gitignore` under the rule `repo-memory`, which could cause git to ignore source files in that directory.
`open-issue · kind: semantic`
