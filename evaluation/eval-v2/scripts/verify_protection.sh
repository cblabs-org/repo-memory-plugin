#!/usr/bin/env bash
# ==============================================================================
# verify_protection.sh — One-Time Protection Verification Check
# ==============================================================================
# Verifies that the evaluation environment protection settings in eval_env.sh
# successfully block:
#   1. GitHub CLI operations (gh api user)
#   2. Git remote HTTPS push operations (git push)
#
# Returns 0 if all protection mechanisms successfully block remote access.
# Returns non-zero if any check unexpectedly succeeds (security violation).
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/eval_env.sh"

if [ ! -f "${ENV_FILE}" ]; then
  echo "Error: eval_env.sh not found at ${ENV_FILE}" >&2
  exit 1
fi

echo "======================================================================"
echo "           Evaluation Protection Verification Check"
echo "======================================================================"
echo "Sourcing protection environment from: ${ENV_FILE}"
# Source protection environment
# shellcheck source=/dev/null
source "${ENV_FILE}"

PASSED_COUNT=0
FAILED_COUNT=0

# ------------------------------------------------------------------------------
# Check 1: GitHub CLI (gh api user)
# ------------------------------------------------------------------------------
echo ""
echo "[Check 1/2] Testing GitHub CLI isolation (gh api user)..."
GH_OUTPUT=""
GH_EXIT=0
if command -v gh >/dev/null 2>&1; then
  set +e
  GH_OUTPUT=$(gh api user 2>&1)
  GH_EXIT=$?
  set -e
else
  echo "  Note: gh CLI is not installed in current PATH."
  # If gh is not installed, it cannot perform remote calls, but report clearly
  GH_EXIT=127
  GH_OUTPUT="gh: command not found"
fi

if [ ${GH_EXIT} -ne 0 ]; then
  echo "  ✓ gh api user failed as expected (exit code: ${GH_EXIT})"
  echo "    Output: $(echo "${GH_OUTPUT}" | head -n 2 | tr '\n' ' ')"
  PASSED_COUNT=$((PASSED_COUNT + 1))
  CHECK_1_STATUS="PASS (Blocked)"
else
  echo "  ✗ CRITICAL: gh api user succeeded! Credentials leaked." >&2
  FAILED_COUNT=$((FAILED_COUNT + 1))
  CHECK_1_STATUS="FAIL (Unprotected — call succeeded)"
fi

# ------------------------------------------------------------------------------
# Check 2: Git push HTTPS remote
# ------------------------------------------------------------------------------
echo ""
echo "[Check 2/2] Testing Git push isolation (git push)..."
TMP_REPO=$(mktemp -d -t eval_protect_test.XXXXXX)

cleanup() {
  if [ -d "${TMP_REPO}" ]; then
    rm -rf "${TMP_REPO}"
  fi
}
trap cleanup EXIT INT TERM

# Initialize temporary dummy repository
git -C "${TMP_REPO}" init -q
git -C "${TMP_REPO}" config user.email "eval-protection@local.test"
git -C "${TMP_REPO}" config user.name "Eval Protection Tester"
git -C "${TMP_REPO}" commit --allow-empty -m "initial test commit" -q
git -C "${TMP_REPO}" remote add origin "https://github.com/matthewmcneill/eval-protection-dummy-repo.git"

set +e
GIT_OUTPUT=$(git -C "${TMP_REPO}" push origin HEAD 2>&1)
GIT_EXIT=$?
set -e

if [ ${GIT_EXIT} -ne 0 ]; then
  echo "  ✓ git push failed as expected (exit code: ${GIT_EXIT})"
  echo "    Output: $(echo "${GIT_OUTPUT}" | head -n 2 | tr '\n' ' ')"
  PASSED_COUNT=$((PASSED_COUNT + 1))
  CHECK_2_STATUS="PASS (Blocked)"
else
  echo "  ✗ CRITICAL: git push succeeded! Push was not blocked." >&2
  FAILED_COUNT=$((FAILED_COUNT + 1))
  CHECK_2_STATUS="FAIL (Unprotected — push succeeded)"
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
echo ""
echo "======================================================================"
echo "                     Verification Summary"
echo "======================================================================"
echo "  1. GitHub CLI API check:      ${CHECK_1_STATUS}"
echo "  2. Git push HTTPS check:      ${CHECK_2_STATUS}"
echo "----------------------------------------------------------------------"

if [ ${FAILED_COUNT} -eq 0 ]; then
  echo "  Overall Result: PASS (All protection mechanisms verified)"
  echo "======================================================================"
  exit 0
else
  echo "  Overall Result: FAIL (${FAILED_COUNT} check(s) failed)"
  echo "======================================================================"
  exit 1
fi
