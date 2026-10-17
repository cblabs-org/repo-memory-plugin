#!/usr/bin/env bash
# repo-memory-plugin (c) 2026 cblabs-org
# https://github.com/cblabs-org/repo-memory-plugin
# SPDX-License-Identifier: Apache-2.0
#
# Module: scripts/install-release.sh
#
# Responsibility:
#   Downloads the pre-compiled `repo-memory` binary for the current OS and CPU
#   architecture from GitHub Releases, installs it into
#   `~/.gemini/config/plugins/repo-memory-daemon/bin/repo-memory` with a symlink
#   at `~/.local/bin/repo-memory`, strips macOS quarantine attributes, and runs
#   `repo-memory install --harness antigravity`.
#
# CLI / Environment Interface:
#   Usage: curl -fsSL https://raw.githubusercontent.com/cblabs-org/repo-memory-plugin/main/scripts/install-release.sh | sh
#   Environment Variables:
#     - REPO_MEMORY_REPO: GitHub owner/repo (default: `cblabs-org/repo-memory-plugin`).
#     - REPO_MEMORY_VERSION: Release tag to install (default: `latest`).
#     - REPO_MEMORY_BIN_DIR: Target CLI symlink directory (default: `$HOME/.local/bin`).
#     - REPO_MEMORY_PLUGIN_DIR: Target daemon plugin directory (default: `$HOME/.gemini/config/plugins/repo-memory-daemon`).
set -euo pipefail

# --- Step 1: Detect host OS and CPU architecture ---
REPO="${REPO_MEMORY_REPO:-cblabs-org/repo-memory-plugin}"
VERSION="${REPO_MEMORY_VERSION:-latest}"
BIN_DIR="${REPO_MEMORY_BIN_DIR:-$HOME/.local/bin}"
PLUGIN_DIR="${REPO_MEMORY_PLUGIN_DIR:-$HOME/.gemini/config/plugins/repo-memory-daemon}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${OS}" in
  darwin|linux) ;;
  *)
    echo "Unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac

case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

ASSET_NAME="repo-memory-${OS}-${ARCH}"
if [[ "${VERSION}" == "latest" ]]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
fi

# --- Step 2: Download release binary and link into BIN_DIR ---
mkdir -p "${BIN_DIR}" "${PLUGIN_DIR}/bin"
TMP_BIN="$(mktemp "${PLUGIN_DIR}/bin/repo-memory.tmp.XXXXXX")"
trap 'rm -f "${TMP_BIN}"' EXIT

echo "Downloading ${ASSET_NAME} (${VERSION}) from ${DOWNLOAD_URL}..."
curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_BIN}"
chmod 0755 "${TMP_BIN}"
mv "${TMP_BIN}" "${PLUGIN_DIR}/bin/repo-memory"
ln -sf "${PLUGIN_DIR}/bin/repo-memory" "${BIN_DIR}/repo-memory"

if [[ "${OS}" == "darwin" ]] && command -v xattr >/dev/null 2>&1; then
  xattr -dr com.apple.quarantine "${PLUGIN_DIR}" 2>/dev/null || true
fi

# --- Step 3: Render daemon and workspace plugin bundles ---
"${BIN_DIR}/repo-memory" install --harness antigravity
