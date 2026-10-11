#!/usr/bin/env python3
"""Second-opinion validation of docs/evaluations/pa-memory-retrieval-effectiveness.md.

Re-derives the report's populations using the authoritative workspace attribution
in Antigravity's conversation_summaries.db (not "transcript mentions esp32"),
counts PA-workspace access across all read-capable tools, and checks sample dates.

Usage: python3 evaluation/scripts/validate_pa_retrieval_report.py
"""
import collections
import json
import os
import re
import sqlite3
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TDIR = ROOT / "evaluation/esp32-transcripts"
ANALYSIS = ROOT / "evaluation/esp32-transcript-analysis.jsonl"
DBS = ["~/.gemini/antigravity/conversation_summaries.db", "~/.gemini/antigravity-cli/conversation_summaries.db"]
ESP32 = os.path.expanduser("~/Documents/Projects/esp32-app-foundation")
READ_TOOLS = {"view_file", "grep_search", "list_dir", "find_by_name", "view_file_outline", "view_code_item"}
SHELL_READ = re.compile(r"\b(cat|head|tail|sed|grep|rg|less|awk|ls)\b")
PA_PATH = re.compile(r"(/[^\s\"'`,;)]*principal-architect[^\s\"'`,;)]*)")


def repo_of(path):
    m = re.search(r"/Projects/([^/]+)/", path)
    if m:
        return m.group(1)
    return "GLOBAL_SKILL" if ".gemini/config/skills" in path else "other"


def category(path):
    if "SKILL.md" in path or "/references/" in path or "/assets/" in path:
        return "process"
    if path.endswith("MEMORY.md") and "MODULE" not in path:
        return "MEMORY"
    if "MODULE_INDEX" in path or "PROJECT_INDEX" in path:
        return "INDEX"
    if path.endswith("PROJECT.md"):
        return "PROJECT"
    if "WORKORDER" in path:
        return "WORKORDER"
    if "PROGRAM" in path:
        return "PROGRAM"
    return "other"


def load_db():
    meta = {}
    for p in DBS:
        con = sqlite3.connect(os.path.expanduser(p))
        q = "select conversation_id, workspace_uris, parent_conversation_id, last_modified_time from conversation_summaries"
        for cid, ws, parent, lm in con.execute(q):
            try:
                uris = json.loads(ws)
            except Exception:
                uris = [ws]
            repos = sorted({repo_of(u.replace("file://", "") + "/") for u in uris}) if uris else []
            meta[cid] = {"ws": "+".join(repos) or "none", "subagent": bool(parent), "month": lm[:7]}
    return meta


def main():
    meta = load_db()
    rtype = {json.loads(l)["conv_id"]: json.loads(l)["trajectory_type"] for l in open(ANALYSIS)}
    rows = []
    for d in sorted(p for p in TDIR.iterdir() if p.is_dir()):
        m = meta.get(d.name, {"ws": "MISSING", "subagent": False, "month": "?"})
        r = {"cid": d.name, **m, "rtype": rtype.get(d.name), "access": collections.Counter(), "reads": collections.Counter()}
        f = d / "transcript_full.jsonl"
        if not f.exists():
            f = d / "transcript.jsonl"
        for line in open(f, errors="ignore"):
            try:
                step = json.loads(line)
            except Exception:
                continue
            for tc in step.get("tool_calls") or []:
                name, args = tc.get("name"), json.dumps(tc.get("args") or {})
                is_read = name in READ_TOOLS or (name == "run_command" and SHELL_READ.search(args))
                for p in PA_PATH.findall(args):
                    key = (repo_of(p), category(p))
                    r["access"][key] += 1
                    if is_read:
                        r["reads"][key] += 1
        rows.append(r)

    print(f"Transcripts: {len(rows)}")
    print("Workspace per conversation DB:", collections.Counter(r["ws"] for r in rows).most_common())
    print("Subagents:", sum(r["subagent"] for r in rows))
    print("\nReport trajectory type x actual workspace:")
    for k, v in collections.Counter((r["rtype"], r["ws"]) for r in rows).most_common():
        print(f"  {k}: {v}")

    cross = [r for r in rows if r["ws"] not in ("MISSING",) and any(
        repo not in (r["ws"], "GLOBAL_SKILL", "other") for (repo, _) in r["access"])]
    print(f"\nConversations touching ANOTHER repo's PA workspace: {len(cross)}")

    for ws in ["esp32-app-foundation", "repo-memory-plugin", "bug-fixer-loop-plugin", "Echenon"]:
        pop = [r for r in rows if r["ws"] == ws]
        def n(cat):
            return sum(1 for r in pop if any(c == cat and repo == ws for (repo, c) in r["reads"]))
        print(f"\n{ws}: n={len(pop)} (subagents {sum(r['subagent'] for r in pop)}) — conversations READING own "
              f"MEMORY={n('MEMORY')} INDEX={n('INDEX')} PROJECT={n('PROJECT')} WORKORDER={n('WORKORDER')} "
              f"PROGRAM={n('PROGRAM')} process={sum(1 for r in pop if any(c == 'process' for (_, c) in r['reads']))}")
        print("   by month:", sorted(collections.Counter(r["month"] for r in pop).items()))

    print("\nesp32 conversations in DB by month (all) — for comparison with the sample:")
    con = sqlite3.connect(os.path.expanduser(DBS[0]))
    q = "select substr(last_modified_time,1,7), count(*) from conversation_summaries where workspace_uris like '%esp32-app-foundation%' group by 1"
    print("  ", con.execute(q).fetchall())
    print("\nesp32 PA PROJECT.md files created per month (git):")
    out = subprocess.run(["git", "-C", ESP32, "log", "--diff-filter=A", "--format=%cs", "--",
                          ".agents/skills/principal-architect-workspace/*/PROJECT.md"], capture_output=True, text=True).stdout
    print("  ", sorted(collections.Counter(x[:7] for x in out.split()).items()))


if __name__ == "__main__":
    main()
