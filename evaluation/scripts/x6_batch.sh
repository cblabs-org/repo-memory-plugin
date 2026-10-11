#!/usr/bin/env bash
# X6-lite v3 Full Batch — 7 WOs × 2 conditions × 1 replicate = 14 runs
# Sequential execution with quota recovery.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PIN_COMMIT=$(cat "${REPO_ROOT}/evaluation/pin/PIN_COMMIT")

echo "═══════════════════════════════════════════════════════"
echo "  X6-lite v3 FULL BATCH — 14 runs"
echo "  PIN_COMMIT: ${PIN_COMMIT:0:12}"
echo "  Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "═══════════════════════════════════════════════════════"
echo ""

RUNNER="${REPO_ROOT}/evaluation/scripts/x6_runner_v3.sh"
RESULTS=()
QUOTA_HIT=false
TOTAL_RUNS=14
RUN_NUM=0

run_one() {
    local IDX="$1" WO="$2" COND="$3" REP="$4"
    RUN_NUM=$((RUN_NUM + 1))
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Run ${RUN_NUM}/${TOTAL_RUNS} (idx=${IDX}): ${WO} / ${COND} / rep${REP}"
    echo "  $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    EXIT=0
    SCHEDULE_INDEX="${IDX}" bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || EXIT=$?
    
    case ${EXIT} in
        0) RESULTS+=("${IDX}: ${WO}/${COND} ✅ VALID") ;;
        1) RESULTS+=("${IDX}: ${WO}/${COND} ❌ INVALID") ;;
        2) RESULTS+=("${IDX}: ${WO}/${COND} ⛔ QUOTA (attempt 1)")
           QUOTA_HIT=true
           echo ""
           echo "QUOTA EXHAUSTED — sleeping 10 minutes..."
           sleep 600
           # Retry
           EXIT=0
           SCHEDULE_INDEX="${IDX}" ATTEMPT=2 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || EXIT=$?
           case ${EXIT} in
               0) RESULTS+=("${IDX}: ${WO}/${COND} ✅ VALID (attempt 2)") ;;
               1) RESULTS+=("${IDX}: ${WO}/${COND} ❌ INVALID (attempt 2)") ;;
               2) RESULTS+=("${IDX}: ${WO}/${COND} ⛔ QUOTA (attempt 2)")
                  echo "Still exhausted. Sleeping 30 minutes..."
                  sleep 1800
                  SCHEDULE_INDEX="${IDX}" ATTEMPT=3 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || true
                  ;;
           esac
           ;;
    esac
}

# Interleaved schedule: alternate (a) and (c) to spread quota load
# Each WO pair runs together so the agent tasks are adjacent for comparison.
WOS=(wo1-op-log wo2-dossier-schema-renderer wo3-scopes-aliases wo4-index-migration wo1-ledger-engine wo4-gates wo6-pull-tools)

IDX=1
for WO in "${WOS[@]}"; do
    run_one "${IDX}" "${WO}" no_memory 1
    IDX=$((IDX + 1))
    run_one "${IDX}" "${WO}" dossiers 1
    IDX=$((IDX + 1))
done

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  BATCH COMPLETE — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "═══════════════════════════════════════════════════════"
for r in "${RESULTS[@]}"; do
    echo "  ${r}"
done
echo ""
echo "Results: ${REPO_ROOT}/evaluation/results/x6-lite-results-v3.jsonl"
echo ""

if [[ "${QUOTA_HIT}" == "true" ]]; then
    echo "⚠️  Quota was hit during the batch."
fi
