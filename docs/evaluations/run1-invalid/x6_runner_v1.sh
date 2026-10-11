#!/usr/bin/env bash
# X6-lite Evaluation Runner
# Usage: ./x6_runner.sh <wo_name> <condition> <seed>
#   wo_name:   e.g. wo1-op-log
#   condition: no_memory | pa_documents | dossiers
#   seed:      42 | 137 | 256
#
# Results are appended to evaluation/results/x6-lite-results.jsonl

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
EVAL_DATA="${REPO_ROOT}/evaluation/data"
RESULTS_FILE="${REPO_ROOT}/evaluation/results/x6-lite-results.jsonl"
WORKTREE_BASE="/tmp/x6-lite-runs"

WO_NAME="${1:?Usage: x6_runner.sh <wo_name> <condition> <seed>}"
CONDITION="${2:?Usage: x6_runner.sh <wo_name> <condition> <seed>}"
SEED="${3:?Usage: x6_runner.sh <wo_name> <condition> <seed>}"

# Map WO names to their WORKORDER.md paths and parent commits
declare -A WO_PATHS
WO_PATHS["wo1-op-log"]="pr-07-v2-storage/wo1-op-log"
WO_PATHS["wo2-dossier-schema-renderer"]="pr-07-v2-storage/wo2-dossier-schema-renderer"
WO_PATHS["wo3-scopes-aliases"]="pr-07-v2-storage/wo3-scopes-aliases"
WO_PATHS["wo4-index-migration"]="pr-07-v2-storage/wo4-index-migration"
WO_PATHS["wo6-seeding"]="pr-07-v2-storage/wo6-seeding"
WO_PATHS["wo1-ledger-engine"]="pr-08-v2-delivery/wo1-ledger-engine"
WO_PATHS["wo4-gates"]="pr-08-v2-delivery/wo4-gates"
WO_PATHS["wo6-pull-tools"]="pr-08-v2-delivery/wo6-pull-tools"

declare -A WO_COMMITS
WO_COMMITS["wo1-op-log"]="42deb642bda94c783f8e070d00eed2aba298d778"
WO_COMMITS["wo2-dossier-schema-renderer"]="0fbfe5a6f8948d7db8dddfc2f46fee9014d76021"
WO_COMMITS["wo3-scopes-aliases"]="3177bfe3cdb9ae98be10c4c60cd69d1a3d1d38d8"
WO_COMMITS["wo4-index-migration"]="501d05d453960b4c59e4e25eb25cfb557bc53948"
WO_COMMITS["wo6-seeding"]="ada94767a9d925dca5591b67b0ecc856fd8449bf"
WO_COMMITS["wo1-ledger-engine"]="732177442cd0179866a184955a640510a4aeda64"
WO_COMMITS["wo4-gates"]="e4acaa2a5cd38af1f3205a4ab0bd64f6930ff6b9"
WO_COMMITS["wo6-pull-tools"]="6a840b3893db013834460cba133eefafbd3b9413"

# Exit test commands per WO (populated from WORKORDER.md verification sections)
declare -A WO_EXIT_TESTS
WO_EXIT_TESTS["wo1-op-log"]="go build ./... && go test ./internal/core/ops/... && go vet ./internal/core/ops/..."
WO_EXIT_TESTS["wo2-dossier-schema-renderer"]="go build ./... && go test ./internal/core/dossiers/... && go vet ./internal/core/dossiers/..."
WO_EXIT_TESTS["wo3-scopes-aliases"]="go build ./... && go test ./internal/core/config/... && go vet ./internal/core/config/..."
WO_EXIT_TESTS["wo4-index-migration"]="go build ./... && go test ./internal/core/index/... && go vet ./internal/core/index/..."
WO_EXIT_TESTS["wo6-seeding"]="go build ./... && go test ./internal/core/seed/... && go vet ./internal/core/seed/..."
WO_EXIT_TESTS["wo1-ledger-engine"]="go build ./... && go test ./internal/core/delivery/... && go vet ./internal/core/delivery/..."
WO_EXIT_TESTS["wo4-gates"]="go build ./... && go test ./internal/harness/antigravity/... && go vet ./..."
WO_EXIT_TESTS["wo6-pull-tools"]="go build ./... && go test ./internal/mcpserver/... && go vet ./..."

# Validate inputs
if [[ -z "${WO_PATHS[$WO_NAME]+_}" ]]; then
    echo "ERROR: Unknown work order: $WO_NAME" >&2
    exit 1
fi

if [[ "$CONDITION" != "no_memory" && "$CONDITION" != "pa_documents" && "$CONDITION" != "dossiers" ]]; then
    echo "ERROR: Unknown condition: $CONDITION (must be no_memory|pa_documents|dossiers)" >&2
    exit 1
fi

WO_PATH=".agents/skills/principal-architect-workspace/${WO_PATHS[$WO_NAME]}/WORKORDER.md"
PARENT_COMMIT="${WO_COMMITS[$WO_NAME]}"
EXIT_TEST_CMD="${WO_EXIT_TESTS[$WO_NAME]}"
RUN_ID="${WO_NAME}-${CONDITION}-seed${SEED}"
WORKTREE_DIR="${WORKTREE_BASE}/${RUN_ID}"
BRANCH_NAME="x6-run/${RUN_ID}"

# Skip if we already have a valid result for this run
if [[ -f "${RESULTS_FILE}" ]]; then
    EXISTING=$(python3 -c "
import json
with open('${RESULTS_FILE}') as f:
    for line in f:
        r = json.loads(line.strip())
        if r['work_order'] == '${WO_NAME}' and r['condition'] == '${CONDITION}' and r['seed'] == ${SEED} and r.get('total_tokens', 0) > 0:
            print('SKIP')
            break
" 2>/dev/null || echo "")
    if [[ "${EXISTING}" == "SKIP" ]]; then
        echo "[SKIP] ${RUN_ID} — valid result already exists."
        exit 0
    fi
fi

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║ X6-lite Run: ${RUN_ID}"
echo "║ Parent commit: ${PARENT_COMMIT:0:12}"
echo "║ Condition: ${CONDITION}"
echo "║ Exit test: ${EXIT_TEST_CMD}"
echo "╚══════════════════════════════════════════════════════════════╝"

# Cleanup function
cleanup() {
    echo "[cleanup] Removing worktree ${WORKTREE_DIR}..."
    git -C "${REPO_ROOT}" worktree remove --force "${WORKTREE_DIR}" 2>/dev/null || true
    git -C "${REPO_ROOT}" branch -D "${BRANCH_NAME}" 2>/dev/null || true
    echo "[cleanup] Done."
}
trap cleanup EXIT

# Step 1: Create isolated worktree at parent commit
echo "[1/5] Creating worktree at parent commit ${PARENT_COMMIT:0:12}~1..."
mkdir -p "${WORKTREE_BASE}"
git -C "${REPO_ROOT}" worktree remove --force "${WORKTREE_DIR}" 2>/dev/null || true
git -C "${REPO_ROOT}" branch -D "${BRANCH_NAME}" 2>/dev/null || true
git -C "${REPO_ROOT}" worktree add -b "${BRANCH_NAME}" "${WORKTREE_DIR}" "${PARENT_COMMIT}~1"
echo "[1/5] Worktree created at ${WORKTREE_DIR}"

# Step 2: Inject condition
echo "[2/5] Injecting condition: ${CONDITION}..."
case "${CONDITION}" in
    no_memory)
        # Disable memory in the worktree
        mkdir -p "${WORKTREE_DIR}/.memory"
        echo "enabled: false" > "${WORKTREE_DIR}/.memory/config.yaml"
        echo "[2/5] Memory disabled."
        ;;
    pa_documents)
        # Enable memory + copy PA docs
        mkdir -p "${WORKTREE_DIR}/.memory"
        echo "enabled: true" > "${WORKTREE_DIR}/.memory/config.yaml"
        PA_SRC="${EVAL_DATA}/${WO_NAME}/pa_docs"
        if [[ -d "${PA_SRC}" ]]; then
            PA_DST="${WORKTREE_DIR}/.agents/skills/principal-architect-workspace/memory"
            mkdir -p "${PA_DST}"
            cp -R "${PA_SRC}/" "${PA_DST}/"
            echo "[2/5] PA documents injected from ${PA_SRC}"
        else
            echo "[2/5] WARNING: No PA docs found at ${PA_SRC}"
        fi
        ;;
    dossiers)
        # Enable memory + copy time-cut dossiers
        mkdir -p "${WORKTREE_DIR}/.memory"
        echo "enabled: true" > "${WORKTREE_DIR}/.memory/config.yaml"
        DOSSIER_SRC="${EVAL_DATA}/${WO_NAME}/dossiers"
        OPS_SRC="${EVAL_DATA}/${WO_NAME}/ops"
        ATLAS_SRC="${EVAL_DATA}/${WO_NAME}/ATLAS.md"
        if [[ -d "${DOSSIER_SRC}" ]]; then
            DOSSIER_DST="${WORKTREE_DIR}/.memory/dossiers"
            mkdir -p "${DOSSIER_DST}"
            cp -R "${DOSSIER_SRC}/" "${DOSSIER_DST}/"
            echo "[2/5] Dossiers injected from ${DOSSIER_SRC}"
        fi
        if [[ -d "${OPS_SRC}" ]]; then
            OPS_DST="${WORKTREE_DIR}/.memory/ops"
            mkdir -p "${OPS_DST}"
            cp -R "${OPS_SRC}/" "${OPS_DST}/"
            echo "[2/5] Ops injected from ${OPS_SRC}"
        fi
        if [[ -f "${ATLAS_SRC}" ]]; then
            cp "${ATLAS_SRC}" "${WORKTREE_DIR}/.memory/ATLAS.md"
            echo "[2/5] ATLAS.md injected."
        fi
        ;;
esac

# Step 3: Extract WO text (strip the completion report — we only give the task, not the answer)
echo "[3/5] Extracting WO text..."
WO_FULL_PATH="${REPO_ROOT}/${WO_PATH}"
# Extract everything up to "## Completion Report" to avoid leaking the answer
WO_TEXT=$(sed '/^## Completion Report/,$d' "${WO_FULL_PATH}")
echo "[3/5] WO text extracted ($(echo "$WO_TEXT" | wc -l | tr -d ' ') lines)"

# Step 4: Run agy headless
echo "[4/5] Running agy (seed=${SEED})..."
START_TIME=$(date +%s)
AGY_PROJECT="repo-memory-x6-${RUN_ID}"

AGY_OUTPUT_FILE="${WORKTREE_BASE}/${RUN_ID}-agy-output.json"
AGY_STDERR_FILE="${WORKTREE_BASE}/${RUN_ID}-agy-stderr.log"

AGY_EXIT_CODE=0
REPO_MEMORY_HOUSEKEEPING=1 agy \
    --agent "Execution-Worker" \
    -p "${WO_TEXT}" \
    --output-format json \
    --project "${AGY_PROJECT}" \
    --dangerously-skip-permissions \
    > "${AGY_OUTPUT_FILE}" \
    2> "${AGY_STDERR_FILE}" || AGY_EXIT_CODE=$?

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
echo "[4/5] agy finished in ${DURATION}s (exit code: ${AGY_EXIT_CODE})"

# Detect quota exhaustion: fast exit + RESOURCE_EXHAUSTED in stderr
if [[ ${DURATION} -lt 30 && ${AGY_EXIT_CODE} -ne 0 ]]; then
    if grep -q "RESOURCE_EXHAUSTED" "${AGY_STDERR_FILE}" 2>/dev/null; then
        RESET_INFO=$(grep -o "reset after [0-9hms]*" "${AGY_STDERR_FILE}" 2>/dev/null | head -1)
        echo "[QUOTA] ⛔ Quota exhausted (${RESET_INFO}). Aborting run — no result recorded."
        exit 2
    fi
fi

# Step 5: Run exit tests and collect results
echo "[5/5] Running exit tests..."
cd "${WORKTREE_DIR}"

EXIT_TEST_PASS="false"
EXIT_TEST_OUTPUT=""
if eval "${EXIT_TEST_CMD}" > /tmp/x6-exit-test-${RUN_ID}.log 2>&1; then
    EXIT_TEST_PASS="true"
    echo "[5/5] EXIT TESTS: PASSED ✓"
else
    echo "[5/5] EXIT TESTS: FAILED ✗"
fi
EXIT_TEST_OUTPUT=$(cat /tmp/x6-exit-test-${RUN_ID}.log 2>/dev/null | tail -20 || echo "no output")
cd "${REPO_ROOT}"

# Parse tokens and steps from agy output
TOTAL_TOKENS=0
STEPS_TAKEN=0
if [[ -f "${AGY_OUTPUT_FILE}" ]]; then
    TOTAL_TOKENS=$(python3 -c "
import json
try:
    d = json.load(open('${AGY_OUTPUT_FILE}'))
    u = d.get('usage', {})
    print(u.get('total_tokens', int(u.get('input_tokens', 0)) + int(u.get('output_tokens', 0))))
except: print(0)
" 2>/dev/null || echo "0")

    STEPS_TAKEN=$(python3 -c "
import json
try:
    d = json.load(open('${AGY_OUTPUT_FILE}'))
    print(d.get('num_turns', 0))
except: print(0)
" 2>/dev/null || echo "0")
fi

# Determine overall status — exit test result is the PRIMARY metric (X6 protocol)
# agy exit code is recorded as secondary signal but doesn't override exit tests
STATUS="fail"
if [[ "${EXIT_TEST_PASS}" == "true" ]]; then
    STATUS="pass"
fi

# Write result
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
ERROR_MSG=""
if [[ "${AGY_EXIT_CODE}" -ne 0 ]]; then
    ERROR_MSG=$(tail -5 "${AGY_STDERR_FILE}" 2>/dev/null | tr '\n' ' ' || echo "agy exit code ${AGY_EXIT_CODE}")
fi

RESULT_JSON=$(python3 -c "
import json
exit_pass = '${EXIT_TEST_PASS}' == 'true'
print(json.dumps({
    'work_order': '${WO_NAME}',
    'condition': '${CONDITION}',
    'seed': ${SEED},
    'status': '${STATUS}',
    'exit_tests_pass': exit_pass,
    'agy_exit_code': ${AGY_EXIT_CODE},
    'steps_taken': ${STEPS_TAKEN},
    'total_tokens': ${TOTAL_TOKENS},
    'duration_seconds': ${DURATION},
    'error': '${ERROR_MSG}',
    'timestamp': '${TIMESTAMP}'
}, sort_keys=True))
")

echo "${RESULT_JSON}" >> "${RESULTS_FILE}"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "RESULT: ${RUN_ID}"
echo "  Status:       ${STATUS}"
echo "  Exit tests:   ${EXIT_TEST_PASS}"
echo "  Steps:        ${STEPS_TAKEN}"
echo "  Tokens:       ${TOTAL_TOKENS}"
echo "  Duration:     ${DURATION}s"
echo "  agy exit:     ${AGY_EXIT_CODE}"
echo "════════════════════════════════════════════════════════════════"
