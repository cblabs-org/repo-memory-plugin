#!/usr/bin/env bash
# X6-lite Evaluation Runner v3
# Usage: ./x6_runner_v3.sh <wo_name> <condition> <replicate> [--attempt N]
#
# Implements the full isolation, condition manifest, hidden test, and audit
# pipeline described in the v3 implementation plan.
#
# Requirements:
#   - git-filter-repo (/opt/homebrew/bin/git-filter-repo)
#   - Pinned binaries at $EVAL_ROOT/pin/$PIN_COMMIT/
#   - Oracle test data at evaluation/oracle/<wo_name>/
#   - Schedule file at evaluation/schedule.jsonl

set -euo pipefail

###############################################################################
# Configuration
###############################################################################
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
EVAL_ROOT="${REPO_ROOT}/evaluation"
RESULTS_FILE="${EVAL_ROOT}/results/x6-lite-results-v3.jsonl"
SCRIPTS_DIR="${EVAL_ROOT}/scripts"
REPLACE_PATHS="${SCRIPTS_DIR}/replace-paths.txt"

# Pinned build — set by Phase 0 or env
PIN_COMMIT="${PIN_COMMIT:-$(cat "${EVAL_ROOT}/pin/PIN_COMMIT" 2>/dev/null || echo '')}"
PIN_DIR="${EVAL_ROOT}/pin/${PIN_COMMIT:0:12}"

# Read model from env or default (R3.2: pin the model)
AGY_MODEL="${AGY_MODEL:-gemini-3.8-flash-high}"

WO_NAME="${1:?Usage: x6_runner_v3.sh <wo_name> <condition> <replicate>}"
CONDITION="${2:?Usage: x6_runner_v3.sh <wo_name> <condition> <replicate>}"
REPLICATE="${3:?Usage: x6_runner_v3.sh <wo_name> <condition> <replicate>}"
ATTEMPT="${ATTEMPT:-1}"
SCHEDULE_INDEX="${SCHEDULE_INDEX:-0}"

###############################################################################
# WO Definitions
###############################################################################
declare -A WO_COMMITS
WO_COMMITS["wo1-op-log"]="42deb642bda94c783f8e070d00eed2aba298d778"
WO_COMMITS["wo2-dossier-schema-renderer"]="0fbfe5a6f8948d7db8dddfc2f46fee9014d76021"
WO_COMMITS["wo3-scopes-aliases"]="3177bfe3cdb9ae98be10c4c60cd69d1a3d1d38d8"
WO_COMMITS["wo4-index-migration"]="501d05d453960b4c59e4e25eb25cfb557bc53948"
WO_COMMITS["wo1-ledger-engine"]="732177442cd0179866a184955a640510a4aeda64"
WO_COMMITS["wo4-gates"]="e4acaa2a5cd38af1f3205a4ab0bd64f6930ff6b9"
WO_COMMITS["wo6-pull-tools"]="6a840b3893db013834460cba133eefafbd3b9413"

declare -A WO_PATHS
WO_PATHS["wo1-op-log"]="pr-07-v2-storage/wo1-op-log"
WO_PATHS["wo2-dossier-schema-renderer"]="pr-07-v2-storage/wo2-dossier-schema-renderer"
WO_PATHS["wo3-scopes-aliases"]="pr-07-v2-storage/wo3-scopes-aliases"
WO_PATHS["wo4-index-migration"]="pr-07-v2-storage/wo4-index-migration"
WO_PATHS["wo1-ledger-engine"]="pr-08-v2-delivery/wo1-ledger-engine"
WO_PATHS["wo4-gates"]="pr-08-v2-delivery/wo4-gates"
WO_PATHS["wo6-pull-tools"]="pr-08-v2-delivery/wo6-pull-tools"

# Hidden test packages per WO
declare -A WO_PACKAGES
WO_PACKAGES["wo1-op-log"]="./internal/core/ops/..."
WO_PACKAGES["wo2-dossier-schema-renderer"]="./internal/core/dossiers/..."
WO_PACKAGES["wo3-scopes-aliases"]="./internal/core/config/..."
WO_PACKAGES["wo4-index-migration"]="./internal/core/index/..."
WO_PACKAGES["wo1-ledger-engine"]="./internal/core/delivery/..."
WO_PACKAGES["wo4-gates"]="./internal/harness/antigravity/..."
WO_PACKAGES["wo6-pull-tools"]="./internal/mcpserver/..."

###############################################################################
# Validate inputs
###############################################################################
if [[ -z "${WO_COMMITS[$WO_NAME]+_}" ]]; then
    echo "ERROR: Unknown work order: $WO_NAME" >&2
    exit 1
fi

if [[ "$CONDITION" != "no_memory" && "$CONDITION" != "pa_documents" && "$CONDITION" != "dossiers" ]]; then
    echo "ERROR: Unknown condition: $CONDITION" >&2
    exit 1
fi

if [[ -z "$PIN_COMMIT" || ! -f "${PIN_DIR}/repo-memory" ]]; then
    echo "ERROR: Pinned build not found. Run Phase 0 first." >&2
    echo "  Expected: ${PIN_DIR}/repo-memory" >&2
    exit 1
fi

WO_COMMIT="${WO_COMMITS[$WO_NAME]}"
WO_MD_PATH=".agents/skills/principal-architect-workspace/${WO_PATHS[$WO_NAME]}/WORKORDER.md"
BINARY_SHA256=$(shasum -a 256 "${PIN_DIR}/repo-memory" | awk '{print $1}')
BUILD_INDEX_SHA256=$(shasum -a 256 "${PIN_DIR}/build-index" | awk '{print $1}')

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║ X6-lite v3 Run"
echo "║   WO:        ${WO_NAME}"
echo "║   Condition: ${CONDITION}"
echo "║   Replicate: ${REPLICATE}, Attempt: ${ATTEMPT}"
echo "║   WO_COMMIT: ${WO_COMMIT:0:12}"
echo "║   PIN:       ${PIN_COMMIT:0:12}"
echo "╚══════════════════════════════════════════════════════════════╝"

###############################################################################
# Setup: RUN_ROOT layout
###############################################################################
RUN_ROOT=$(mktemp -d)
RUN_DIR="${RUN_ROOT}/repo"
RUN_DATA="${RUN_ROOT}/data"
RUN_BIN="${RUN_ROOT}/bin"
RUN_PLUGIN="${RUN_ROOT}/plugin/repo-memory"
RUN_OPS="${RUN_ROOT}/ops"
RUN_ARCHIVE="${EVAL_ROOT}/results/archive/${SCHEDULE_INDEX}-${WO_NAME}-${CONDITION}-rep${REPLICATE}"

INVALID_REASON=""

cleanup() {
    echo "[cleanup] Cleaning up ${RUN_ROOT}..."
    # Remove any temp branch left behind
    git -C "${REPO_ROOT}" branch -D "x6-base-${TEMP_BRANCH_ID}" 2>/dev/null || true
    # Archive artifacts before deleting
    if [[ -d "${RUN_ROOT}" ]]; then
        # Clear archive dir to prevent old attempt data from mixing (R3.4)
        rm -rf "${RUN_ARCHIVE}" 2>/dev/null || true
        mkdir -p "${RUN_ARCHIVE}"
        cp "${RUN_ROOT}"/*.json "${RUN_ARCHIVE}/" 2>/dev/null || true
        cp "${RUN_ROOT}"/*.log "${RUN_ARCHIVE}/" 2>/dev/null || true
        if [[ "${CONDITION}" == "dossiers" && -d "${RUN_DATA}" ]]; then
            cp -R "${RUN_DATA}" "${RUN_ARCHIVE}/data" 2>/dev/null || true
        fi
        rm -rf "${RUN_ROOT}"
    fi
    echo "[cleanup] Done."
}
trap cleanup EXIT

###############################################################################
# Phase 1: Isolation — Standalone clone with sanitised history
###############################################################################
echo "[1/8] Creating standalone clone at ${WO_COMMIT:0:12}..."
TEMP_BRANCH_ID=$(uuidgen | head -c 8)
TEMP_BRANCH="x6-base-${TEMP_BRANCH_ID}"
git -C "${REPO_ROOT}" branch "${TEMP_BRANCH}" "${WO_COMMIT}"
git clone --no-local --single-branch --no-tags -b "${TEMP_BRANCH}" "${REPO_ROOT}" "${RUN_DIR}" 2>/dev/null
git -C "${REPO_ROOT}" branch -D "${TEMP_BRANCH}" 2>/dev/null

# Read the oracle SHA before filtering (need it for pre-filter check)
ORACLE_SHA=$(cat "${EVAL_ROOT}/oracle/${WO_NAME}/ORACLE_SHA" 2>/dev/null || echo "")
if [[ -n "${ORACLE_SHA}" ]]; then
    if git -C "${RUN_DIR}" cat-file -e "${ORACLE_SHA}" 2>/dev/null; then
        echo "[1/8] ⚠️  Future oracle commit ${ORACLE_SHA:0:12} is reachable — clone is not truncated"
        # This is expected before filter-repo; single-branch clone includes all ancestors
    fi
fi

# Build PATH_ARGS for git-filter-repo based on condition
FILTER_ARGS="--invert-paths"
case "${CONDITION}" in
    no_memory|dossiers)
        FILTER_ARGS="${FILTER_ARGS} --path .agents/skills/principal-architect-workspace/"
        ;;
    pa_documents)
        # Keep PA workspace for condition (b)
        ;;
esac
# Remove v1 .memory artifacts in all conditions (keep config.yaml and .gitignore)
FILTER_ARGS="${FILTER_ARGS} --path .memory/archive/ --path .memory/views/ --path .memory/B5_EVALUATION.md"

echo "[1/8] Running git-filter-repo (condition=${CONDITION})..."
git -C "${RUN_DIR}" filter-repo ${FILTER_ARGS} --replace-text "${REPLACE_PATHS}" --force 2>/dev/null
git -C "${RUN_DIR}" branch -m main 2>/dev/null || true
rm -rf "${RUN_DIR}/.git/filter-repo"
# Remove origin if filter-repo didn't already
git -C "${RUN_DIR}" remote | grep -q . && git -C "${RUN_DIR}" remote remove origin 2>/dev/null || true

BASE_SHA_FILTERED=$(git -C "${RUN_DIR}" rev-parse HEAD)

# Validation
echo "[1/8] Validating clone..."
CLONE_VALID=true

# Check 1: newest commit date
NEWEST_DATE=$(git -C "${RUN_DIR}" log --all -1 --format=%cI)
echo "  Newest commit: ${NEWEST_DATE}"

# Check 2: no future content (oracle paths absent from history)
if [[ -n "${ORACLE_SHA}" ]]; then
    ORACLE_TEST_FILES=$(cat "${EVAL_ROOT}/oracle/${WO_NAME}/TEST_FILES" 2>/dev/null || echo "")
    for tf in ${ORACLE_TEST_FILES}; do
        if git -C "${RUN_DIR}" log --all --oneline -- "${tf}" 2>/dev/null | grep -q .; then
            echo "  ⚠️  Oracle test file ${tf} found in history!"
            CLONE_VALID=false
        fi
    done
fi

# Check 3: PA workspace absent for (a)/(c)
if [[ "${CONDITION}" != "pa_documents" ]]; then
    PA_OBJECTS=$(git -C "${RUN_DIR}" rev-list --all --objects 2>/dev/null | grep -c principal-architect-workspace 2>/dev/null || true)
    PA_OBJECTS="${PA_OBJECTS:-0}"
    if [[ "${PA_OBJECTS}" -ne 0 ]]; then
        echo "  ⚠️  PA workspace objects found in filtered history!"
        CLONE_VALID=false
    fi
fi

# Check 4: no home path in history
HOME_PATH_HITS=$(git -C "${RUN_DIR}" log --all -p 2>/dev/null | grep -c '/Users/mcneillm' 2>/dev/null || true)
HOME_PATH_HITS="${HOME_PATH_HITS:-0}"
if [[ "${HOME_PATH_HITS}" -ne 0 ]]; then
    echo "  ⚠️  Home path found ${HOME_PATH_HITS} times in history!"
    CLONE_VALID=false
fi

# Check 5: no home path in working tree
TREE_PATH_HITS=$(grep -r '/Users/mcneillm' "${RUN_DIR}" --exclude-dir=.git 2>/dev/null | wc -l | tr -d ' ' || true)
TREE_PATH_HITS="${TREE_PATH_HITS:-0}"
if [[ "${TREE_PATH_HITS}" -ne 0 ]]; then
    echo "  ⚠️  Home path found ${TREE_PATH_HITS} times in working tree!"
    CLONE_VALID=false
fi

# Check 6: clean tree
PORCELAIN=$(git -C "${RUN_DIR}" status --porcelain)
if [[ -n "${PORCELAIN}" ]]; then
    echo "  ⚠️  Working tree not clean: ${PORCELAIN}"
    CLONE_VALID=false
fi

if [[ "${CLONE_VALID}" != "true" ]]; then
    echo "[1/8] ❌ Clone validation FAILED"
    INVALID_REASON="invalid_clone_validation"
fi
echo "[1/8] Clone validated ✓ (filtered HEAD: ${BASE_SHA_FILTERED:0:12})"

###############################################################################
# Contamination snapshots (before)
###############################################################################
echo "[2/8] Taking contamination snapshots..."
MAIN_STATUS_BEFORE=$(git -C "${REPO_ROOT}" status --porcelain 2>/dev/null | sort || true)
INTERNAL_HASH_BEFORE=$(find "${REPO_ROOT}/internal" "${REPO_ROOT}/cmd" -type f -name "*.go" -exec shasum {} \; 2>/dev/null | sort | shasum | awk '{print $1}' || true)
IMPLICIT_BEFORE=$(find ~/.gemini/antigravity-cli/implicit/ -type f 2>/dev/null | wc -l | tr -d ' ' || echo "0")
KNOWLEDGE_BEFORE=$(find ~/.gemini/antigravity-cli/knowledge/ -type f 2>/dev/null | wc -l | tr -d ' ' || echo "0")

###############################################################################
# Phase 2: Condition setup
###############################################################################
echo "[3/8] Setting up condition: ${CONDITION}..."
mkdir -p "${RUN_DATA}" "${RUN_BIN}"

# Copy pinned binary
cp "${PIN_DIR}/repo-memory" "${RUN_BIN}/repo-memory"
cp "${PIN_DIR}/build-index" "${RUN_BIN}/build-index"

# Build env vars for agy
AGY_ENV_VARS="REPO_MEMORY_DATA_DIR=${RUN_DATA}"

case "${CONDITION}" in
    no_memory)
        AGY_ENV_VARS="${AGY_ENV_VARS} REPO_MEMORY_DISABLED=1"
        echo "  [a] No memory — REPO_MEMORY_DISABLED=1"
        ;;
    pa_documents)
        AGY_ENV_VARS="${AGY_ENV_VARS} REPO_MEMORY_DISABLED=1"
        echo "  [b] PA documents — kept in tree, REPO_MEMORY_DISABLED=1"
        ;;
    dossiers)
        echo "  [c] Dossiers — setting up hooks-only delivery..."
        
        # 2a. Build per-run plugin dir
        mkdir -p "${RUN_PLUGIN}"
        
        # Copy plugin.json from installed workspace plugin
        INSTALLED_PLUGIN="${HOME}/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory"
        if [[ -f "${INSTALLED_PLUGIN}/plugin.json" ]]; then
            cp "${INSTALLED_PLUGIN}/plugin.json" "${RUN_PLUGIN}/"
        fi
        
        # Write hooks.json with env-prefixed commands pointing at per-run binary
        cat > "${RUN_PLUGIN}/hooks.json" <<HOOKEOF
{
  "repo-memory": {
    "enabled": true,
    "PreInvocation": [
      {
        "type": "command",
        "command": "env REPO_MEMORY_DATA_DIR=${RUN_DATA} ${RUN_BIN}/repo-memory hook antigravity PreInvocation"
      }
    ],
    "PreToolUse": [
      {
        "matcher": "run_command|write_to_file|replace_file_content|call_mcp_tool",
        "hooks": [
          {
            "type": "command",
            "command": "env REPO_MEMORY_DATA_DIR=${RUN_DATA} ${RUN_BIN}/repo-memory hook antigravity PreToolUse"
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
            "command": "env REPO_MEMORY_DATA_DIR=${RUN_DATA} ${RUN_BIN}/repo-memory hook antigravity PostToolUse"
          }
        ]
      }
    ],
    "Stop": [
      {
        "type": "command",
        "command": "env REPO_MEMORY_DATA_DIR=${RUN_DATA} ${RUN_BIN}/repo-memory hook antigravity Stop"
      }
    ]
  }
}
HOOKEOF

        # Write mcp_config.json
        cat > "${RUN_PLUGIN}/mcp_config.json" <<MCPEOF
{
  "command": "${RUN_BIN}/repo-memory",
  "args": ["mcp"],
  "env": {
    "REPO_MEMORY_DATA_DIR": "${RUN_DATA}"
  }
}
MCPEOF
        
        # 2b. Enroll the run dir
        echo "  [c] Enrolling run dir..."
        REPO_MEMORY_DATA_DIR="${RUN_DATA}" "${RUN_BIN}/repo-memory" enroll "${RUN_DIR}" 2>/dev/null || {
            echo "  ⚠️  Enrollment failed"
            INVALID_REASON="invalid_enrollment_fail"
        }
        
        # 2c. Overwrite plugins.json to point at per-run plugin
        mkdir -p "${RUN_DIR}/.agents"
        cat > "${RUN_DIR}/.agents/plugins.json" <<PLUGEOF
{
  "entries": [
    {
      "path": "${RUN_ROOT}/plugin"
    }
  ]
}
PLUGEOF
        
        # 2d. Read repoID from registry (format: map of paths → {repo_id, root_path, enabled})
        REPO_ID=$(python3 -c "
import json, sys
reg = json.load(open('${RUN_DATA}/registry.json'))
for path, entry in reg.items():
    rid = entry.get('repo_id', entry.get('RepoID', ''))
    if rid:
        print(rid)
        sys.exit(0)
sys.exit(1)
" 2>/dev/null || echo "")
        
        if [[ -z "${REPO_ID}" ]]; then
            echo "  ⚠️  Could not read repoID from registry"
            INVALID_REASON="invalid_no_repoid"
        else
            echo "  [c] repoID: ${REPO_ID}"
        fi
        
        # 2e. Build index
        echo "  [c] Building index..."
        WO_COMMIT_DATE=$(git -C "${REPO_ROOT}" log -1 --format=%cI "${WO_COMMIT}")
        mkdir -p "${RUN_DATA}/${REPO_ID}"
        
        "${RUN_BIN}/build-index" \
            -ops "${EVAL_ROOT}/data/${WO_NAME}/ops" \
            -filtered "${RUN_OPS}" \
            -db "${RUN_DATA}/${REPO_ID}/index.db" \
            -cutoff "${WO_COMMIT_DATE}" \
            > "${RUN_ROOT}/build-index.log" 2>&1 || {
            echo "  ⚠️  Index build failed"
            cat "${RUN_ROOT}/build-index.log"
            INVALID_REASON="invalid_index_build"
        }
        cat "${RUN_ROOT}/build-index.log"
        # Extract dropped ops count from build-index output (R3.3)
        SNAPSHOT_OPS_DROPPED=$(grep -o 'dropped [0-9]* batches ([0-9]* ops)' "${RUN_ROOT}/build-index.log" 2>/dev/null | grep -o '[0-9]* ops' | grep -o '[0-9]*' || echo "0")
        SNAPSHOT_OPS_DROPPED="${SNAPSHOT_OPS_DROPPED:-0}"
        
        # 2f. Preflight: no main checkout path in run tree or plugin dir
        LEAK_HITS=$(grep -r '/Users/mcneillm/Documents' "${RUN_DIR}" "${RUN_PLUGIN}" --exclude-dir=.git 2>/dev/null | wc -l | tr -d ' ' || true)
        if [[ "${LEAK_HITS}" -ne 0 ]]; then
            echo "  ⚠️  Main checkout path found in run tree!"
            INVALID_REASON="invalid_path_leak"
        fi
        
        echo "  [c] Hooks-only setup complete."
        ;;
esac

###############################################################################
# Extract WO text from WO_COMMIT (not HEAD) — N1 fix
###############################################################################
echo "[4/8] Extracting WO text from ${WO_COMMIT:0:12}..."
WO_TEXT=$(git -C "${REPO_ROOT}" show "${WO_COMMIT}:${WO_MD_PATH}" 2>/dev/null | sed '/^## Completion Report/,$d')
WO_LINES=$(echo "$WO_TEXT" | wc -l | tr -d ' ')
echo "  WO text: ${WO_LINES} lines"

# W6 fix: append API contract (same for both conditions)
API_CONTRACT_FILE="${EVAL_ROOT}/oracle/${WO_NAME}/API_CONTRACT.md"
if [[ -f "${API_CONTRACT_FILE}" ]]; then
    API_CONTRACT=$(cat "${API_CONTRACT_FILE}")
    WO_TEXT="${WO_TEXT}

${API_CONTRACT}"
    WO_LINES=$(echo "$WO_TEXT" | wc -l | tr -d ' ')
    echo "  API contract appended (now ${WO_LINES} lines)"
fi

# Check for ticked checklist items (answer leak)
TICKED=$(echo "$WO_TEXT" | grep -c '\- \[x\]' 2>/dev/null || true)
TICKED="${TICKED:-0}"
if [[ "${TICKED}" -ne 0 ]]; then
    echo "  ⚠️  WO text has ${TICKED} ticked checklist items!"
fi

###############################################################################
# Pre-launch check: working tree must be clean
###############################################################################
PORCELAIN_PRE=$(git -C "${RUN_DIR}" status --porcelain)
if [[ -n "${PORCELAIN_PRE}" && "${CONDITION}" != "dossiers" ]]; then
    echo "  ⚠️  Run dir not clean before launch: ${PORCELAIN_PRE}"
fi

###############################################################################
# Phase 5: Run agy
###############################################################################
if [[ -n "${INVALID_REASON}" ]]; then
    echo "[5/8] SKIPPING agy — already invalid: ${INVALID_REASON}"
    AGY_EXIT_CODE=-1
    DURATION=0
    CONVERSATION_ID=""
    TOTAL_TOKENS=0
    AGY_STATUS="SKIPPED"
else
    echo "[5/8] Running agy in clone..."
    START_TIME=$(date +%s)
    
    AGY_OUTPUT="${RUN_ROOT}/agy-output.json"
    AGY_STDERR="${RUN_ROOT}/agy-stderr.log"
    
    AGY_CMD_ARGS=(
        agy
        -p "${WO_TEXT}"
        --output-format json
        --project "x6-eval"
        --dangerously-skip-permissions
    )
    if [[ -n "${AGY_MODEL}" ]]; then
        AGY_CMD_ARGS+=(--model "${AGY_MODEL}")
    fi
    
    AGY_EXIT_CODE=0
    cd "${RUN_DIR}"
    env -i \
        PATH="${PATH}" \
        HOME="${HOME}" \
        USER="${USER}" \
        TMPDIR="${TMPDIR:-/tmp}" \
        GOROOT="$(go env GOROOT)" \
        GOPATH="$(go env GOPATH)" \
        GOCACHE="$(go env GOCACHE)" \
        GOMODCACHE="$(go env GOMODCACHE)" \
        ${AGY_ENV_VARS} \
        "${AGY_CMD_ARGS[@]}" \
        > "${AGY_OUTPUT}" \
        2> "${AGY_STDERR}" || AGY_EXIT_CODE=$?
    cd "${REPO_ROOT}"
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    echo "[5/8] agy finished in ${DURATION}s (exit code: ${AGY_EXIT_CODE})"
    
    # Parse agy output
    if [[ -f "${AGY_OUTPUT}" ]]; then
        CONVERSATION_ID=$(python3 -c "import json; d=json.load(open('${AGY_OUTPUT}')); print(d.get('conversation_id',''))" 2>/dev/null || echo "")
        AGY_STATUS=$(python3 -c "import json; d=json.load(open('${AGY_OUTPUT}')); print(d.get('status','UNKNOWN'))" 2>/dev/null || echo "UNKNOWN")
        TOTAL_TOKENS=$(python3 -c "import json; d=json.load(open('${AGY_OUTPUT}')); u=d.get('usage',{}); print(u.get('total_tokens', int(u.get('input_tokens',0))+int(u.get('output_tokens',0))))" 2>/dev/null || echo "0")
        AGY_ERROR=$(python3 -c "import json; d=json.load(open('${AGY_OUTPUT}')); print(d.get('error',''))" 2>/dev/null || echo "")
    else
        CONVERSATION_ID=""
        AGY_STATUS="NO_OUTPUT"
        TOTAL_TOKENS=0
        AGY_ERROR="no output file"
    fi
    
    # Detect quota exhaustion
    if [[ ${AGY_EXIT_CODE} -ne 0 ]]; then
        if grep -q "RESOURCE_EXHAUSTED" "${AGY_STDERR}" 2>/dev/null || echo "${AGY_ERROR}" | grep -q "RESOURCE_EXHAUSTED"; then
            RESET_INFO=$(grep -o "reset after [0-9hms]*" "${AGY_STDERR}" 2>/dev/null | head -1 || echo "unknown")
            echo "[QUOTA] ⛔ Quota exhausted (${RESET_INFO})."
            INVALID_REASON="invalid_quota"
        fi
    fi
    
    # Check for invalid agy result
    if [[ -z "${INVALID_REASON}" ]]; then
        if [[ ${AGY_EXIT_CODE} -ne 0 ]]; then
            INVALID_REASON="invalid_error"
        elif [[ "${AGY_STATUS}" != "SUCCESS" ]]; then
            INVALID_REASON="invalid_error"
        elif [[ -n "${AGY_ERROR}" ]]; then
            INVALID_REASON="invalid_error"
        fi
    fi
fi

# W5 fix: archive agent's code changes for re-scoring
if [[ -z "${INVALID_REASON}" && -d "${RUN_DIR}" ]]; then
    git -C "${RUN_DIR}" diff HEAD > "${RUN_ROOT}/agent-diff.patch" 2>/dev/null || true
    git -C "${RUN_DIR}" diff HEAD --stat > "${RUN_ROOT}/agent-diff-stat.txt" 2>/dev/null || true
    git -C "${RUN_DIR}" ls-files --others --exclude-standard > "${RUN_ROOT}/agent-untracked.txt" 2>/dev/null || true
fi

###############################################################################
# Phase 6: Contamination checks (after)
###############################################################################
echo "[6/8] Checking contamination..."
MAIN_STATUS_AFTER=$(git -C "${REPO_ROOT}" status --porcelain 2>/dev/null | sort || true)
INTERNAL_HASH_AFTER=$(find "${REPO_ROOT}/internal" "${REPO_ROOT}/cmd" -type f -name "*.go" -exec shasum {} \; 2>/dev/null | sort | shasum | awk '{print $1}' || true)
IMPLICIT_AFTER=$(find ~/.gemini/antigravity-cli/implicit/ -type f 2>/dev/null | wc -l | tr -d ' ' || echo "0")
KNOWLEDGE_AFTER=$(find ~/.gemini/antigravity-cli/knowledge/ -type f 2>/dev/null | wc -l | tr -d ' ' || echo "0")

TRANSCRIPT_SCAN_OK=true

if [[ "${MAIN_STATUS_BEFORE}" != "${MAIN_STATUS_AFTER}" ]]; then
    echo "  ⚠️  Main checkout status changed!"
    INVALID_REASON="invalid_contamination"
fi
if [[ "${INTERNAL_HASH_BEFORE}" != "${INTERNAL_HASH_AFTER}" ]]; then
    echo "  ⚠️  Internal/cmd code changed!"
    INVALID_REASON="invalid_contamination"
fi
if [[ "${IMPLICIT_BEFORE}" != "${IMPLICIT_AFTER}" ]]; then
    echo "  ⚠️  agy implicit store changed (${IMPLICIT_BEFORE} → ${IMPLICIT_AFTER})"
fi

# Transcript scan (allowlist-based)
if [[ -n "${CONVERSATION_ID}" ]]; then
    BRAIN_DIR="${HOME}/.gemini/antigravity-cli/brain/${CONVERSATION_ID}"
    TRANSCRIPT="${BRAIN_DIR}/.system_generated/logs/transcript_full.jsonl"
    if [[ -f "${TRANSCRIPT}" ]]; then
        # Check for main checkout path references
        MAIN_PATH_REFS=$(grep -c '/Users/mcneillm/Documents/Projects' "${TRANSCRIPT}" 2>/dev/null || true)
        MAIN_PATH_REFS="${MAIN_PATH_REFS:-0}"
        if [[ "${MAIN_PATH_REFS}" -gt 0 ]]; then
            echo "  ⚠️  Transcript references main checkout ${MAIN_PATH_REFS} times!"
            TRANSCRIPT_SCAN_OK=false
            INVALID_REASON="invalid_contamination"
        fi
    fi
fi

# Check for tick sidecar activity (only actual tick artifacts, not layout dirs from enroll)
TICK_ACTIVITY=false
if [[ -d "${RUN_DATA}" ]]; then
    # jobs.sqlite is the definitive tick marker
    if find "${RUN_DATA}" -name "jobs.sqlite" 2>/dev/null | grep -q .; then
        TICK_ACTIVITY=true
        echo "  ⚠️  Tick sidecar activity detected (jobs.sqlite found)!"
    # Non-empty queue dir means tick was scheduling work
    elif find "${RUN_DATA}" -path "*/queue/*" -type f 2>/dev/null | grep -q .; then
        TICK_ACTIVITY=true
        echo "  ⚠️  Tick sidecar activity detected (queue files found)!"
    fi
fi

echo "[6/8] Contamination check complete."

###############################################################################
# Phase 7: Hidden tests (if agent ran successfully)
###############################################################################
HIDDEN_TESTS_TOTAL=0
HIDDEN_TESTS_PASSED=0
HIDDEN_TESTS_FRACTION=0
HIDDEN_COMPILE_OK=true
AGENT_TESTS_PASS=false
REGRESSION_TESTS_TOTAL=0
REGRESSION_TESTS_PASSED=0
SNAPSHOT_OPS_DROPPED="${SNAPSHOT_OPS_DROPPED:-0}"

if [[ -z "${INVALID_REASON}" || "${INVALID_REASON}" == "invalid_quota" ]]; then
    if [[ -z "${INVALID_REASON}" ]]; then
        echo "[7/8] Running hidden tests..."
        
        ORACLE_DIR="${EVAL_ROOT}/oracle/${WO_NAME}"
        WO_PKG="${WO_PACKAGES[$WO_NAME]}"
        
        # 7a. Run agent's own tests first (secondary metric)
        # Unset REPO_MEMORY_* env vars so tests don't pollute the run's data dir (R3.4)
        cd "${RUN_DIR}"
        if env -u REPO_MEMORY_DATA_DIR -u REPO_MEMORY_DISABLED -u REPO_MEMORY_HOUSEKEEPING \
            go build ./... 2>/dev/null && \
           env -u REPO_MEMORY_DATA_DIR -u REPO_MEMORY_DISABLED -u REPO_MEMORY_HOUSEKEEPING \
            go test -count=1 ${WO_PKG} > "${RUN_ROOT}/agent-tests.log" 2>&1; then
            AGENT_TESTS_PASS=true
            echo "  Agent's tests: PASS"
        else
            echo "  Agent's tests: FAIL"
        fi
        
        # 7b. Replace test files with oracle's
        if [[ -d "${ORACLE_DIR}/tests" ]]; then
            # W3 fix: delete ALL agent _test.go files in the WO package first
            WO_PKG_DIR=$(echo "${WO_PKG}" | sed 's|^\./||; s|/\.\.\.$||')
            if [[ -d "${RUN_DIR}/${WO_PKG_DIR}" ]]; then
                find "${RUN_DIR}/${WO_PKG_DIR}" -name '*_test.go' -delete
                echo "  Cleared agent test files from ${WO_PKG_DIR}/"
            fi
            
            # Copy oracle test files
            while IFS= read -r test_file; do
                src="${ORACLE_DIR}/tests/${test_file}"
                dst="${RUN_DIR}/${test_file}"
                mkdir -p "$(dirname "${dst}")"
                cp "${src}" "${dst}"
            done < "${ORACLE_DIR}/TEST_FILES"
            
            # Copy testdata if present
            if [[ -d "${ORACLE_DIR}/testdata" ]]; then
                cp -R "${ORACLE_DIR}/testdata/"* "${RUN_DIR}/" 2>/dev/null || true
            fi
            
            # Read calibrated total (R2/F1: use known count even on compile fail)
            CALIBRATED_TOTAL=$(cat "${ORACLE_DIR}/CALIBRATED_TOTAL" 2>/dev/null || echo "0")
            PRIMARY_RUN=$(cat "${ORACLE_DIR}/PRIMARY_RUN" 2>/dev/null || echo "")
            
            if ! env -u REPO_MEMORY_DATA_DIR -u REPO_MEMORY_DISABLED -u REPO_MEMORY_HOUSEKEEPING \
                go build ./... > "${RUN_ROOT}/hidden-compile.log" 2>&1; then
                HIDDEN_COMPILE_OK=false
                HIDDEN_TESTS_TOTAL="${CALIBRATED_TOTAL}"
                HIDDEN_TESTS_FRACTION=0
                echo "  Hidden tests: COMPILE FAIL (go build) — 0/${CALIBRATED_TOTAL}"
            else
                RUN_FLAG=""
                if [[ -n "${PRIMARY_RUN}" ]]; then
                    RUN_FLAG="-run ${PRIMARY_RUN}"
                fi
                
                env -u REPO_MEMORY_DATA_DIR -u REPO_MEMORY_DISABLED -u REPO_MEMORY_HOUSEKEEPING \
                    go test -json -count=1 ${RUN_FLAG} ${WO_PKG} > "${RUN_ROOT}/hidden-tests.json" 2>&1 || true
                
                # Parse go test -json output: count top-level Test* pass/fail
                # Also detect build-fail (test compilation failure)
                HIDDEN_RESULTS=$(python3 -c "
import json, sys
passed = set()
failed = set()
build_failed = False
for line in open('${RUN_ROOT}/hidden-tests.json'):
    try:
        ev = json.loads(line)
        if ev.get('Action') == 'build-fail':
            build_failed = True
        if ev.get('Test') and '/' not in ev['Test']:
            if ev.get('Action') == 'pass':
                passed.add(ev['Test'])
            elif ev.get('Action') == 'fail':
                failed.add(ev['Test'])
    except: pass
total = len(passed | failed)
p = len(passed - failed)
if build_failed and total == 0:
    print(f'BUILD_FAIL 0 0')
else:
    print(f'OK {p} {total}')
" 2>/dev/null || echo "ERROR 0 0")
                
                BUILD_STATUS=$(echo "${HIDDEN_RESULTS}" | awk '{print $1}')
                HIDDEN_TESTS_PASSED=$(echo "${HIDDEN_RESULTS}" | awk '{print $2}')
                HIDDEN_TESTS_TOTAL=$(echo "${HIDDEN_RESULTS}" | awk '{print $3}')
                
                if [[ "${BUILD_STATUS}" == "BUILD_FAIL" ]]; then
                    HIDDEN_COMPILE_OK=false
                    HIDDEN_TESTS_TOTAL="${CALIBRATED_TOTAL}"
                    HIDDEN_TESTS_FRACTION=0
                    echo "  Hidden tests: COMPILE FAIL (test build) — 0/${CALIBRATED_TOTAL}"
                elif [[ ${HIDDEN_TESTS_TOTAL} -gt 0 ]]; then
                    HIDDEN_TESTS_FRACTION=$(python3 -c "print(round(${HIDDEN_TESTS_PASSED}/${HIDDEN_TESTS_TOTAL}, 3))" 2>/dev/null || echo "0")
                    echo "  Hidden tests: ${HIDDEN_TESTS_PASSED}/${HIDDEN_TESTS_TOTAL} (${HIDDEN_TESTS_FRACTION})"
                else
                    echo "  Hidden tests: 0/0 (no test results)"
                fi
            fi
        else
            echo "  ⚠️  No oracle test data for ${WO_NAME}"
        fi
        cd "${REPO_ROOT}"
    fi
else
    echo "[7/8] SKIPPED — run is invalid (${INVALID_REASON})"
fi

###############################################################################
# Phase 8: Delivery validation (condition c only)
###############################################################################
DELIVERY_COUNT="null"
HOOK_PANIC=false
if [[ "${CONDITION}" == "dossiers" && -n "${CONVERSATION_ID}" && -n "${REPO_ID}" ]]; then
    # Check for hook panics in agy log — only the most recent log (this run)
    HOOK_PANIC=false
    LATEST_AGY_LOG=$(ls -t ~/.gemini/antigravity-cli/log/cli-*.log 2>/dev/null | head -1 || true)
    if [[ -n "${LATEST_AGY_LOG}" ]] && grep -q "repo-memory.*panicked" "${LATEST_AGY_LOG}" 2>/dev/null; then
        HOOK_PANIC=true
        echo "  ⚠️  Hook panic detected: $(grep 'repo-memory.*panicked' "${LATEST_AGY_LOG}" | head -1)"
    fi

    # Find ledger file — check both direct and nested data/ paths
    LEDGER_FILE=""
    for candidate in \
        "${RUN_DATA}/${REPO_ID}/ledgers/${CONVERSATION_ID}.json" \
        "${RUN_DATA}/data/${REPO_ID}/ledgers/${CONVERSATION_ID}.json"; do
        if [[ -f "${candidate}" ]]; then
            LEDGER_FILE="${candidate}"
            break
        fi
    done

    if [[ -n "${LEDGER_FILE}" ]]; then
        DELIVERY_COUNT=$(python3 -c "
import json
ledger = json.load(open('${LEDGER_FILE}'))
# Actual format: 'Injections' array (PascalCase from Go struct)
injections = ledger.get('Injections', ledger.get('injections', ledger.get('records', [])))
print(len(injections))
" 2>/dev/null || echo "0")
        echo "[8/8] Delivery ledger: ${DELIVERY_COUNT} injection records"
        if [[ "${HOOK_PANIC}" == "true" ]]; then
            echo "  ⚠️  Hooks panicked — injections were logged but NOT delivered to model"
            if [[ -z "${INVALID_REASON}" ]]; then
                INVALID_REASON="invalid_hook_panic"
            fi
        fi
    else
        if [[ -z "${INVALID_REASON}" ]]; then
            echo "[8/8] ⚠️  No delivery ledger — hooks didn't fire!"
            INVALID_REASON="invalid_no_hooks"
            DELIVERY_COUNT="0"
        fi
    fi
else
    echo "[8/8] Delivery check: N/A (condition=${CONDITION})"
fi

###############################################################################
# Write result
###############################################################################
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
AGY_VERSION=$(agy --version 2>/dev/null | head -1 || echo "unknown")
TOOL_CALL_COUNT=0
if [[ -n "${CONVERSATION_ID}" ]]; then
    TRANSCRIPT="${HOME}/.gemini/antigravity-cli/brain/${CONVERSATION_ID}/.system_generated/logs/transcript_full.jsonl"
    if [[ -f "${TRANSCRIPT}" ]]; then
        TOOL_CALL_COUNT=$(grep -c '"tool_calls"' "${TRANSCRIPT}" 2>/dev/null || true)
        TOOL_CALL_COUNT="${TOOL_CALL_COUNT:-0}"
    fi
fi

ERROR_MSG=""
if [[ -n "${INVALID_REASON}" && "${INVALID_REASON}" != "invalid_quota" ]]; then
    ERROR_MSG="${INVALID_REASON}"
elif [[ -f "${RUN_ROOT}/agy-stderr.log" ]]; then
    ERROR_MSG=$(tail -3 "${RUN_ROOT}/agy-stderr.log" 2>/dev/null | tr '\n' ' ' | head -c 200 || echo "")
fi

# Export all result variables for the Python script
export _WO_NAME="${WO_NAME}" _CONDITION="${CONDITION}" _REPLICATE="${REPLICATE}"
export _ATTEMPT="${ATTEMPT}" _SCHEDULE_INDEX="${SCHEDULE_INDEX}"
export _HIDDEN_TESTS_TOTAL="${HIDDEN_TESTS_TOTAL}" _HIDDEN_TESTS_PASSED="${HIDDEN_TESTS_PASSED}"
export _HIDDEN_TESTS_FRACTION="${HIDDEN_TESTS_FRACTION}" _HIDDEN_COMPILE_OK="${HIDDEN_COMPILE_OK}"
export _AGENT_TESTS_PASS="${AGENT_TESTS_PASS}" _REGRESSION_TESTS_TOTAL="${REGRESSION_TESTS_TOTAL}"
export _REGRESSION_TESTS_PASSED="${REGRESSION_TESTS_PASSED}" _AGY_EXIT_CODE="${AGY_EXIT_CODE}"
export _TOTAL_TOKENS="${TOTAL_TOKENS}" _DURATION="${DURATION}"
export _CONVERSATION_ID="${CONVERSATION_ID}" _BASE_SHA_ORIGINAL="${WO_COMMIT}"
export _BASE_SHA_FILTERED="${BASE_SHA_FILTERED}" _PIN_COMMIT="${PIN_COMMIT}"
export _BINARY_SHA256="${BINARY_SHA256}" _BUILD_INDEX_SHA256="${BUILD_INDEX_SHA256}"
export _AGY_VERSION="${AGY_VERSION}" _AGY_MODEL="${AGY_MODEL}"
export _DELIVERY_COUNT="${DELIVERY_COUNT}" _TOOL_CALL_COUNT="${TOOL_CALL_COUNT}"
export _TRANSCRIPT_SCAN_OK="${TRANSCRIPT_SCAN_OK}" _TICK_ACTIVITY="${TICK_ACTIVITY}"
export _INVALID_REASON="${INVALID_REASON}" _ERROR_MSG="${ERROR_MSG}" _TIMESTAMP="${TIMESTAMP}"
export _SNAPSHOT_OPS_DROPPED="${SNAPSHOT_OPS_DROPPED}" _HOOK_PANIC="${HOOK_PANIC}"
export _ANALYSIS_SET="${ANALYSIS_SET:-block1-w6}"

# Write result JSON using a standalone Python script (avoids nested shell/python issues)
RESULT_JSON=$(python3 << 'PYEOF'
import json, os

def to_bool(s):
    return s.lower() == 'true'

def to_int(s):
    try: return int(s)
    except: return 0

def to_float(s):
    try: return float(s)
    except: return 0.0

env = os.environ
result = {
    'work_order': env.get('_WO_NAME', ''),
    'condition': env.get('_CONDITION', ''),
    'replicate': to_int(env.get('_REPLICATE', '0')),
    'attempt': to_int(env.get('_ATTEMPT', '0')),
    'schedule_index': to_int(env.get('_SCHEDULE_INDEX', '0')),
    'hidden_tests_total': to_int(env.get('_HIDDEN_TESTS_TOTAL', '0')),
    'hidden_tests_passed': to_int(env.get('_HIDDEN_TESTS_PASSED', '0')),
    'hidden_tests_fraction': to_float(env.get('_HIDDEN_TESTS_FRACTION', '0')),
    'hidden_compile_ok': to_bool(env.get('_HIDDEN_COMPILE_OK', 'true')),
    'agent_tests_pass': to_bool(env.get('_AGENT_TESTS_PASS', 'false')),
    'regression_tests_total': to_int(env.get('_REGRESSION_TESTS_TOTAL', '0')),
    'regression_tests_passed': to_int(env.get('_REGRESSION_TESTS_PASSED', '0')),
    'agy_exit_code': to_int(env.get('_AGY_EXIT_CODE', '-1')),
    'total_tokens': to_int(env.get('_TOTAL_TOKENS', '0')),
    'duration_seconds': to_int(env.get('_DURATION', '0')),
    'conversation_id': env.get('_CONVERSATION_ID', ''),
    'base_sha_original': env.get('_BASE_SHA_ORIGINAL', ''),
    'base_sha_filtered': env.get('_BASE_SHA_FILTERED', ''),
    'condition_manifest_hash': '',
    'pin_commit': env.get('_PIN_COMMIT', ''),
    'binary_sha256': env.get('_BINARY_SHA256', ''),
    'build_index_sha256': env.get('_BUILD_INDEX_SHA256', ''),
    'agy_version': env.get('_AGY_VERSION', ''),
    'model_name': env.get('_AGY_MODEL', ''),
    'agent_resolved': 'default',
    'delivery_count': None if env.get('_DELIVERY_COUNT', 'null') == 'null' else to_int(env.get('_DELIVERY_COUNT', '0')),
    'tool_call_count': to_int(env.get('_TOOL_CALL_COUNT', '0')),
    'mcp_tool_calls': 0,
    'transcript_scan_ok': to_bool(env.get('_TRANSCRIPT_SCAN_OK', 'true')),
    'tick_activity': to_bool(env.get('_TICK_ACTIVITY', 'false')),
    'invalid_reason': env.get('_INVALID_REASON', '') or None,
    'error': env.get('_ERROR_MSG', '') or '',
    'timestamp': env.get('_TIMESTAMP', ''),
    'snapshot_ops_dropped': to_int(env.get('_SNAPSHOT_OPS_DROPPED', '0')),
    'hook_panic': to_bool(env.get('_HOOK_PANIC', 'false')),
    'analysis_set': env.get('_ANALYSIS_SET', 'block1'),
}
print(json.dumps(result, sort_keys=True))
PYEOF
)

mkdir -p "$(dirname "${RESULTS_FILE}")"
echo "${RESULT_JSON}" >> "${RESULTS_FILE}"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "RESULT: ${WO_NAME} / ${CONDITION} / rep${REPLICATE}"
echo "  Hidden tests: ${HIDDEN_TESTS_PASSED}/${HIDDEN_TESTS_TOTAL} (${HIDDEN_TESTS_FRACTION})"
echo "  Agent tests:  ${AGENT_TESTS_PASS}"
echo "  Tokens:       ${TOTAL_TOKENS}"
echo "  Duration:     ${DURATION}s"
echo "  agy exit:     ${AGY_EXIT_CODE}"
echo "  Delivery:     ${DELIVERY_COUNT}"
if [[ -n "${INVALID_REASON}" ]]; then
echo "  INVALID:      ${INVALID_REASON}"
fi
echo "════════════════════════════════════════════════════════════════"

# Exit code: 0 = valid run, 1 = invalid, 2 = quota exhausted
if [[ "${INVALID_REASON}" == "invalid_quota" ]]; then
    exit 2
elif [[ -n "${INVALID_REASON}" ]]; then
    exit 1
fi
exit 0
