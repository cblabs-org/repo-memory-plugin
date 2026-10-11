#!/usr/bin/env python3
"""Final comprehensive X6-lite analysis with all 3 conditions and tiebreaker resolution."""
import json
from collections import defaultdict

results = []
with open("evaluation/results/x6-lite-results.jsonl") as f:
    for line in f:
        results.append(json.loads(line.strip()))

wos = ["wo1-op-log", "wo2-dossier-schema-renderer", "wo3-scopes-aliases", "wo4-index-migration",
       "wo6-seeding", "wo1-ledger-engine", "wo4-gates", "wo6-pull-tools"]

by = defaultdict(lambda: defaultdict(list))
for r in results:
    by[r["work_order"]][r["condition"]].append(r)

print(f"X6-LITE FINAL RESULTS")
print(f"Total valid runs: {len(results)}")
print(f"Total tokens: {sum(r['total_tokens'] for r in results):,}")
print()

# Full 3-condition comparison table
conds = ["no_memory", "pa_documents", "dossiers"]
header = f"{'WO':35s}"
for c in conds:
    header += f" {c:>14s}"
header += "  nm->dos"
print(header)
print("-" * 95)

totals = {c: {"pass": 0, "total": 0, "tokens": 0} for c in conds}

for wo in wos:
    row = f"  {wo:33s}"
    rates = {}
    for c in conds:
        runs = by[wo].get(c, [])
        p = sum(1 for r in runs if r["status"] == "pass")
        totals[c]["pass"] += p
        totals[c]["total"] += len(runs)
        totals[c]["tokens"] += sum(r["total_tokens"] for r in runs)
        rates[c] = p / len(runs) if runs else 0
        row += f"   {p}/{len(runs)}       "
    
    # Delta column (no_memory -> dossiers)
    if rates.get("dossiers", 0) > rates.get("no_memory", 0):
        row += "    UP"
    elif rates.get("dossiers", 0) < rates.get("no_memory", 0):
        row += "    DN"
    else:
        row += "    =="
    print(row)

print("-" * 95)

# Aggregate row
row = f"  {'AGGREGATE':33s}"
for c in conds:
    t = totals[c]
    pct = t["pass"] / t["total"] * 100 if t["total"] else 0
    row += f" {t['pass']}/{t['total']} ({pct:.0f}%)    "
print(row)

print()

# Kill gate
nm_rate = totals["no_memory"]["pass"] / totals["no_memory"]["total"] * 100
dos_rate = totals["dossiers"]["pass"] / totals["dossiers"]["total"] * 100
pa_rate = totals["pa_documents"]["pass"] / totals["pa_documents"]["total"] * 100

gate = "PASS" if dos_rate > nm_rate else "FAIL"
print(f"KILL GATE: {gate}")
print(f"  (c) dossiers   : {dos_rate:.1f}%")
print(f"  (b) pa_documents: {pa_rate:.1f}%")
print(f"  (a) no_memory  : {nm_rate:.1f}%")
print()

# Tiebreaker resolution
print("TIEBREAKER RESOLUTION:")
for wo in wos:
    for c in conds:
        runs = by[wo].get(c, [])
        if len(runs) >= 3:
            s42 = next((r["status"] for r in runs if r["seed"] == 42), "?")
            s137 = next((r["status"] for r in runs if r["seed"] == 137), "?")
            s256 = next((r["status"] for r in runs if r["seed"] == 256), "?")
            majority = "PASS" if sum(1 for s in [s42, s137, s256] if s == "pass") >= 2 else "FAIL"
            print(f"  {wo}/{c}: 42={s42}, 137={s137}, 256={s256} -> {majority}")

# PA tiebreakers check
print()
print("PA_DOCUMENTS TIEBREAKERS:")
pa_ties = []
for wo in wos:
    runs = by[wo].get("pa_documents", [])
    if len(runs) >= 2:
        statuses = set(r["status"] for r in runs)
        if len(statuses) > 1:
            s42 = next((r["status"] for r in runs if r["seed"] == 42), "?")
            s137 = next((r["status"] for r in runs if r["seed"] == 137), "?")
            pa_ties.append((wo, s42, s137))
            print(f"  {wo}: seed42={s42}, seed137={s137}")
if not pa_ties:
    print("  None needed")

# Token breakdown per condition
print()
print("TOKEN BREAKDOWN:")
for c in conds:
    t = totals[c]
    avg = t["tokens"] // t["total"] if t["total"] else 0
    print(f"  {c:20s}: {t['tokens']:>12,} total, {avg:>8,} avg/run, {t['total']:2d} runs")

total_all = sum(t["tokens"] for t in totals.values())
print(f"  {'TOTAL':20s}: {total_all:>12,}")
