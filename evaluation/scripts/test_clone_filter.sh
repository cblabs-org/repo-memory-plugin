#!/usr/bin/env bash
# Quick test of the clone + filter-repo pipeline
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RUN_ROOT=$(mktemp -d)
RUN_DIR="${RUN_ROOT}/repo"
WO_COMMIT="42deb642bda9"
REPLACE_PATHS="${REPO_ROOT}/evaluation/scripts/replace-paths.txt"

echo "=== Testing clone + filter-repo for wo1-op-log / no_memory ==="

TEMP_BRANCH="x6-test-$(date +%s)"
git -C "${REPO_ROOT}" branch "${TEMP_BRANCH}" "${WO_COMMIT}"
git clone --no-local --single-branch --no-tags -b "${TEMP_BRANCH}" "${REPO_ROOT}" "${RUN_DIR}" 2>/dev/null
git -C "${REPO_ROOT}" branch -D "${TEMP_BRANCH}" 2>/dev/null
echo "Clone OK."

git -C "${RUN_DIR}" filter-repo \
    --invert-paths \
    --path .agents/skills/principal-architect-workspace/ \
    --path .memory/archive/ \
    --path .memory/views/ \
    --path .memory/B5_EVALUATION.md \
    --replace-text "${REPLACE_PATHS}" \
    --force 2>/dev/null
echo "Filter OK."

git -C "${RUN_DIR}" branch -m main 2>/dev/null || true

echo "Checking..."
HOME_HITS=$(git -C "${RUN_DIR}" log --all -p 2>/dev/null | grep -c '/Users/mcneillm' || echo "0")
echo "  Home path in history: ${HOME_HITS}"

PA_HITS=$(git -C "${RUN_DIR}" rev-list --all --objects 2>/dev/null | grep -c principal-architect-workspace || echo "0")
echo "  PA workspace in objects: ${PA_HITS}"

PORCELAIN=$(git -C "${RUN_DIR}" status --porcelain | wc -l | tr -d ' ')
echo "  Dirty files: ${PORCELAIN}"

HEAD=$(git -C "${RUN_DIR}" rev-parse HEAD)
echo "  HEAD: ${HEAD}"

# Check .memory/config.yaml preserved
if [[ -f "${RUN_DIR}/.memory/config.yaml" ]]; then
    echo "  .memory/config.yaml: preserved ✓"
else
    echo "  .memory/config.yaml: MISSING ✗"
fi

# Check v1 artifacts removed
if [[ -d "${RUN_DIR}/.memory/archive" ]]; then
    echo "  .memory/archive: STILL PRESENT ✗"
else
    echo "  .memory/archive: removed ✓"
fi

rm -rf "${RUN_ROOT}"
echo ""
if [[ "${HOME_HITS}" == "0" && "${PA_HITS}" == "0" && "${PORCELAIN}" == "0" ]]; then
    echo "ALL CHECKS PASSED ✓"
else
    echo "SOME CHECKS FAILED ✗"
fi
