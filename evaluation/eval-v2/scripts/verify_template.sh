#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-.}"

if [ ! -d "$TARGET_DIR" ]; then
  echo "Error: Directory '$TARGET_DIR' does not exist." >&2
  exit 1
fi

TARGET_DIR="$(cd "$TARGET_DIR" && pwd)"
echo "=== Verifying Template at: $TARGET_DIR ==="

echo "--- Running native tests (pio test -e native) ---"
TEST_STATUS=0
(cd "$TARGET_DIR" && pio test -e native) || TEST_STATUS=$?

if [ $TEST_STATUS -eq 0 ]; then
  echo ">>> Native tests: PASSED <<<"
else
  echo ">>> Native tests: FAILED (exit code $TEST_STATUS) <<<"
fi

echo ""
echo "--- SHA-256 Manifest of Key Files ---"
(
  cd "$TARGET_DIR"
  if [ -f "platformio.ini" ]; then
    shasum -a 256 "platformio.ini"
  fi
  for dir in src modules include test; do
    if [ -d "$dir" ]; then
      find "$dir" -type f -not -name ".DS_Store" | LC_ALL=C sort | while IFS= read -r file; do
        shasum -a 256 "$file"
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
