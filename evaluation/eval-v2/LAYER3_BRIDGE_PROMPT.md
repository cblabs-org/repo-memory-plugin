# Layer 3 Live Workstation Evaluation Guide & Bridge Prompt (`PR-10-eval-v2`)

This document is the self-contained workstation execution guide and copy-pasteable agent bridge prompt for **Layer 3 (`PR-10-eval-v2` Stage 2a Live Pilot, Stage 2b Pre-Registered 2×2 Main Evaluation, and Final Report)** on a macOS (or Linux) workstation equipped with PlatformIO (`pio`) and the Antigravity CLI (`agy`).

All Layer 2 component micro-probes (`WO1`–`WO4` in `PR-14`) passed with **`VERDICT: GO`** ([`docs/evaluations/v2-2-micro-probe-verdict.md`](../../docs/evaluations/v2-2-micro-probe-verdict.md)).

---

## 1. Workstation Prerequisites

Ensure the target macOS (or Linux) workstation has the following installed and on `PATH`:
- `git` ($\ge 2.39$)
- `go` ($\ge 1.22$)
- `sqlite3` (with FTS5 support)
- `python3` ($\ge 3.10$)
- **PlatformIO Core CLI (`pio`)** — capable of executing `pio test -e native` and `pio run` on `esp32-app-foundation`
- **Antigravity CLI (`agy`)** — capable of running `agy -p "<prompt>"` in each isolated run directory
- Local checkout of `matthewmcneill/esp32-app-foundation` at `~/Documents/Projects/esp32-app-foundation` (with generated build assets present so `prepare_templates.sh` can rsync a build-ready working tree)

---

## 2. Step-by-Step Terminal Pre-Flight Commands

Run the following commands in your terminal on the evaluation workstation to pull the latest branch, compile the `repo-memory` `v2.2` binary, stage and verify the `esp32-app-foundation` condition templates (`tpl-a`, `tpl-b`, `tpl-c`, `tpl-d`), build the `1,283`-entry `v2.2` combined index (`seed` + `seed-docs`), and verify the §10 local protection harness:

```bash
git fetch origin
git checkout pr-15-v2-learning/wo5-verification-and-skill-lint # or current branch
go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
export PATH="$HOME/.local/bin:$PATH"
bash evaluation/eval-v2/scripts/prepare_templates.sh
bash evaluation/eval-v2/scripts/verify_template.sh evaluation/eval-v2/template
bash evaluation/eval-v2/scripts/build_index.sh --clean
bash evaluation/eval-v2/scripts/verify_protection.sh
```

### Expected Pre-Flight Verification Outputs
1. `verify_template.sh`: Reports `=== Template Verification Succeeded ===` (`0` git remotes, single `baseline` commit, all generated files present).
2. `build_index.sh --clean`: Reports `1,283` total indexed entries (`256` PA workorder `seed` + `1,027` curated developer guide/skill `doc-seed`), `3,203` file anchors, and `54` projected subsystem dossiers in `evaluation/eval-v2/seed/`.
3. `verify_protection.sh`: Reports `Overall Result: PASS (All protection mechanisms verified)` (`gh api user` blocked with exit code `1` and `git push` blocked with exit code `128`).

---

## 3. Sidecar & Global Plugin Isolation Checklist (`PROTECTION.md` §5)

Before starting `WO5` (Stage 2a Pilot) and `WO6` (Stage 2b Main Evaluation), pause the global `repo-memory-daemon` tick sidecar and the `BugFixer` daemon plugin so background processes do not interfere with per-run directories or quota:

### 3.1 Disable Background Daemons (Before Batch Runs)

```bash
# 1. Disable global repo-memory-daemon tick sidecar
mkdir -p ~/.gemini/config/plugins/repo-memory-daemon
cat > ~/.gemini/config/plugins/repo-memory-daemon/config.json <<'EOF'
{
  "enabled": false
}
EOF

# 2. Disable BugFixer Daemon plugin via Antigravity Language Server RPC
: "${ANTIGRAVITY_LS_ADDRESS:=localhost:5387}"
curl -sS -X POST \
  "http://${ANTIGRAVITY_LS_ADDRESS}/exa.language_server_pb.LanguageServerService/JetboxWriteState" \
  -H "Content-Type: application/json" \
  -H "x-codeium-csrf-token: ${ANTIGRAVITY_CSRF_TOKEN:-}" \
  -d '{"userConfig":{"plugins":{"bug-fixer-daemon":{"enabled":false}}}}' || true
```
*(Alternatively, toggle **BugFixer Daemon** Off in the Antigravity UI under **Plugins & Integrations**.)*

### 3.2 Restore Background Daemons (After Batch Runs Complete in `WO7`)

```bash
# 1. Re-enable global repo-memory-daemon tick sidecar
cat > ~/.gemini/config/plugins/repo-memory-daemon/config.json <<'EOF'
{
  "enabled": true
}
EOF

# 2. Re-enable BugFixer Daemon plugin
: "${ANTIGRAVITY_LS_ADDRESS:=localhost:5387}"
curl -sS -X POST \
  "http://${ANTIGRAVITY_LS_ADDRESS}/exa.language_server_pb.LanguageServerService/JetboxWriteState" \
  -H "Content-Type: application/json" \
  -H "x-codeium-csrf-token: ${ANTIGRAVITY_CSRF_TOKEN:-}" \
  -d '{"userConfig":{"plugins":{"bug-fixer-daemon":{"enabled":true}}}}' || true
```

---

## 4. Copy-Pasteable Agent Bridge Prompt (`WO5` $\to$ `WO6` $\to$ `WO7`)

Copy and paste the markdown block below into a new Antigravity (or Claude Code) session opened at the root of `~/Documents/Projects/repo-memory-plugin` on your workstation:

````markdown
# Role & Mission: Execute Layer 3 Live Evaluation (`pr-10-eval-v2` Work Orders WO5 -> WO6 -> WO7)

You are orchestrating and executing **Layer 3 (`PR-10-eval-v2` Stages 2a, 2b, and Final Report)** of the `repo-memory v2.2` evaluation on this workstation, where PlatformIO (`pio`) and the Antigravity CLI (`agy`) are installed.

## 1. Mandatory Context Files to Page In First
Before running any commands, read these files in full to load the architecture, Layer 2 micro-probe baselines, protection rules, blind grading rubrics, and work orders:
1. `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`
2. `.agents/skills/principal-architect-workspace/pr-10-eval-v2/chief-architect-handover.md`
3. `docs/evaluations/v2-2-micro-probe-verdict.md`
4. `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo5-pilot/WORKORDER.md`
5. `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo6-main-evaluation/WORKORDER.md`
6. `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo7-report/WORKORDER.md`
7. `evaluation/eval-v2/rubrics/task-1-temperature-module.yaml`
8. `evaluation/eval-v2/rubrics/task-2-bitcoin-price-module.yaml`
9. `evaluation/eval-v2/rubrics/grade_run.sh`

## 2. Execution Sequence

### Phase A: Workstation Pre-Flight Verification
1. Verify `~/.local/bin/repo-memory`, `pio --version`, and `agy --version` are available.
2. If `evaluation/eval-v2/tpl-a` through `tpl-d` or `evaluation/eval-v2/seed/index.db` (`1,283` entries) are not yet built on this machine, run:
   - `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`
   - `bash evaluation/eval-v2/scripts/prepare_templates.sh`
   - `bash evaluation/eval-v2/scripts/verify_template.sh evaluation/eval-v2/template`
   - `bash evaluation/eval-v2/scripts/build_index.sh --clean`
   - `bash evaluation/eval-v2/scripts/verify_protection.sh`
3. Confirm `~/.gemini/config/plugins/repo-memory-daemon/config.json` has `"enabled": false` during batch runs.

### Phase B: Execute `WO5` — Stage 2a Live 1-Task Workstation Pilot (`wo5-pilot/WORKORDER.md`)
1. Stage and run `pilot-t1-a-live` (`Condition A`) and `pilot-t1-c-live` (`Condition C`) on `evaluation/eval-v2/tasks/task-1-temperature-module.txt` using `evaluation/eval-v2/scripts/eval_run.sh`.
2. In each run's `repo/` directory before archival, run `pio test -e native` and save the output to `native_test_result.txt`.
3. Archive both runs with `eval_run.sh --archive`, copy `native_test_result.txt` into `archive/`, verify zero contamination with `evaluation/eval-v2/scripts/check_contamination.sh`, and grade both archives blind to condition using `evaluation/eval-v2/rubrics/grade_run.sh --run-dir <archive> --task 1`.
4. Check mean per-run token cost (including carried `cache_read` tokens) in `scorecard.json` against the 450K/run threshold and 7.5M total cap, record the Completion Report in `wo5-pilot/WORKORDER.md`, and pause for human gate review if needed.

### Phase C: Execute `WO6` — Stage 2b Pre-Registered 2x2 Main Evaluation (`wo6-main-evaluation/WORKORDER.md`)
1. Run the pre-registered counterbalanced interleaved schedule across `task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt`, Conditions `a`, `b`, `c`, `d`:
   - Replicate 1: `[run-t1-a-rep1, run-t1-c-rep1, run-t1-b-rep1, run-t1-d-rep1, run-t2-c-rep1, run-t2-a-rep1, run-t2-d-rep1, run-t2-b-rep1]`
   - Replicate 2 (Reversed): `[run-t2-b-rep2, run-t2-d-rep2, run-t2-a-rep2, run-t2-c-rep2, run-t1-d-rep2, run-t1-b-rep2, run-t1-c-rep2, run-t1-a-rep2]`
2. For each run: execute `pio test -e native` -> `eval_run.sh --archive` -> `check_contamination.sh` -> `grade_run.sh --run-dir <archive> --task auto`.
3. Record the Completion Report in `wo6-main-evaluation/WORKORDER.md`.

### Phase D: Execute `WO7` — Final Evaluation v2 Report (`wo7-report/WORKORDER.md`)
1. Aggregate all `scorecard.json` files, compute paired per-task differences (`C - A`, `C - B`, `D - C`), Wilcoxon signed-rank / paired t-test p-values, Holm step-down corrections, 95% confidence intervals, and the Read + Amortised Write Cost/Quality Frontier.
2. Author `docs/evaluations/eval-v2-report.md` including explicit `Q1` and `Q2` verdicts and the mandatory `## What this does not show` honesty section.
3. Complete `wo7-report/WORKORDER.md` and update `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`.

## 3. Repository Hygiene & Safety Rules
- Never push or interact with GitHub during agent evaluation runs (`eval_env.sh` enforces `GH_TOKEN=invalid` and `GIT_CONFIG_NOSYSTEM=1`).
- Use portable `$HOME` or `~/` paths in all committed scripts and reports.
- Commit all evaluation artifacts and reports with author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.
````
