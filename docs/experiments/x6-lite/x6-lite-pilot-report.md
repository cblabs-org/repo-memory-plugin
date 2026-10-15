# X6-lite v3 Batch Report — FINAL

**Date:** 2026-10-10/11  
**PIN_COMMIT:** `3036eb46c45a` (all valid runs)  
**Model:** `gemini-3.8-flash-high` (pinned)  
**Valid runs:** 14/14 (7 WOs × 2 conditions × 1 replicate)  
**Total tokens:** 4,841,146

---

## Results Table

| # | Work Order | Condition | Hidden Tests | Fraction | Tokens | Duration | Delivery | Moments |
|---|-----------|-----------|:------------:|:--------:|-------:|---------:|---------:|--------:|
| 1 | wo1-op-log | no_memory | 0/13 | 0.000 | 289,963 | 576s | — | — |
| 2 | wo1-op-log | dossiers | 0/13 | 0.000 | 345,166 | 481s | **29** | task+touch |
| 3 | wo2-dossier-schema | no_memory | 0/3 | 0.000 | 397,463 | 626s | — | — |
| 4 | wo2-dossier-schema | dossiers | 0/3 | 0.000 | 251,112 | 541s | **32** | task+touch |
| 5 | wo3-scopes-aliases | no_memory | 0/7 | 0.000 | 290,921 | 655s | — | — |
| 6 | wo3-scopes-aliases | dossiers | 0/7 | 0.000 | 295,065 | 641s | **38** | task+touch |
| 7 | wo4-index-migration | no_memory | 0/9 | 0.000 | 331,179 | — | — | — |
| 8 | wo4-index-migration | dossiers | 0/9 | 0.000 | 606,646 | — | **43** | task+touch |
| 9 | wo1-ledger-engine | no_memory | 0/5 | 0.000 | 326,795 | — | — | — |
| 10 | wo1-ledger-engine | dossiers | 0/5 | 0.000 | 301,493 | — | **38** | task+touch |
| 11 | wo4-gates | no_memory | **5/5** | **1.000** | 311,105 | — | — | — |
| 12 | wo4-gates | dossiers | **5/5** | **1.000** | 306,643 | — | **57** | task+touch |
| 13 | wo6-pull-tools | no_memory | 0/8 | 0.000 | 431,290 | 554s | — | — |
| 14 | wo6-pull-tools | dossiers | **3/8** | **0.375** | 356,305 | 538s | **60** | task+touch |

---

## Key Findings

### 1. Memory Effect on Test Pass Rate

```
WO                          no_memory    dossiers     Δ (dossier − no_memory)
─────────────────────────── ──────────── ──────────── ──────────────────────
wo1-op-log                  0/13 (0.0)   0/13 (0.0)   0  (floor)
wo2-dossier-schema          0/3  (0.0)   0/3  (0.0)   0  (floor)
wo3-scopes-aliases          0/7  (0.0)   0/7  (0.0)   0  (floor)
wo4-index-migration         0/9  (0.0)   0/9  (0.0)   0  (floor)
wo1-ledger-engine           0/5  (0.0)   0/5  (0.0)   0  (floor)
wo4-gates                   5/5  (1.0)   5/5  (1.0)   0  (ceiling)
wo6-pull-tools              0/8  (0.0)   3/8  (0.38)  +3 ✨
─────────────────────────── ──────────── ──────────── ──────────────────────
TOTAL                       5/50 (0.10)  8/50 (0.16)  +3/50 (+6pp)
```

### 2. Token Efficiency

| WO | no_memory | dossiers | Δ% |
|---|---:|---:|---:|
| wo1-op-log | 289,963 | 345,166 | **+19%** |
| wo2-dossier-schema | 397,463 | 251,112 | **−37%** |
| wo3-scopes-aliases | 290,921 | 295,065 | +1% |
| wo4-index-migration | 331,179 | 606,646 | **+83%** |
| wo1-ledger-engine | 326,795 | 301,493 | **−8%** |
| wo4-gates | 311,105 | 306,643 | **−1%** |
| wo6-pull-tools | 431,290 | 356,305 | **−17%** |
| **Mean** | **341,245** | **380,347** | **+11%** |
| **Median** | **326,795** | **306,643** | **−6%** |

Mixed picture: median is 6% lower with dossiers, but mean is 11% higher (driven by wo4-index-migration outlier at +83%).

### 3. Delivery System Performance

All 7 dossier runs had successful delivery:

| WO | Injections | Index Entries |
|---|---:|---:|
| wo1-op-log | 29 | 147 |
| wo2-dossier-schema | 32 | ~147 |
| wo3-scopes-aliases | 38 | ~148 |
| wo4-index-migration | 43 | ~155 |
| wo1-ledger-engine | 38 | ~160 |
| wo4-gates | 57 | 169 |
| wo6-pull-tools | 60 | 172 |

**Delivery is correlated with index size** — larger indexes deliver more content because more anchors match. `first_touch` moments consistently fire alongside `task_start`.

---

## Interpretation

### wo6-pull-tools: The Signal (0 → 3/8)

This is the **only WO where memory made a measurable difference at n=1**:
- **no_memory:** Compile fail — agent built the MCP server types but missed helper functions
- **dossiers:** 3/8 hidden tests pass — agent got enough context from 60 injected entries to implement some helpers correctly
- **Token savings:** −17% (356K vs 431K) — memory helped the agent work more efficiently

> [!IMPORTANT]
> At n=1, this could be sampling noise. A 3/8 pass on one run doesn't prove causation. Replicates are needed to determine if the effect is reliable.

### wo4-gates: Ceiling Effect (5/5 both)

Both conditions achieve perfect scores. This WO is "too easy" for the model — it completes the task regardless of memory. The dossier condition used 1% fewer tokens, suggesting memory didn't hurt but didn't measurably help.

### 5 WOs: Floor Effect (0/0 both)

All compile-fail in both conditions. The agent builds core types/structs but consistently misses helper functions that the hidden tests call. Memory entries (decisions, dead-ends, open-issues) don't provide enough implementation guidance to cross the compile threshold. This is a **product quality signal** — the current seeded content may not be the right content for this task type.

---

## Product Fixes Validated

| Fix | Commit | Impact |
|---|---|---|
| Delivery channel (`systemMessage` → `ephemeralMessage`) | `8d7ad7b` | Content now reaches model |
| Tool call parser (Antigravity native format) | `d57f561` | `first_touch` moments fire (41-60 per run) |
| Junk entry filter (seeder root cause) | `3036eb4` | 10% of entries were placeholder text |

---

## Recommendations

### For Replication
1. **wo6-pull-tools** needs replicates (n≥3) to confirm the 0→3/8 effect
2. **wo4-gates** provides no signal at n=1 (ceiling) — consider dropping or replacing with harder WO
3. The 5 floor-effect WOs need investigation: are the hidden tests too demanding, or is the memory content not useful enough?

### For Product
1. **Seeded content quality:** The current "decision/dead-end/open-issue" entries from PA reports may be too abstract. Consider seeding with more concrete implementation hints (e.g., function signatures, test patterns).
2. **Index size scaling:** wo6-pull-tools had 172 entries and 60 injections. Monitor whether larger indexes cause context overload.
3. **about_to_edit never fires:** Known limitation — the model decides and edits in the same turn. Consider redesigning this moment or removing it.
4. **Migration scripts needed:** [Issue #16](https://github.com/cblabs-org/repo-memory-plugin/issues/16) — clean junk from existing ops logs.

---

## Raw Data

- **Results JSONL:** ``evaluation/results/x6-lite-results-v3.jsonl`` (29 rows: 18 valid + 11 invalid/historical)
- **Archives:** `evaluation/results/archive/{1..14}-*-rep1/`
- **Run 1 archive:** `evaluation/archive/run1-invalid/` (invalidated 52-run batch)
- **Pilot archive:** `evaluation/results/x6-lite-pilot-results.jsonl` (not created — pilot data is in the v3 JSONL)

---

## Reviewer Comments (2026-10-11, Claude Code / Opus 5.5)

**Verdict: these results can't answer the memory question yet. Please don't report the +6pp or the wo6-pull-tools "signal".**

12 of 14 runs failed to compile against the hidden tests. Reading the compile errors in `archive/*/hidden-tests.json` shows that, for every one of those WOs, the hidden tests require identifiers or signatures that the WO text never specifies, or specifies differently. In one run the harness itself caused the failure.

So the floor effect measures how well the agent guessed the original implementer's names. It says nothing about task capability or memory.

The good news: the infrastructure (isolation, delivery, moments, cutoff, junk filter) is now working, and the fix is in the scoring, not the runs' setup.

### W1. Correction to my own earlier audit

My earlier comments (D2.3, R2) said wo1-op-log's oracle tests use only identifiers named in the WO text. **That was wrong.** My check used the shell's `grep`, which on this machine is a ugrep wrapper running in basic-regex mode, so `\b…\b` never matched and every identifier looked "named". Re-running with `/usr/bin/grep -w`:

| Identifier used by oracle `ops_test.go` | Occurrences in test | In WO text |
|---|---:|---:|
| `OpAdd` | 21 | **0** (the WO only says "`OpType` string enum: `add`, …") |
| `NewBatchIDWithTime` | 10 | **0** |
| `NewBatchID` | 5 | **0** |
| `OpAmend` | 4 | **0** |
| `NewEntryID` | 2 | **0** |
| `SourceTurnNote` | 2 | **0** |
| `ValidateOp`, `NewWriter` | 1, 7 | 2, 1 ✓ |

The report's F1 statement ("These are named in the WO text") inherited my error. I'm sorry for the misdirection.

### W2. Why each floor-effect WO fails to compile

| WO | Compile error (both conditions unless noted) | WO text at `WO_COMMIT` | Cause |
|---|---|---|---|
| wo1-op-log | `undefined: OpAdd`, `NewBatchID`, `NewEntryID`, `SourceTurnNote` | None of these named | **Unspecified API** |
| wo2-dossier-schema | `undefined: EntryTypeInvariant` | Defines an `EntryType` enum with value `invariant`, but no constant names | **Unspecified API** |
| wo3-scopes-aliases | `DeclaredScope` / `ModuleScope` has no field `Kind` | Struct fields listed without `Kind` (the "Kind" mentions are about dossiers) | **Unspecified API** |
| wo4-index-migration | `undefined: index.ErrNotFound` | Not mentioned | **Unspecified API** |
| wo4-index-migration (a) only | `TestRebuildFromOpLog_EndToEnd redeclared` (and one more) | — | **Harness bug** (W3) |
| wo1-ledger-engine | `too many arguments in call to eng.GateEntries` (test passes a `*Ledger`) | WO line 199: `GateEntries(ctx context.Context, targetFile string)` | **Hidden test contradicts the WO.** The agent implemented the specified signature |
| wo6-pull-tools (a) | `undefined: mcpserver.WithOpsDir` | Not mentioned (only "requires writing to the ops directory") | **Unspecified API** |

The original implementers made API choices beyond the WO, or departed from it, and the hidden tests encode those choices. An agent can only compile against them by guessing.

### W3. Harness bug: the test set isn't replaced

wo4-index-migration (a) fails partly because `TestRebuildFromOpLog_EndToEnd` and `TestBackwardCompatibility_EntitiesAndClaims` are declared both in the oracle's `v2_test.go` and in `index_test.go:536`. The oracle commit declares the first only in `v2_test.go`, so the duplicate comes from the **agent's** `index_test.go`. The plan's step "replace the package's *entire* test set with the oracle's" wasn't implemented: agent test files that the oracle doesn't overwrite are left in place. Delete every `_test.go` in the touched packages, then copy the oracle commit's full test set.

### W4. The wo6-pull-tools "signal" is a naming coincidence

- **The (c) agent got past the compile error only because it named an option `WithOpsDir`, matching the oracle.** The (a) agent named its option `WithOpsWriter` (both also defined `WithLedger`). After that, 3 of 8 tests pass.
- **No injected entry mentions it.** None of the filtered index's entries contains `OpsDir`/`ops dir`, and no `EPHEMERAL_MESSAGE` step in the (c) transcript mentions it.
- **So it isn't attributable to memory.** The 0 → 3/8 is the agent's naming choice crossing the compile threshold.
- The token deltas (−37% … +83%) at n=1 per cell are noise. The "delivery correlates with index size" observation is confounded with the WO (7 points, one per WO).

### W5. Other points

- **The pre-registered analysis isn't applied.** The plan required committing the kill-gate rule before Block 1, and the report doesn't evaluate it. Under any reasonable rule, these data are **INCONCLUSIVE (measurement invalid)**, not a pass or a fail.
- **The agent's code wasn't archived.** The archives keep logs and the data dir, so these runs can't be re-scored without re-running the agents. From now on, archive `git -C $RUN_DIR diff <base>` plus untracked files per run, so scoring changes never need new runs.
- **Injection volume (29–60 per run) is high.** Record the total injected tokens per run, and check whether the moment budgets in `DefaultBudgets` are being respected. That number is needed anyway to interpret any token effect.
- **`about_to_edit` never firing** is a product finding worth its own issue, as the report suggests.
- **The JSONL has 29 rows** (18 valid, 11 invalid or historical). Add a field marking which rows belong to this analysis (e.g. `analysis_set: "block1"`), so pilot and batch rows don't mix.

### W6. Recommended fix (cheap, keeps the WO set)

1. **Give every condition the API contract.** For each WO, extract the exported identifiers and signatures the oracle tests use, and append a short "API contract" section to the WO prompt **in all conditions** (e.g. "`func NewBatchID() string`, `const OpAdd OpType = "add"`, `func (e *Engine) GateEntries(ctx, file string, l *Ledger) …`"). This measures implementation correctness, not name guessing, and it's the same for (a) and (c), so it doesn't bias the comparison. Generate it mechanically with `go/types` or `go doc` on the oracle package, then review each by hand.
2. **Re-audit with an exact matcher.** Use `/usr/bin/grep -w` or a Go AST walk to confirm every identifier the oracle tests use appears in the prompt.
3. **Fix the test-set replacement (W3)** and add a calibration check for it: apply the oracle's non-test code plus a dummy extra agent test file, and confirm the hidden tests still compile.
4. **Archive each agent's diff** (W5).
5. **Re-run Block 1** (14 runs, ~4.8M tokens at this run's rate). Then apply the pre-registered rule before deciding on more replicates.

If the budget can't cover a re-run, the next-best option is to report this batch as a validated harness with an invalid scoring layer, and to commit the W6 changes for the next attempt.
