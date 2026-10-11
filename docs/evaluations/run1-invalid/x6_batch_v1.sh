#!/usr/bin/env bash
# X6-lite Batch Orchestrator
# Runs multiple x6_runner.sh invocations with controlled parallelism.
#
# Usage:
#   ./x6_batch.sh pilot          # Pilot: 2 runs (wo1-op-log, conditions a+c, seed 42)
#   ./x6_batch.sh wave1          # All 8 WOs × condition (a) × seeds 42,137
#   ./x6_batch.sh wave2          # All 8 WOs × condition (c) × seeds 42,137
#   ./x6_batch.sh wave3          # All 8 WOs × condition (b) × seeds 42,137
#   ./x6_batch.sh tiebreakers    # Disagreements from waves 1-3 with seed 256
#   ./x6_batch.sh full           # All waves sequentially
#
# Set MAX_PARALLEL=N to control concurrency (default: 2)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNNER="${SCRIPT_DIR}/x6_runner.sh"
RESULTS_FILE="${SCRIPT_DIR}/../results/x6-lite-results.jsonl"

MAX_PARALLEL="${MAX_PARALLEL:-2}"
BATCH="${1:?Usage: x6_batch.sh <pilot|wave1|wave2|wave3|tiebreakers|full>}"

ALL_WOS=(
    wo1-op-log
    wo2-dossier-schema-renderer
    wo3-scopes-aliases
    wo4-index-migration
    wo6-seeding
    wo1-ledger-engine
    wo4-gates
    wo6-pull-tools
)

SEEDS_BASE=(42 137)

# Track PIDs for parallel execution
declare -a PIDS=()
declare -a LABELS=()

run_job() {
    local wo="$1" cond="$2" seed="$3"
    local label="${wo}/${cond}/seed${seed}"
    echo "[batch] Starting: ${label}"
    bash "${RUNNER}" "${wo}" "${cond}" "${seed}" &
    PIDS+=($!)
    LABELS+=("${label}")
}

QUOTA_HIT=0

wait_batch() {
    local failed=0
    for i in "${!PIDS[@]}"; do
        local rc=0
        wait "${PIDS[$i]}" || rc=$?
        if [[ $rc -eq 0 ]]; then
            echo "[batch] ✓ ${LABELS[$i]}"
        elif [[ $rc -eq 2 ]]; then
            echo "[batch] ⛔ ${LABELS[$i]} (QUOTA EXHAUSTED)"
            QUOTA_HIT=1
        else
            echo "[batch] ✗ ${LABELS[$i]} (exit code: $rc)"
            failed=$((failed + 1))
        fi
    done
    PIDS=()
    LABELS=()
    if [[ $failed -gt 0 ]]; then
        echo "[batch] WARNING: ${failed} job(s) failed in this wave."
    fi
    if [[ $QUOTA_HIT -eq 1 ]]; then
        echo "[batch] ⛔ QUOTA EXHAUSTED — stopping wave. Re-run this batch later to resume."
    fi
}

throttle() {
    while [[ ${#PIDS[@]} -ge ${MAX_PARALLEL} ]]; do
        # Wait for any one to finish
        for i in "${!PIDS[@]}"; do
            if ! kill -0 "${PIDS[$i]}" 2>/dev/null; then
                local rc=0
                wait "${PIDS[$i]}" || rc=$?
                if [[ $rc -eq 0 ]]; then
                    echo "[batch] ✓ ${LABELS[$i]}"
                elif [[ $rc -eq 2 ]]; then
                    echo "[batch] ⛔ ${LABELS[$i]} (QUOTA EXHAUSTED)"
                    QUOTA_HIT=1
                else
                    echo "[batch] ✗ ${LABELS[$i]} (exit code: $rc)"
                fi
                unset "PIDS[$i]"
                unset "LABELS[$i]"
                # Re-index arrays
                PIDS=("${PIDS[@]}")
                LABELS=("${LABELS[@]}")
                return
            fi
        done
        sleep 2
    done
}

run_wave() {
    local condition="$1"
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║ Wave: ${condition} (max ${MAX_PARALLEL} parallel)"
    echo "╚══════════════════════════════════════════════════════════════╝"
    
    for wo in "${ALL_WOS[@]}"; do
        for seed in "${SEEDS_BASE[@]}"; do
            if [[ $QUOTA_HIT -eq 1 ]]; then
                echo "[batch] ⛔ Skipping remaining runs — quota exhausted."
                break 2
            fi
            throttle
            if [[ $QUOTA_HIT -eq 1 ]]; then
                break 2
            fi
            run_job "${wo}" "${condition}" "${seed}"
        done
    done
    wait_batch
}

check_tiebreakers() {
    # Find WO/condition pairs where seeds 42 and 137 disagree
    if [[ ! -f "${RESULTS_FILE}" ]]; then
        echo "[tiebreakers] No results file found."
        return
    fi
    
    echo "[tiebreakers] Scanning for disagreements..."
    python3 -c "
import json, sys
from collections import defaultdict

results = defaultdict(dict)
with open('${RESULTS_FILE}') as f:
    for line in f:
        r = json.loads(line.strip())
        key = (r['work_order'], r['condition'])
        results[key][r['seed']] = r['status']

tiebreakers = []
for (wo, cond), seeds in sorted(results.items()):
    if 42 in seeds and 137 in seeds:
        if seeds[42] != seeds[137]:
            tiebreakers.append((wo, cond))
            print(f'  DISAGREE: {wo}/{cond} - seed42={seeds[42]}, seed137={seeds[137]}')

if not tiebreakers:
    print('  No disagreements found. No tiebreakers needed.')
else:
    print(f'  {len(tiebreakers)} tiebreaker(s) needed.')

# Write tiebreaker list
with open('/tmp/x6-tiebreakers.txt', 'w') as f:
    for wo, cond in tiebreakers:
        f.write(f'{wo} {cond}\n')
"
}

run_tiebreakers() {
    check_tiebreakers
    if [[ ! -f /tmp/x6-tiebreakers.txt ]] || [[ ! -s /tmp/x6-tiebreakers.txt ]]; then
        echo "[tiebreakers] Nothing to do."
        return
    fi
    
    while IFS=' ' read -r wo cond; do
        throttle
        run_job "${wo}" "${cond}" 256
    done < /tmp/x6-tiebreakers.txt
    wait_batch
}

print_summary() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║ SUMMARY"
    echo "╚══════════════════════════════════════════════════════════════╝"
    
    if [[ ! -f "${RESULTS_FILE}" ]]; then
        echo "No results found."
        return
    fi
    
    python3 -c "
import json
from collections import Counter

results = []
with open('${RESULTS_FILE}') as f:
    for line in f:
        results.append(json.loads(line.strip()))

total = len(results)
passed = sum(1 for r in results if r['status'] == 'pass')
total_tokens = sum(r.get('total_tokens', 0) for r in results)
total_duration = sum(r.get('duration_seconds', 0) for r in results)

print(f'Total runs:    {total}')
print(f'Passed:        {passed}/{total} ({100*passed/total:.0f}%)')
print(f'Total tokens:  {total_tokens:,}')
print(f'Total time:    {total_duration//3600}h {(total_duration%3600)//60}m')
print()

# Per-condition breakdown
by_cond = {}
for r in results:
    c = r['condition']
    if c not in by_cond:
        by_cond[c] = {'total': 0, 'passed': 0}
    by_cond[c]['total'] += 1
    if r['status'] == 'pass':
        by_cond[c]['passed'] += 1

for c in ['no_memory', 'pa_documents', 'dossiers']:
    if c in by_cond:
        d = by_cond[c]
        print(f'  {c:20s}: {d[\"passed\"]}/{d[\"total\"]} passed ({100*d[\"passed\"]/d[\"total\"]:.0f}%)')
"
}

# Main dispatch
case "${BATCH}" in
    pilot)
        echo "═══ PILOT: 2 runs (wo1-op-log, conditions a+c, seed 42) ═══"
        run_job "wo1-op-log" "no_memory" 42
        wait_batch
        run_job "wo1-op-log" "dossiers" 42
        wait_batch
        print_summary
        ;;
    wave1)
        run_wave "no_memory"
        print_summary
        ;;
    wave2)
        run_wave "dossiers"
        print_summary
        ;;
    wave3)
        run_wave "pa_documents"
        print_summary
        ;;
    tiebreakers)
        run_tiebreakers
        print_summary
        ;;
    full)
        run_wave "no_memory"
        run_wave "dossiers"
        run_wave "pa_documents"
        run_tiebreakers
        print_summary
        ;;
    summary)
        print_summary
        ;;
    *)
        echo "ERROR: Unknown batch: ${BATCH}" >&2
        echo "Usage: x6_batch.sh <pilot|wave1|wave2|wave3|tiebreakers|full|summary>" >&2
        exit 1
        ;;
esac
