#!/usr/bin/env bash
# ==============================================================================
# check_contamination.sh — Transcript Contamination Scanner
# ==============================================================================
# Usage:
#   check_contamination.sh [--run-id <current-run-id>] <transcript-path>
#   check_contamination.sh --help
#
# Scans an evaluation agent transcript (.jsonl, .txt, or log file) for:
#   1. References to the real esp32 checkout path (~/Documents/Projects/esp32-app-foundation)
#   2. Any `git push` command
#   3. Any `gh` write command (gh pr create, gh issue create, gh api -X POST, etc.)
#   4. References to another run's directory (/runs/<other-run-id>)
#
# Returns 0 if clean.
# Returns 1 (non-zero) if contamination is detected.
# ==============================================================================

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  check_contamination.sh [options] <transcript-file>

Options:
  -r, --run-id <id>     Current run ID (used to distinguish own run dir from other runs)
  -h, --help            Show this help message

Scans:
  1. Real esp32 checkout:  ~/Documents/Projects/esp32-app-foundation or absolute expansion
  2. Git push commands:    git push
  3. GitHub CLI writes:    gh pr create, gh issue create, gh api -X POST, etc.
  4. Cross-run paths:      References to /runs/<other-run-id>
EOF
  exit 0
}

CURRENT_RUN_ID=""
TRANSCRIPT_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -r|--run-id)
      CURRENT_RUN_ID="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    -*)
      echo "Error: Unknown option: $1" >&2
      exit 2
      ;;
    *)
      if [ -z "${TRANSCRIPT_FILE}" ]; then
        TRANSCRIPT_FILE="$1"
      else
        echo "Error: Multiple transcript files specified: $1" >&2
        exit 2
      fi
      shift
      ;;
  esac
done

if [ -z "${TRANSCRIPT_FILE}" ]; then
  echo "Error: No transcript file specified." >&2
  usage
fi

if [ ! -f "${TRANSCRIPT_FILE}" ]; then
  echo "Error: Transcript file not found: ${TRANSCRIPT_FILE}" >&2
  exit 2
fi

echo "======================================================================"
echo "           Evaluation Transcript Contamination Scan"
echo "======================================================================"
echo "Transcript file: ${TRANSCRIPT_FILE}"
if [ -n "${CURRENT_RUN_ID}" ]; then
  echo "Current Run ID:  ${CURRENT_RUN_ID}"
fi
echo "----------------------------------------------------------------------"

VIOLATIONS=0

report_violation() {
  local category="$1"
  local line_num="$2"
  local snippet="$3"
  VIOLATIONS=$((VIOLATIONS + 1))
  echo "  [VIOLATION #${VIOLATIONS}] Category: ${category}"
  echo "    Line ${line_num}: ${snippet}"
  echo ""
}

# ------------------------------------------------------------------------------
# Check 1: Real esp32 checkout path references
# ------------------------------------------------------------------------------
checkout_matches=$(grep -n -E 'Documents/Projects/esp32-app-foundation' "${TRANSCRIPT_FILE}" 2>/dev/null || true)
if [ -n "${checkout_matches}" ]; then
  while IFS= read -r hit; do
    [ -z "${hit}" ] && continue
    lnum="${hit%%:*}"
    content="${hit#*:}"
    truncated=$(echo "${content}" | cut -c 1-120)
    report_violation "REAL_CHECKOUT_PATH" "${lnum}" "${truncated}"
  done <<< "${checkout_matches}"
fi

# ------------------------------------------------------------------------------
# Check 2: Any `git push` command
# ------------------------------------------------------------------------------
# Match git push in tool calls or commands
git_push_matches=$(grep -n -E '\bgit([[:space:]]+[A-Za-z0-9_-]+)*[[:space:]]+push\b' "${TRANSCRIPT_FILE}" 2>/dev/null || true)
if [ -n "${git_push_matches}" ]; then
  while IFS= read -r hit; do
    [ -z "${hit}" ] && continue
    lnum="${hit%%:*}"
    content="${hit#*:}"
    truncated=$(echo "${content}" | cut -c 1-120)
    report_violation "GIT_PUSH_COMMAND" "${lnum}" "${truncated}"
  done <<< "${git_push_matches}"
fi

# ------------------------------------------------------------------------------
# Check 3: Any `gh` write commands
# ------------------------------------------------------------------------------
# Patterns for gh mutations:
# gh pr create/edit/close/merge/comment/review
# gh issue create/edit/close/reopen/comment/delete
# gh release create/delete/edit/upload
# gh repo create/delete/fork
# gh api with -X POST/PUT/PATCH/DELETE or --method POST/PUT/PATCH/DELETE or -f/-F
GH_WRITE_REGEX='\bgh[[:space:]]+(pr[[:space:]]+(create|edit|close|merge|reopen|comment|review)|issue[[:space:]]+(create|edit|close|reopen|comment|delete)|release[[:space:]]+(create|edit|delete|upload)|repo[[:space:]]+(create|delete|fork|archive)|api[[:space:]]+.*(-X[[:space:]]*(POST|PUT|PATCH|DELETE)|--method[[:space:]]+(POST|PUT|PATCH|DELETE)|-[fF][[:space:]]|--field|--raw-field))\b'

gh_matches=$(grep -n -E "${GH_WRITE_REGEX}" "${TRANSCRIPT_FILE}" 2>/dev/null || true)
if [ -n "${gh_matches}" ]; then
  while IFS= read -r hit; do
    [ -z "${hit}" ] && continue
    lnum="${hit%%:*}"
    content="${hit#*:}"
    truncated=$(echo "${content}" | cut -c 1-120)
    report_violation "GH_WRITE_COMMAND" "${lnum}" "${truncated}"
  done <<< "${gh_matches}"
fi

# ------------------------------------------------------------------------------
# Check 4: References to another run's directory
# ------------------------------------------------------------------------------
# Match occurrences of /runs/<run_id>/
runs_matches=$(grep -n -o -E '/runs/([A-Za-z0-9._-]+)' "${TRANSCRIPT_FILE}" 2>/dev/null || true)
if [ -n "${runs_matches}" ]; then
  while IFS= read -r hit; do
    [ -z "${hit}" ] && continue
    lnum="${hit%%:*}"
    ref_run_path="${hit#*:}"
    ref_run_id="${ref_run_path#/runs/}"

    # Ignore generic placeholders
    if [[ "${ref_run_id}" == "<id>" || "${ref_run_id}" == "<run-id>" || "${ref_run_id}" == "RUN_ID" || "${ref_run_id}" == '$RUN_ID' ]]; then
      continue
    fi

    if [ -n "${CURRENT_RUN_ID}" ]; then
      if [ "${ref_run_id}" != "${CURRENT_RUN_ID}" ]; then
        report_violation "CROSS_RUN_PATH" "${lnum}" "Reference to external run '${ref_run_id}' (current is '${CURRENT_RUN_ID}')"
      fi
    fi
  done <<< "${runs_matches}"

  # If CURRENT_RUN_ID was not passed, check if multiple unique run IDs are present
  if [ -z "${CURRENT_RUN_ID}" ]; then
    unique_runs=$(echo "${runs_matches}" | awk -F':/runs/' '{print $2}' | grep -v -E '^(\$RUN_ID|<run-id>|<id>|RUN_ID)$' | sort -u || true)
    run_count=$(echo "${unique_runs}" | grep -c . || true)
    if [ "${run_count}" -gt 1 ]; then
      report_violation "CROSS_RUN_PATH" "multiple" "Transcript references multiple distinct runs: $(echo "${unique_runs}" | tr '\n' ' ')"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# Summary & Exit
# ------------------------------------------------------------------------------
echo "======================================================================"
if [ ${VIOLATIONS} -eq 0 ]; then
  echo "Result: PASS (No contamination detected in transcript)"
  echo "======================================================================"
  exit 0
else
  echo "Result: CONTAMINATION DETECTED (${VIOLATIONS} violation(s) found)"
  echo "======================================================================"
  exit 1
fi
