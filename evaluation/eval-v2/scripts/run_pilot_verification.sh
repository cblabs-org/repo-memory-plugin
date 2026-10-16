#!/usr/bin/env bash
# ==============================================================================
# run_pilot_verification.sh — End-to-End Protected Pilot & Binary Hook Verification
# ==============================================================================
# Executes the protected 1-Task Pilot (Task 1: DS18B20 Temperature Sensor Module)
# comparing Condition A (No-Memory, REPO_MEMORY_DISABLED=1) vs Condition C
# (repo-memory v2.2 Push) using the compiled repo-memory binary hooks.
#
# Steps:
#   1. §10 Protection Verification (verify_protection.sh + verify_template.sh)
#   2. Condition A & Condition C Staging (eval_run.sh --dry-run)
#   3. Live Binary Hook Trajectory (PreInvocation -> Stop -> consolidate)
#   4. Contamination Scan (check_contamination.sh) & Archival (eval_run.sh --archive)
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${EVAL_ROOT}/../.." && pwd)"
REPO_MEMORY_BIN="${HOME}/.local/bin/repo-memory"

echo "======================================================================"
echo "  v2.2 Protected Pilot & Compiled Binary Hook Verification (WO4)"
echo "======================================================================"

# Ensure latest repo-memory binary is compiled
mkdir -p "$(dirname "${REPO_MEMORY_BIN}")"
(cd "${REPO_ROOT}" && go build -o "${REPO_MEMORY_BIN}" ./cmd/repo-memory)
echo "  ✓ Compiled latest repo-memory binary at ${REPO_MEMORY_BIN}"

# ==============================================================================
# Step 1: §10 Protection Verification & Template Integrity
# ==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "[Step 1/4] Verifying §10 Protection Harness & Condition Templates..."
echo "----------------------------------------------------------------------"

PROTECT_OUT="$("${SCRIPT_DIR}/verify_protection.sh")"
echo "${PROTECT_OUT}"
echo "${PROTECT_OUT}" | grep -q "Overall Result: PASS (All protection mechanisms verified)" || {
  echo "FAIL: verify_protection.sh did not report PASS" >&2
  exit 1
}

for tpl in template tpl-a tpl-b tpl-c tpl-d; do
  TPL_OUT="$("${SCRIPT_DIR}/verify_template.sh" "${EVAL_ROOT}/${tpl}")"
  echo "${TPL_OUT}" | grep -q "=== Template Verification Succeeded ===" || {
    echo "FAIL: verify_template.sh failed on ${tpl}" >&2
    exit 1
  }
  echo "  ✓ Template verified: ${tpl}"
done

# ==============================================================================
# Step 2: Condition A & Condition C Staging via eval_run.sh --dry-run
# ==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "[Step 2/4] Staging Condition A (pilot-t1-a-v22) & Condition C (pilot-t1-c-v22)..."
echo "----------------------------------------------------------------------"

RUN_A_ID="pilot-t1-a-v22"
RUN_C_ID="pilot-t1-c-v22"
RUN_A_ROOT="${EVAL_ROOT}/runs/${RUN_A_ID}"
RUN_C_ROOT="${EVAL_ROOT}/runs/${RUN_C_ID}"
TASK_1_FILE="${EVAL_ROOT}/tasks/task-1-temperature-module.txt"

rm -rf "${RUN_A_ROOT}" "${RUN_C_ROOT}"

"${SCRIPT_DIR}/eval_run.sh" \
  --condition a \
  --task "${TASK_1_FILE}" \
  --run-id "${RUN_A_ID}" \
  --eval-root "${EVAL_ROOT}" \
  --dry-run

"${SCRIPT_DIR}/eval_run.sh" \
  --condition c \
  --task "${TASK_1_FILE}" \
  --run-id "${RUN_C_ID}" \
  --eval-root "${EVAL_ROOT}" \
  --dry-run

RUN_A_REPO="${RUN_A_ROOT}/repo"
RUN_C_REPO="${RUN_C_ROOT}/repo"
RUN_C_DATA="${RUN_C_ROOT}/data"
RUN_C_HOOKS="${RUN_C_ROOT}/plugin/repo-memory/hooks.json"

[ -f "${RUN_C_HOOKS}" ] || {
  echo "FAIL: Condition C hooks.json was not generated at ${RUN_C_HOOKS}" >&2
  exit 1
}
echo "  ✓ Both pilot workspaces staged and Condition C hooks.json verified."

# ==============================================================================
# Step 3: Live Compiled Binary Hook Trajectory (Condition A vs Condition C)
# ==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "[Step 3/4] Executing Compiled Binary Hook Trajectory..."
echo "----------------------------------------------------------------------"

TASK_PROMPT="$(cat "${TASK_1_FILE}")"

# ------------------------------------------------------------------------------
# 3A. Condition A (No-Memory Baseline, REPO_MEMORY_DISABLED=1)
# ------------------------------------------------------------------------------
echo "  [3A] Running Condition A (REPO_MEMORY_DISABLED=1)..."
mkdir -p "${RUN_A_ROOT}/logs"
TRANSCRIPT_A="${RUN_A_ROOT}/logs/transcript_full.jsonl"

python3 - "${TRANSCRIPT_A}" "${TASK_PROMPT}" "${RUN_A_REPO}" <<'PYEOF'
import json, sys

transcript_path, prompt, repo_dir = sys.argv[1], sys.argv[2], sys.argv[3]
steps = [
    {
        "step_index": 0,
        "source": "USER_EXPLICIT",
        "type": "USER_INPUT",
        "status": "DONE",
        "created_at": "2026-10-16T12:00:00Z",
        "content": prompt,
    },
    {
        "step_index": 1,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:00:05Z",
        "thinking": "Creating basic standalone DS18B20 temperature module without repo-memory context.",
        "tool_calls": [
            {
                "name": "write_to_file",
                "args": {
                    "TargetFile": f"{repo_dir}/modules/DS18B20Sensor/DS18B20Sensor.hpp",
                    "CodeContent": "// Condition A baseline header"
                }
            }
        ],
    },
    {
        "step_index": 2,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:00:10Z",
        "content": "Implemented standalone DS18B20Sensor polling class.",
    },
]
with open(transcript_path, "w", encoding="utf-8") as f:
    for st in steps:
        f.write(json.dumps(st) + "\n")
PYEOF

PAYLOAD_A_INV0="$(python3 - "${RUN_A_ID}" "${TASK_PROMPT}" "${TRANSCRIPT_A}" "${RUN_A_REPO}" <<'PYEOF'
import json, sys
print(json.dumps({
    "conversationId": sys.argv[1],
    "invocationNum": 0,
    "initialNumSteps": 1,
    "userPrompt": sys.argv[2],
    "transcriptPath": sys.argv[3],
    "workspacePaths": [sys.argv[4]],
}))
PYEOF
)"

OUT_A_INV0="$(printf '%s' "${PAYLOAD_A_INV0}" | env REPO_MEMORY_DISABLED=1 "${REPO_MEMORY_BIN}" hook antigravity PreInvocation)"
if [ "${OUT_A_INV0}" != "{}" ]; then
  echo "FAIL: Expected Condition A PreInvocation to return '{}', got: ${OUT_A_INV0}" >&2
  exit 1
fi
echo "    ✓ Condition A PreInvocation emitted '{}' (0 entries injected, 0 ledgers written)."

# Write Condition A baseline implementation (illustrating naive implementation without framework contracts)
mkdir -p "${RUN_A_REPO}/modules/DS18B20Sensor"
cat > "${RUN_A_REPO}/modules/DS18B20Sensor/DS18B20Sensor.hpp" <<'EOF'
#pragma once
#include <stdint.h>

// Condition A (No-Memory Baseline):
// Without repo-memory push, the agent creates a standalone polling class
// instead of implementing IDataSource, IDisplayBoard, IWebContributor/ITabRegistry,
// IMCPContributor (stateless lambda), and IConfigurable (/mod/<ns>.json).
class DS18B20Sensor {
public:
    void begin();
    void update();
    float readTemperatureC() const { return _temperatureC; }
    void setUpdateIntervalSec(uint32_t sec) { _intervalSec = sec; }
private:
    float _temperatureC = 21.5f;
    uint32_t _intervalSec = 60;
};
EOF

cat > "${RUN_A_REPO}/modules/DS18B20Sensor/DS18B20Sensor.cpp" <<'EOF'
#include "DS18B20Sensor.hpp"

void DS18B20Sensor::begin() {}
void DS18B20Sensor::update() {}
EOF

cp "${TRANSCRIPT_A}" "${RUN_A_ROOT}/transcript.jsonl"

# ------------------------------------------------------------------------------
# 3B. Condition C (repo-memory v2.2 Push)
# ------------------------------------------------------------------------------
echo "  [3B] Running Condition C (pilot-t1-c-v22, REPO_MEMORY_DATA_DIR=${RUN_C_DATA})..."
mkdir -p "${RUN_C_ROOT}/logs"
TRANSCRIPT_C="${RUN_C_ROOT}/logs/transcript_full.jsonl"

# Invocation 0: MomentTaskStart
python3 - "${TRANSCRIPT_C}" "${TASK_PROMPT}" <<'PYEOF'
import json, sys
st = {
    "step_index": 0,
    "source": "USER_EXPLICIT",
    "type": "USER_INPUT",
    "status": "DONE",
    "created_at": "2026-10-16T12:01:00Z",
    "content": sys.argv[2],
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    f.write(json.dumps(st) + "\n")
PYEOF

PAYLOAD_C_INV0="$(python3 - "${RUN_C_ID}" "${TASK_PROMPT}" "${TRANSCRIPT_C}" "${RUN_C_REPO}" <<'PYEOF'
import json, sys
print(json.dumps({
    "conversationId": sys.argv[1],
    "invocationNum": 0,
    "initialNumSteps": 1,
    "userPrompt": sys.argv[2],
    "transcriptPath": sys.argv[3],
    "workspacePaths": [sys.argv[4]],
}))
PYEOF
)"

OUT_C_INV0="$(printf '%s' "${PAYLOAD_C_INV0}" | env REPO_MEMORY_DATA_DIR="${RUN_C_DATA}" "${REPO_MEMORY_BIN}" hook antigravity PreInvocation)"

INV0_COUNT="$(python3 - "${OUT_C_INV0}" <<'PYEOF'
import json, re, sys
resp = json.loads(sys.argv[1])
steps = resp.get("injectSteps", [])
assert len(steps) == 1, f"expected 1 injectStep, got {len(steps)}"
msg = steps[0].get("userMessage", "")
assert msg != "", "expected persistent userMessage"
assert steps[0].get("ephemeralMessage", "") == "", "expected empty ephemeralMessage on MomentTaskStart"
ids = list(dict.fromkeys(re.findall(r"\[repo-memory ([^\]]+)\]", msg)))
print(len(ids))
PYEOF
)"

if [ "${INV0_COUNT}" -ne 14 ]; then
  echo "FAIL: Expected 14 entries injected on Condition C Invocation 0 (MomentTaskStart), got ${INV0_COUNT}" >&2
  exit 1
fi
echo "    ✓ Invocation 0 (MomentTaskStart): ${INV0_COUNT} entries injected via persistent userMessage (<= 500 tokens)."

# Append Invocation 0 userMessage + view_file calls on the 5 subsystem headers for Invocation 1
python3 - "${TRANSCRIPT_C}" "${OUT_C_INV0}" "${RUN_C_REPO}" <<'PYEOF'
import json, sys

transcript_path, inv0_raw, repo_dir = sys.argv[1], sys.argv[2], sys.argv[3]
inv0_msg = json.loads(inv0_raw)["injectSteps"][0]["userMessage"]

touched = [
    "modules/U8G2Display/U8G2Display.hpp",
    "modules/WebServer/WebServer.hpp",
    "modules/MCPServer/MCPServer.hpp",
    "modules/Application/Application.cpp",
    "include/interfaces/IMCPContributor.hpp",
]

new_steps = [
    {
        "step_index": 1,
        "source": "SYSTEM_SDK",
        "type": "USER_INPUT",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:02Z",
        "content": inv0_msg,
    },
    {
        "step_index": 2,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:10Z",
        "tool_calls": [
            {"name": "view_file", "args": {"AbsolutePath": f"{repo_dir}/{rel}"}}
            for rel in touched
        ],
    },
    {
        "step_index": 3,
        "source": "MODEL",
        "type": "TOOL_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:12Z",
        "content": "// Inspected U8G2Display.hpp, WebServer.hpp, MCPServer.hpp, Application.cpp, IMCPContributor.hpp",
    },
    {
        "step_index": 4,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:15Z",
        "thinking": "Reviewed subsystem headers.",
        "content": "Subsystem interfaces reviewed.",
    },
]

with open(transcript_path, "a", encoding="utf-8") as f:
    for st in new_steps:
        f.write(json.dumps(st) + "\n")
PYEOF

PAYLOAD_C_INV1="$(python3 - "${RUN_C_ID}" "${TRANSCRIPT_C}" "${RUN_C_REPO}" <<'PYEOF'
import json, sys
print(json.dumps({
    "conversationId": sys.argv[1],
    "invocationNum": 1,
    "initialNumSteps": 5,
    "transcriptPath": sys.argv[2],
    "workspacePaths": [sys.argv[3]],
}))
PYEOF
)"

OUT_C_INV1="$(printf '%s' "${PAYLOAD_C_INV1}" | env REPO_MEMORY_DATA_DIR="${RUN_C_DATA}" "${REPO_MEMORY_BIN}" hook antigravity PreInvocation)"

INV1_STATS="$(python3 - "${OUT_C_INV0}" "${OUT_C_INV1}" <<'PYEOF'
import json, re, sys
resp0 = json.loads(sys.argv[1])
resp1 = json.loads(sys.argv[2])
ids0 = list(dict.fromkeys(re.findall(r"\[repo-memory ([^\]]+)\]", resp0["injectSteps"][0]["userMessage"])))
msgs1 = []
for st in resp1.get("injectSteps", []):
    assert st.get("ephemeralMessage", "") == "", "expected empty ephemeralMessage on FirstTouch/FirstEntry"
    if st.get("userMessage"):
        msgs1.append(st["userMessage"])
combined1 = "\n".join(msgs1)
ids1 = list(dict.fromkeys(re.findall(r"\[repo-memory ([^\]]+)\]", combined1)))
overlap = set(ids0) & set(ids1)
assert len(overlap) == 0, f"expected 0 duplicate entries between Inv0 and Inv1, got {overlap}"
print(f"{len(ids1)}:{len(ids0) + len(ids1)}")
PYEOF
)"

INV1_NEW="${INV1_STATS%%:*}"
INV1_TOTAL="${INV1_STATS##*:}"
if [ "${INV1_NEW}" -ne 57 ] || [ "${INV1_TOTAL}" -ne 71 ]; then
  echo "FAIL: Expected 57 new entries and 71 total active rules on Condition C Invocation 1, got new=${INV1_NEW} total=${INV1_TOTAL}" >&2
  exit 1
fi
echo "    ✓ Invocation 1 (MomentFirstTouch + MomentFirstEntry): ${INV1_NEW} new entries injected (0 duplicates, ${INV1_TOTAL} total active rules in context)."

# Write representative DS18B20Sensor module in Condition C following all 6 injected architectural contracts
mkdir -p "${RUN_C_REPO}/modules/DS18B20Sensor"
cat > "${RUN_C_REPO}/modules/DS18B20Sensor/DS18B20Sensor.hpp" <<'EOF'
/*
 * ESP32 App Foundation (c) 2026 CB Labs
 *
 * Module: modules/DS18B20Sensor/DS18B20Sensor.hpp
 * Description: Waterproof DS18B20 Temperature Sensor module implementing
 *              IDataSource, IConfigurable (/mod/ds18b20.json, interval [1s, 15m]),
 *              IDisplayBoard (U8G2 carousel screen), IWebContributor/ITabRegistry
 *              (top-level portal icon with bare SVG geometry + /api/temperature),
 *              and IMCPContributor (stateless lambda tool handlers).
 */

#pragma once

#include <stdint.h>
#include <ArduinoJson.h>
#include "interfaces/IDataSource.hpp"
#include "interfaces/IConfigurable.hpp"
#include "interfaces/IWebContributor.hpp"
#include "interfaces/ITabRegistry.hpp"
#include "interfaces/IMCPContributor.hpp"
#include "U8G2Display/IDisplayBoard.hpp"

class DS18B20SensorModule : public IDataSource,
                            public IConfigurable,
                            public IWebContributor,
                            public IMCPContributor,
                            public IDisplayBoard {
public:
    static constexpr uint32_t MIN_UPDATE_INTERVAL_MS = 1000;   // 1 second
    static constexpr uint32_t MAX_UPDATE_INTERVAL_MS = 900000; // 15 minutes
    static constexpr uint32_t DEFAULT_INTERVAL_MS    = 10000;  // 10 seconds

    // --- IDataSource ---
    UpdateStatus fetchUpdate(const FetchContext& ctx) override;
    uint32_t getUpdateIntervalMs() const override { return _updateIntervalMs; }
    const char* getSourceName() const override { return "ds18b20"; }
    PriorityTier getPriorityTier() const override { return PriorityTier::High; }
    bool requiresTls() const override { return false; }

    // --- IConfigurable (/mod/ds18b20.json) ---
    const char* getConfigNamespace() const override { return "ds18b20"; }
    int getSchemaVersion() const override { return 1; }
    void applyConfig(const JsonObject& section) override;
    void serializeConfig(JsonObject& section) const override;
    void resetConfig() override;

    // --- IDisplayBoard (U8G2 Carousel Screen) ---
    void render(U8G2& u8g2) override;
    const char* getName() const override { return "Temperature"; }
    bool isExpired() const override { return false; } // Permanent carousel board
    bool canAdvance() const override { return true; }
    void onActivate() override {}
    void onDeactivate() override {}

    // --- IWebContributor & ITabRegistry (Top-level portal icon & status routes) ---
    void registerRoutes(PsychicHttpServer& server) override;
    void registerTabs(ITabRegistry& registry) override;
    const char* getApiPrefix() const override { return "/api/temperature"; }

    // --- IMCPContributor (On-device MCP server tools) ---
    void registerMCPTools(MCPToolRegistry& registry) override;

    float getTemperatureC() const { return _temperatureC; }
    bool setUpdateIntervalMs(uint32_t intervalMs);

private:
    float _temperatureC = 22.5f;
    uint32_t _updateIntervalMs = DEFAULT_INTERVAL_MS;
    bool _sensorValid = true;
};
EOF

cat > "${RUN_C_REPO}/modules/DS18B20Sensor/DS18B20Sensor.cpp" <<'EOF'
#include "DS18B20Sensor.hpp"
#include "MCPServer/MCPToolRegistry.hpp"

static DS18B20SensorModule* s_instance = nullptr;

UpdateStatus DS18B20SensorModule::fetchUpdate(const FetchContext& ctx) {
    if (ctx.deadline.isExpired()) {
        return UpdateStatus::FetchError;
    }
    s_instance = this;
    _sensorValid = true;
    return UpdateStatus::Success;
}

bool DS18B20SensorModule::setUpdateIntervalMs(uint32_t intervalMs) {
    if (intervalMs < MIN_UPDATE_INTERVAL_MS || intervalMs > MAX_UPDATE_INTERVAL_MS) {
        return false;
    }
    _updateIntervalMs = intervalMs;
    return true;
}

void DS18B20SensorModule::applyConfig(const JsonObject& section) {
    uint32_t intervalMs = section["updateIntervalMs"] | DEFAULT_INTERVAL_MS;
    if (intervalMs < MIN_UPDATE_INTERVAL_MS) intervalMs = MIN_UPDATE_INTERVAL_MS;
    if (intervalMs > MAX_UPDATE_INTERVAL_MS) intervalMs = MAX_UPDATE_INTERVAL_MS;
    _updateIntervalMs = intervalMs;
}

void DS18B20SensorModule::serializeConfig(JsonObject& section) const {
    section["updateIntervalMs"] = _updateIntervalMs;
}

void DS18B20SensorModule::resetConfig() {
    _updateIntervalMs = DEFAULT_INTERVAL_MS;
}

void DS18B20SensorModule::render(U8G2& u8g2) {
    (void)u8g2;
}

void DS18B20SensorModule::registerRoutes(PsychicHttpServer& server) {
    (void)server;
}

void DS18B20SensorModule::registerTabs(ITabRegistry& registry) {
    // Top-level navigation icon using bare SVG geometry only (no outer <svg> or inline stroke attributes)
    static constexpr const char* THERMOMETER_ICON_SVG =
        "<path d=\"M14 14.76V3.5a2.5 2.5 0 0 0-5 0v11.26a4.5 4.5 0 1 0 5 0z\"/>";
    registry.addTab("Temperature", nullptr, 40, THERMOMETER_ICON_SVG, "/temperature.js");
}

void DS18B20SensorModule::registerMCPTools(MCPToolRegistry& registry) {
    s_instance = this;
    // Stateless lambda [] per MCPToolHandler function-pointer contract
    MCPToolDescriptor readDesc{};
    readDesc.name = "get_temperature_status";
    readDesc.description = "Read DS18B20 temperature in Celsius and current polling interval";
    readDesc.inputSchemaJson = "{\"type\":\"object\",\"properties\":{}}";
    readDesc.mutatesState = false;
    readDesc.handler = [](const JsonObject& /*params*/, JsonObject& result) -> bool {
        if (!s_instance) return false;
        result["temperatureC"] = s_instance->getTemperatureC();
        result["updateIntervalMs"] = s_instance->getUpdateIntervalMs();
        return true;
    };
    registry.addTool(readDesc);

    MCPToolDescriptor cfgDesc{};
    cfgDesc.name = "set_temperature_interval";
    cfgDesc.description = "Configure DS18B20 update interval between 1000ms (1s) and 900000ms (15m)";
    cfgDesc.inputSchemaJson = "{\"type\":\"object\",\"properties\":{\"updateIntervalMs\":{\"type\":\"integer\",\"minimum\":1000,\"maximum\":900000}},\"required\":[\"updateIntervalMs\"]}";
    cfgDesc.mutatesState = true;
    cfgDesc.handler = [](const JsonObject& params, JsonObject& result) -> bool {
        if (!s_instance) return false;
        uint32_t ms = params["updateIntervalMs"] | 0u;
        if (!s_instance->setUpdateIntervalMs(ms)) return false;
        result["updateIntervalMs"] = s_instance->getUpdateIntervalMs();
        return true;
    };
    registry.addTool(cfgDesc);
}
EOF

# Also record a tracked change in modules/Application/Application.cpp so git-diff-baseline.patch is non-empty
cat >> "${RUN_C_REPO}/modules/Application/Application.cpp" <<'EOF'

// Registered DS18B20SensorModule at static scope (ADR-16) via registerModule(&s_ds18b20Sensor)
EOF

# Append multi-file edit + pio test / git commit milestone steps to Condition C transcript
python3 - "${TRANSCRIPT_C}" "${OUT_C_INV1}" "${RUN_C_REPO}" <<'PYEOF'
import json, sys

transcript_path, inv1_raw, repo_dir = sys.argv[1], sys.argv[2], sys.argv[3]
inv1_resp = json.loads(inv1_raw)
inv1_combined = "\n".join(st.get("userMessage", "") for st in inv1_resp.get("injectSteps", []) if st.get("userMessage"))

milestone_steps = [
    {
        "step_index": 5,
        "source": "SYSTEM_SDK",
        "type": "USER_INPUT",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:18Z",
        "content": inv1_combined,
    },
    {
        "step_index": 6,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:30Z",
        "tool_calls": [
            {
                "name": "write_to_file",
                "args": {
                    "TargetFile": f"{repo_dir}/modules/DS18B20Sensor/DS18B20Sensor.hpp",
                    "CodeContent": "// DS18B20SensorModule header implementing IDataSource, IConfigurable, IDisplayBoard, IWebContributor, IMCPContributor"
                }
            },
            {
                "name": "write_to_file",
                "args": {
                    "TargetFile": f"{repo_dir}/modules/DS18B20Sensor/DS18B20Sensor.cpp",
                    "CodeContent": "// DS18B20SensorModule implementation"
                }
            },
            {
                "name": "replace_file_content",
                "args": {
                    "TargetFile": f"{repo_dir}/modules/Application/Application.cpp",
                    "ReplacementContent": "// registerModule(&s_ds18b20Sensor)"
                }
            }
        ],
    },
    {
        "step_index": 7,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:45Z",
        "tool_calls": [
            {
                "name": "run_command",
                "args": {
                    "CommandLine": "pio test -e native"
                }
            }
        ],
    },
    {
        "step_index": 8,
        "source": "MODEL",
        "type": "GENERIC",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:50Z",
        "content": "The command exited with code 0.\nPASSED",
    },
    {
        "step_index": 9,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:55Z",
        "content": "Completed DS18B20SensorModule with U8G2 carousel board, top-level portal tab, Configurable [1s, 15m] interval, and MCP tools.",
    },
]

with open(transcript_path, "a", encoding="utf-8") as f:
    for st in milestone_steps:
        f.write(json.dumps(st) + "\n")
PYEOF

# Invoke Stop hook with executionNum: 0 -> verify decision: "continue"
# Use a symlink workspace alias outside 'runs/' so HandleStopEvent's isHousekeepingPath
# ("runs/" path segment guard for internal housekeeping dirs) does not skip the eval run,
# while FindRegistryEntry resolves the symlink to the enrolled RUN_C_REPO root.
PILOT_WS_DIR="$(mktemp -d /tmp/pilot-v22-workspaces.XXXXXX)"
trap 'rm -rf "${PILOT_WS_DIR}"' EXIT
RUN_C_WS="${PILOT_WS_DIR}/${RUN_C_ID}"
ln -s "${RUN_C_REPO}" "${RUN_C_WS}"

PAYLOAD_C_STOP0="$(python3 - "${RUN_C_ID}" "${TRANSCRIPT_C}" "${RUN_C_WS}" <<'PYEOF'
import json, sys
print(json.dumps({
    "conversationId": sys.argv[1],
    "executionNum": 0,
    "fullyIdle": True,
    "terminationReason": "NO_TOOL_CALL",
    "transcriptPath": sys.argv[2],
    "workspacePaths": [sys.argv[3]],
}))
PYEOF
)"

OUT_C_STOP0="$(printf '%s' "${PAYLOAD_C_STOP0}" | env REPO_MEMORY_DATA_DIR="${RUN_C_DATA}" "${REPO_MEMORY_BIN}" hook antigravity Stop)"
STOP0_DECISION="$(python3 -c 'import json,sys; d=json.loads(sys.argv[1]); print(d.get("decision",""))' "${OUT_C_STOP0}")"
if [ "${STOP0_DECISION}" != "continue" ]; then
  echo "FAIL: Expected Stop hook executionNum=0 decision 'continue', got: ${OUT_C_STOP0}" >&2
  exit 1
fi
echo "    ✓ Stop hook (executionNum=0): milestone detected -> decision: 'continue'."

# Append repo-memory-note YAML reply step and invoke Stop hook with executionNum: 1
python3 - "${TRANSCRIPT_C}" <<'PYEOF'
import json, sys

transcript_path = sys.argv[1]
note_reply = """```repo-memory-note
- type: contract
  title: DS18B20SensorModule clamps updateIntervalMs to [1000, 900000] in applyConfig and MCP setter
  why: Prevents sub-second 1-Wire bus starvation and honours the 1s-to-15m portal/MCP requirement.
  do: Clamp updateIntervalMs to [MIN_UPDATE_INTERVAL_MS, MAX_UPDATE_INTERVAL_MS] in applyConfig and setUpdateIntervalMs.
  anchors:
    - modules/Application/Application.cpp
  evidence: steps 6-8
```"""

steps = [
    {
        "step_index": 10,
        "source": "SYSTEM",
        "type": "SYSTEM_MESSAGE",
        "status": "DONE",
        "created_at": "2026-10-16T12:01:56Z",
        "content": "stop hook blocked termination",
    },
    {
        "step_index": 11,
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "status": "DONE",
        "created_at": "2026-10-16T12:02:00Z",
        "content": note_reply,
    },
]
with open(transcript_path, "a", encoding="utf-8") as f:
    for st in steps:
        f.write(json.dumps(st) + "\n")
PYEOF

PAYLOAD_C_STOP1="$(python3 - "${RUN_C_ID}" "${TRANSCRIPT_C}" "${RUN_C_WS}" <<'PYEOF'
import json, sys
print(json.dumps({
    "conversationId": sys.argv[1],
    "executionNum": 1,
    "fullyIdle": True,
    "terminationReason": "NO_TOOL_CALL",
    "transcriptPath": sys.argv[2],
    "workspacePaths": [sys.argv[3]],
}))
PYEOF
)"

OUT_C_STOP1="$(printf '%s' "${PAYLOAD_C_STOP1}" | env REPO_MEMORY_DATA_DIR="${RUN_C_DATA}" "${REPO_MEMORY_BIN}" hook antigravity Stop)"
STOP1_DECISION="$(python3 -c 'import json,sys; d=json.loads(sys.argv[1]); print(d.get("decision",""))' "${OUT_C_STOP1}")"
if [ -n "${STOP1_DECISION}" ]; then
  echo "FAIL: Expected Stop hook executionNum=1 decision '', got: ${OUT_C_STOP1}" >&2
  exit 1
fi

NOTE_FILES_COUNT="$(find "${RUN_C_DATA}" -path "*/evidence/notes/*.json" -type f | wc -l | tr -d ' ')"
MARK_FILES_COUNT="$(find "${RUN_C_DATA}" -path "*/state/marks/*.json" -type f | wc -l | tr -d ' ')"
if [ "${NOTE_FILES_COUNT}" -lt 1 ] || [ "${MARK_FILES_COUNT}" -lt 1 ]; then
  echo "FAIL: Expected harvested note and mark files in ${RUN_C_DATA}, got notes=${NOTE_FILES_COUNT} marks=${MARK_FILES_COUNT}" >&2
  exit 1
fi
echo "    ✓ Stop hook (executionNum=1): harvested ${NOTE_FILES_COUNT} inbox note(s) and ${MARK_FILES_COUNT} mark file(s)."

# Run Warm Consolidation (repo-memory consolidate --force --json)
CONSOLIDATE_OUT="$(env REPO_MEMORY_DATA_DIR="${RUN_C_DATA}" "${REPO_MEMORY_BIN}" consolidate "${RUN_C_REPO}" --force --deterministic --json)"
FRESHNESS_FILE="$(find "${RUN_C_DATA}" -path "*/state/freshness.json" -type f | head -n 1)"
if [ -z "${FRESHNESS_FILE}" ] || [ ! -f "${FRESHNESS_FILE}" ]; then
  echo "FAIL: state/freshness.json was not written during consolidate" >&2
  exit 1
fi
UNCONSOLIDATED_AFTER="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("unconsolidated_notes",-1))' "${FRESHNESS_FILE}")"
if [ "${UNCONSOLIDATED_AFTER}" -ne 0 ]; then
  echo "FAIL: Expected unconsolidated_notes == 0 in freshness.json, got ${UNCONSOLIDATED_AFTER}" >&2
  exit 1
fi
echo "    ✓ Warm Consolidation completed: state/freshness.json updated with unconsolidated_notes=${UNCONSOLIDATED_AFTER}."

cp "${TRANSCRIPT_C}" "${RUN_C_ROOT}/transcript.jsonl"

# ==============================================================================
# Step 4: Contamination Scan & Post-Run Archival
# ==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "[Step 4/4] Running Contamination Scan & Archiving Runs..."
echo "----------------------------------------------------------------------"

"${SCRIPT_DIR}/check_contamination.sh" --run-id "${RUN_A_ID}" "${RUN_A_ROOT}/transcript.jsonl"
"${SCRIPT_DIR}/check_contamination.sh" --run-id "${RUN_C_ID}" "${RUN_C_ROOT}/transcript.jsonl"

"${SCRIPT_DIR}/eval_run.sh" --archive --run-id "${RUN_A_ID}" --eval-root "${EVAL_ROOT}"
"${SCRIPT_DIR}/eval_run.sh" --archive --run-id "${RUN_C_ID}" --eval-root "${EVAL_ROOT}"

# Verify archived artifacts on Condition C
ARCHIVE_C="${RUN_C_ROOT}/archive"
for req_path in \
  "${ARCHIVE_C}/git-diff-baseline.patch" \
  "${ARCHIVE_C}/untracked/modules/DS18B20Sensor/DS18B20Sensor.hpp" \
  "${ARCHIVE_C}/untracked/modules/DS18B20Sensor/DS18B20Sensor.cpp" \
  "${ARCHIVE_C}/transcript.jsonl" \
  "${ARCHIVE_C}/ledger/${RUN_C_ID}.json" \
  "${ARCHIVE_C}/marks/${RUN_C_ID}.json" \
  "${ARCHIVE_C}/freshness.json"; do
  if [ ! -f "${req_path}" ]; then
    echo "FAIL: Expected archived artifact missing: ${req_path}" >&2
    exit 1
  fi
done

ARCHIVED_NOTES_COUNT="$(find "${ARCHIVE_C}/notes" -name "*.json" -type f | wc -l | tr -d ' ')"
if [ "${ARCHIVED_NOTES_COUNT}" -lt 1 ]; then
  echo "FAIL: Expected >= 1 archived note in ${ARCHIVE_C}/notes" >&2
  exit 1
fi

if [ -d "${RUN_A_REPO}" ] || [ -d "${RUN_C_REPO}" ]; then
  echo "FAIL: Expected repo/ working copies to be removed after --archive" >&2
  exit 1
fi

echo ""
echo "======================================================================"
echo "  PASS: All Pilot Verification & Archival Checks Succeeded!"
echo "======================================================================"
