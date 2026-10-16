#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

cd "${REPO_ROOT}"
echo "[run_write_probe] Running v2.2 Write-Side Distillation & Consolidation Micro-Probe..."
go test -v -run TestEvalV2WriteSideMicroProbe ./internal/core/tick/...
echo "[run_write_probe] PASS: v2.2 Write-Side Micro-Probe completed with 0 failures."
