#!/bin/bash
# Save all esp32-app-foundation transcripts from the brain before pruning
set -euo pipefail

BRAIN_DIR="$HOME/.gemini/antigravity/brain"
OUT_DIR="$HOME/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcripts"
mkdir -p "$OUT_DIR"

count=0
for transcript in "$BRAIN_DIR"/*/".system_generated/logs/transcript.jsonl"; do
    if [ ! -f "$transcript" ]; then
        continue
    fi
    if grep -q "esp32-app-foundation" "$transcript" 2>/dev/null; then
        convdir=$(dirname "$(dirname "$(dirname "$transcript")")")
        convid=$(basename "$convdir")
        mkdir -p "$OUT_DIR/$convid"
        cp "$transcript" "$OUT_DIR/$convid/transcript.jsonl"
        
        # Also grab the full transcript
        full="${transcript%.jsonl}_full.jsonl"
        if [ -f "$full" ]; then
            cp "$full" "$OUT_DIR/$convid/transcript_full.jsonl"
        fi
        
        # Save any artifacts in the brain dir (not .system_generated)
        for artifact in "$convdir"/*.md; do
            if [ -f "$artifact" ]; then
                cp "$artifact" "$OUT_DIR/$convid/"
            fi
        done
        
        count=$((count + 1))
    fi
done

echo "Saved $count conversation transcripts to $OUT_DIR"
du -sh "$OUT_DIR"
ls "$OUT_DIR" | wc -l
echo "conversation dirs"
