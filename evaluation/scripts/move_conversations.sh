#!/usr/bin/env bash
# Move old CLI conversations out of the brain dir for the batch
# Usage: ./move_conversations.sh backup   (moves out)
#        ./move_conversations.sh restore  (moves back)
set -euo pipefail

BRAIN_DIR="${HOME}/.gemini/antigravity-cli/brain"
BACKUP_DIR="/tmp/x6-cli-brain-backup"

case "${1:-}" in
    backup)
        mkdir -p "${BACKUP_DIR}"
        COUNT=$(ls "${BRAIN_DIR}" 2>/dev/null | wc -l | tr -d ' ')
        echo "Moving ${COUNT} conversations to ${BACKUP_DIR}..."
        mv "${BRAIN_DIR}"/* "${BACKUP_DIR}/" 2>/dev/null || true
        REMAINING=$(ls "${BRAIN_DIR}" 2>/dev/null | wc -l | tr -d ' ')
        echo "Done. Remaining in brain dir: ${REMAINING}"
        ;;
    restore)
        if [[ ! -d "${BACKUP_DIR}" ]]; then
            echo "No backup found at ${BACKUP_DIR}"
            exit 1
        fi
        COUNT=$(ls "${BACKUP_DIR}" 2>/dev/null | wc -l | tr -d ' ')
        echo "Restoring ${COUNT} conversations..."
        mv "${BACKUP_DIR}"/* "${BRAIN_DIR}/" 2>/dev/null || true
        rmdir "${BACKUP_DIR}" 2>/dev/null || true
        echo "Done."
        ;;
    *)
        echo "Usage: $0 backup|restore"
        exit 1
        ;;
esac
