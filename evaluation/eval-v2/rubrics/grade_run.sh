#!/usr/bin/env bash
# ==============================================================================
# grade_run.sh — Blind Architectural & Convention Rubric Grader
# ==============================================================================
# Evaluates an archived (or active) evaluation run directory against the frozen
# Task 1 or Task 2 YAML rubric strictly BLIND to experimental condition.
#
# Usage:
#   grade_run.sh --run-dir <path-to-run-or-archive> [--task {1,2,auto}] [--json <out.json>] [--md <out.md>]
#   grade_run.sh --help
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAMINATION_SCRIPT="${EVAL_ROOT}/scripts/check_contamination.sh"

RUN_DIR=""
TASK_ARG="auto"
JSON_OUT=""
MD_OUT=""

usage() {
  cat <<'EOF'
Usage:
  grade_run.sh --run-dir <path-to-run-or-archive> [options]

Required:
  -d, --run-dir <path>      Path to the run directory or its archive/ subdirectory

Options:
  -t, --task <1|2|auto>     Task number to grade against (default: auto)
  -j, --json <path>         Output JSON scorecard path (default: <archive_dir>/scorecard.json)
  -m, --md <path>           Output Markdown report path (default: <archive_dir>/scorecard.md)
  -h, --help                Show this help message

Notes:
  - Operates strictly BLIND to experimental condition (never reads 'condition' from
    run_metadata.json when computing rubric scores).
  - Inspects git-diff-baseline.patch (or git-diff.patch) and untracked source files
    (excluding .memory/, .agents/, GEMINI.md, CLAUDE.md, and AGENTS.md so memory
    artifacts never leak into source grading).
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -d|--run-dir)
      RUN_DIR="$2"
      shift 2
      ;;
    -t|--task)
      TASK_ARG="$2"
      shift 2
      ;;
    -j|--json)
      JSON_OUT="$2"
      shift 2
      ;;
    -m|--md)
      MD_OUT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Error: Unknown argument '$1'" >&2
      usage
      ;;
  esac
done

if [ -z "${RUN_DIR}" ]; then
  echo "Error: --run-dir <path> is required." >&2
  exit 1
fi

if [ ! -d "${RUN_DIR}" ]; then
  echo "Error: Run directory not found: ${RUN_DIR}" >&2
  exit 1
fi

RUN_DIR="$(cd "${RUN_DIR}" && pwd)"
if [ -d "${RUN_DIR}/archive" ]; then
  ARCHIVE_DIR="${RUN_DIR}/archive"
  PARENT_RUN_DIR="${RUN_DIR}"
else
  ARCHIVE_DIR="${RUN_DIR}"
  PARENT_RUN_DIR="$(cd "${RUN_DIR}/.." && pwd)"
fi

if [ -z "${JSON_OUT}" ]; then
  JSON_OUT="${ARCHIVE_DIR}/scorecard.json"
fi
if [ -z "${MD_OUT}" ]; then
  MD_OUT="${ARCHIVE_DIR}/scorecard.md"
fi

python3 - "${ARCHIVE_DIR}" "${PARENT_RUN_DIR}" "${TASK_ARG}" "${JSON_OUT}" "${MD_OUT}" "${CONTAMINATION_SCRIPT}" <<'PYEOF'
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone

archive_dir, parent_run_dir, task_arg, json_out, md_out, contamination_script = sys.argv[1:7]

# 1. Resolve run_id and task strictly WITHOUT reading 'condition'
run_id = os.path.basename(parent_run_dir) if os.path.basename(archive_dir) == "archive" else os.path.basename(archive_dir)
meta_path = os.path.join(archive_dir, "run_metadata.json")
if not os.path.isfile(meta_path):
    meta_path = os.path.join(parent_run_dir, "run_metadata.json")

task_file_hint = ""
if os.path.isfile(meta_path):
    try:
        with open(meta_path, "r", encoding="utf-8") as f:
            meta = json.load(f)
            # BLINDNESS INVARIANT: Do NOT read meta.get("condition")
            run_id = meta.get("run_id", run_id)
            task_file_hint = meta.get("task_file", "")
    except Exception:
        pass

prompt_text = ""
for p_cand in [os.path.join(archive_dir, "task_prompt.txt"), os.path.join(parent_run_dir, "task_prompt.txt")]:
    if os.path.isfile(p_cand):
        try:
            with open(p_cand, "r", encoding="utf-8") as f:
                prompt_text = f.read()
            break
        except Exception:
            pass

task_num = 1
if task_arg in ("1", "2"):
    task_num = int(task_arg)
else:
    combined_hint = (task_file_hint + " " + prompt_text + " " + run_id).lower()
    if "bitcoin" in combined_hint or "coinmarketcap" in combined_hint or "task-2" in combined_hint or "-t2-" in combined_hint:
        task_num = 2
    else:
        task_num = 1

# 2. Build unified source inspection corpus (excluding .memory/, .agents/, GEMINI.md, CLAUDE.md, AGENTS.md)
EXCLUDED_PREFIXES = (".memory/", ".agents/", ".git/", ".pio/", "managed_components/")
EXCLUDED_FILENAMES = {"GEMINI.md", "CLAUDE.md", "AGENTS.md", "disabled-skills.txt"}

def is_excluded_relpath(rel_path: str) -> bool:
    rel = rel_path.replace("\\", "/")
    while rel.startswith("./"):
        rel = rel[2:]
    rel = rel.lstrip("/")
    if not rel or rel in EXCLUDED_FILENAMES or os.path.basename(rel) in EXCLUDED_FILENAMES:
        return True
    for pref in EXCLUDED_PREFIXES:
        if rel.startswith(pref):
            return True
    return False

corpus_parts = []
inspected_files = []

# 2a. Parse git-diff-baseline.patch (or git-diff.patch)
diff_path = os.path.join(archive_dir, "git-diff-baseline.patch")
if not os.path.isfile(diff_path):
    diff_path = os.path.join(archive_dir, "git-diff.patch")

if os.path.isfile(diff_path):
    try:
        with open(diff_path, "r", encoding="utf-8", errors="replace") as f:
            raw_diff = f.read()
        current_file = None
        keep_section = False
        section_lines = []
        for line in raw_diff.splitlines():
            if line.startswith("diff --git "):
                if keep_section and section_lines:
                    corpus_parts.append("\n".join(section_lines))
                section_lines = []
                m = re.match(r"diff --git a/(\S+) b/(\S+)", line)
                current_file = m.group(2) if m else ""
                keep_section = not is_excluded_relpath(current_file)
                if keep_section and current_file not in inspected_files:
                    inspected_files.append(current_file)
            if keep_section:
                if line.startswith("+") and not line.startswith("+++"):
                    section_lines.append(line[1:])
                elif not line.startswith("-") and not line.startswith("---"):
                    section_lines.append(line)
        if keep_section and section_lines:
            corpus_parts.append("\n".join(section_lines))
    except Exception:
        pass

# 2b. Collect untracked source files under archive/untracked/ (or repo/ if unarchived)
untracked_root = os.path.join(archive_dir, "untracked")
if os.path.isdir(untracked_root):
    for root, _, files in os.walk(untracked_root):
        for fname in sorted(files):
            abs_p = os.path.join(root, fname)
            rel_p = os.path.relpath(abs_p, untracked_root).replace("\\", "/")
            if is_excluded_relpath(rel_p):
                continue
            try:
                with open(abs_p, "r", encoding="utf-8", errors="replace") as f:
                    content = f.read()
                corpus_parts.append(f"// File: {rel_p}\n{content}")
                if rel_p not in inspected_files:
                    inspected_files.append(rel_p)
            except Exception:
                pass
elif os.path.isdir(os.path.join(parent_run_dir, "repo")):
    repo_dir = os.path.join(parent_run_dir, "repo")
    try:
        out = subprocess.check_output(
            ["git", "-C", repo_dir, "ls-files", "--others", "--exclude-standard"],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        for rel_p in out.splitlines():
            rel_p = rel_p.strip()
            if not rel_p or is_excluded_relpath(rel_p):
                continue
            abs_p = os.path.join(repo_dir, rel_p)
            if os.path.isfile(abs_p):
                with open(abs_p, "r", encoding="utf-8", errors="replace") as f:
                    corpus_parts.append(f"// File: {rel_p}\n{f.read()}")
                if rel_p not in inspected_files:
                    inspected_files.append(rel_p)
    except Exception:
        pass

corpus = "\n\n".join(corpus_parts)

def strip_cpp_comments(text: str) -> str:
    text = re.sub(r"/\*.*?\*/", "", text, flags=re.DOTALL)
    text = re.sub(r"//[^\n]*", "", text)
    return text

corpus_no_comments = strip_cpp_comments(corpus)

# 3. Check native test execution status (native_test_result.txt or transcript pio test -e native)
native_test_passed = False
native_test_source = "none"
for nt_cand in [
    os.path.join(archive_dir, "native_test_result.txt"),
    os.path.join(parent_run_dir, "native_test_result.txt"),
]:
    if os.path.isfile(nt_cand):
        try:
            with open(nt_cand, "r", encoding="utf-8", errors="replace") as f:
                nt_text = f.read()
            if re.search(r"\b(PASSED|0 failed|test cases:\s*\d+\s*\|\s*\d+\s*passed)\b", nt_text, re.IGNORECASE) and not re.search(r"\b[1-9]\d*\s+failed\b", nt_text, re.IGNORECASE):
                native_test_passed = True
                native_test_source = "native_test_result.txt"
        except Exception:
            pass
        break

transcript_path = os.path.join(archive_dir, "transcript.jsonl")
if not os.path.isfile(transcript_path):
    transcript_path = os.path.join(parent_run_dir, "transcript.jsonl")

# Parse transcript for token accounting, step counts, revision cycles, and test pass fallback
total_steps = 0
tool_call_count = 0
input_tokens = 0
output_tokens = 0
cache_read_tokens = 0
revision_cycles = 0
saw_test_fail = False
transcript_pio_test_passed = False

if os.path.isfile(transcript_path):
    try:
        with open(transcript_path, "r", encoding="utf-8", errors="replace") as f:
            last_cmd_was_test = False
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    st = json.loads(line)
                except Exception:
                    continue
                total_steps += 1
                usage = st.get("usage") or st.get("token_usage") or {}
                if isinstance(usage, dict):
                    input_tokens += int(usage.get("input_tokens", usage.get("prompt_tokens", 0)) or 0)
                    output_tokens += int(usage.get("output_tokens", usage.get("completion_tokens", 0)) or 0)
                    cache_read_tokens += int(usage.get("cache_read_input_tokens", usage.get("cached_tokens", 0)) or 0)

                tcalls = st.get("tool_calls") or []
                if isinstance(tcalls, list):
                    tool_call_count += len(tcalls)
                    for tc in tcalls:
                        if not isinstance(tc, dict):
                            continue
                        tname = tc.get("name", "")
                        targs = tc.get("args") or {}
                        cmd_str = str(targs.get("CommandLine", targs.get("command", "")))
                        if tname in ("run_command", "bash") and ("pio test" in cmd_str or "pio run" in cmd_str):
                            last_cmd_was_test = True
                        elif tname in ("write_to_file", "replace_file_content", "edit_file", "multi_replace_file_content") and saw_test_fail:
                            revision_cycles += 1
                            saw_test_fail = False

                content_str = str(st.get("content", ""))
                if last_cmd_was_test and st.get("type") in ("GENERIC", "TOOL_RESPONSE"):
                    if "exited with code 0" in content_str or "PASSED" in content_str:
                        transcript_pio_test_passed = True
                    elif "exited with code" in content_str or "FAILED" in content_str:
                        saw_test_fail = True
                    last_cmd_was_test = False
    except Exception:
        pass

if not native_test_passed and transcript_pio_test_passed:
    native_test_passed = True
    native_test_source = "transcript.jsonl"

has_test_files = any(f.startswith("test/") for f in inspected_files)
has_blocking_delay = bool(re.search(r"\bdelay\s*\(", corpus_no_comments))

# 4. Evaluate rubric criteria deterministically
def has_all(patterns):
    return all(bool(re.search(p, corpus, re.MULTILINE)) for p in patterns)

def has_any(patterns):
    return any(bool(re.search(p, corpus, re.MULTILINE)) for p in patterns)

criteria_results = []

if task_num == 1:
    task_title = "Waterproof DS18B20-Compatible Temperature Sensor Module"
    # 1. t1_data_source_and_config (20)
    c1_ds = has_all([r"\bIDataSource\b", r"\bfetchUpdate\b"])
    c1_cfg = has_all([r"\bIConfigurable\b"]) and has_any([r"\bapplyConfig\b", r"\bserializeConfig\b", r"\bgetConfigNamespace\b"])
    c1_bounds = has_any([r"\b900000\b", r"\b900\b", r"15\s*\*\s*60"])
    c1_score = (8 if c1_ds else 0) + (7 if c1_cfg else 0) + (5 if c1_bounds else 0)
    criteria_results.append({
        "id": "t1_data_source_and_config",
        "title": "IDataSource & IConfigurable Update Interval ([1s, 15m] in /mod/<ns>.json)",
        "weight": 20,
        "score": c1_score,
        "passed": c1_score == 20,
        "notes": f"IDataSource={c1_ds}, IConfigurable={c1_cfg}, [1s,15m]_bounds={c1_bounds}",
    })

    # 2. t1_u8g2_carousel_board (20)
    c2_board = has_all([r"\bIDisplayBoard\b", r"\brender\s*\(\s*U8G2"])
    c2_expired = bool(re.search(r"\bisExpired\b", corpus)) and bool(re.search(r"\bfalse\b", corpus))
    c2_advance = bool(re.search(r"\bcanAdvance\b", corpus))
    c2_score = (10 if c2_board else 0) + (5 if c2_expired else 0) + (5 if c2_advance else 0)
    criteria_results.append({
        "id": "t1_u8g2_carousel_board",
        "title": "U8G2Display IDisplayBoard Permanent Carousel Screen",
        "weight": 20,
        "score": c2_score,
        "passed": c2_score == 20,
        "notes": f"IDisplayBoard+render={c2_board}, isExpired_false={c2_expired}, canAdvance={c2_advance}",
    })

    # 3. t1_web_portal_top_level_tab (20)
    c3_web = has_all([r"\bIWebContributor\b", r"\bregisterRoutes\b", r"\bgetApiPrefix\b"])
    c3_tab = has_all([r"\baddTab\b", r"\bnullptr\b"])
    c3_bare_svg = bool(re.search(r"<(path|circle|rect|line|polyline|polygon)\b", corpus_no_comments)) and not bool(re.search(r"<svg[\s>]", corpus_no_comments, re.IGNORECASE))
    c3_score = (8 if c3_web else 0) + (6 if c3_tab else 0) + (6 if c3_bare_svg else 0)
    criteria_results.append({
        "id": "t1_web_portal_top_level_tab",
        "title": "IWebContributor & ITabRegistry Top-Level Portal Nav Icon (Bare SVG Geometry)",
        "weight": 20,
        "score": c3_score,
        "passed": c3_score == 20,
        "notes": f"IWebContributor={c3_web}, top_level_addTab={c3_tab}, bare_svg_geometry={c3_bare_svg}",
    })

    # 4. t1_mcp_contributor_stateless (20)
    c4_mcp = has_all([r"\bIMCPContributor\b", r"\bregisterMCPTools\b", r"\bMCPToolRegistry\b"])
    c4_lambda = bool(re.search(r"\[\s*\]\s*\(", corpus))
    c4_mutates = bool(re.search(r"\bmutatesState\b", corpus))
    c4_score = (8 if c4_mcp else 0) + (6 if c4_lambda else 0) + (6 if c4_mutates else 0)
    criteria_results.append({
        "id": "t1_mcp_contributor_stateless",
        "title": "IMCPContributor On-Device MCP Tool Registration with Stateless Lambda ([])",
        "weight": 20,
        "score": c4_score,
        "passed": c4_score == 20,
        "notes": f"IMCPContributor={c4_mcp}, stateless_lambda={c4_lambda}, mutatesState={c4_mutates}",
    })

    # 5. t1_application_unified_registration (10)
    c5_reg = bool(re.search(r"\bregisterModule\b", corpus))
    c5_no_raw_new = not bool(re.search(r"registerModule\s*\(\s*new\b", corpus))
    c5_score = (10 if (c5_reg and c5_no_raw_new) else (5 if c5_reg else 0))
    criteria_results.append({
        "id": "t1_application_unified_registration",
        "title": "Application::registerModule() Unified Registration & ADR-16 Static Scope",
        "weight": 10,
        "score": c5_score,
        "passed": c5_score == 10,
        "notes": f"registerModule={c5_reg}, static_scope_no_raw_new={c5_no_raw_new}",
    })

    # 6. t1_native_tests_and_non_blocking (10)
    c6_tested = native_test_passed or has_test_files
    c6_non_blocking = not has_blocking_delay and len(corpus.strip()) > 0
    c6_score = (10 if (c6_tested and c6_non_blocking) else (5 if c6_tested else 0))
    criteria_results.append({
        "id": "t1_native_tests_and_non_blocking",
        "title": "Native Unit Tests & Non-Blocking Main Loop (No delay())",
        "weight": 10,
        "score": c6_score,
        "passed": c6_score == 10,
        "notes": f"tested_or_native_pass={c6_tested} ({native_test_source}), no_blocking_delay={c6_non_blocking}",
    })

else:
    task_title = "Bitcoin Price Module via CoinMarketCap API (Hourly + U8G2 + Portal + MCP)"
    # 1. t2_data_source_and_hourly_tier (20)
    c1_ds = has_all([r"\bIDataSource\b", r"\bfetchUpdate\b"])
    c1_tier = has_any([r"PriorityTier::Low", r"\b3600000\b", r"\b3600\b"])
    c1_status = bool(re.search(r"\bUpdateStatus::", corpus))
    c1_score = (8 if c1_ds else 0) + (6 if c1_tier else 0) + (6 if c1_status else 0)
    criteria_results.append({
        "id": "t2_data_source_and_hourly_tier",
        "title": "IDataSource & Hourly Cadence (PriorityTier::Low / 3600s) with UpdateStatus",
        "weight": 20,
        "score": c1_score,
        "passed": c1_score == 20,
        "notes": f"IDataSource={c1_ds}, hourly_tier={c1_tier}, UpdateStatus={c1_status}",
    })

    # 2. t2_tls_and_streaming_json_safety (20)
    c2_ctx = bool(re.search(r"\bFetchContext\b", corpus)) and bool(re.search(r"coinmarketcap\.com", corpus, re.IGNORECASE))
    c2_tls_safe = not bool(re.search(r"\bsetInsecure\s*\(", corpus_no_comments)) and not bool(re.search(r"\bWiFiClientSecure\b", corpus_no_comments))
    c2_json = has_any([r"\bJsonStreamingParser\b", r"\bArduinoJson\b", r"\bdeserializeJson\b"])
    c2_score = (8 if c2_ctx else 0) + (6 if (c2_tls_safe and c2_ctx) else 0) + (6 if c2_json else 0)
    criteria_results.append({
        "id": "t2_tls_and_streaming_json_safety",
        "title": "Shared FetchContext TLS Safety (No setInsecure(), No Member WiFiClientSecure) & Bounded JSON Parsing",
        "weight": 20,
        "score": c2_score,
        "passed": c2_score == 20,
        "notes": f"FetchContext+CMC={c2_ctx}, tls_safe={c2_tls_safe}, bounded_json={c2_json}",
    })

    # 3. t2_u8g2_carousel_board (15)
    c3_board = has_all([r"\bIDisplayBoard\b", r"\brender\s*\(\s*U8G2"])
    c3_expired = bool(re.search(r"\bisExpired\b", corpus)) and bool(re.search(r"\bfalse\b", corpus))
    c3_score = (10 if c3_board else 0) + (5 if c3_expired else 0)
    criteria_results.append({
        "id": "t2_u8g2_carousel_board",
        "title": "U8G2Display IDisplayBoard Permanent Carousel Screen",
        "weight": 15,
        "score": c3_score,
        "passed": c3_score == 15,
        "notes": f"IDisplayBoard+render={c3_board}, isExpired_false={c3_expired}",
    })

    # 4. t2_web_portal_contributor (15)
    c4_web = has_all([r"\bIWebContributor\b", r"\bregisterRoutes\b", r"\bgetApiPrefix\b"])
    c4_score = 15 if c4_web else 0
    criteria_results.append({
        "id": "t2_web_portal_contributor",
        "title": "IWebContributor Portal Route & Status Exposure",
        "weight": 15,
        "score": c4_score,
        "passed": c4_score == 15,
        "notes": f"IWebContributor+routes+prefix={c4_web}",
    })

    # 5. t2_mcp_contributor_stateless (15)
    c5_mcp = has_all([r"\bIMCPContributor\b", r"\bregisterMCPTools\b", r"\bMCPToolRegistry\b"])
    c5_lambda = bool(re.search(r"\[\s*\]\s*\(", corpus)) and bool(re.search(r"\bmutatesState\b", corpus))
    c5_score = (8 if c5_mcp else 0) + (7 if c5_lambda else 0)
    criteria_results.append({
        "id": "t2_mcp_contributor_stateless",
        "title": "IMCPContributor On-Device MCP Tool with Stateless Lambda ([]) & mutatesState = false",
        "weight": 15,
        "score": c5_score,
        "passed": c5_score == 15,
        "notes": f"IMCPContributor={c5_mcp}, stateless_lambda+mutatesState={c5_lambda}",
    })

    # 6. t2_application_and_native_tests (15)
    c6_reg = bool(re.search(r"\bregisterModule\b", corpus)) and not bool(re.search(r"registerModule\s*\(\s*new\b", corpus))
    c6_tested = (native_test_passed or has_test_files) and not has_blocking_delay
    c6_score = (8 if c6_reg else 0) + (7 if c6_tested else 0)
    criteria_results.append({
        "id": "t2_application_and_native_tests",
        "title": "Application::registerModule() Static Scope Wiring, Native Tests & No delay()",
        "weight": 15,
        "score": c6_score,
        "passed": c6_score == 15,
        "notes": f"registerModule_static={c6_reg}, tested_and_no_delay={c6_tested}",
    })

# 5. Run check_contamination.sh on transcript.jsonl if present
contamination_clean = True
contamination_status = "not_checked"
if os.path.isfile(transcript_path) and os.path.isfile(contamination_script):
    res = subprocess.run(
        [contamination_script, "--run-id", run_id, transcript_path],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    contamination_clean = (res.returncode == 0)
    contamination_status = "PASS" if contamination_clean else "CONTAMINATED"

total_score = sum(item["score"] for item in criteria_results)
max_score = sum(item["weight"] for item in criteria_results)
criteria_passed = sum(1 for item in criteria_results if item["passed"])

scorecard = {
    "run_id": run_id,
    "task": task_num,
    "task_title": task_title,
    "blind_grading": True,
    "graded_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "total_score": total_score,
    "max_score": max_score,
    "percentage": round((total_score / max_score) * 100.0, 2) if max_score else 0.0,
    "criteria_passed": criteria_passed,
    "criteria_total": len(criteria_results),
    "native_test_passed": native_test_passed,
    "native_test_source": native_test_source,
    "contamination_status": contamination_status,
    "contamination_clean": contamination_clean,
    "inspected_files": inspected_files,
    "metrics": {
        "transcript_steps": total_steps,
        "tool_calls": tool_call_count,
        "revision_cycles": revision_cycles,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "cache_read_tokens": cache_read_tokens,
        "total_tokens_with_cache": input_tokens + output_tokens + cache_read_tokens,
    },
    "criteria": criteria_results,
}

os.makedirs(os.path.dirname(os.path.abspath(json_out)), exist_ok=True)
with open(json_out, "w", encoding="utf-8") as f:
    json.dump(scorecard, f, indent=2)
    f.write("\n")

md_lines = [
    f"# Blind Rubric Scorecard — `{run_id}`",
    "",
    f"- **Run ID:** `{run_id}`",
    f"- **Task:** Task {task_num} ({task_title})",
    f"- **Blind Score:** **{total_score} / {max_score} ({scorecard['percentage']}%)** ({criteria_passed}/{len(criteria_results)} criteria fully passed)",
    f"- **Native Tests (`pio test -e native`):** `{'PASS' if native_test_passed else 'NOT_RECORDED_OR_FAIL'}` (source: `{native_test_source}`)",
    f"- **Contamination Audit:** `{contamination_status}`",
    f"- **Inspected Source Files:** `{len(inspected_files)}` ({', '.join(f'`{p}`' for p in inspected_files) if inspected_files else 'none'})",
    "",
    "## Rubric Criterion Breakdown",
    "",
    "| Criterion ID | Title | Score | Max | Passed | Evidence / Notes |",
    "| :--- | :--- | ---: | ---: | :---: | :--- |",
]
for c in criteria_results:
    badge = "PASS" if c["passed"] else "FAIL"
    md_lines.append(f"| `{c['id']}` | {c['title']} | {c['score']} | {c['weight']} | {badge} | {c['notes']} |")

md_lines.extend([
    "",
    "## Efficiency & Token Telemetry",
    "",
    "| Metric | Value |",
    "| :--- | ---: |",
    f"| Transcript Steps | {total_steps} |",
    f"| Tool Calls | {tool_call_count} |",
    f"| Build/Test Revision Cycles | {revision_cycles} |",
    f"| Input Tokens | {input_tokens} |",
    f"| Output Tokens | {output_tokens} |",
    f"| Carried Cache-Read Tokens | {cache_read_tokens} |",
    f"| **Total Read-Side Tokens (incl. Cache)** | **{input_tokens + output_tokens + cache_read_tokens}** |",
    "",
])

os.makedirs(os.path.dirname(os.path.abspath(md_out)), exist_ok=True)
with open(md_out, "w", encoding="utf-8") as f:
    f.write("\n".join(md_lines))

print(f"Graded {run_id} (Task {task_num}): {total_score}/{max_score} ({scorecard['percentage']}%) -> {json_out}")
PYEOF
