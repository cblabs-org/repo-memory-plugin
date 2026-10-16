#!/usr/bin/env bash
# ==============================================================================
# build_index.sh — Build Seeded repo-memory Index from PA Workspace
# ==============================================================================
# Usage:
#   ./evaluation/eval-v2/scripts/build_index.sh [--clean] [PA_WORKSPACE_DIR] [OUTPUT_DIR]
#
# Options:
#   --clean, -c       Remove existing ops and index in OUTPUT_DIR before seeding.
#   -h, --help        Show this help message.
#
# Arguments:
#   PA_WORKSPACE_DIR  Path to PA workspace directory containing project directories.
#                     Default: evaluation/eval-v2/template/.agents/skills/principal-architect-workspace
#   OUTPUT_DIR        Path to directory where seed ops and index.db will be stored.
#                     Default: evaluation/eval-v2/seed
#
# Environment variables:
#   REPO_MEMORY_BIN   Override path to repo-memory binary (default: ~/.local/bin/repo-memory)
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${EVAL_DIR}/../.." && pwd)"

CLEAN=false
PA_DIR=""
OUT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean|-c)
      CLEAN=true
      shift
      ;;
    -h|--help)
      echo "Usage: ./evaluation/eval-v2/scripts/build_index.sh [--clean] [PA_WORKSPACE_DIR] [OUTPUT_DIR]"
      exit 0
      ;;
    *)
      if [ -z "${PA_DIR}" ]; then
        PA_DIR="$1"
      elif [ -z "${OUT_DIR}" ]; then
        OUT_DIR="$1"
      fi
      shift
      ;;
  esac
done

PA_DIR="${PA_DIR:-${EVAL_DIR}/template/.agents/skills/principal-architect-workspace}"
OUT_DIR="${OUT_DIR:-${EVAL_DIR}/seed}"
TEMPLATE_DIR="${EVAL_DIR}/template"
BASELINE_SHA="45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa"

REPO_MEMORY_BIN="${REPO_MEMORY_BIN:-${HOME}/.local/bin/repo-memory}"
mkdir -p "$(dirname "${REPO_MEMORY_BIN}")"

echo "Building latest repo-memory binary at ${REPO_MEMORY_BIN}..."
(cd "${REPO_ROOT}" && go build -o "${REPO_MEMORY_BIN}" ./cmd/repo-memory)

if [ ! -x "${REPO_MEMORY_BIN}" ]; then
  echo "Error: repo-memory binary not found or not executable at: ${REPO_MEMORY_BIN}" >&2
  exit 1
fi

if [ ! -d "${TEMPLATE_DIR}" ]; then
  echo "Error: template directory does not exist: ${TEMPLATE_DIR}" >&2
  exit 1
fi

if [ ! -d "${PA_DIR}" ]; then
  echo "Error: PA workspace directory does not exist: ${PA_DIR}" >&2
  exit 1
fi

if [ "${CLEAN}" = true ]; then
  echo "Cleaning existing seed artifacts in ${OUT_DIR}..."
  rm -rf "${OUT_DIR}/ops" "${OUT_DIR}/dossiers" "${OUT_DIR}/ATLAS.md" "${OUT_DIR}/index.db"
fi

# Ensure output directories exist
mkdir -p "${OUT_DIR}/ops"

# Resolve absolute paths
TEMPLATE_DIR="$(cd "${TEMPLATE_DIR}" && pwd)"
PA_DIR="$(cd "${PA_DIR}" && pwd)"
OUT_DIR="$(cd "${OUT_DIR}" && pwd)"

echo "======================================================================"
echo "          repo-memory v2.2 Seed & Index Builder"
echo "======================================================================"
echo "Template Dir: ${TEMPLATE_DIR}"
echo "PA Workspace: ${PA_DIR}"
echo "Output Dir:   ${OUT_DIR}"
echo "Binary:       ${REPO_MEMORY_BIN}"
echo "----------------------------------------------------------------------"

# 1. Run PA Workorder Seeder
echo "[1/5] Running repo-memory seed (PA completion reports)..."
"${REPO_MEMORY_BIN}" seed \
  --repo "${TEMPLATE_DIR}" \
  --pa-dir "${PA_DIR}" \
  --ops-dir "${OUT_DIR}/ops" \
  --before "${BASELINE_SHA}"

# 2. Run Documentation & Skill Seeder
echo "[2/5] Running repo-memory seed-docs (curated docs & skills)..."
"${REPO_MEMORY_BIN}" seed-docs \
  --repo "${TEMPLATE_DIR}" \
  --ops-dir "${OUT_DIR}/ops"

# 3. Run Ops GC & Project Dossiers + ATLAS.md
echo "[3/5] Running repo-memory ops gc & projecting dossiers + ATLAS.md..."
"${REPO_MEMORY_BIN}" ops gc \
  --repo "${TEMPLATE_DIR}" \
  --ops-dir "${OUT_DIR}/ops" \
  --dossiers-dir "${OUT_DIR}"

OP_COUNT=$(find "${OUT_DIR}/ops" -type f -name "*.json" | wc -l | tr -d ' ')
DOSSIER_COUNT=$(find "${OUT_DIR}/dossiers" -type f -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
echo "  ✓ Op batch files in ${OUT_DIR}/ops: ${OP_COUNT}"
echo "  ✓ Projected dossiers in ${OUT_DIR}/dossiers: ${DOSSIER_COUNT}"

# 4. Rebuild SQLite Index
echo "[4/5] Building SQLite index..."
TMP_WORK_DIR=$(mktemp -d /tmp/seed_work.XXXXXX)
TMP_DATA_DIR=$(mktemp -d /tmp/seed_data.XXXXXX)

cleanup() {
  rm -rf "${TMP_WORK_DIR}" "${TMP_DATA_DIR}"
}
trap cleanup EXIT INT TERM

# Initialize hermetic git repo so gitops.FindRepoRoot succeeds without touching template
git -C "${TMP_WORK_DIR}" init -q
mkdir -p "${TMP_WORK_DIR}/.memory"
cp -R "${OUT_DIR}/ops" "${TMP_WORK_DIR}/.memory/ops"
if [ -d "${OUT_DIR}/dossiers" ]; then
  cp -R "${OUT_DIR}/dossiers" "${TMP_WORK_DIR}/.memory/dossiers"
fi
if [ -f "${OUT_DIR}/ATLAS.md" ]; then
  cp "${OUT_DIR}/ATLAS.md" "${TMP_WORK_DIR}/.memory/ATLAS.md"
fi

# Execute reindex pointing data dir to TMP_DATA_DIR
REPO_MEMORY_DATA_DIR="${TMP_DATA_DIR}" "${REPO_MEMORY_BIN}" reindex "${TMP_WORK_DIR}"

# Locate generated index.db
GENERATED_DB=$(find "${TMP_DATA_DIR}" -name "index.db" -type f | head -n 1)
if [ -z "${GENERATED_DB}" ] || [ ! -f "${GENERATED_DB}" ]; then
  echo "Error: index.db was not generated by reindex" >&2
  exit 1
fi

# Checkpoint SQLite WAL mode cleanly before copying
if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "${GENERATED_DB}" "PRAGMA wal_checkpoint(TRUNCATE);" >/dev/null 2>&1 || true
elif command -v python3 >/dev/null 2>&1; then
  python3 -c "import sqlite3, sys; c=sqlite3.connect(sys.argv[1]); c.execute('PRAGMA wal_checkpoint(TRUNCATE)'); c.close()" "${GENERATED_DB}" >/dev/null 2>&1 || true
fi

cp "${GENERATED_DB}" "${OUT_DIR}/index.db"
echo "  ✓ Copied built index to ${OUT_DIR}/index.db"

# 5. Verification
echo "[5/5] Verifying index..."
if command -v sqlite3 >/dev/null 2>&1; then
  ENTRY_COUNT=$(sqlite3 "${OUT_DIR}/index.db" "SELECT COUNT(*) FROM entries;" 2>/dev/null || echo "0")
  ANCHOR_COUNT=$(sqlite3 "${OUT_DIR}/index.db" "SELECT COUNT(*) FROM entry_anchors;" 2>/dev/null || echo "0")
  FTS_COUNT=$(sqlite3 "${OUT_DIR}/index.db" "SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'display OR portal OR OTA';" 2>/dev/null || echo "0")

  echo "  ✓ Total entries in index: ${ENTRY_COUNT}"
  echo "  ✓ Total entry anchors:    ${ANCHOR_COUNT}"
  echo "  --- Breakdown by source ---"
  sqlite3 "${OUT_DIR}/index.db" "SELECT '    ' || source || ': ' || COUNT(*) FROM entries GROUP BY source ORDER BY COUNT(*) DESC;"
  echo "  --- Breakdown by kind ---"
  sqlite3 "${OUT_DIR}/index.db" "SELECT '    ' || kind || ': ' || COUNT(*) FROM entries GROUP BY kind ORDER BY COUNT(*) DESC;"
  echo "  --- Breakdown by type ---"
  sqlite3 "${OUT_DIR}/index.db" "SELECT '    ' || type || ': ' || COUNT(*) FROM entries GROUP BY type ORDER BY COUNT(*) DESC;"
  echo "  ✓ FTS matches for 'display OR portal OR OTA': ${FTS_COUNT}"

  if [ "${ENTRY_COUNT}" -lt 50 ]; then
    echo "  ⚠️ Warning: Entry count (${ENTRY_COUNT}) is lower than expected (≥50)." >&2
  fi
elif command -v python3 >/dev/null 2>&1; then
  python3 - "${OUT_DIR}/index.db" <<'PYEOF'
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
cur = db.cursor()
entry_count = cur.execute("SELECT COUNT(*) FROM entries").fetchone()[0]
anchor_count = cur.execute("SELECT COUNT(*) FROM entry_anchors").fetchone()[0]
fts_count = cur.execute("SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH 'display OR portal OR OTA'").fetchone()[0]
print(f"  ✓ Total entries in index: {entry_count}")
print(f"  ✓ Total entry anchors:    {anchor_count}")
print("  --- Breakdown by source ---")
for src, cnt in cur.execute("SELECT source, COUNT(*) FROM entries GROUP BY source ORDER BY COUNT(*) DESC"):
    print(f"    {src}: {cnt}")
print("  --- Breakdown by kind ---")
for kind, cnt in cur.execute("SELECT kind, COUNT(*) FROM entries GROUP BY kind ORDER BY COUNT(*) DESC"):
    print(f"    {kind}: {cnt}")
print("  --- Breakdown by type ---")
for etype, cnt in cur.execute("SELECT type, COUNT(*) FROM entries GROUP BY type ORDER BY COUNT(*) DESC"):
    print(f"    {etype}: {cnt}")
print(f"  ✓ FTS matches for 'display OR portal OR OTA': {fts_count}")
if entry_count < 50:
    sys.stderr.write(f"  ⚠️ Warning: Entry count ({entry_count}) is lower than expected (≥50).\n")
PYEOF
else
  echo "  sqlite3/python3 not found, skipping detailed SQL query verification."
fi

echo "======================================================================"
echo "Index build completed successfully."
echo "======================================================================"
