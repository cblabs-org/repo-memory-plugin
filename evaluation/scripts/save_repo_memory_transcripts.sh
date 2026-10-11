#!/bin/bash
# Save all repo-memory-plugin transcripts from the brain
set -euo pipefail

BRAIN_DIR="$HOME/.gemini/antigravity/brain"
OUT_DIR="$HOME/Documents/Projects/repo-memory-plugin/evaluation/repo-memory-transcripts"
mkdir -p "$OUT_DIR"

count=0
for transcript in "$BRAIN_DIR"/*/".system_generated/logs/transcript.jsonl"; do
    if [ ! -f "$transcript" ]; then
        continue
    fi
    if grep -q "repo-memory-plugin" "$transcript" 2>/dev/null; then
        convdir=$(dirname "$(dirname "$(dirname "$transcript")")")
        convid=$(basename "$convdir")
        # Skip if already saved as esp32
        if [ -d "$HOME/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcripts/$convid" ]; then
            continue
        fi
        mkdir -p "$OUT_DIR/$convid"
        cp "$transcript" "$OUT_DIR/$convid/transcript.jsonl"
        full="${transcript%.jsonl}_full.jsonl"
        if [ -f "$full" ]; then
            cp "$full" "$OUT_DIR/$convid/transcript_full.jsonl"
        fi
        for artifact in "$convdir"/*.md; do
            if [ -f "$artifact" ]; then
                cp "$artifact" "$OUT_DIR/$convid/"
            fi
        done
        count=$((count + 1))
    fi
done

echo "Saved $count repo-memory conversation transcripts to $OUT_DIR"
du -sh "$OUT_DIR"
