#!/bin/bash
set -e

mkdir -p evaluation/data

WOS=(
  "pr-07-v2-storage/wo1-op-log"
  "pr-07-v2-storage/wo2-dossier-schema-renderer"
  "pr-07-v2-storage/wo3-scopes-aliases"
  "pr-07-v2-storage/wo4-index-migration"
  "pr-07-v2-storage/wo6-seeding"
  "pr-08-v2-delivery/wo1-ledger-engine"
  "pr-08-v2-delivery/wo4-gates"
  "pr-08-v2-delivery/wo6-pull-tools"
)

for WO in "${WOS[@]}"; do
  WO_NAME=$(basename $WO)
  echo "Processing $WO_NAME..."
  
  # Find parent PA commit
  PARENT_COMMIT=$(git log --oneline $WO | grep 'chore(PA)' | head -n 1 | awk '{print $1}')
  
  if [ -z "$PARENT_COMMIT" ]; then
    echo "No parent commit found for $WO"
    exit 1
  fi
  
  echo "Parent commit: $PARENT_COMMIT"
  
  # Create dir
  mkdir -p "evaluation/data/$WO_NAME/ops"
  mkdir -p "evaluation/data/$WO_NAME/pa_docs"
  
  # 2. Seed time-cut ops
  go run ./cmd/repo-memory seed --before "$PARENT_COMMIT" --ops-dir "evaluation/data/$WO_NAME/ops/"
  
  # 3. Capture PA documents at that time
  # PROJECT.md and MEMORY.md if they exist
  git show $PARENT_COMMIT:.agents/skills/principal-architect-workspace/memory/MEMORY.md > "evaluation/data/$WO_NAME/pa_docs/MEMORY.md" || true
  git show $PARENT_COMMIT:.agents/skills/principal-architect-workspace/$(dirname $WO)/PROJECT.md > "evaluation/data/$WO_NAME/pa_docs/PROJECT.md" || true
done

echo "Done"
