#!/usr/bin/env bash
# ==============================================================================
# eval_run.sh — Evaluation Run Orchestration Script
# ==============================================================================
# Orchestrates isolated, protected evaluation runs across conditions (a, b, c, d).
#
# Usage:
#   # Setup and staging (Phase 1):
#   eval_run.sh --condition {a,b,c,d} --task <task-file> --run-id <id> [--eval-root <path>]
#
#   # Post-run archival (Phase 2):
#   eval_run.sh --archive --run-id <id> [--eval-root <path>] [--transcript <path>] [--conversation-id <id>]
#
#   # Help:
#   eval_run.sh --help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_EVAL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CONDITION=""
TASK_FILE=""
RUN_ID=""
EVAL_ROOT=""
DO_ARCHIVE=false
TRANSCRIPT_PATH=""
CONVERSATION_ID=""

usage() {
  cat <<'EOF'
Usage:
  eval_run.sh --condition {a,b,c,d} --task <task-file> --run-id <id> [options]
  eval_run.sh --archive --run-id <id> [options]

Required for run setup:
  -c, --condition <a|b|c|d>   Condition:
                                a: Baseline (No PA, No repo-memory)
                                b: PA documents only (PA workspace, No repo-memory)
                                c: repo-memory push only (No PA, repo-memory enrolled)
                                d: Both (PA workspace + repo-memory enrolled)
  -t, --task <task-file>      Path to task definition / issue prompt file
  -r, --run-id <id>           Unique run identifier (e.g. run-t1-c-rep1)

Required for post-run archival:
  --archive                   Archive the completed run artifacts and delete repo copy
  -r, --run-id <id>           Run identifier to archive

Options:
  -e, --eval-root <path>      Path to eval-v2 root (default: directory above scripts/)
  --transcript <path>         Path to conversation transcript file (for --archive)
  --conversation-id <id>      Antigravity conversation ID to locate transcript
  -h, --help                  Show this usage guide

Examples:
  # 1. Setup a condition 'c' run:
  ./eval_run.sh --condition c --task ./tasks/task-01.txt --run-id pilot-c-01

  # 2. Archive after agent run:
  ./eval_run.sh --archive --run-id pilot-c-01 --transcript /path/to/transcript.jsonl
EOF
  exit 0
}

# ------------------------------------------------------------------------------
# Argument Parsing
# ------------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -c|--condition)
      CONDITION="$2"
      shift 2
      ;;
    -t|--task)
      TASK_FILE="$2"
      shift 2
      ;;
    -r|--run-id)
      RUN_ID="$2"
      shift 2
      ;;
    -e|--eval-root)
      EVAL_ROOT="$2"
      shift 2
      ;;
    --archive)
      DO_ARCHIVE=true
      shift
      ;;
    --transcript)
      TRANSCRIPT_PATH="$2"
      shift 2
      ;;
    --conversation-id)
      CONVERSATION_ID="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    -*)
      echo "Error: Unknown option $1" >&2
      exit 2
      ;;
    *)
      echo "Error: Unexpected positional argument $1" >&2
      exit 2
      ;;
  esac
done

# Default EVAL_ROOT if not specified
if [ -z "${EVAL_ROOT}" ]; then
  EVAL_ROOT="${DEFAULT_EVAL_ROOT}"
fi
EVAL_ROOT="$(cd "${EVAL_ROOT}" && pwd)"

# Validate RUN_ID
if [ -z "${RUN_ID}" ]; then
  echo "Error: --run-id <id> is required." >&2
  exit 1
fi

RUN_ROOT="${EVAL_ROOT}/runs/${RUN_ID}"
REPO_DIR="${RUN_ROOT}/repo"
DATA_DIR="${RUN_ROOT}/data"
PLUGIN_DIR="${RUN_ROOT}/plugin/repo-memory"
ARCHIVE_DIR="${RUN_ROOT}/archive"
METADATA_FILE="${RUN_ROOT}/run_metadata.json"

# Discover repo-memory binary
REPO_MEMORY_BIN="${REPO_MEMORY_BIN:-$(which repo-memory 2>/dev/null || echo "${HOME}/.local/bin/repo-memory")}"

# ==============================================================================
# Workflow 2: Post-Run Archival (--archive)
# ==============================================================================
if [ "${DO_ARCHIVE}" = true ]; then
  echo "======================================================================"
  echo "         Archiving Evaluation Run: ${RUN_ID}"
  echo "======================================================================"

  if [ ! -d "${RUN_ROOT}" ]; then
    echo "Error: Run root does not exist: ${RUN_ROOT}" >&2
    exit 1
  fi

  # Read condition from metadata if available
  if [ -f "${METADATA_FILE}" ] && [ -z "${CONDITION}" ]; then
    CONDITION=$(grep -o '"condition": *"[^"]*"' "${METADATA_FILE}" | cut -d'"' -f4 || echo "")
  fi

  mkdir -p "${ARCHIVE_DIR}"

  # 1. Archive git diff baseline + untracked files
  if [ -d "${REPO_DIR}" ]; then
    echo "[1/4] Archiving Git changes against baseline..."
    if [ -d "${REPO_DIR}/.git" ]; then
      # Determine baseline reference (tag 'baseline' or root commit)
      BASE_REF="baseline"
      if ! git -C "${REPO_DIR}" rev-parse -q --verify baseline >/dev/null 2>&1; then
        BASE_REF="$(git -C "${REPO_DIR}" rev-list --max-parents=0 HEAD 2>/dev/null | head -n 1 || echo "HEAD")"
      fi

      git -C "${REPO_DIR}" diff "${BASE_REF}" > "${ARCHIVE_DIR}/git-diff-baseline.patch" 2>/dev/null || true
      git -C "${REPO_DIR}" diff "${BASE_REF}" --stat > "${ARCHIVE_DIR}/git-diff-stat.txt" 2>/dev/null || true
      git -C "${REPO_DIR}" status --porcelain > "${ARCHIVE_DIR}/git-status.txt" 2>/dev/null || true

      # Untracked files
      git -C "${REPO_DIR}" ls-files --others --exclude-standard > "${ARCHIVE_DIR}/untracked-files.txt" 2>/dev/null || true

      # Copy untracked files to archive/untracked/
      if [ -s "${ARCHIVE_DIR}/untracked-files.txt" ]; then
        mkdir -p "${ARCHIVE_DIR}/untracked"
        while IFS= read -r ufile; do
          [ -z "${ufile}" ] && continue
          if [ -f "${REPO_DIR}/${ufile}" ]; then
            mkdir -p "${ARCHIVE_DIR}/untracked/$(dirname "${ufile}")"
            cp "${REPO_DIR}/${ufile}" "${ARCHIVE_DIR}/untracked/${ufile}" 2>/dev/null || true
          fi
        done < "${ARCHIVE_DIR}/untracked-files.txt"
        echo "  ✓ Archived $(wc -l < "${ARCHIVE_DIR}/untracked-files.txt" | tr -d ' ') untracked file(s)."
      fi
      echo "  ✓ Saved git-diff-baseline.patch and stat."
    else
      echo "  ⚠️ ${REPO_DIR} is not a Git repository."
    fi
  else
    echo "[1/4] Repo dir already removed; skipping git diff generation."
  fi

  # 2. Archive transcript
  echo "[2/4] Archiving transcript..."
  FOUND_TRANSCRIPT=false
  # Priority 1: explicitly passed --transcript
  if [ -n "${TRANSCRIPT_PATH}" ] && [ -f "${TRANSCRIPT_PATH}" ]; then
    cp "${TRANSCRIPT_PATH}" "${ARCHIVE_DIR}/transcript.jsonl"
    FOUND_TRANSCRIPT=true
  # Priority 2: conversation ID lookup in ~/.gemini/antigravity*/brain/
  elif [ -n "${CONVERSATION_ID}" ]; then
    for candidate in \
      "${HOME}/.gemini/antigravity/brain/${CONVERSATION_ID}/.system_generated/logs/transcript_full.jsonl" \
      "${HOME}/.gemini/antigravity/brain/${CONVERSATION_ID}/.system_generated/logs/transcript.jsonl" \
      "${HOME}/.gemini/antigravity-cli/brain/${CONVERSATION_ID}/.system_generated/logs/transcript_full.jsonl" \
      "${HOME}/.gemini/antigravity-cli/brain/${CONVERSATION_ID}/.system_generated/logs/transcript.jsonl"; do
      if [ -f "${candidate}" ]; then
        cp "${candidate}" "${ARCHIVE_DIR}/transcript.jsonl"
        FOUND_TRANSCRIPT=true
        echo "  ✓ Found transcript for conversation ${CONVERSATION_ID}"
        break
      fi
    done
  fi

  # Priority 3: check if transcript was saved inside run dir
  if [ "${FOUND_TRANSCRIPT}" = false ]; then
    if [ -f "${RUN_ROOT}/transcript.jsonl" ]; then
      cp "${RUN_ROOT}/transcript.jsonl" "${ARCHIVE_DIR}/transcript.jsonl"
      FOUND_TRANSCRIPT=true
    fi
  fi

  if [ "${FOUND_TRANSCRIPT}" = true ]; then
    echo "  ✓ Transcript archived to ${ARCHIVE_DIR}/transcript.jsonl"
  else
    echo "  ⚠️ No transcript found. Provide --transcript or --conversation-id if available."
  fi

  # 3. Archive delivery ledger
  echo "[3/4] Archiving delivery ledger..."
  mkdir -p "${ARCHIVE_DIR}/ledger"
  LEDGER_FOUND=false
  if [ -d "${DATA_DIR}" ]; then
    # Find all ledgers in data dir
    while IFS= read -r ledger_file; do
      [ -z "${ledger_file}" ] && continue
      cp "${ledger_file}" "${ARCHIVE_DIR}/ledger/"
      LEDGER_FOUND=true
    done < <(find "${DATA_DIR}" -path "*/ledgers/*.json" -type f 2>/dev/null || true)

    if [ -f "${DATA_DIR}/registry.json" ]; then
      cp "${DATA_DIR}/registry.json" "${ARCHIVE_DIR}/ledger/registry.json"
    fi
  fi

  if [ "${LEDGER_FOUND}" = true ]; then
    echo "  ✓ Ledger files archived to ${ARCHIVE_DIR}/ledger/"
  else
    cat > "${ARCHIVE_DIR}/ledger/NO_LEDGER.txt" <<LEOF
Condition: ${CONDITION:-unknown}
Note: No delivery ledger was found. (Expected if condition is 'a' or 'b', or hooks did not record injections).
LEOF
    echo "  Note: No delivery ledger found (recorded in NO_LEDGER.txt)."
  fi

  # Copy run prompt and metadata into archive
  [ -f "${RUN_ROOT}/task_prompt.txt" ] && cp "${RUN_ROOT}/task_prompt.txt" "${ARCHIVE_DIR}/"
  [ -f "${METADATA_FILE}" ] && cp "${METADATA_FILE}" "${ARCHIVE_DIR}/"

  # 4. Delete the run's repo copy (keep the archive)
  echo "[4/4] Deleting run repo copy (${REPO_DIR})..."
  if [ -d "${REPO_DIR}" ]; then
    rm -rf "${REPO_DIR}"
    echo "  ✓ Deleted ${REPO_DIR}."
  else
    echo "  Note: Repo directory did not exist."
  fi

  echo "======================================================================"
  echo "Archival Complete for Run: ${RUN_ID}"
  echo "Archive location: ${ARCHIVE_DIR}"
  echo "======================================================================"
  exit 0
fi

# ==============================================================================
# Workflow 1: Run Setup and Staging
# ==============================================================================
if [ -z "${CONDITION}" ]; then
  echo "Error: --condition {a,b,c,d} is required." >&2
  exit 1
fi

case "${CONDITION}" in
  a|b|c|d) ;;
  *)
    echo "Error: Invalid condition '${CONDITION}'. Must be one of: a, b, c, d" >&2
    exit 1
    ;;
esac

if [ -z "${TASK_FILE}" ]; then
  echo "Error: --task <task-file> is required." >&2
  exit 1
fi

if [ ! -f "${TASK_FILE}" ]; then
  echo "Error: Task file not found: ${TASK_FILE}" >&2
  exit 1
fi

TEMPLATE_SRC="${EVAL_ROOT}/tpl-${CONDITION}"
if [ ! -d "${TEMPLATE_SRC}" ]; then
  echo "Error: Template directory not found: ${TEMPLATE_SRC}" >&2
  exit 1
fi

ENV_FILE="${EVAL_ROOT}/scripts/eval_env.sh"
if [ ! -f "${ENV_FILE}" ]; then
  echo "Error: eval_env.sh not found at ${ENV_FILE}" >&2
  exit 1
fi

echo "======================================================================"
echo "         Setting Up Evaluation Run: ${RUN_ID}"
echo "======================================================================"
echo "Condition:     ${CONDITION}"
echo "Task File:     ${TASK_FILE}"
echo "Eval Root:     ${EVAL_ROOT}"
echo "Template:      ${TEMPLATE_SRC}"
echo "Target Dir:    ${RUN_ROOT}"
echo "----------------------------------------------------------------------"

# 1. Source protection environment
echo "[1/6] Sourcing protection environment..."
# shellcheck source=/dev/null
source "${ENV_FILE}"
echo "  ✓ Protection environment sourced (GH_TOKEN, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT)."

# 2. Create per-run directory structure
echo "[2/6] Creating per-run workspace..."
if [ -d "${REPO_DIR}" ]; then
  echo "Error: Repo directory already exists at ${REPO_DIR}." >&2
  echo "If this is a new run, choose a fresh --run-id or remove the previous run." >&2
  exit 1
fi
mkdir -p "${RUN_ROOT}"

# 3. Copy appropriate template via APFS clonefile (cp -c -R)
echo "[3/6] Copying template via APFS clonefile..."
cp -c -R "${TEMPLATE_SRC}" "${REPO_DIR}"

# Ensure 'baseline' tag exists in the run's git repository
if [ -d "${REPO_DIR}/.git" ]; then
  git -C "${REPO_DIR}" tag -f baseline HEAD >/dev/null 2>&1 || true
fi
echo "  ✓ Cloned ${TEMPLATE_SRC} to ${REPO_DIR}."

# 4. Condition configuration (repo-memory enrolment vs disabled)
AGENT_ENV_EXPORTS="export GH_TOKEN=invalid
export GIT_CONFIG_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0"

case "${CONDITION}" in
  c|d)
    echo "[4/6] Configuring repo-memory (Condition ${CONDITION})..."
    mkdir -p "${DATA_DIR}" "${PLUGIN_DIR}"

    # Verify binary
    if [ ! -x "${REPO_MEMORY_BIN}" ]; then
      echo "Warning: repo-memory binary not found or not executable at ${REPO_MEMORY_BIN}." >&2
    fi

    # Enrol repository with per-run data dir
    echo "  Enrolling repository into per-run data dir..."
    REPO_MEMORY_DATA_DIR="${DATA_DIR}" "${REPO_MEMORY_BIN}" enroll "${REPO_DIR}" >/dev/null 2>&1 || {
      echo "  ⚠️ Warning: repo-memory enroll returned non-zero. Check binary permissions."
    }

    # Setup per-run plugin files
    cat > "${PLUGIN_DIR}/plugin.json" <<'EOF'
{
  "name": "repo-memory",
  "version": "0.1.0",
  "description": "Per-run repo-memory plugin"
}
EOF

    cat > "${PLUGIN_DIR}/hooks.json" <<EOF
{
  "repo-memory": {
    "enabled": true,
    "PreInvocation": [
      {
        "type": "command",
        "command": "env REPO_MEMORY_DATA_DIR=${DATA_DIR} ${REPO_MEMORY_BIN} hook antigravity PreInvocation"
      }
    ],
    "PreToolUse": [
      {
        "matcher": "run_command|write_to_file|replace_file_content|call_mcp_tool",
        "hooks": [
          {
            "type": "command",
            "command": "env REPO_MEMORY_DATA_DIR=${DATA_DIR} ${REPO_MEMORY_BIN} hook antigravity PreToolUse"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "run_command",
        "hooks": [
          {
            "type": "command",
            "command": "env REPO_MEMORY_DATA_DIR=${DATA_DIR} ${REPO_MEMORY_BIN} hook antigravity PostToolUse"
          }
        ]
      }
    ],
    "Stop": [
      {
        "type": "command",
        "command": "env REPO_MEMORY_DATA_DIR=${DATA_DIR} ${REPO_MEMORY_BIN} hook antigravity Stop"
      }
    ]
  }
}
EOF

    cat > "${PLUGIN_DIR}/mcp_config.json" <<EOF
{
  "command": "${REPO_MEMORY_BIN}",
  "args": ["mcp"],
  "env": {
    "REPO_MEMORY_DATA_DIR": "${DATA_DIR}"
  }
}
EOF

    # Point repo's .agents/plugins.json to the per-run plugin directory
    mkdir -p "${REPO_DIR}/.agents"
    cat > "${REPO_DIR}/.agents/plugins.json" <<EOF
{
  "entries": [
    {
      "path": "${RUN_ROOT}/plugin"
    }
  ]
}
EOF

    AGENT_ENV_EXPORTS="${AGENT_ENV_EXPORTS}
export REPO_MEMORY_DATA_DIR=\"${DATA_DIR}\""
    echo "  ✓ Per-run plugin and data dirs configured."
    ;;

  a|b)
    echo "[4/6] Disabling repo-memory (Condition ${CONDITION})..."
    AGENT_ENV_EXPORTS="${AGENT_ENV_EXPORTS}
export REPO_MEMORY_DISABLED=1"
    echo "  ✓ REPO_MEMORY_DISABLED=1 configured."
    ;;
esac

# 5. Append safety instruction to task prompt
echo "[5/6] Preparing task prompt with safety instructions..."
TASK_PROMPT_CONTENT=$(cat "${TASK_FILE}")
SAFETY_INSTRUCTION="Work locally. Do not push, open PRs, or comment on GitHub."

FINAL_PROMPT="${TASK_PROMPT_CONTENT}

${SAFETY_INSTRUCTION}"

PROMPT_FILE="${RUN_ROOT}/task_prompt.txt"
printf "%s\n" "${FINAL_PROMPT}" > "${PROMPT_FILE}"
echo "  ✓ Final prompt written to: ${PROMPT_FILE}"

# Record metadata
BASELINE_COMMIT=$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || echo "unknown")
cat > "${METADATA_FILE}" <<EOF
{
  "run_id": "${RUN_ID}",
  "condition": "${CONDITION}",
  "task_file": "${TASK_FILE}",
  "eval_root": "${EVAL_ROOT}",
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "baseline_commit": "${BASELINE_COMMIT}"
}
EOF

# 6. Generate interactive launch script
LAUNCH_SCRIPT="${RUN_ROOT}/run_agent.sh"
cat > "${LAUNCH_SCRIPT}" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "${REPO_DIR}"
source "${ENV_FILE}"
${AGENT_ENV_EXPORTS}

echo "Launching agent in ${REPO_DIR}..."
agy -p "\$(cat "${PROMPT_FILE}")" "\$@"
EOF
chmod +x "${LAUNCH_SCRIPT}"

# Print launch command
echo "[6/6] Run preparation complete!"
echo ""
echo "======================================================================"
echo "                     AGENT LAUNCH COMMAND"
echo "======================================================================"
echo "To execute this evaluation run interactively, run:"
echo ""
echo "  cd \"${REPO_DIR}\" && \\"
echo "  source \"${ENV_FILE}\" && \\"
echo "${AGENT_ENV_EXPORTS}" | sed 's/^/  /' | sed 's/$/ \&\& \\/'
echo "  agy -p \"\$(cat \"${PROMPT_FILE}\")\""
echo ""
echo "Or use the generated launch script:"
echo "  ${LAUNCH_SCRIPT}"
echo ""
echo "======================================================================"
echo "After the run completes, archive the results with:"
echo "  ${SCRIPT_DIR}/eval_run.sh --archive --run-id ${RUN_ID} --eval-root ${EVAL_ROOT} --conversation-id <CONV_ID>"
echo "======================================================================"
