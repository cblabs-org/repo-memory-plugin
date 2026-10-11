# Evaluation Protection Layer Specification (`PROTECTION.md`)

## 1. Executive Summary

This document specifies the local protection architecture and operating procedures for **Evaluation v2 (`eval-v2`)** on the `esp32-app-foundation` benchmark codebase, as outlined in §10 of the [Evaluation Proposal](../../docs/evaluations/memory-value-evaluation-proposal.md).

The evaluation benchmarks autonomous coding agents across four conditions in a 2×2 factorial design:
- **(a) Baseline:** No Principal Architect (PA) workspace/skill, No repo-memory
- **(b) PA:** PA workspace & skill active, No repo-memory
- **(c) repo-memory push:** No PA, repo-memory active via push hooks
- **(d) Both:** Both PA workspace and repo-memory active

Because agents execute terminal and file operations autonomously, rigorous local protection ensures that evaluation runs are strictly isolated, unable to touch remote repositories or GitHub issues/PRs, and fully auditable.

---

## 2. Threat Model & Why Protection is Needed

On the evaluation host machine (macOS):
1. **GitHub CLI (`gh`) is fully authenticated:** The developer environment has `gh` logged in via the macOS system keychain with elevated OAuth scopes (`repo`, `workflow`, `delete_repo`).
2. **Git HTTPS pushes use `osxkeychain`:** Xcode and macOS system git configuration (`/Library/Developer/CommandLineTools/.../gitconfig`) register `credential.helper = osxkeychain` at the system level. Any `git push` over HTTPS to GitHub would silently succeed without prompting.
3. **Installed Skills and Rules Encourage GitHub Activity:** Project guidelines (such as `todo` and `bug-fixer`) encourage filing issues or opening pull requests.
4. **GitHub Branch Protection is Unavailable:** The target repositories are private and on GitHub Free plans where branch protection rulesets are not available via API ("Upgrade to GitHub Pro"). Branch protection would also not prevent creating new issues, commenting on existing PRs, or opening and merging independent PRs.

Therefore, protection is **enforced locally on disk**, covering every repository copy and command invocation.

---

## 3. Multi-Layer Protection Architecture

Protection is built across four complementary layers:

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Structural Isolation (Zero Remotes & Squashed History)  │
│    • Fresh git init, single commit baseline, no origin URL   │
├─────────────────────────────────────────────────────────────┤
│ 2. Environment Shielding (`eval_env.sh`)                    │
│    • GH_TOKEN=invalid (blocks all gh CLI write/read calls)   │
│    • GIT_CONFIG_NOSYSTEM=1 (drops osxkeychain credential helper)│
│    • GIT_TERMINAL_PROMPT=0 (fail-closed, no hanging prompt) │
│    • unset SSH_AUTH_SOCK (detaches ssh-agent)                │
├─────────────────────────────────────────────────────────────┤
│ 3. Explicit Agent Prompt Instruction                        │
│    • "Work locally. Do not push, open PRs, or comment..."   │
├─────────────────────────────────────────────────────────────┤
│ 4. Post-Run Contamination Audit (`check_contamination.sh`)  │
│    • Automated scan of full transcripts for leaks/writes    │
└─────────────────────────────────────────────────────────────┘
```

### Layer 1: Structural Isolation (No Remotes)
Each evaluation copy is generated from a template (`tpl-a`, `tpl-b`, `tpl-c`, `tpl-d`) having a squashed, single-commit history on `main` tagged as `baseline`.
- No remote (`origin`) is configured.
- Even if an agent blindly runs `git push`, Git fails immediately with `fatal: No configured push destination`.

### Layer 2: Environment Shielding (`eval_env.sh`)
The file `evaluation/eval-v2/scripts/eval_env.sh` is sourced before launching any run or verification check:
- `export GH_TOKEN=invalid`: Overrides keychain credentials for `gh`. All GitHub CLI queries (`gh pr create`, `gh issue create`, `gh api`) fail immediately with HTTP 401 Unauthorized.
- `export GIT_CONFIG_NOSYSTEM=1`: Instructs Git to ignore system-level configurations (`/etc/gitconfig` and Xcode's system config). This strips `credential.helper = osxkeychain`. Even if an agent sets up a remote pointing to GitHub, Git cannot read credentials from the keychain.
- `export GIT_TERMINAL_PROMPT=0`: Prevents Git from prompting interactively for credentials on the terminal, failing closed instantly with exit code 128.
- `unset SSH_AUTH_SOCK`: Disconnects from any active SSH agent.

### Layer 3: Task Prompt Safety Instruction
Every task prompt passed to the agent automatically appends the clear negative constraint:
> *"Work locally. Do not push, open PRs, or comment on GitHub."*

### Layer 4: Post-Run Transcript Audit (`check_contamination.sh`)
After a run completes, `check_contamination.sh` inspects the conversation transcript log:
- Flag any references to the real development checkout (`~/Documents/Projects/esp32-app-foundation` or `/Users/mcneillm/Documents/Projects/esp32-app-foundation`).
- Flag any `git push` command execution.
- Flag any `gh` write subcommands (`gh pr create`, `gh issue create`, `gh api -X POST`, etc.).
- Flag any cross-run directory access (`/runs/<other-id>`).
Any detected violation marks the run as invalid (`invalid_contamination`).

---

## 4. Scope and Limitations (Threat Model Boundary)

### What Protection Covers
- All standard coding agent invocations, autonomous tool calls (`run_command`), and script runs.
- Accidental pushes or branch updates.
- Accidental issue/PR creation or GitHub comments triggered by rules or skills.
- Accidental modifications to the main `esp32-app-foundation` checkout or other evaluation runs.

### What Protection Does NOT Cover
- **Deliberately Adversarial Agents:** The local protection layer is designed for non-adversarial evaluation benchmarking of coding models. It does not provide kernel-level or hypervisor-level sandboxing. An intentionally adversarial agent with shell execution privileges could theoretically inspect bash history, read unencrypted private keys from `~/.ssh/`, or invoke macOS `security` CLI tools directly.
- **Root / Multi-User Isolation:** If adversarial model testing is required in future work, runs must be hosted in an unprivileged dedicated macOS user account, a virtual machine, or an isolated container.

---

## 5. Procedure to Disable & Restore Background Daemons

During batch evaluation runs, background daemons must be temporarily disabled to prevent background interference, queue activity, or accidental webhook triggers.

### Daemons to Disable
1. **Repo-Memory Tick Sidecar:** Background scheduler that runs periodic maintenance (`tick`).
2. **BugFixer Daemon Plugin:** Autonomous bug-fixer loop that monitors GitHub and local issues.

### Step-by-Step Disable Procedure (Before Batch Runs)

#### Step 1: Disable Repo-Memory Tick Sidecar
Edit `~/.gemini/config/plugins/repo-memory-daemon/config.json`:
```bash
# Set enabled to false
cat > ~/.gemini/config/plugins/repo-memory-daemon/config.json <<'EOF'
{
  "enabled": false
}
EOF
```
Verify the sidecar config file is updated:
```bash
cat ~/.gemini/config/plugins/repo-memory-daemon/config.json
# Output: {"enabled": false}
```

#### Step 2: Disable BugFixer Daemon Plugin
Using Antigravity's Language Server RPC:
```bash
: "${ANTIGRAVITY_LS_ADDRESS:=localhost:5387}"
curl -sS -X POST \
  "http://${ANTIGRAVITY_LS_ADDRESS}/exa.language_server_pb.LanguageServerService/JetboxWriteState" \
  -H "Content-Type: application/json" \
  -H "x-codeium-csrf-token: ${ANTIGRAVITY_CSRF_TOKEN}" \
  -d '{"userConfig":{"plugins":{"bug-fixer-daemon":{"enabled":false}}}}'
```
*Alternatively:* In the Antigravity UI, navigate to **Plugins & Integrations**, locate **BugFixer Daemon**, and toggle it **Off**.

---

### Step-by-Step Restore Procedure (After Batch Runs)

#### Step 1: Re-enable Repo-Memory Tick Sidecar
```bash
cat > ~/.gemini/config/plugins/repo-memory-daemon/config.json <<'EOF'
{
  "enabled": true
}
EOF
```

#### Step 2: Re-enable BugFixer Daemon Plugin
```bash
: "${ANTIGRAVITY_LS_ADDRESS:=localhost:5387}"
curl -sS -X POST \
  "http://${ANTIGRAVITY_LS_ADDRESS}/exa.language_server_pb.LanguageServerService/JetboxWriteState" \
  -H "Content-Type: application/json" \
  -H "x-codeium-csrf-token: ${ANTIGRAVITY_CSRF_TOKEN}" \
  -d '{"userConfig":{"plugins":{"bug-fixer-daemon":{"enabled":true}}}}'
```
*Alternatively:* In the Antigravity UI, toggle **BugFixer Daemon** back **On**.

---

## 6. Verification Procedures

### 1. Pre-Run Protection Verification
Run the automated verification script:
```bash
./evaluation/eval-v2/scripts/verify_protection.sh
```

**Expected Output:**
```
======================================================================
           Evaluation Protection Verification Check
======================================================================
Sourcing protection environment from: .../eval_env.sh

[Check 1/2] Testing GitHub CLI isolation (gh api user)...
  ✓ gh api user failed as expected (exit code: 1)
    Output: { "message": "Bad credentials", ...

[Check 2/2] Testing Git push isolation (git push)...
  ✓ git push failed as expected (exit code: 128)
    Output: fatal: could not read Username for 'https://github.com': terminal prompts disabled

======================================================================
                     Verification Summary
======================================================================
  1. GitHub CLI API check:      PASS (Blocked)
  2. Git push HTTPS check:      PASS (Blocked)
----------------------------------------------------------------------
  Overall Result: PASS (All protection mechanisms verified)
======================================================================
```

### 2. Contamination Verification
To verify that transcript scanning correctly catches potential violations:
```bash
# Scan a completed run transcript
./evaluation/eval-v2/scripts/check_contamination.sh --run-id <run-id> /path/to/transcript.jsonl
```
If clean, it returns exit code `0`. If any checkout path leak, `git push`, or `gh` mutation is found, it reports the exact line numbers and exits with code `1`.
