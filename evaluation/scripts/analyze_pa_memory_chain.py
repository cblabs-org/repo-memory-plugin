#!/usr/bin/env python3
"""Analyze PA memory reads broken down by trajectory type."""
import json
from collections import Counter, defaultdict
from pathlib import Path

TDIR = Path("evaluation/esp32-transcripts")

# Load trajectory types
traj_types = {}
with open("evaluation/esp32-transcript-analysis.jsonl") as f:
    for line in f:
        r = json.loads(line)
        traj_types[r["conv_id"]] = r["trajectory_type"]

# Re-analyze per trajectory type
type_pa_reads = defaultdict(lambda: {"total": 0, "skill": 0, "memory": 0, "index": 0, "project": 0, "workorder": 0, "program": 0, "other_ref": 0, "convs_with_reads": 0})

for conv_dir in sorted(TDIR.iterdir()):
    if not conv_dir.is_dir():
        continue
    transcript = conv_dir / "transcript.jsonl"
    if not transcript.exists():
        continue
    conv_id = conv_dir.name
    ttype = traj_types.get(conv_id, "unknown")
    has_pa_read = False
    
    with open(transcript) as f:
        for line_text in f:
            try:
                step = json.loads(line_text.strip())
            except:
                continue
            for tc in (step.get("tool_calls") or []):
                name = tc.get("name", "")
                args = tc.get("args", {}) or tc.get("arguments", {}) or {}
                if isinstance(args, str):
                    try: args = json.loads(args)
                    except: args = {}
                if name not in ("view_file", "read_file"):
                    continue
                path = args.get("AbsolutePath", "") or args.get("path", "")
                if "principal-architect" not in path:
                    continue
                has_pa_read = True
                bucket = type_pa_reads[ttype]
                bucket["total"] += 1
                if "SKILL.md" in path: bucket["skill"] += 1
                elif "MEMORY.md" in path and "MODULE" not in path: bucket["memory"] += 1
                elif "MODULE_INDEX" in path or "PROJECT_INDEX" in path: bucket["index"] += 1
                elif path.endswith("PROJECT.md"): bucket["project"] += 1
                elif "WORKORDER.md" in path: bucket["workorder"] += 1
                elif "PROGRAM" in path: bucket["program"] += 1
                elif "references/" in path: bucket["other_ref"] += 1
    
    if has_pa_read:
        type_pa_reads[ttype]["convs_with_reads"] += 1

type_counts = Counter(traj_types.values())

print("=== PA MEMORY READS BY TRAJECTORY TYPE ===")
print()
header = f"{'Type':15s} {'n':>4s} {'PA#':>4s} {'%PA':>5s} | {'SKILL':>5s} {'MEM':>5s} {'IDX':>5s} {'PROJ':>5s} {'WO':>5s} {'PROG':>5s} {'REF':>5s} {'Total':>6s}"
print(header)
print("-" * len(header))
for ttype in ["bugfixer", "pa_driven", "interactive", "autonomous", "multi_agent"]:
    n = type_counts.get(ttype, 0)
    b = type_pa_reads[ttype]
    pct = 100 * b["convs_with_reads"] / n if n else 0
    print(f"{ttype:15s} {n:4d} {b['convs_with_reads']:4d} {pct:4.0f}% | {b['skill']:5d} {b['memory']:5d} {b['index']:5d} {b['project']:5d} {b['workorder']:5d} {b['program']:5d} {b['other_ref']:5d} {b['total']:6d}")

print()
print("LEGEND:")
print("  PA# = conversations that read ANY PA file")
print("  SKILL = SKILL.md (process instructions)")
print("  MEM = MEMORY.md (the memory pager)")
print("  IDX = PROJECT_INDEX or MODULE_INDEX (knowledge indexes)")
print("  PROJ = */PROJECT.md (project knowledge)")
print("  WO  = */WORKORDER.md (task knowledge)")
print("  PROG = PROGRAM.md/PROGRAM-PLAN.md")
print("  REF = references/* (harness docs, templates)")

# Now the critical question: of bug-fixers that read PA, how many read MEMORY?
print()
print("=== BUG-FIXER DEEP DIVE ===")
bf_pa = type_pa_reads["bugfixer"]
print(f"Bug-fixers reading PA files: {bf_pa['convs_with_reads']}/139")
print(f"  Read SKILL.md:    {bf_pa['skill']}")
print(f"  Read MEMORY.md:   {bf_pa['memory']}")
print(f"  Read indexes:     {bf_pa['index']}")
print(f"  Read PROJECT.md:  {bf_pa['project']}")
print(f"  Read WORKORDER:   {bf_pa['workorder']}")
print(f"  Read PROGRAM:     {bf_pa['program']}")
print(f"  Read references:  {bf_pa['other_ref']}")

if __name__ == "__main__":
    pass
