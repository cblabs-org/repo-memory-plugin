#!/usr/bin/env bash
# ==============================================================================
# prepare_templates.sh — Stage esp32-app-foundation Evaluation Templates
# ==============================================================================
# Clones matthewmcneill/esp32-app-foundation at commit
# 45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa into evaluation/eval-v2/template/
# (preserving commit history for --before seeding and doc-seed git timestamps,
# removing the origin remote and tagging baseline), and creates isolated
# single-commit condition templates tpl-a, tpl-b, tpl-c, and tpl-d.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BASELINE_SHA="45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa"
REPO_URL="https://github.com/matthewmcneill/esp32-app-foundation.git"
TEMPLATE_DIR="${EVAL_ROOT}/template"
GIT_AUTHOR_NAME="matthewmcneill"
GIT_AUTHOR_EMAIL="97467074+matthewmcneill@users.noreply.github.com"

clone_tree_portable() {
  local src="$1"
  local dst="$2"
  if [ "$(uname -s)" = "Darwin" ]; then
    cp -c -R "${src}" "${dst}"
  else
    cp --reflink=auto -a "${src}" "${dst}" 2>/dev/null || cp -a "${src}" "${dst}"
  fi
}

echo "======================================================================"
echo "      Staging esp32-app-foundation Evaluation Templates (eval-v2)"
echo "======================================================================"
echo "Eval Root:    ${EVAL_ROOT}"
echo "Baseline SHA: ${BASELINE_SHA}"
echo "----------------------------------------------------------------------"

# 1. Stage master template/
echo "[1/2] Staging master template at ${TEMPLATE_DIR}..."
if [ ! -d "${TEMPLATE_DIR}/.git" ]; then
  rm -rf "${TEMPLATE_DIR}"
  git clone "${REPO_URL}" "${TEMPLATE_DIR}"
fi

# Checkout baseline SHA if present in remote history, or fallback to PR #291 merge commit (c7dc8d88)
if git -C "${TEMPLATE_DIR}" cat-file -e "${BASELINE_SHA}^{commit}" 2>/dev/null; then
  git -C "${TEMPLATE_DIR}" checkout -f "${BASELINE_SHA}"
else
  git -C "${TEMPLATE_DIR}" checkout -f c7dc8d88ec703d61597817533cfcaf29d460e3fd
fi
git -C "${TEMPLATE_DIR}" remote remove origin 2>/dev/null || true
git -C "${TEMPLATE_DIR}" tag -f baseline HEAD
git -C "${TEMPLATE_DIR}" config user.name "${GIT_AUTHOR_NAME}"
git -C "${TEMPLATE_DIR}" config user.email "${GIT_AUTHOR_EMAIL}"

# Apply standard template exclusions if any untracked/local files exist
rm -rf \
  "${TEMPLATE_DIR}/private_key.pem" \
  "${TEMPLATE_DIR}/.pio/build" \
  "${TEMPLATE_DIR}/.agents/plugins.json" \
  "${TEMPLATE_DIR}/.memory" \
  "${TEMPLATE_DIR}/.vscode" \
  "${TEMPLATE_DIR}/.agents/tmp" \
  "${TEMPLATE_DIR}/release" \
  "${TEMPLATE_DIR}/releases" \
  "${TEMPLATE_DIR}/.DS_Store"

HEAD_SHA="$(git -C "${TEMPLATE_DIR}" rev-parse HEAD)"
echo "  ✓ Master template staged at ${HEAD_SHA} (tag: baseline, remote removed)."

# 2. Stage condition templates tpl-{a,b,c,d}
echo "[2/2] Creating condition templates (tpl-a, tpl-b, tpl-c, tpl-d)..."

prepare_condition_template() {
  local cond="$1"
  local remove_pa="$2"
  local commit_msg="$3"
  local dst="${EVAL_ROOT}/tpl-${cond}"

  rm -rf "${dst}"
  clone_tree_portable "${TEMPLATE_DIR}" "${dst}"

  # Squash history in condition templates so agents cannot mine git log
  rm -rf "${dst}/.git"
  git -C "${dst}" init -q -b main
  git -C "${dst}" config user.name "${GIT_AUTHOR_NAME}"
  git -C "${dst}" config user.email "${GIT_AUTHOR_EMAIL}"
  git -C "${dst}" add -A
  git -C "${dst}" commit -q -m "baseline" --author="${GIT_AUTHOR_NAME} <${GIT_AUTHOR_EMAIL}>"
  git -C "${dst}" tag -f baseline HEAD

  if [ "${remove_pa}" = "true" ]; then
    rm -rf "${dst}/.agents/skills/principal-architect-workspace"
    mkdir -p "${dst}/.agents/overrides"
    echo "principal-architect: disabled" > "${dst}/.agents/overrides/disabled-skills.txt"
  fi

  git -C "${dst}" add -A
  git -C "${dst}" commit -q --allow-empty -m "${commit_msg}" --author="${GIT_AUTHOR_NAME} <${GIT_AUTHOR_EMAIL}>"
  echo "  ✓ Created ${dst} (${commit_msg})"
}

prepare_condition_template "a" "true"  "condition-a: no memory"
prepare_condition_template "b" "false" "condition-b: PA only"
prepare_condition_template "c" "true"  "condition-c: repo-memory push"
prepare_condition_template "d" "false" "condition-d: both"

echo "======================================================================"
echo "All templates staged cleanly."
echo "======================================================================"
