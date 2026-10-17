#!/usr/bin/env bash
# repo-memory-plugin (c) 2026 cblabs-org
# https://github.com/cblabs-org/repo-memory-plugin
# SPDX-License-Identifier: Apache-2.0
#
# Module: scripts/install.sh
#
# Responsibility:
#   Builds the `repo-memory` CLI binary from the local checkout with the Git
#   version stamped via `-ldflags`, installs it into `<PLUGIN_DIR>/bin/repo-memory`
#   with a symlink at `<BIN_DIR>/repo-memory`, and runs `repo-memory install`
#   to generate the `repo-memory-daemon` and `workspace-plugin/repo-memory`
#   bundles.
#
# CLI / Environment Interface:
#   Usage: ./scripts/install.sh
#   Environment Variables:
#     - REPO_MEMORY_BIN_DIR: Target CLI symlink directory (default: `$HOME/.local/bin`).
#     - REPO_MEMORY_PLUGIN_DIR: Target daemon plugin directory (default: `$HOME/.gemini/config/plugins/repo-memory-daemon`).
#     - VERSION: Optional explicit semantic version override.
set -euo pipefail

# --- Step 1: Resolve repository paths, target directories, and version string ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

BIN_DIR="${REPO_MEMORY_BIN_DIR:-$HOME/.local/bin}"
PLUGIN_DIR="${REPO_MEMORY_PLUGIN_DIR:-$HOME/.gemini/config/plugins/repo-memory-daemon}"

if [[ -z "${VERSION:-}" ]]; then
  RAW_VERSION="$(git -C "${REPO_ROOT}" describe --tags --always --dirty 2>/dev/null || echo "0.1.0")"
  VERSION="${RAW_VERSION#v}"
fi

# --- Step 2: Compile binary into the plugin bin directory and link into BIN_DIR ---
mkdir -p "${BIN_DIR}" "${PLUGIN_DIR}/bin"

echo "Building repo-memory (${VERSION})..."
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X 'main.Version=${VERSION}'" \
  -o "${PLUGIN_DIR}/bin/repo-memory" \
  "${REPO_ROOT}/cmd/repo-memory"

chmod 0755 "${PLUGIN_DIR}/bin/repo-memory"
ln -sf "${PLUGIN_DIR}/bin/repo-memory" "${BIN_DIR}/repo-memory"

# Strip macOS Gatekeeper quarantine attribute if present
if [[ "$(uname -s)" == "Darwin" ]] && command -v xattr >/dev/null 2>&1; then
  xattr -dr com.apple.quarantine "${PLUGIN_DIR}" 2>/dev/null || true
fi

# --- Step 3: Render daemon and workspace plugin bundles ---
"${BIN_DIR}/repo-memory" install --harness antigravity
