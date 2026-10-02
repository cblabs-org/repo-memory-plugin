---
id: clm-01M3YZCDQA3GC24E7APA17HY9N
type: architecture
title: The `agy -p` CLI command silently ignores `--project`; process and database isolation requires dedicated run directories (`runs/<repo-id>`) and setting `REPO_MEMORY_HOUSEKEEPING=1`.
relevant_when: Configuring process isolation and database environments for CLI housekeeping tasks.
apply: Isolate CLI runs by executing in dedicated `runs/<repo-id>` directories with `REPO_MEMORY_HOUSEKEEPING=1` instead of relying on `--project`.
status: proposed
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E07.md
    fp: E07
created: 2026-10-02T19:52:46.698623+01:00
updated: 2026-10-02T19:59:55.367888+01:00
---


