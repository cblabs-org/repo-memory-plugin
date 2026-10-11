#!/usr/bin/env python3
"""Remove junk placeholder entries from evaluation ops batch files.

Usage: python3 clean_junk_ops.py <ops_dir> [<ops_dir2> ...]
"""
import json, os, sys, glob

JUNK_EXACT = {'', 'none.', 'none', '(pending)', 'n/a', 'pending', 'tbd', 'todo'}
JUNK_PREFIXES = ['none encountered', 'none identified', 'no issues', 'nothing to']

def is_junk(text):
    t = text.strip().lower()
    if t in JUNK_EXACT:
        return True
    return any(t.startswith(p) for p in JUNK_PREFIXES)

total_removed = 0
for ops_dir in sys.argv[1:]:
    for batch_file in sorted(glob.glob(f"{ops_dir}/**/*.json", recursive=True)):
        with open(batch_file) as f:
            batch = json.load(f)
        
        original = batch.get('ops', [])
        cleaned = [op for op in original
                   if not is_junk(op.get('fields', {}).get('text', 'non-empty'))]
        removed = len(original) - len(cleaned)
        
        if removed > 0:
            total_removed += removed
            if len(cleaned) == 0:
                os.remove(batch_file)
                print(f"  {os.path.basename(batch_file)}: removed {removed} junk ops (file deleted — empty)")
            else:
                batch['ops'] = cleaned
                with open(batch_file, 'w') as f:
                    json.dump(batch, f, indent=2)
                    f.write('\n')
                print(f"  {os.path.basename(batch_file)}: removed {removed} junk ops")

print(f"\nTotal junk ops removed: {total_removed}")
