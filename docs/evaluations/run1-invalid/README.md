# Run 1 Evidence Archive — Known Issues

## Overwritten Files (pilot used same filenames)

The following Run 1 output files were overwritten by the Run 2 pilot
(wo1-op-log/no_memory/seed42 and wo1-op-log/dossiers/seed42):

- `wo1-op-log-no_memory-seed42-agy-output.json` — NOW CONTAINS PILOT DATA (Oct 10 08:12)
- `wo1-op-log-no_memory-seed42-agy-stderr.log` — NOW CONTAINS PILOT DATA (Oct 10 07:58)
- `wo1-op-log-dossiers-seed42-agy-output.json` — NOW CONTAINS PILOT DATA (Oct 10 08:20)
- `wo1-op-log-dossiers-seed42-agy-stderr.log` — NOW CONTAINS PILOT DATA (Oct 10 08:12)

The original Run 1 data for these two runs is **lost**.
All other Run 1 artifacts (50 other runs) retain their original Run 1 data.

## Note on Validity

All Run 1 results are INVALID regardless (see x6-lite-run1-postmortem.md).
The overwritten files were already invalid — the loss is archival only.

## Archive Contents

- `x6-lite-results.jsonl` — Run 1 JSONL results (52 entries, all invalid)
- `x6-lite-run1-results-INVALID.jsonl` — duplicate of above
- `x6-lite-run1-postmortem.md` — post-mortem investigation report
- `x6_runner_v1.sh` — original runner script with defects
- `x6_batch_v1.sh` — original batch orchestrator
- `x6_analyze_v1.py` — original analysis script
- `tmp-artifacts/` — agy output JSONs, stderr logs, exit test logs from /tmp
