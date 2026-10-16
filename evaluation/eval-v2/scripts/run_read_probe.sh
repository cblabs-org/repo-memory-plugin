#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

cd "${REPO_ROOT}"
echo "[run_read_probe] Running v2.2 Read-Side Multi-Invocation Micro-Probe..."
go test -v -run TestEvalV2ReadSideMicroProbe ./internal/harness/antigravity/...
echo "[run_read_probe] PASS: v2.2 Read-Side Micro-Probe completed with 0 failures."
