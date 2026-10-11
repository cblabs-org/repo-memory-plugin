#!/usr/bin/env bash
# W6 Remediation Pilot — 4 runs to demonstrate fixes
# wo4-index-migration: demonstrates W3 fix (test-set replacement)
# wo1-op-log: demonstrates W6 fix (API contract, most missing identifiers)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PIN_COMMIT=$(cat "${REPO_ROOT}/evaluation/pin/PIN_COMMIT")
export ANALYSIS_SET="w6-pilot"

echo "═══════════════════════════════════════════════════════"
echo "  W6 Remediation Pilot — 4 runs"
echo "  PIN: ${PIN_COMMIT:0:12}"
echo "  Fixes: W3 (test-set replace), W5 (agent diff), W6 (API contract)"
echo "═══════════════════════════════════════════════════════"

RUNNER="${REPO_ROOT}/evaluation/scripts/x6_runner_v3.sh"
RESULTS=()

run_one() {
    local IDX="$1" WO="$2" COND="$3"
    echo ""
    echo "━━━ Run ${IDX}/4: ${WO} / ${COND} ━━━"
    EXIT=0
    SCHEDULE_INDEX="${IDX}" bash "${RUNNER}" "${WO}" "${COND}" 1 || EXIT=$?
    case ${EXIT} in
        0) RESULTS+=("${IDX}: ${WO}/${COND} ✅") ;;
        1) RESULTS+=("${IDX}: ${WO}/${COND} ❌ INVALID") ;;
        2) RESULTS+=("${IDX}: ${WO}/${COND} ⛔ QUOTA")
           echo "Sleeping 10 minutes for quota..."
           sleep 600
           SCHEDULE_INDEX="${IDX}" ATTEMPT=2 bash "${RUNNER}" "${WO}" "${COND}" 1 || true
           ;;
    esac
}

# wo4-index-migration: had W3 redeclared bug
run_one 1 wo4-index-migration no_memory
run_one 2 wo4-index-migration dossiers

# wo1-op-log: most unspecified identifiers (9+)
run_one 3 wo1-op-log no_memory
run_one 4 wo1-op-log dossiers

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  PILOT COMPLETE"
echo "═══════════════════════════════════════════════════════"
for r in "${RESULTS[@]}"; do
    echo "  ${r}"
done
