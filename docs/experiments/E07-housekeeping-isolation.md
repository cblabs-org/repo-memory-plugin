# E07: Housekeeping Run Isolation

> **Experiment ID:** E07
> **Date:** 2026-10-01
> **Status:** Complete
> **Platform:** macOS (Apple Silicon), `agy` CLI v-current

## Question

Are housekeeping runs (triggered via `agy -p`) isolated from the user's normal conversation history? Specifically:
1. Does `--project` place the conversation in a separate project namespace?
2. Can `--continue` from the repo directory resume the housekeeping conversation?
3. Does the housekeeping conversation appear in the CLI and/or IDE conversation databases?
4. Does it show in the IDE's conversation history UI?

## Method

1. Ran a tagged housekeeping call: `agy --model "Gemini 3.8 Flash (Low)" --project repo-memory-housekeeping-test --output-format json -p="Say hello and list your conversation ID"`
2. Tested `--continue` to see which conversation it resumes
3. Tested `--conversation <id>` to explicitly resume the housekeeping conversation
4. Inspected both `conversation_summaries.db` stores (IDE and CLI)

## Raw Evidence

### Step 1: Initial housekeeping call
```json
{
  "conversation_id": "8c84513a-5fe1-44bb-a525-d283d71b9ad5",
  "status": "SUCCESS",
  "response": "Hello! \n\nYour conversation ID is: `8c84513a-5fe1-44bb-a525-d283d71b9ad5`\n",
  "duration_seconds": 3.776657,
  "num_turns": 1,
  "usage": {
    "input_tokens": 16814,
    "output_tokens": 44
  }
}
```

### Step 2: `--continue` test
After running a separate "Return the word HELLO" conversation (ID `2a444abc`), tested `--continue`:
```json
{
  "conversation_id": "2a444abc-b74d-4427-beae-1b01a908b654",
  "status": "SUCCESS",
  "response": "Your previous message was:\n\n> Return the word HELLO\n",
  "num_turns": 2
}
```
**Result:** `--continue` resumed the **most recent conversation** (`2a444abc`), NOT the housekeeping one (`8c84513a`). This is by design — `--continue` picks up the last conversation in the workspace, not the last `--project`-tagged one.

### Step 3: Explicit resume by conversation ID
```json
{
  "conversation_id": "8c84513a-5fe1-44bb-a525-d283d71b9ad5",
  "status": "SUCCESS",
  "response": "I am in the **repo-memory-plugin** project...",
  "num_turns": 2,
  "usage": {
    "input_tokens": 33959,
    "output_tokens": 337
  }
}
```
**Result:** The housekeeping conversation CAN be resumed by explicit ID. It reports being in the repo-memory-plugin workspace, not a separate project.

### Step 4: Database inspection

**Database locations:**
- IDE: `~/.gemini/antigravity/conversation_summaries.db` (2,324 conversations)
- CLI: `~/.gemini/antigravity-cli/conversation_summaries.db` (156 conversations)

**Housekeeping conversation in IDE DB:**
```
(no results)
```

**Housekeeping conversation in CLI DB:**
```
8c84513a-5fe1-44bb-a525-d283d71b9ad5|Conversation ID Request|default-cli-project|
```

**Key observation:** The `--project repo-memory-housekeeping-test` flag was **silently ignored**. The conversation was stored with `project_id = "default-cli-project"`, the same as all other CLI conversations.

**All recent CLI conversations:**
```
76616016|Reply With Number One|default-cli-project
c9567f52|Output The Word Hello|default-cli-project
84dfc1da|Simple Hello Request|default-cli-project
ea5456f7|Output The Word Hello|default-cli-project
8c84513a|Conversation ID Request|default-cli-project  ← housekeeping
c1c5c364|Requesting The Word Hello|default-cli-project
2a444abc|Request To Say Hello|default-cli-project
```

**Brain data locations:**
- IDE brain: `~/.gemini/antigravity/brain/` (not found for CLI conversations)
- CLI brain: `~/.gemini/antigravity-cli/brain/` (conversation data stored here)

### Step 5: IDE visibility
The housekeeping conversation does NOT appear in the IDE conversation database. CLI and IDE maintain completely separate conversation stores and brain directories.

## Answer

1. **`--project` does NOT create a separate namespace.** The flag is silently ignored — all CLI conversations are stored with `project_id = "default-cli-project"` regardless of the `--project` value provided.

2. **`--continue` does NOT resume housekeeping conversations.** It resumes the most recent conversation in the workspace, which is determined by recency across ALL `default-cli-project` conversations, not by `--project` tag.

3. **CLI and IDE have fully separate databases.** CLI conversations go to `~/.gemini/antigravity-cli/conversation_summaries.db`, IDE conversations to `~/.gemini/antigravity/conversation_summaries.db`. No cross-contamination.

4. **CLI conversations do NOT appear in the IDE UI.** The housekeeping conversation is invisible to the IDE user.

5. **Conversations CAN be resumed by explicit ID.** Using `--conversation <id>` can resume any CLI conversation, including housekeeping ones. This means isolation depends on not knowing the conversation ID, not on true namespace separation.

### Isolation Assessment

| Isolation Vector | Isolated? | Mechanism |
|---|---|---|
| IDE ↔ CLI stores | ✅ Yes | Separate databases and brain directories |
| CLI ↔ CLI conversations | ❌ No | All share `default-cli-project` |
| `--project` namespace | ❌ No | Flag is silently ignored |
| `--continue` crossover | ⚠️ Partial | Resumes most recent, but could accidentally resume a housekeeping conversation if it was the last one run |
| Explicit ID resume | ❌ No | Any conversation can be resumed by ID |

## SPEC Impact

- **SPEC §Housekeeping**: The `--project` flag cannot be used for isolation. Housekeeping calls are mixed with user CLI conversations in `default-cli-project`. Need an alternative isolation strategy.
- **Design decision needed**: Options for housekeeping isolation:
  1. Use a non-obvious conversation ID pattern and rely on `--continue` not picking it up (fragile)
  2. Use a dedicated agent name (via `--agent`) to tag housekeeping conversations
  3. Accept that CLI-level isolation is sufficient (housekeeping won't appear in IDE)
  4. File a feature request for proper `--project` namespace support
- **SPEC §Cost**: The housekeeping call consumed 16,814 input tokens for a trivial prompt. This is the fixed system prompt overhead. If housekeeping runs every 5 minutes, that's ~4.8M tokens/day just for system prompt overhead.
- **Bug/Gap found**: `--project` flag is silently ignored for CLI conversations. Should either work or produce a warning.
