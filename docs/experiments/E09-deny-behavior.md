# E09: `deny` + `reason` Behavior

## Question
1. When a `PreToolUse` hook returns `{"decision": "deny", "reason": "..."}`, does the model see the denial and reason?
2. Does the model re-issue the same call, try a different approach, or abandon?
3. What effect does the **wording** of the reason have on model behavior?
4. In a **batch of parallel tool calls**, does denying one disrupt the others?
5. Does the `overwrite` decision work as a non-blocking advisory?
6. What happens when a hook **command fails** (exits non-zero)?

## Method

### Test 1: Basic Deny with Re-Issue Encouragement
1. Created a `PreToolUse` hook script (`testdata/e9-deny-hook.sh`) that returns:
   ```json
   {"decision": "deny", "reason": "This action was previously tried and failed because the flash partition was misaligned (clm-test-001). If your approach differs, re-issue the call."}
   ```
2. Installed via `.agents/hooks.json` with `"matcher": "run_command"`.
3. Opened session `1d574b9a-a225-45bd-a1d9-5f63b94f8b5f`.
4. Asked: *"Run `ls -la` in the current directory"*

**Result**: The model saw the denial and reason. It re-issued 3 times with different approaches:
- Attempt 1: `ls -la` → denied
- Attempt 2: `/bin/ls -la` (absolute path — model interpreted "different approach" literally) → denied
- Attempt 3: `command ls -la` (shell builtin prefix) → denied
- Gave up after 3 attempts, concluded "bug or misconfiguration" and offered to help fix the hook.

### Test 2: Parallel Tool Calls (Selective Deny)
1. Modified the hook to selectively deny: only denies `write_to_file` calls where the target filename contains "denied"; allows all others.
2. Installed via `.agents/hooks.json` with `"matcher": "write_to_file"`.
3. Opened session `f5378730-9894-482e-8f25-b15319efde35`.
4. Asked: *"Create 3 files at once: `/tmp/e9-allowed-one.txt`, `/tmp/e9-denied-two.txt`, `/tmp/e9-allowed-three.txt`"*

**Result**: The model issued 3 parallel `write_to_file` calls (step 1). Results:
- Step 2: `e9-allowed-one.txt` → ✅ Created (allowed)
- Step 3: `e9-denied-two.txt` → ❌ Denied (hook returned deny)
- Step 4: `e9-allowed-three.txt` → ✅ Created (allowed)

The model produced a clean summary table (2 successes, 1 failure) and offered to retry the denied file. **Denial of one call did NOT disrupt the others.**

### Test 3: Policy-Sounding Wording
1. Changed the deny reason to: *"This action is not permitted because it violates the repository safety policy. File writes to the /tmp directory are restricted during testing."*
2. Opened session `579ab124-8d36-4124-988c-b1e97afd2853`.
3. Asked: *"Run `ls -la` in the current directory"*

**Result**: The model retried twice (fewer than the 3 retries with "re-issue if different"):
- Attempt 1: `ls -la` → denied
- Attempt 2: `ls -la /full/path` → denied
- Pivoted to `view_file` (a completely different tool) instead of retrying `run_command`.

### Test 4: `overwrite` Decision
1. Changed the hook to return:
   ```json
   {"decision": "allow", "reason": "Command modified for safety.", "overwrite": {"CommandLine": "echo 'HOOK-REWRITTEN: The original command was intercepted.'"}}
   ```
2. Observed in the **live experiment session** (`41ec4be3`) when hooks.json was hot-reloaded.

**Result**: The overwrite worked:
- The original command did NOT execute.
- The rewritten command (`echo 'HOOK-REWRITTEN...'`) executed instead.
- The tool output was prefixed with: *"A pre-tool hook changed the arguments of this tool call before it ran. Changed: CommandLine."*
- The model saw both the notice and the rewritten output.

### Test 5: Hook Command Failure (Exit Non-Zero)
1. First attempt at E9 had a broken hook path (`./testdata/e9-deny-hook.sh` — resolved relative to `.agents/` not repo root).
2. The hook command failed with exit code 127 (`sh: ./testdata/e9-deny-hook.sh: No such file or directory`).
3. Observed in sessions `ab689674` (test session) and `41ec4be3` (experiment session).

**Result**:
- **Hook failures BLOCK tool execution** — the tool call does not proceed when the hook command exits non-zero.
- The error is delivered to the model as: `"tool call denied by pre-tool hook: ..."` (same format as an explicit deny).
- The error includes the stderr output from the failed command.
- The model treats a hook failure identically to an explicit deny — it sees the error and tries to work around it.

### Bonus Finding: hooks.json Hot-Reload
- Initially hypothesized that hooks.json was cached at session start.
- **Disproven**: hooks.json changes were picked up mid-session. The earlier "caching" behavior was because the script file path was wrong (resolving from `.agents/` directory).
- Confirmed: when hooks.json was modified to point to the overwrite hook, the change was picked up by the existing session without restart.

## Raw Evidence

### Test 1 — Deny with Re-Issue (session `1d574b9a`, steps 1-7)
```
step=1: tool_call: run_command "ls -la"
step=2: ERROR: "tool call denied by pre-tool hook: This action was previously tried and
        failed because the flash partition was misaligned (clm-test-001). If your approach
        differs, re-issue the call."
step=3: tool_call: run_command "/bin/ls -la"  [model tried different approach]
step=4: ERROR: same deny reason
step=5: tool_call: run_command "command ls -la"  [model tried another variant]
step=6: ERROR: same deny reason
step=7: "All `run_command` calls are being blocked by a pre-tool hook. This looks like
        a bug or misconfiguration..."  [gave up]
```

### Test 2 — Parallel Calls (session `f5378730`, steps 1-5)
```
step=1: 3 parallel write_to_file calls:
        - /tmp/e9-allowed-one.txt
        - /tmp/e9-denied-two.txt
        - /tmp/e9-allowed-three.txt
step=2: e9-allowed-one.txt → DONE (created)
step=3: e9-denied-two.txt → ERROR (denied by hook)
step=4: e9-allowed-three.txt → DONE (created)
step=5: Model summary: "Two out of three files were created successfully."
        | e9-allowed-one.txt   | ✅ Created |
        | e9-denied-two.txt    | ❌ Failed  |
        | e9-allowed-three.txt | ✅ Created |
```

### Test 3 — Policy Wording (session `579ab124`, steps 1-7)
```
step=1: tool_call: run_command "ls -la"
step=2: ERROR: "tool call denied by pre-tool hook: This action is not permitted because
        it violates the repository safety policy."
step=3: tool_call: run_command "ls -la /full/path"  [retried once]
step=4: ERROR: same deny
step=5: Pivoted to view_file instead of retrying run_command [gave up on run_command]
```

### Test 4 — Overwrite (session `41ec4be3`, live)
```
Tool output prefix: "A pre-tool hook changed the arguments of this tool call before it
ran. Changed: CommandLine."
Actual output: "HOOK-REWRITTEN: The original command was intercepted and replaced by
the safety hook."
```

### Test 5 — Hook Failure (session `ab689674`)
```
ERROR: "JSON hook "jsonhook__e9-deny-probe_PreToolUse_0_0" failed: command failed:
exit status 127, stderr: sh: ./testdata/e9-deny-hook.sh: No such file or directory"
```

## Answer

1. **Model sees deny + reason: YES.** The reason string is delivered verbatim in the error message, prefixed with *"tool call denied by pre-tool hook:"*. The model reads and reasons about the content.

2. **Re-issue vs. abandon depends on wording:**
   - *"If your approach differs, re-issue the call"* → model retried **3 times** with different command variants (absolute path, `command` prefix), interpreting "different approach" as command variation.
   - *"This action is not permitted"* (policy-sounding) → model retried **2 times**, then pivoted to a completely different tool (`view_file`). Policy language caused faster abandonment.
   - In both cases, the model eventually concluded the hook was misconfigured and gave up.

3. **Parallel call denial is isolated: YES.** Denying one tool call in a parallel batch does NOT prevent the others from executing. Each call is independently evaluated by the hook. The model receives individual success/error results for each call.

4. **`overwrite` decision works as a non-blocking rewrite:**
   - The original arguments are replaced (shallow merge) before execution.
   - A notice is prepended to the tool output: *"A pre-tool hook changed the arguments of this tool call before it ran. Changed: [key names]."*
   - The model sees both the notice and the rewritten output.
   - This is usable as a **non-blocking advisory** — the tool executes but with modified arguments.

5. **Hook failures = implicit deny:** When the hook command fails (non-zero exit), the tool call is blocked. The error message replaces the tool output. The model treats it identically to an explicit deny.

6. **hooks.json is hot-reloaded** — changes are picked up mid-session without restart.

### Wording Impact Summary

| Wording Style | Retries Before Giving Up | Model Behavior |
|:---|:---:|:---|
| *"Re-issue if your approach differs"* (advisory) | 3 | Tried command variants (absolute path, `command` prefix) |
| *"Not permitted... safety policy"* (policy) | 2 | Tried one variant, then pivoted to different tool entirely |

### Hook Path Resolution
- Hook `command` paths in `hooks.json` resolve **relative to the `hooks.json` directory**, NOT the repo root.
- For `.agents/hooks.json`, the working directory is `.agents/`.
- Use **absolute paths** to avoid ambiguity.

## SPEC Impact

| SPEC Section | Status | Notes |
|:---|:---|:---|
| `deny` + `reason` delivery | **Confirmed** | Reason string delivered verbatim to model as error |
| Deny behavior | **Model retries then gives up** | Advisory wording → more retries; policy wording → fewer |
| Parallel call isolation | **Confirmed** | Each call independently evaluated; denials are isolated |
| `overwrite` decision | **Confirmed working** | Non-blocking rewrite with notice; usable for advisory/steering |
| Hook failure behavior | **Implicit deny** | Non-zero exit blocks tool execution |
| hooks.json reload | **Hot-reloaded mid-session** | Changes picked up without restart |
| Hook path resolution | **Relative to hooks.json directory** | `.agents/hooks.json` → cwd is `.agents/` |
| Déjà-vu design | **Validated** | `deny` + reason is viable for "this was tried before" warnings; advisory wording triggers retries (good for suggesting alternatives); policy wording causes faster abandonment (good for hard blocks) |
| Overwrite for steering | **New capability** | Can rewrite tool args non-destructively; model sees the rewrite notice; usable for command sanitization or injection of safety flags |
