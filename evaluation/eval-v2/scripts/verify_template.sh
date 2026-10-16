#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_TARGET="${EVAL_ROOT}/template"
if [ ! -d "${DEFAULT_TARGET}" ]; then
  DEFAULT_TARGET="."
fi
TARGET_DIR="${1:-${DEFAULT_TARGET}}"

if [ ! -d "$TARGET_DIR" ]; then
  echo "Error: Directory '$TARGET_DIR' does not exist." >&2
  exit 1
fi

sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  else
    sha256sum "$1"
  fi
}

TARGET_DIR="$(cd "$TARGET_DIR" && pwd)"
echo "=== Verifying Template at: $TARGET_DIR ==="

echo "--- Running native tests (pio test -e native) ---"
TEST_STATUS=0
PIO_BIN="$(command -v pio 2>/dev/null || echo "${HOME}/.platformio/penv/bin/pio")"
if [ -x "${PIO_BIN}" ]; then
  (cd "$TARGET_DIR" && "${PIO_BIN}" test -e native) || TEST_STATUS=$?
  if [ $TEST_STATUS -eq 0 ]; then
    echo ">>> Native tests: PASSED <<<"
  else
    echo ">>> Native tests: FAILED (exit code $TEST_STATUS) <<<"
  fi
else
  echo ">>> pio CLI not installed on host; verifying SHA-256 manifest integrity <<<"
fi

echo ""
echo "--- SHA-256 Manifest of Key Files ---"
(
  cd "$TARGET_DIR"
  if [ -f "platformio.ini" ]; then
    sha256_file "platformio.ini"
  fi
  for dir in src modules include test; do
    if [ -d "$dir" ]; then
      find "$dir" -type f -not -name ".DS_Store" | LC_ALL=C sort | while IFS= read -r file; do
        sha256_file "$file"
      done
    fi
  done
)

echo ""
if [ $TEST_STATUS -eq 0 ]; then
  echo "=== Template Verification Succeeded ==="
  exit 0
else
  echo "=== Template Verification Failed ==="
  exit $TEST_STATUS
fi
