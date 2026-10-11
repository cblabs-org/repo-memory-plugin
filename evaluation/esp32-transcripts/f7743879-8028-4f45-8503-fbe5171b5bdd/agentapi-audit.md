# agentapi Command Audit — bug-fixer-loop-plugin

## 🔥 Root Cause of ALL Conversation Reuse Failures

The deployed dispatchers use `agentapi message --conversation-id <id>` — but **`agentapi` has no `message` command**. The correct command is `agentapi send-message <recipient_id> <content>`.

Every single `send-message` attempt in production has been failing with:
```json
{"response": {}, "error": "unknown command: message"}
```

The dispatchers treat this as "conversation expired/deleted" and silently fall back to `new-conversation`. **Conversation reuse has never worked.**

---

## Working Tree (plugin source) — All Call Sites

| File | Line | Command | Status |
|------|------|---------|--------|
| `engineering_dispatcher.py` | 294 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `engineering_dispatcher.py` | 408 | `agentapi send-message <conv_id> <prompt>` | ✅ Fixed (was `message --conversation-id`) |
| `engineering_dispatcher.py` | 437 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `engineering_dispatcher.py` | 541 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `architect_dispatcher.py` | 231 | `agentapi send-message <conv_id> <prompt>` | ✅ Fixed (was `message --conversation-id`) |
| `architect_dispatcher.py` | 243 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `triage_dispatcher.py` | 159 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `trajectory_dispatcher.py` | 193 | `agentapi new-conversation --model=... --title=... <prompt>` | ✅ Correct |
| `liveness.py` | 230 | `agentapi send-message <conv_id> <message>` | ✅ Already correct |

## Deployed (esp32-app-foundation) — All Call Sites

| File | Line | Command | Status |
|------|------|---------|--------|
| `tick_dispatcher.py` | 217 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |
| `tick_dispatcher.py` | 311 | `agentapi message --conversation-id <id> <prompt>` | ❌ **BROKEN** — wrong command + wrong args |
| `tick_dispatcher.py` | 333 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |
| `tick_dispatcher.py` | 420 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |
| `architect_dispatcher.py` | 169 | `agentapi message --conversation-id <id> <prompt>` | ❌ **BROKEN** — wrong command + wrong args |
| `architect_dispatcher.py` | 179 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |
| `triage_dispatcher.py` | 158 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |
| `trajectory_dispatcher.py` | 192 | `agentapi new-conversation --title=... <prompt>` | ⚠️ Missing `--model` |

> [!CAUTION]
> The two ❌ lines are the **only** places that attempt conversation reuse. Both are broken with the same bug. This means conversation reuse has **never worked** in any deployed version of the plugin.

## Issues Found

### Issue 1: Wrong `agentapi` command for send-message (CRITICAL)

**Affected (deployed):** `tick_dispatcher.py:311`, `architect_dispatcher.py:169`
**Affected (working tree):** Already fixed ✅

```python
# BROKEN (deployed)
run(["agentapi", "message", "--conversation-id", conv_id, prompt])

# CORRECT (working tree)
run(["agentapi", "send-message", conv_id, prompt])
```

Two errors in one line:
1. Command `message` → should be `send-message`
2. `--conversation-id <id>` → should be positional `<recipient_id>`

### Issue 2: Missing `--model` flag on `new-conversation` (MODERATE)

**Affected (deployed):** All `new-conversation` calls in all 4 deployed dispatchers
**Affected (working tree):** Already fixed ✅

Without `--model`, `agentapi` uses whatever default the system chooses. The working tree now passes the configured model from `config.json`:
```python
# DEPLOYED (no model control)
run(["agentapi", "new-conversation", f"--title={title}", prompt])

# WORKING TREE (model from config)
model = config.get("engineering_model", "pro")
run(["agentapi", "new-conversation", f"--model={model}", f"--title={title}", prompt])
```

### Issue 3: `transcript` NameError in orphan reattach (MODERATE)

**Affected:** `engineering_dispatcher.py:222` (working tree, now fixed)
**Details:** `transcript` variable used without being defined. Missing `transcript_path` import.

### Issue 4: No dispatcher-side conversation tag injection (MODERATE)

**Affected:** Both dispatchers relied entirely on LLM to write tags into PR bodies.
**Fix applied:** Both dispatchers now call `inject_conv_tag()` after every dispatch.

## agentapi Command Reference

```
agentapi new-conversation [--model=<flash_lite|flash|pro>] [--title=<title>] <prompt>
agentapi send-message [--title=<title>] <recipient_id> <content>
agentapi get-conversation-metadata <conversation_id>
```

## Deployment Checklist

After committing, re-deploy to esp32-app-foundation:

1. `cd /path/to/esp32-app-foundation`
2. Update the plugin submodule / copy latest scripts
3. Re-run `install.sh` from an active Antigravity session (captures fresh `ANTIGRAVITY_PROJECT_ID`)
4. Verify sidecars restarted: check `~/.gemini/antigravity/sidecar_data/*/logs/sidecar.log`
5. Trigger a test: add `needs-iteration` label to a PR and watch for "Resuming developer conversation" in the engineering sidecar log — it should **succeed** now instead of falling back
