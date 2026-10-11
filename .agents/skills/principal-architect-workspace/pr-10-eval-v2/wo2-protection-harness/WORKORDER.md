---
name: pr-10-eval-v2-wo2
description: Handoff instructions for Work Order 2 — Protection harness and run orchestration script
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Implement the local protection layer (§10 of the [evaluation proposal](../../../../docs/evaluations/memory-value-evaluation-proposal.md)) and create the run orchestration script that ensures every evaluation run is isolated, protected from GitHub/remote access, and reproducible.

## Context

The evaluation runs agents on copies of the esp32 template. Without protection, those agents can:
- Push to GitHub (git credentials in the system keychain via `osxkeychain`)
- Create issues, PRs, comments via `gh` (authenticated via keychain with `repo`, `workflow`, `delete_repo` scopes)
- Access the real esp32 checkout or other run directories

Protection is **local, covering every repo on disk** — no GitHub Pro needed.

The evaluation templates are at `evaluation/eval-v2/tpl-{a,b,c,d}/`. Each has a squashed single-commit history with no remote. Per-run copies are made with APFS clonefile (`cp -c -R`).

### X6-lite reusable components
The following from X6-lite should be reused or adapted:
- Per-run plugin and data dirs
- Allowlist transcript scan (access to real esp32 checkout or another run's dir → invalid)
- Interleaved scheduling
- `analysis_set` tagging
- Conversation backup/restore

Check `evaluation/scripts/` for existing X6-lite harness scripts that can be referenced or adapted.

## Strict Boundary Box

- Do NOT modify any evaluation templates (`tpl-a/`, `tpl-b/`, etc.) or the master `template/`.
- Do NOT modify any repo-memory-plugin source code.
- Do NOT push, create PRs, or interact with GitHub.
- Work exclusively in `evaluation/eval-v2/scripts/` for scripts.
- Configuration docs go in `evaluation/eval-v2/`.

## Tooling Constraints

Use standard shell scripting (bash/zsh). Scripts must be portable across macOS (the only target platform). Use `shasum -a 256` for checksums.

## Line-Item Instructions

- [ ] **Create `evaluation/eval-v2/scripts/eval_env.sh`** — a sourceable environment file that sets:
  ```bash
  export GH_TOKEN=invalid
  export GIT_CONFIG_NOSYSTEM=1
  export GIT_TERMINAL_PROMPT=0
  # Optional: unset SSH_AUTH_SOCK (remotes are HTTPS, SSH isn't the default route)
  ```
  Include a comment block explaining why each variable is set.

- [ ] **Create `evaluation/eval-v2/scripts/verify_protection.sh`** — the one-time check script:
  1. Sources `eval_env.sh`
  2. Runs `gh api user` and asserts it fails (non-zero exit)
  3. Creates a temporary git repo with a dummy remote, runs `git push`, and asserts it fails
  4. Prints a clear PASS/FAIL summary
  5. Cleans up the temporary repo

- [ ] **Create `evaluation/eval-v2/scripts/eval_run.sh`** — the per-run orchestration script:
  Takes arguments: `--condition {a,b,c,d}`, `--task <task-file>`, `--run-id <id>`, `--eval-root <path>`
  
  The script must:
  1. Source `eval_env.sh` for protection
  2. Create a per-run directory: `$EVAL_ROOT/runs/$RUN_ID/`
  3. Copy the appropriate template: `cp -c -R $EVAL_ROOT/tpl-$CONDITION/ $EVAL_ROOT/runs/$RUN_ID/repo/`
  4. For repo-memory-on conditions (c, d): create per-run plugin and data dirs, enrol with the per-run data dir
  5. Append the safety instruction to the task prompt: "Work locally. Do not push, open PRs, or comment on GitHub."
  6. Print the full command to launch the agent (but do NOT launch it — that's interactive)
  7. After the agent run (when called with `--archive`): archive the `git diff baseline` + untracked files, transcript, and ledger to `$EVAL_ROOT/runs/$RUN_ID/archive/`
  8. Delete the run's repo copy (but keep the archive)

- [ ] **Create `evaluation/eval-v2/scripts/check_contamination.sh`** — the transcript scan:
  Takes a transcript file path.
  Scans for:
  - Any reference to the real esp32 checkout path (`~/Documents/Projects/esp32-app-foundation` and its absolute expansion)
  - Any `git push` command
  - Any `gh` write command (`gh pr create`, `gh issue create`, `gh api -X POST`, etc.)
  - Any reference to another run's directory
  Reports findings and exits non-zero if contamination detected.

- [ ] **Create `evaluation/eval-v2/PROTECTION.md`** — document:
  1. What the protection covers and its scope
  2. What it does NOT cover (deliberately adversarial agent reading key files — a separate macOS user would be needed)
  3. Step-by-step instructions to disable BugFixer daemon plugin and repo-memory tick sidecar during batch runs, and restore them afterwards
  4. The one-time verification procedure

- [ ] **Verify protection works** by running `verify_protection.sh` in the current environment and confirming both checks fail as expected.

## Verification Requirement

1. `verify_protection.sh` passes (gh and git push both fail under the eval environment)
2. `eval_run.sh --help` shows usage
3. `check_contamination.sh` correctly flags a test transcript containing `git push` and passes on a clean one
4. All scripts are executable (`chmod +x`)
5. `PROTECTION.md` accurately documents the protection model

**CRITICAL:** Before committing any files to the repo-memory-plugin repo, ask the user if they are happy with the changes.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

<!-- Template — to be filled in by the worker -->
