# X6-lite Runner v3 — Implementation Plan (Revised, Round 2 items applied)

## Design Decisions (Final)

| # | Decision | Resolution | Source |
|---|---|---|---|
| 1 | Hooks-only vs raw files | **Hooks-only.** Index built from ops OUTSIDE the run tree. No v2 `.memory/` files in the tree. | C1.1 |
| 2 | Drop PR-07 WOs? | **Drop wo6-seeding entirely** (the PA workspace is its task input, and seeded entries leak). Keep wo1-op-log and wo2-dossier-schema-renderer. | C1.2, D1.7 |
| 3 | Condition definitions | **(a)** = repo only. **(b)** = repo + PA workspace at WO_COMMIT. **(c)** = repo + plugin hooks. | C1.3 |
| 4 | Metric + replicates | Primary: fraction of hidden tests passed (continuous, paired by WO). Replicate count set after the first interleaved block. | C1.4, E2.4 |
| 5 | Base commit | **`WO_COMMIT`, not `WO_COMMIT~1`.** For 7 of 8 WOs this adds only PA files. For wo2 it also adds the spec amendments the WO needs. | D2.1 |
| 6 | Budget staging | **(a) vs (c) is primary** (the kill gate); (b) only if budget remains. After the pilot, **one interleaved (a)/(c) schedule**; its first block doubles as the difficulty probe. | D2.8, E2.4 |
| 7 | Agent | **Drop `--agent Execution-Worker`** (it doesn't exist in the CLI; agy falls back to its default). Record the resolved agent. | D1.8 |
| 8 | Cut point for ops | **`WO_COMMIT` timestamp, applied to every WO.** The helper filters out later batches; it doesn't just check. | D1.6, E1.2 |
| 9 | System-under-test build | **Build `repo-memory` and the index helper from one recorded commit.** Never use the main checkout's `./repo-memory`: it predates the delivery engine. | E1.1 |
| 10 | Background tick | **Daemon tick sidecar disabled for the batch** if the pilot shows it running in eval sessions; the learning loop isn't under test. | E1.6 |

## WO Set (7 WOs, wo6-seeding dropped)

| WO | Base (WO_COMMIT) | Conditions | Snapshot ops dropped by cutoff |
|---|---|---|---|
| wo1-op-log | `42deb642bda9` | a, b, c | 0 |
| wo2-dossier-schema-renderer | `0fbfe5a6f894` | a, b, c | 0 |
| wo3-scopes-aliases | `3177bfe3cdb9` | a, b, c | 0 |
| wo4-index-migration | `501d05d45396` | a, b, c | 0 |
| wo1-ledger-engine | `732177442cd0` | a, b, c | 3 (`(pending)`, stamped at impl commit `7bff5a4`) |
| wo4-gates | `e4acaa2a5cd3` | a, b, c | 3 (`(pending)`, stamped at impl commit `5cf3fde`) |
| wo6-pull-tools | `6a840b3893db` | a, b, c | 9 (4 from WO5 impl `c8ece1a`, 5 from **its own impl** `99994b5`) |

## Phase 0: Pinned build (E1.1)

```bash
PIN_COMMIT=$(git -C "${REPO_ROOT}" rev-parse main)     # record it; don't change it mid-batch
PIN_DIR="${EVAL_ROOT}/pin/${PIN_COMMIT:0:12}"
git -C "${REPO_ROOT}" worktree add --detach "${PIN_DIR}/src" "${PIN_COMMIT}"
( cd "${PIN_DIR}/src" && go build -o "${PIN_DIR}/repo-memory" ./cmd/repo-memory \
                      && go build -o "${PIN_DIR}/build-index" ./evaluation/cmd/build-index )
shasum -a 256 "${PIN_DIR}/repo-memory" "${PIN_DIR}/build-index"
```
Note: `evaluation/cmd/build-index` must be committed at `PIN_COMMIT`, or built from the same tree.

- **Preflight:** `strings repo-memory | grep -c 'internal/core/delivery'` > 0, and the binary emits the `[repo-memory %s] %s: %s` format.
- **Recorded in every result row:** `PIN_COMMIT` and both sha256 values.

## Phase 1: Isolation

### 1a. Run directory layout (D1.4)

Keep the data dir outside the repo. Layout:
```
$RUN_ROOT/          (mktemp -d, name encodes nothing)
├── repo/           (the standalone clone; agy CWD)
├── data/           (REPO_MEMORY_DATA_DIR)
│   └── <repoID>/
│       ├── index.db
│       └── ledgers/
├── ops/            ((c) only: cutoff-filtered copy of the snapshot ops; never inside repo/)
├── bin/
│   └── repo-memory (copy of the pinned binary)
└── plugin/
    └── repo-memory/
        ├── plugin.json      (copied from the installed workspace plugin)
        ├── hooks.json       (commands use `env REPO_MEMORY_DATA_DIR=… $RUN_ROOT/bin/repo-memory …`)
        └── mcp_config.json  (same binary, `"env": {"REPO_MEMORY_DATA_DIR": …}`)
```

### 1b. Standalone clone with sanitised history (D1.1, E1.3, E1.4)

```bash
RUN_ROOT=$(mktemp -d)
RUN_DIR="${RUN_ROOT}/repo"
TEMP_BRANCH="x6-base-$(uuidgen | head -c 8)"
trap 'git -C "${REPO_ROOT}" branch -D "${TEMP_BRANCH}" 2>/dev/null; …' EXIT

git -C "${REPO_ROOT}" branch "${TEMP_BRANCH}" "${WO_COMMIT}"
git clone --no-local --single-branch --no-tags -b "${TEMP_BRANCH}" "${REPO_ROOT}" "${RUN_DIR}"
git -C "${REPO_ROOT}" branch -D "${TEMP_BRANCH}"

# Pre-filter check, on the fresh clone, while original shas still mean something:
! git -C "${RUN_DIR}" cat-file -e "${ORACLE_SHA}" 2>/dev/null || fail "future commit present"

# Filter IMMEDIATELY (fresh clone, so no --force needed; filter-repo also removes origin).
# One pass: drop paths per condition + rewrite home-dir paths in every blob in history.
git -C "${RUN_DIR}" filter-repo ${PATH_ARGS} --replace-text "${EVAL_ROOT}/replace-paths.txt"
git -C "${RUN_DIR}" branch -m main
rm -rf "${RUN_DIR}/.git/filter-repo"
git -C "${RUN_DIR}" remote | grep -q . && git -C "${RUN_DIR}" remote remove origin
```
If filter-repo still refuses ("not a fresh clone"), add `--force`. It's safe on this throwaway clone.

**`PATH_ARGS` per condition** (`--invert-paths` plus one `--path` per entry):

| Condition | Paths removed from history |
|---|---|
| (a) no_memory | `.agents/skills/principal-architect-workspace/`, `.memory/archive/`, `.memory/views/`, `.memory/B5_EVALUATION.md` |
| (b) pa_documents | `.memory/archive/`, `.memory/views/`, `.memory/B5_EVALUATION.md` |
| (c) dossiers | `.agents/skills/principal-architect-workspace/`, `.memory/archive/`, `.memory/views/`, `.memory/B5_EVALUATION.md` |

**Keep in all conditions:** `.memory/config.yaml` and `.memory/.gitignore` (tests need them; D1.2).

**`replace-paths.txt`** (literal replacements, applied in order, history-wide):
```
/Users/mcneillm/Documents/Projects/repo-memory-plugin==>/workspace/repo-memory-plugin
/Users/mcneillm/==>/home/dev/
```
This covers every tracked file containing a home-dir path, not just the original 4. At wo6-pull-tools' WO_COMMIT, 8 non-PA files contain one: `docs/dogfood-evaluation-01.md`, `docs/experiments/E02-include-resolution.md`, `E03-transcript-currency.md`, `E04-rule-activation.md`, `E05-hook-latency.md`, `phase0-plugin-split.md`, `packaging/antigravity/hook-wrapper.sh` and `testdata/e1-marker-hook.sh`. PA files kept in (b) are covered too. No Go test references the two scripts.

**Validation (all content-based, after filtering):**
1. Newest commit date: `git -C RUN_DIR log --all -1 --format=%cI` ≤ the WO_COMMIT date.
2. No future content: `git -C RUN_DIR log --all --oneline -- <paths the oracle adds>` is empty.
3. (a)/(c) only: `git -C RUN_DIR rev-list --all --objects | grep -c principal-architect-workspace` == 0.
4. No home path anywhere in history: `git -C RUN_DIR log --all -p | grep -c '/Users/mcneillm'` == 0. A working-tree `grep -r` can't see packed objects.
5. No home path in the working tree: `grep -r '/Users/mcneillm' RUN_DIR --exclude-dir=.git` finds 0.
6. Clean tree: `git -C RUN_DIR status --porcelain` is empty.

Record `base_sha_original` (WO_COMMIT) and `base_sha_filtered` (rewritten HEAD).

### 1c. Sibling isolation
- `mktemp -d` names encode nothing.
- `MAX_PARALLEL=1`. Archive, then delete `RUN_ROOT` before the next run starts.
- No condition, WO or replicate appears in any path or branch name the agent can see.

### 1d. Contamination guards → hard fail (E2.1)
**Before the run:**
- Snapshot `git -C REPO_ROOT status --porcelain`, plus sha256 of `internal/` and `cmd/`.
- Snapshot agy `implicit/` and `knowledge/` file counts and mtimes.

**After the run:**
- If any snapshot changed → `invalid_contamination`.
- **Allowlist transcript scan.** Parse `transcript_full.jsonl`. For every tool call, collect `Cwd`, `AbsolutePath`, `TargetFile`, and absolute paths inside `CommandLine`. Any path outside the allowlist → `invalid_contamination`. The allowlist:
  - `$RUN_ROOT`,
  - the agent's **own** brain dir (`~/.gemini/antigravity-cli/brain/<conversation_id>/`),
  - `$(go env GOROOT)`, `$(go env GOMODCACHE)`, `$(go env GOCACHE)`,
  - the system temp dir.

  Other brain dirs count as violations: 74 older CLI conversations, mostly Run 1, hold transcripts of finished code.
- **Optional, with your approval:** move the Run 1 and pilot conversation dirs out of `~/.gemini/antigravity-cli/brain/` for the batch, and restore them afterwards.

### 1e. Tick sidecar control (E1.6)
- The global `repo-memory-daemon` plugin runs `repo-memory tick` every minute (`~/.gemini/config/plugins/repo-memory-daemon/sidecars/tick/sidecar.json`).
- **Pilot check:** after each (c) pilot run, look for `jobs.sqlite`, `spool/` or `queue/` under `$RUN_ROOT/data`.
- **If any appear:** set the daemon plugin's `config.json` `enabled: false` for the batch (back up first, and restore on exit via trap), then re-pilot.
- **Each batch run:** assert none of those appear; otherwise → `invalid_contamination`.

## Phase 2: Condition Setup

### Condition (a): no_memory
1. Clone + filter (Phase 1): removes the PA workspace and the v1 `.memory/` artifacts.
2. Create `$RUN_ROOT/data/` (empty).
3. Launch agy with `REPO_MEMORY_DISABLED=1` and `REPO_MEMORY_DATA_DIR=$RUN_ROOT/data`.
4. No `.agents/plugins.json`, no `.memory/.enrolled`.
5. **Validation:** no delivery ledger exists after the run.

### Condition (b): pa_documents
1. Clone + filter (Phase 1): removes only the v1 `.memory/` artifacts and keeps the PA workspace (home paths rewritten).
2. Create `$RUN_ROOT/data/` (empty).
3. Launch agy with `REPO_MEMORY_DISABLED=1` and `REPO_MEMORY_DATA_DIR=$RUN_ROOT/data`.
4. No `.agents/plugins.json`, no `.memory/.enrolled`.
5. **Validation:** no delivery ledger exists after the run.

### Condition (c): dossiers (hooks-only)
1. Clone + filter (Phase 1): removes the PA workspace and the v1 `.memory/` artifacts.
2. Build the per-run plugin dir (E1.5):
   - `cp "${PIN_DIR}/repo-memory" $RUN_ROOT/bin/repo-memory`.
   - `cp -R ~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory $RUN_ROOT/plugin/`. This brings `plugin.json`, `hooks.json` and `mcp_config.json`.
   - Rewrite every hook command. agy hooks have no `env` field, so put the variable in the command string:
     `"command": "env REPO_MEMORY_DATA_DIR=$RUN_ROOT/data $RUN_ROOT/bin/repo-memory hook antigravity PreInvocation"` (and the same for PreToolUse, PostToolUse and Stop).
   - Rewrite `mcp_config.json`: set `command` to `$RUN_ROOT/bin/repo-memory`, keep `args: ["mcp"]`, and add `"env": {"REPO_MEMORY_DATA_DIR": "$RUN_ROOT/data"}`.
3. Enroll: `REPO_MEMORY_DATA_DIR=$RUN_ROOT/data $RUN_ROOT/bin/repo-memory enroll $RUN_DIR`. The origin is already gone, so the repoID is derived from the run path and can't collide with the real repo. Read the repoID from `$RUN_ROOT/data/registry.json`.
4. Overwrite `$RUN_DIR/.agents/plugins.json` with `{"entries":[{"path":"$RUN_ROOT/plugin"}]}`.
5. Build `index.db` with the pinned helper (2b) into `$RUN_ROOT/data/<repoID>/index.db`.
6. **Preflight:**
   - grep `$RUN_DIR` and `$RUN_ROOT/plugin` for `/Users/mcneillm/Documents` → 0;
   - `index.db` has > 0 entries, with the newest ≤ cutoff;
   - the leak test (2b) passes.
7. Launch agy with `REPO_MEMORY_DATA_DIR=$RUN_ROOT/data` (no `REPO_MEMORY_DISABLED`, no `REPO_MEMORY_HOUSEKEEPING`).
8. **Validation:**
   - The delivery ledger at `$RUN_ROOT/data/<repoID>/ledgers/<conversation_id>.json` must exist.
   - No ledger file means the hooks didn't fire → `invalid_no_hooks`, retry.
   - A ledger with 0 records means the product chose nothing → a valid outcome, flagged.

### All conditions: final pre-launch check (E2.5)
Right before launching agy, re-check `git -C RUN_DIR status --porcelain` is empty. `.enrolled` and `plugins.json` are gitignored at every WO_COMMIT, so (c) stays clean.

### 2b. Index builder (Go helper) (D1.5, E1.2)

`repo-memory reindex` writes to the wrong path (`.memory/.cache/index.db`) and needs ops in the tree. Instead:

```go
// evaluation/cmd/build-index/main.go — built in Phase 0 from PIN_COMMIT
func main() {
    opsDir := flag.String("ops", "", "snapshot ops dir (evaluation/data/<wo>/ops)")
    tmpOps := flag.String("tmp-ops", "", "output dir for cutoff-filtered ops ($RUN_ROOT/ops)")
    dbPath := flag.String("db", "", "output index.db path")
    cutoff := flag.String("cutoff", "", "RFC3339; WO_COMMIT committer date")
    flag.Parse()

    // 1. FILTER: copy only batches whose ops all have timestamp <= cutoff into tmpOps
    //    (late ops sit in whole batches, all `add`, nothing later references them).
    //    Fail if any batch mixes pre- and post-cutoff ops. Print dropped count.
    // 2. BUILD: idx := must(index.NewIndex(*dbPath)); must(idx.RebuildFromOpLog(ctx, ops.NewReader(*tmpOps)))
    // 3. VERIFY: entry count > 0; max(entry timestamp) <= cutoff. Exit non-zero on any error.
}
```

Expected drops: wo1-ledger-engine 3, wo4-gates 3, wo6-pull-tools 9, others 0. The runner asserts these counts.

**Content leak test (once per WO, before the batch):**
- Collect the identifiers the oracle **adds**: new exported funcs, types and methods in the oracle diff.
- Remove any the WO text names.
- None of the remaining identifiers may appear in any entry text of the filtered snapshot.
- For example, `HandleMemoryHistory` and `HandleMemoryNote` must be absent from wo6-pull-tools' filtered snapshot unless its WO text names them.

**Follow-up:** find out how WO2 produced the snapshots. The PR-08 ones were cut at or after their implementation commits, and the same method built all of them.

### WO text extraction (N1 fix)
```bash
WO_TEXT=$(git -C "${REPO_ROOT}" show "${WO_COMMIT}:${WO_PATH}" | sed '/^## Completion Report/,$d')
```
Verify that the extracted text has no ticked `- [x]` checklist items.

### Environment (D2.7)
- Launch agy with `env -i` plus an allowlist: `PATH`, `HOME`, `USER`, `TMPDIR`, the Go variables (`GOROOT`, `GOPATH`, `GOCACHE`, `GOMODCACHE`), and whatever agy needs for auth.
- Pass `REPO_MEMORY_DATA_DIR` explicitly to agy, and in the hook commands and `mcp_config.json`.
- No other `REPO_MEMORY_*` variable, except `REPO_MEMORY_DISABLED=1` for (a) and (b).
- Share the Go build cache (it's content-keyed); keep `-count=1`.
- Pin `--model` with the exact string from `agy models`.
- Don't use agy interactively during the batch.

## Phase 3: Hidden Tests + Scoring

### Oracle commit identification
For each WO:
- `ORACLE_SHA` = the implementation commit(s) between `WO_COMMIT` and the PA "WOn complete" commit.
- Test files: `git diff --name-only --diff-filter=AM <WO_COMMIT>..<WO-complete> -- '*_test.go'`.
- Take the tests from the WO's **completion commit**, not HEAD.

Known: wo1-op-log → `internal/core/ops/ops_test.go` (complete commit `4004825`). wo4-gates → `cmd/repo-memory/hook_test.go` and `internal/harness/antigravity/pretooluse_test.go` (complete commit `6a840b3`).

### Hidden test procedure (D2.3)
1. **Secondary metric:** run the agent's own tests in the WO's packages and record the result.
2. **Replace** each oracle-touched package's entire test set with the completion commit's version.
3. Copy any `testdata/` and fixtures added in the oracle range.
4. **Primary:** `go test -json -count=1 -run '^(<primary set>)$'`. Count top-level `Test*` functions; one passes only if all its subtests pass.
5. **Regression:** run the calibrated regression set (below) the same way.
6. **Compile failure** → fraction = 0, recorded as `hidden_compile_ok: false`. Report the compile-ok rate per condition, and the fraction among compile-ok runs.

### Per-WO API audit (D2.3.5)
Before using a WO, verify:
- the oracle tests use only exported identifiers that the WO text names;
- there are no internal (`package foo`) test references to unexported names;
- spot-check signatures as well as names.

✅ wo1-op-log is done: 13 test functions, 25 exported identifiers, all named in the WO text, no unexported dependencies. Repeat for the other 6 WOs.

### Calibration (no agent; once per WO × condition) (E2.2)

| Check | Tree | Must |
|---|---|---|
| **Base** | Each condition's prepared tree (Phase 1 + 2, no agent) | Primary set **FAILS** in all 3 |
| **Oracle × condition** | Each condition's prepared tree + the oracle's non-test changes, **excluding** `.agents/skills/principal-architect-workspace/**` and v1 `.memory/` paths | Primary set **PASSES** in all 3 |

How the two sets are derived:
- **Primary set** = the test functions added or modified in the oracle range, minus any that pass on the Base tree (no signal).
- **Regression set** = the other tests in the touched packages that pass on the oracle × condition tree. That excludes `TestSessionAttributor_Integration` (clock-dependent) and PA-dependent tests such as `cmd/repo-memory/cmd_import_pa_test.go`.
- **wo4-gates:** no separate skip or fix needed. Its oracle tests don't include the clock test, so the selection above handles it.

## Phase 4: Result Schema + Audit

### Result row schema (D2.6)
```json
{
  "work_order": "wo1-op-log",
  "condition": "no_memory",
  "replicate": 1,
  "attempt": 1,
  "schedule_index": 7,
  "block": 1,
  "hidden_tests_total": 13,
  "hidden_tests_passed": 11,
  "hidden_tests_fraction": 0.846,
  "hidden_compile_ok": true,
  "agent_tests_pass": true,
  "regression_tests_total": 5,
  "regression_tests_passed": 5,
  "agy_exit_code": 0,
  "total_tokens": 339600,
  "duration_seconds": 831,
  "conversation_id": "ca393b4b-...",
  "base_sha_original": "42deb642bda9...",
  "base_sha_filtered": "a1b2c3d4...",
  "snapshot_ops_dropped": 0,
  "condition_manifest_hash": "sha256:abc...",
  "pin_commit": "…",
  "binary_sha256": "sha256:def...",
  "build_index_sha256": "sha256:…",
  "agy_version": "2.1.0",
  "model_name": "<exact agy models string>",
  "agent_resolved": "default",
  "delivery_count": null,
  "tool_call_count": 38,
  "mcp_tool_calls": 0,
  "transcript_scan_ok": true,
  "tick_activity": false,
  "invalid_reason": null,
  "error": "",
  "timestamp": "2026-10-10T12:00:00Z"
}
```

Notes:
- `delivery_count` is `null` for (a) and (b), and the ledger record count for (c).
- `status` and `exit_tests_pass` are removed (v2 leftovers).
- The model is pinned via `--model` and recorded.

### Invalid run handling (D2.4, D2.5)
- Mark the attempt `invalid_*`, keep it in the results file, and retry (max 2 attempts).
- Reasons:
  - `invalid_error`: agy exit ≠ 0, status ≠ SUCCESS, or `RESOURCE_EXHAUSTED` even with exit 0;
  - `invalid_contamination`: snapshots, transcript scan or tick activity;
  - `invalid_no_hooks`: (c) with no ledger file.
- A cell invalid on both attempts → **missing** (not fail).
- Report invalid and missing rates per condition.

### Run order (E2.4)
- After the pilot, generate one shuffled, **interleaved (a)/(c) schedule** in blocks. Record the RNG seed.
  - **Block 1** = every calibrated WO × {(a), (c)} × 1. It is the difficulty probe **and** the first paired replicate.
  - **After Block 1:** drop WOs where both conditions score 1.0 or both score 0 with `hidden_compile_ok: true` (saturated or impossible).
  - **Blocks 2…n:** more paired replicates on the remaining WOs until the budget is spent.
  - **(b)** is added as an interleaved arm only if budget remains after the (a)/(c) target.
- The schedule is resumable: skip existing valid results by `schedule_index`.
- Delete the previous run's `RUN_ROOT` before starting the next.

### Pre-registered analysis (write and commit BEFORE Block 1)
- **Unit:** per-WO mean over replicates of `hidden_tests_fraction(c) − hidden_tests_fraction(a)`.
- **Estimate:** mean of the per-WO differences, with a 95% bootstrap CI (resampling WOs).
- **Kill-gate rule (fill in before Block 1):**
  - PASS if estimate ≥ __ and CI lower bound > __;
  - FAIL if CI upper bound < __;
  - otherwise INCONCLUSIVE.
- **Secondary:** compile-ok rate, tokens, wall time, regression pass rate, per condition.
- Any later change to the analysis must be logged as a deviation.

### Artifact archival
- Per-run archive: `docs/evaluations/run2/<schedule_index>-<wo>-<condition>-rep<N>/`.
- Archive the agy output JSON, the stderr log, the hidden-test, regression and agent-test logs, and the transcript scan report.
- For (c), also archive `$RUN_ROOT/data/`; its ledger is the injection evidence.
- Keep transcripts out of git (size, and local paths).

## Budget + Staging (D2.8, E2.4)

**Subscription: Google AI Ultra \$200/mo (20× Pro quota, 5h refresh window).**

No hard token cap. The constraint is wall time across quota windows. Strategy:

| Stage | Runs | Purpose |
|---|---|---|
| 0. Calibration + leak tests | 0 agent runs | Phase 3 calibration, snapshot leak test |
| 1. Pilot | 4 (2 WOs × (a)+(c) × 1) | Validate harness per A9 checklist + E-items |
| 2. Block 1 | ≤14 (7 WOs × (a)+(c)) | Probe + first paired replicate |
| 3. Blocks 2…n | as many as needed | More paired replicates on viable WOs |
| 4. (b) arm | after (a)/(c) target met | PA comparison |

**Quota exhaustion handling:**
- Detect: agy exit ≠ 0 + `RESOURCE_EXHAUSTED` in stderr or JSON error (any duration, not just <30s)
- Record attempt as `invalid_quota`, do NOT count toward the 2-attempt retry cap
- Log the quota reset time from the error message
- Set a timer for the reset time + 5 minutes, then resume the schedule
- The schedule is fully resumable by `schedule_index`

Pilot runs count toward the analysis only if the harness is unchanged after the pilot.

## Product Findings (D1.9, E1.1)

> [!WARNING]
> **The live plugin can't deliver memory on this repo today:**
> - **Index wiring:** `reindex` writes `.memory/.cache/index.db`, the tick pipeline writes `<data>/<repoID>/index.sqlite`, and the hooks and MCP read `<data>/<repoID>/index.db`. The live `index.db` has **0 entries**.
> - **Stale binary:** the installed hooks, MCP server and tick sidecar all run the main checkout's `./repo-memory`, built 2026-10-06. It predates the delivery engine (`7bff5a4`, 2026-10-07).

Condition (c) works only because the runner builds `index.db` by hand and pins a current binary. The verdict must say:
1. This tests **capability** ("does delivery help when the index is populated?"), not the shipped setup path.
2. A positive result doesn't validate the product's own index-building pipeline or install.
3. Both are fixed separately, as a product WO: index wiring, plus a binary rebuild and reinstall.

## Pilot checklist (A9 + E-items)
- [ ] Phase 0 binary contains the delivery engine; `PIN_COMMIT` and sha256 values recorded
- [ ] Clone validations 1–6 pass, including `log --all -p` showing no `/Users/mcneillm`
- [ ] Main checkout, sibling dirs, the real data dir, and agy `implicit/` and `knowledge/` unchanged
- [ ] (c): ledger exists with ≥1 injection record; (a): no ledger
- [ ] (c): index helper dropped the expected op count; leak test clean
- [ ] Allowlist transcript scan clean for all 4 runs
- [ ] No tick activity in `$RUN_ROOT/data` (or the daemon is disabled and the run is re-piloted)
- [ ] Hidden tests: Base FAIL and oracle × condition PASS for both pilot WOs
- [ ] agy exit 0, `status` SUCCESS, `error` empty
- [ ] Result row has every schema field populated

## Next Steps

1. ☐ **Phase 0:** commit `evaluation/cmd/build-index`, choose `PIN_COMMIT`, build and verify the binaries
2. ☐ **Snapshots:** run the helper's cutoff for all 7 WOs (expect drops of 3/3/9 on the PR-08 WOs) and the content leak test; investigate how WO2 cut the snapshots
3. ☐ **Oracle commits + API audit** for all 7 WOs
4. ☐ **Implement runner v3:** clone + filter-repo (paths + `--replace-text`), content validations, `RUN_ROOT` layout, per-run plugin dir (`plugin.json`, env-prefixed hooks, MCP env), allowlist transcript scan, tick check, hidden tests
5. ☐ **Calibration** (Base and oracle × condition), all 7 WOs × 3 conditions, no agent
6. ☐ **Write and commit the pre-registered analysis**, with the kill-gate thresholds filled in
7. ☐ **Pilot:** 2 WOs × (a)+(c) × 1, against the checklist above; decide on the tick sidecar
8. ☐ **Block 1**, then prune WOs and run Blocks 2…n
9. ☐ **Separately:** product WO for the index wiring and the binary rebuild and reinstall

---

## Review log

- **Round 1 (D-items):** all applied in the previous revision.
- **Round 2 (E-items), applied in this revision:**
  - E1.1 pinned build → Decision 9, Phase 0
  - E1.2 cutoff for all WOs + filtering helper + leak test → Decision 8, WO Set table, 2b
  - E1.3 history-wide home-path rewrite → 1b `replace-paths.txt` + validations 4–5
  - E1.4 filter-repo ordering + content-based validation → 1b
  - E1.5 hook env via command prefix + `plugin.json` → 1a, (c) step 2
  - E1.6 tick sidecar → Decision 10, 1e
  - E2.1 allowlist transcript scan → 1d
  - E2.2 calibration redefinition → Phase 3 Calibration
  - E2.3 wo4-gates clock test handled by selection → Phase 3
  - E2.4 interleaved blocks + pre-registration → Run order, Pre-registered analysis, Budget
  - E2.5 pre-launch status re-check → Phase 2
- **Open decisions for you:**
  - the kill-gate thresholds (pre-registration);
  - whether to move old CLI conversation dirs during the batch (1d);
  - what the 7M budget counts.
