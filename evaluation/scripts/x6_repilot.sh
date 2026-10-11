#!/usr/bin/env bash
# X6-lite Re-Pilot — condition (c) only, with fixed delivery channel
# The (a) runs from the previous pilot are kept (harness unchanged for those).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PIN_COMMIT=$(cat "${REPO_ROOT}/evaluation/pin/PIN_COMMIT")

echo "═══════════════════════════════════════════════════════"
echo "  X6-lite v3 RE-PILOT — 2 dossier runs"
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
    echo "  Re-pilot run ${IDX}: ${WO} / ${COND} / rep${REP}"
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
           SCHEDULE_INDEX="${IDX}" ATTEMPT=2 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || EXIT=$?
           if [[ ${EXIT} -eq 2 ]]; then
               echo "Still exhausted. Sleeping 30 minutes..."
               sleep 1800
               SCHEDULE_INDEX="${IDX}" ATTEMPT=3 bash "${RUNNER}" "${WO}" "${COND}" "${REP}" || true
           fi
           ;;
    esac
}

# Only re-run dossier conditions (a) runs kept from previous pilot
run_one 2 wo1-op-log dossiers 1
run_one 4 wo4-gates dossiers 1

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  RE-PILOT COMPLETE"
echo "═══════════════════════════════════════════════════════"
for r in "${RESULTS[@]}"; do
    echo "  ${r}"
done
echo ""
echo "Results: ${REPO_ROOT}/evaluation/results/x6-lite-results-v3.jsonl"
echo ""

if [[ "${QUOTA_HIT}" == "true" ]]; then
    echo "⚠️  Quota was hit during the re-pilot."
fi
