#!/usr/bin/env bash
# X6-lite v3 Pilot — full 4-run pilot, all on current pin
# All runs use the same binary and pinned model for clean comparison.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PIN_COMMIT=$(cat "${REPO_ROOT}/evaluation/pin/PIN_COMMIT")

echo "═══════════════════════════════════════════════════════"
echo "  X6-lite v3 PILOT — 4 runs (unified pin)"
echo "  PIN_COMMIT: ${PIN_COMMIT:0:12}"
echo "═══════════════════════════════════════════════════════"
echo ""

RUNNER="${REPO_ROOT}/evaluation/scripts/x6_runner_v3.sh"
RESULTS=()
QUOTA_HIT=false

run_one() {
    local IDX="$1" WO="$2" COND="$3" REP="$4"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Pilot run ${IDX}/4: ${WO} / ${COND} / rep${REP}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    EXIT=0
    SCHEDULE_INDEX="${IDX}" bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || EXIT=$?
    
    case ${EXIT} in
        0) RESULTS+=("${IDX}: ${WO}/${COND} ✅ VALID") ;;
        1) RESULTS+=("${IDX}: ${WO}/${COND} ❌ INVALID") ;;
        2) RESULTS+=("${IDX}: ${WO}/${COND} ⛔ QUOTA")
           QUOTA_HIT=true
           echo ""
           echo "QUOTA EXHAUSTED — waiting for refresh..."
           echo "Sleeping 5 minutes, then will retry..."
           sleep 300
           # Retry
           SCHEDULE_INDEX="${IDX}" ATTEMPT=2 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || EXIT=$?
           if [[ ${EXIT} -eq 2 ]]; then
               echo "Still exhausted. Sleeping 30 minutes..."
               sleep 1800
               SCHEDULE_INDEX="${IDX}" ATTEMPT=3 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || true
           fi
           ;;
    esac
}

# All 4 runs: 2 WOs × {no_memory, dossiers}, same pin
run_one 1 wo1-op-log no_memory 1
run_one 2 wo1-op-log dossiers 1
run_one 3 wo4-gates no_memory 1
run_one 4 wo4-gates dossiers 1

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  PILOT COMPLETE"
echo "═══════════════════════════════════════════════════════"
for r in "${RESULTS[@]}"; do
    echo "  ${r}"
done
echo ""
echo "Results: ${REPO_ROOT}/evaluation/results/x6-lite-results-v3.jsonl"
echo ""

if [[ "${QUOTA_HIT}" == "true" ]]; then
    echo "⚠️  Quota was hit during the pilot."
fi
