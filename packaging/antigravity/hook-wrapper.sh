#!/bin/sh
# Fail-open wrapper for repo-memory hooks.
# If the binary doesn't exist or crashes, PreToolUse returns allow.
# Other hook types (PreInvocation, PostToolUse, Stop) return {} on failure.

BINARY="/Users/mcneillm/Documents/Projects/repo-memory-plugin/repo-memory"

# Detect PreToolUse from any argument position
IS_PRE_TOOL_USE=0
for arg in "$@"; do
  case "$arg" in
    PreToolUse|pre_tool_use) IS_PRE_TOOL_USE=1 ;;
  esac
done

if [ ! -x "$BINARY" ]; then
  if [ $IS_PRE_TOOL_USE -eq 1 ]; then
    echo '{"decision":"allow","reason":"fail-open: binary not found"}'
  else
    echo '{}'
  fi
  exit 0
fi

# Run the binary, capture output and exit code
OUTPUT=$("$BINARY" "$@" 2>/dev/null)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
  if [ $IS_PRE_TOOL_USE -eq 1 ]; then
    echo '{"decision":"allow","reason":"fail-open: binary exit '$EXIT_CODE'"}'
  else
    echo '{}'
  fi
  exit 0
fi

echo "$OUTPUT"
exit 0
