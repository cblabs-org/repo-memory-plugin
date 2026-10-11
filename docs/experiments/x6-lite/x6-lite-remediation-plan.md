# X6-lite Batch Review — Remediation Plan

**Reviewer:** Claude Code / Opus 5.5 (2026-10-11)  
**Verdict:** All 6 reviewer items verified. The review is correct.

---

## Verification Status

| Item | Claim | Verified? | Impact |
|------|-------|:---------:|--------|
| W1 | Earlier audit used broken grep (ugrep `\b` doesn't match) | ✅ Correct | My F1 inherited the error |
| W2 | 5 floor-effect WOs fail because oracle uses unspecified identifiers | ✅ Correct | 10/14 runs measure name-guessing, not capability |
| W3 | Runner doesn't delete agent `_test.go` before copying oracle | ✅ Confirmed in runner L593-601 | Causes redeclared errors in wo4-index-migration(a) |
| W4 | wo6 "signal" is naming coincidence — no injected entry mentions `WithOpsDir` | ✅ Zero `OpsDir` in ephemeral messages; (a) agent used `WithOpsWriter`, (c) used `WithOpsDir` | The +3/8 is not attributable to memory |
| W5 | Pre-registered analysis not applied, agent code not archived | ✅ Correct | Results are INCONCLUSIVE |
| W6 | Fix: append API contract to WO prompt (same in both conditions) | ✅ Sound approach | Measures implementation, not name-guessing |

---

## Fixes Required

### Fix 1: API Contract Appendix (W2, W6) — HIGH PRIORITY

For each WO, extract every exported identifier and signature the oracle tests reference, and append them to the WO prompt text. **Both conditions get the same appendix** so it doesn't bias the comparison.

**Method:** For each oracle test file, use `go doc` or AST walk on the oracle commit to extract:
- Exported type names and their signatures
- Exported function/method signatures  
- Exported constants and their values
- Struct field names referenced by tests

**Generate mechanically, review by hand** to ensure no implementation hints leak.

Example for wo1-op-log:
```
## API Contract (exact identifiers for test compatibility)

const (
    OpAdd   OpType = "add"
    OpAmend OpType = "amend"
)

func NewBatchID() string
func NewBatchIDWithTime(t time.Time) string  
func NewEntryID() string
const SourceTurnNote = "turn-note"
```

### Fix 2: Test-Set Replacement (W3) — HIGH PRIORITY

Before copying oracle tests, delete ALL `_test.go` files in the WO's target package:

```bash
# In x6_runner_v3.sh, before the oracle copy loop:
WO_PKG_DIR=$(echo "${WO_PKG}" | sed 's|^\./||; s|/\.\.\.$||')
find "${RUN_DIR}/${WO_PKG_DIR}" -name '*_test.go' -delete
```

Add a **calibration check**: after replacing, confirm `go build ./...` passes with a dummy extra test file to catch any remaining conflicts.

### Fix 3: Archive Agent Diff (W5) — MEDIUM PRIORITY

After the agy run completes, archive the agent's code changes:

```bash
# After step 5 (agy run), before step 6 (contamination):
git -C "${RUN_DIR}" diff HEAD > "${RUN_ROOT}/agent-diff.patch"
git -C "${RUN_DIR}" diff HEAD --stat > "${RUN_ROOT}/agent-diff-stat.txt"
git -C "${RUN_DIR}" ls-files --others --exclude-standard > "${RUN_ROOT}/agent-untracked.txt"
```

This allows re-scoring without re-running.

### Fix 4: Analysis Set Field (W5) — LOW PRIORITY

Add `"analysis_set": "block1"` to the result JSON so pilot, batch, and re-run rows can be distinguished.

### Fix 5: Record Injected Token Count (W5) — MEDIUM PRIORITY

Parse the ledger to sum the total tokens injected per run:

```bash
INJECTED_TOKENS=$(python3 -c "
import json, sys
l = json.load(open(sys.argv[1]))
total = sum(inj.get('TokenCount', 0) for inj in l.get('Injections', []))
print(total)
" "${LEDGER_FILE}" 2>/dev/null || echo "0")
```

### Fix 6: `about_to_edit` Issue (W5) — LOW PRIORITY

Create GitHub issue documenting the product limitation (model decides and edits in same turn).

---

## Estimated Effort

| Fix | Work | Risk |
|-----|------|------|
| API contract generation | ~2h (script + manual review per WO) | Medium — must not leak implementation hints |
| Test-set replacement | ~15 min (3 lines in runner) | Low |
| Agent diff archive | ~15 min (4 lines in runner) | Low |
| Analysis set field | ~5 min (1 line in runner) | Low |
| Injected token count | ~15 min | Low |
| Re-run Block 1 | ~3h wall clock, ~4.8M tokens | Medium — quota management |

**Total: ~6h including re-run, ~4.8M tokens**

---

## Corrected Interpretation

The batch results should be reported as:

> **INCONCLUSIVE — measurement invalid.** 12 of 14 runs failed to compile against the hidden tests because the oracle tests use exported identifiers not specified in the WO text. The scoring layer measures name-guessing, not task capability or memory effect. The only non-zero result (wo6-pull-tools 0→3/8) is a naming coincidence, not attributable to memory.  
>  
> **Infrastructure validated.** Clone isolation, delivery (29-60 injections/run), `first_touch` moments, junk filtering, and contamination checks all work correctly. The fix is in the scoring (API contract appendix), not the infrastructure.
