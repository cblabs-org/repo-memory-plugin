# PR-10b: Delivery Persistence & Storage/Seeder Fixes

> **Programme:** repo-memory
> **PR:** pr-10b-delivery-and-storage-fixes
> **Base branch:** `pr-10a-v2-2-docs-and-specs/wo5-verification-and-handover` (`69a4cba`)
> **Status:** 🔄 Ready (0/5 WOs complete)
> **Human approval:** Decomposition approved 2026-10-15 as part of the v2.2 Programme Transition Plan (`PROGRAM.md` / `PROGRAM-PLAN.md`).

## Objective

Fix the `8d7ad7b` + `d3ab994` "One-Invocation Vanishing Memory" bug (`EphemeralMessage` $\to$ `UserMessage` for persistent moments), wire `DetectRewind` into `HandlePreInvocation`, and resolve open storage/seeder defects (GitHub Issues `#10`, `#11`, `#12`, `#13`, `#16`) before resuming `PR-10-eval-v2` Stage 2 (`wo4-dry-run` / `wo5-pilot`).

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-persistent-usermessage-and-rewind](wo1-persistent-usermessage-and-rewind/WORKORDER.md) | In `internal/harness/antigravity/preinvocation.go` and `compaction.go`: switch `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection` from `EphemeralMessage` to `UserMessage` (keeping `EphemeralMessage` strictly for 1-step `about-to-edit`, `error-match`, and search warnings). Wire `DetectRewind(steps, ledger.LastKnownStepCount)` into `HandlePreInvocation` before moment evaluation. Also add `-c user.name=test -c user.email=test@example.com` in `hooks_test.go` `TestStopHook_CompletePipeline` for hermetic test execution. Add regression tests verifying persistent vs ephemeral channel assignment and rewind re-arming. | standard | ☐ Pending |
| 2 | [wo2-index-and-projector-fixes](wo2-index-and-projector-fixes/WORKORDER.md) | Fix GitHub Issue **#13** (`internal/core/index/entries.go`: delete existing child rows in `entries_fts`, `entry_anchors`, and `error_patterns` before `INSERT OR REPLACE` on `entries`) and Issue **#11** (`internal/core/dossiers`: deduplicate entries by `ID` across scopes in the projector when an entry is re-added or moved to a new scope). Add unit tests for both fixes. | standard | ☐ Pending |
| 3 | [wo3-seeder-fixes](wo3-seeder-fixes/WORKORDER.md) | Fix GitHub Issue **#10** (`internal/core/seed/convert.go`: validate extracted backtick anchor candidates as real repository file paths / symbols rather than arbitrary code tokens) and Issue **#12** (`internal/core/seed`: stamp `--before` seeded ops with the workorder/cutoff commit's timestamp and SHA rather than `HEAD`). Add unit tests for both fixes. | standard | ☐ Pending |
| 4 | [wo4-ops-cleanup-and-rebuild](wo4-ops-cleanup-and-rebuild/WORKORDER.md) | Fix GitHub Issue **#16** (clean junk placeholder ops from `.memory/ops/`, re-render `.memory/dossiers/` and `ATLAS.md`), rebuild and install `~/.local/bin/repo-memory`, and re-seed/rebuild the `esp32-app-foundation` evaluation template index (`tpl-c`, `tpl-d`) so `PR-10` WO4/WO5 runs against clean anchors and persistent `UserMessage` delivery. | standard | ☐ Pending |
| 5 | [wo5-dry-run-verification](wo5-dry-run-verification/WORKORDER.md) | Run multi-invocation verification confirming that `task-start`, `first-touch`, and `first-entry` injections emitted as `UserMessage` remain in the conversation context across subsequent tool calls while `InjectedSet` suppresses duplicates, verify full Go test suite (`go test ./...`), and finalize `PROJECT.md`, `PROGRAM.md`, and `MEMORY.md`. | standard | ☐ Pending |

## Dependencies

- **Base:** `pr-10a-v2-2-docs-and-specs/wo5-verification-and-handover`
- **Preconditions:** `SPEC.md` v1.0 and `docs/design-v2-distillation-retrieval.md` Rev 4 (`VD13`) landed in `PR-10a`
- **Downstream:** Gates `PR-10-eval-v2` Stage 2 (`wo4-dry-run`, `wo5-pilot`, `wo6-main-evaluation`) and `PR-11-v2-doc-seeding-and-lifecycles`

## Exit Criteria

- `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection` emit `UserMessage` (plain string, persistent until `CHECKPOINT`), while `about-to-edit`, `error-match`, and FTS warnings emit `EphemeralMessage`.
- `DetectRewind` is wired into `HandlePreInvocation` and covered by unit tests.
- Open GitHub Issues `#10`, `#11`, `#12`, `#13`, and `#16` are resolved and covered by unit tests.
- `~/.local/bin/repo-memory` and the `esp32` evaluation template index (`tpl-c`, `tpl-d`) are rebuilt with the fixes.
- All Go tests (`go test ./...`) pass.

## Session Log

### 2026-10-15 — Project Creation & Human Approval Recorded
- Initialized by Chief Architect following completion of `PR-10a-v2-2-docs-and-specs` (`69a4cba`).
- Human approval of the 5-WO decomposition recorded in `PROGRAM.md` and `PROGRAM-PLAN.md` (`2026-10-15`).
