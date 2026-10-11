#!/usr/bin/env bash
# W6 baseline sweep — no_memory only on 4 untested WOs
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PIN_COMMIT=$(cat "${REPO_ROOT}/evaluation/pin/PIN_COMMIT")
export ANALYSIS_SET="w6-baseline"

echo "═══════════════════════════════════════════════════════"
echo "  W6 Baseline Sweep — 4 WOs × no_memory"
echo "  PIN: ${PIN_COMMIT:0:12}"
echo "═══════════════════════════════════════════════════════"

RUNNER="${REPO_ROOT}/evaluation/scripts/x6_runner_v3.sh"
RESULTS=()

run_one() {
    local IDX="$1" WO="$2"
    echo ""
    echo "━━━ Run ${IDX}/4: ${WO} / no_memory ━━━"
    EXIT=0
    SCHEDULE_INDEX="${IDX}" bash "${RUNNER}" "${WO}" no_memory 1 || EXIT=$?
    case ${EXIT} in
        0) RESULTS+=("${IDX}: ${WO} ✅") ;;
        1) RESULTS+=("${IDX}: ${WO} ❌ INVALID") ;;
        2) RESULTS+=("${IDX}: ${WO} ⛔ QUOTA")
           echo "Sleeping 10 minutes..."
           sleep 600
           SCHEDULE_INDEX="${IDX}" ATTEMPT=2 bash "${RUNNER}" "${WO}" no_memory 1 || true
           ;;
    esac
}

run_one 15 wo2-dossier-schema-renderer
run_one 16 wo3-scopes-aliases
run_one 17 wo1-ledger-engine
run_one 18 wo6-pull-tools

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  BASELINE SWEEP COMPLETE"
echo "═══════════════════════════════════════════════════════"
for r in "${RESULTS[@]}"; do
    echo "  ${r}"
done
