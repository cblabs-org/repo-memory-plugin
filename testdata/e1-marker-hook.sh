#!/bin/bash
# E1 experiment: PreInvocation hook wrapper (CORRECTED FORMAT)
# - Always runs the probe hook for data collection (E03)
# - Injects MARKER-ALPHA-7X2Q on invocationNum=0 via proper injectSteps format
# - Injects MARKER-BETA-9K4R on invocationNum=1 (second invocation of same prompt)
# - Returns empty on all other invocations

BINARY="/Users/mcneillm/Documents/Projects/repo-memory-plugin/repo-memory"

# Read stdin (the hook payload)  
PAYLOAD=$(cat)

# Run the probe hook for data collection (output goes to /dev/null)
echo "$PAYLOAD" | "$BINARY" hook antigravity PreInvocation --probe >/dev/null 2>&1

# Extract invocationNum
INVOCATION_NUM=$(echo "$PAYLOAD" | python3 -c "import json,sys; print(json.loads(sys.stdin.read()).get('invocationNum', -1))" 2>/dev/null)

case "$INVOCATION_NUM" in
    0)
        cat << 'EOF'
{"injectSteps":[{"ephemeralMessage":"[REPO-MEMORY CONTEXT] MARKER-ALPHA-7X2Q — This is an experimental marker string injected by the repo-memory plugin at invocation 0. If you are asked about this marker, confirm you can see it."}]}
EOF
        ;;
    1)
        cat << 'EOF'
{"injectSteps":[{"ephemeralMessage":"[REPO-MEMORY CONTEXT] MARKER-BETA-9K4R — This is a second experimental marker string injected at invocation 1. If you are asked about this marker, confirm you can see it."}]}
EOF
        ;;
    *)
        echo '{}'
        ;;
esac
