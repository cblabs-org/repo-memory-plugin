# Issue 268 Conversation Reuse Failure — Root Cause Analysis

## Timeline

| Time (UTC) | Conv ID | Role | Notes |
|---|---|---|---|
| Sep 28 19:46 | ??? (original) | Developer | **Created PR #268** — did NOT embed `agy_dev_conv_id` tag |
| Oct 01 11:46 | `c6c2691b` | Developer (iteration) | 143 steps — **spawned fresh** because no tag in PR body |
| Oct 01 12:02 | `1139731e` | Architect (review) | 94 steps — spawned fresh, wrote `agy_arch_conv_id` tag ✅ |
| Oct 01 12:30 | `eaf20f2c` | Developer (iteration) | 99 steps — **spawned fresh AGAIN** because still no tag |

## Root Cause

**The original developer conversation that created PR #268 (Sep 28) did not write the `<!-- agy_dev_conv_id: ... -->` HTML comment into the PR body.** This is the root of the cascade:

```
Original dev conv → creates PR → MISSING agy_dev_conv_id tag
                                         ↓
Architect reviews → needs-iteration → dispatcher looks for tag → NOT FOUND
                                         ↓
Dispatcher falls back to new-conversation (c6c2691b) → full context re-gathered
                                         ↓
c6c2691b pushes fix → PR body STILL has no dev tag (it was an iteration conv, not PR creator)
                                         ↓
Next architect cycle → reviews again → needs-iteration → tag still missing
                                         ↓
Another new-conversation (eaf20f2c) → full context re-gathered AGAIN
```

> [!CAUTION]
> Each fallback `new-conversation` starts from scratch — reading the SKILL.md, config, issue, PR diff, codebase files. For PR #268, this happened **3 times** instead of once.

## Why the Tag Was Missing

The `agy_dev_conv_id` tag instruction is at [SKILL.md line 357](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/skills/bug-fixer-engineering/SKILL.md#L357):

```
The agent MUST include the following HTML comment at the very end of the body:
`<!-- agy_dev_conv_id: <this_conversation_id> -->`
```

This is a **soft instruction to an LLM** — it can be missed, especially when:
1. The PR body is long and complex (the agent may compose it without remembering the tag)
2. The agent uses `gh pr create --body` with a multi-line body and omits the tag
3. The agent edits the PR body later (e.g. adding test results) and loses/overwrites the tag

**The architect tag works** because the architect SKILL.md has a stronger instruction *and* the architect conversation reliably wrote `<!-- agy_arch_conv_id: 1139731e-43e2-426f-9603-5358d2fd2e95 -->` at the end. But the original developer didn't.

## Additional Bugs Found

### BUG 1: `NameError` on `transcript` (engineering_dispatcher.py:222)

```python
# Line 222 — `transcript` is NEVER defined in this scope
if transcript.exists():
    reattach_lines.append(
        f"Previous agent transcript: {transcript}\n"
        ...
```

Should be:
```python
transcript = transcript_path(conv_id)  # from liveness.py
if transcript.exists():
```

### BUG 2: Missing `transcript_path` import (engineering_dispatcher.py:25)

```python
from liveness import (record_dispatch, sweep_dispatches,
                      transcript_age_seconds, is_stalled)
#                     ^^^^^^^^^^^^^^^^ transcript_path is NOT imported
```

These two bugs mean the **orphan reattach flow always crashes** with a `NameError`, falling through to the `except` handler which reclaims the issue for a fresh retry — losing the transcript context.

### GAP: Iteration convs don't update the dev tag

When [engineering_dispatcher.py:376-402](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L376-L402) spawns a fresh developer conversation for PR iteration (because the tag was missing), **that new conversation also won't add the tag** because:
1. The fallback prompt doesn't mention tagging the PR
2. Even if the agent adds it, it's adding *its own* conv ID — which won't survive the next iteration if the process repeats

## Recommended Fixes

### Fix 1: Dispatcher-side tag injection (robust, no LLM dependency)

Instead of relying on the LLM to remember to write the tag, have the **dispatcher** inject/update it after spawning:

```python
# After dispatching a developer conversation (both fresh and iteration):
def inject_dev_conv_tag(pr_number: int, conv_id: str):
    """Ensure PR body has current agy_dev_conv_id tag."""
    r = gh("pr", "view", str(pr_number), "--repo", TARGET_REPO,
           "--json", "body", "--jq", ".body")
    body = r.stdout.strip()
    
    # Remove any existing tag
    body = re.sub(r'<!-- agy_dev_conv_id:.*?-->\s*', '', body)
    
    # Append new tag
    body += f"\n<!-- agy_dev_conv_id: {conv_id} -->"
    
    gh("pr", "edit", str(pr_number), "--repo", TARGET_REPO,
       "--body", body)
```

> [!IMPORTANT]
> This should be called in BOTH the fresh-fix flow (after PR creation) AND the iteration flow (after dispatch). The dispatcher knows the conversation ID — it doesn't need to trust the LLM to write it.

### Fix 2: Fix the `transcript` NameError

```python
# engineering_dispatcher.py line 25 — add transcript_path to imports
from liveness import (record_dispatch, sweep_dispatches,
                      transcript_age_seconds, is_stalled,
                      transcript_path)

# engineering_dispatcher.py line 222 — use the function
transcript = transcript_path(conv_id)
if transcript.exists():
```

### Fix 3: Iteration fallback prompt should include tag instruction

When the fallback `fresh_prompt` is built at [line 378-389](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L378-L389), add the tag instruction:

```python
fresh_prompt = (f"...\n\n"
                f"IMPORTANT: After pushing, update the PR body to include this HTML comment "
                f"at the very end: <!-- agy_dev_conv_id: {{your_conversation_id}} -->")
```

### Fix 4 (Stretch): Dispatcher-side tag for architect too

Same pattern — have `architect_dispatcher.py` inject `agy_arch_conv_id` into the PR body after dispatch, rather than relying on the LLM.

## Impact

- **3 conversations** spawned for issue 268 when **1 developer + 1 architect** should have sufficed
- Each fresh conversation re-reads: SKILL.md (~385 lines), config, issue body, PR diff, codebase files
- Estimated wasted context: **~500K+ tokens** across the redundant conversations
- Fix 1 (dispatcher-side injection) eliminates the LLM-reliability dependency entirely
