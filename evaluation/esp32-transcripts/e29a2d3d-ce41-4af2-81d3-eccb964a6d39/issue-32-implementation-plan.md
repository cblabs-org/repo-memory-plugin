# Issue #32 — Implementation Plan

## Pre-flight

- **Branch:** `feature/32-consumer-hook-api`
- **Base:** `main`
- **Files changed:** 5 modified + 2 new
- **Estimated size:** ~200 lines new code + ~50 lines doc additions

---

## Step 1: Create `scripts/bugfixer_api.py`

> The core deliverable — the Consumer Hook API module.

**New file: [`scripts/bugfixer_api.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/bugfixer_api.py)**

```python
#!/usr/bin/env python3
"""Consumer Hook API for bug-fixer-loop plugin.

Provides a stable, namespaced interface for consumer PreToolUse hook scripts
to query bug-fixer session context. Abstracts consumers from the plugin's
internal state files (dispatch tracking, claims, config layout).

Usage in consumer hooks:
    from bugfixer_api import BugFixer

    payload = json.loads(sys.stdin.read())
    ctx = BugFixer.get_session_context(payload.get("conversationId", ""))

    if ctx.is_autonomous:
        print(json.dumps({"decision": "deny", "reason": "..."}))
    else:
        print(json.dumps({"decision": "force_ask"}))
"""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class SessionContext:
    """Structured context for a bug-fixer session.

    All fields have safe defaults — if the session is not autonomous
    or the plugin state can't be found, consumers get a safe "unknown
    interactive session" context.
    """

    is_autonomous: bool = False
    """True if this conversation was spawned by a bug-fixer dispatcher."""

    session_type: str | None = None
    """Dispatch type: 'fresh_fix', 'reattach', 'pr_iteration',
    'architect_review', 'triage', 'trajectory', or None."""

    issue_number: int | None = None
    """GitHub issue number this session is working on, if applicable."""

    pr_number: int | None = None
    """GitHub PR number this session is working on, if applicable."""

    worktree_path: str | None = None
    """Path to the git worktree this session is operating in."""

    target_repo: str | None = None
    """GitHub owner/repo this plugin instance targets."""

    config: dict = field(default_factory=dict)
    """Full plugin config.json contents (read-only snapshot)."""


# ---------------------------------------------------------------------------
# Internal discovery
# ---------------------------------------------------------------------------

_WORKSPACE_NAME = "bug-fixer-loop-workspace"


def _find_workspace_dir() -> Path | None:
    """Discover the bug-fixer workspace directory.

    Search order:
    1. BUG_FIXER_CONFIG env var → parent dir
    2. Derive from this script's own __file__ location (standard plugin layout)
    3. Relative from cwd (hook cwd is .agents/)
    """
    # 1. Env var
    config_env = os.environ.get("BUG_FIXER_CONFIG", "")
    if config_env:
        p = Path(config_env).resolve()
        if p.is_file():
            return p.parent
        if p.is_dir():
            return p

    # 2. Script location: scripts/ → plugin dir → sibling workspace
    script_dir = Path(__file__).resolve().parent
    plugin_dir = script_dir.parent
    workspace = plugin_dir.parent / _WORKSPACE_NAME
    if workspace.is_dir():
        return workspace

    # 3. Hook cwd fallback (.agents/)
    cwd_candidate = Path.cwd() / "plugins" / _WORKSPACE_NAME
    if cwd_candidate.is_dir():
        return cwd_candidate

    return None


def _load_config(workspace_dir: Path) -> dict:
    """Load config.json from the workspace directory."""
    config_file = workspace_dir / "config.json"
    if not config_file.is_file():
        return {}
    try:
        with open(config_file, "r", encoding="utf-8") as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError):
        return {}


def _find_dispatch(workspace_dir: Path, conversation_id: str) -> dict | None:
    """Scan dispatches/ for a matching conversation_id."""
    if not conversation_id:
        return None

    dispatches_dir = workspace_dir / "dispatches"
    if not dispatches_dir.is_dir():
        return None

    # Fast path: dispatch files are named {conv_id}.json
    direct = dispatches_dir / f"{conversation_id}.json"
    if direct.is_file():
        try:
            return json.loads(direct.read_text())
        except (json.JSONDecodeError, OSError):
            pass

    # Slow path: scan all files (handles edge cases)
    for f in dispatches_dir.glob("*.json"):
        try:
            data = json.loads(f.read_text())
            if data.get("conversation_id") == conversation_id:
                return data
        except (json.JSONDecodeError, OSError):
            continue

    return None


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

_DEFAULT_CONTEXT = SessionContext()


class BugFixer:
    """Consumer Hook API for the bug-fixer-loop plugin.

    All methods are static — no instantiation required.

        from bugfixer_api import BugFixer

        ctx = BugFixer.get_session_context(conversation_id)
        if ctx.is_autonomous:
            ...
    """

    @staticmethod
    def get_session_context(conversation_id: str) -> SessionContext:
        """Look up session context for a conversation ID.

        Scans dispatch tracking files to determine if this conversation
        was spawned by a bug-fixer dispatcher, and returns the full
        session context.

        Returns a SessionContext with safe defaults (is_autonomous=False)
        if the conversation is not found or plugin state can't be located.
        """
        if not conversation_id:
            return _DEFAULT_CONTEXT

        workspace = _find_workspace_dir()
        if workspace is None:
            return _DEFAULT_CONTEXT

        dispatch = _find_dispatch(workspace, conversation_id)
        config = _load_config(workspace)

        if dispatch is None:
            return SessionContext(
                is_autonomous=False,
                target_repo=config.get("target_repo"),
                config=config,
            )

        return SessionContext(
            is_autonomous=True,
            session_type=dispatch.get("type"),
            issue_number=dispatch.get("issue_number"),
            pr_number=dispatch.get("pr_number"),
            worktree_path=dispatch.get("worktree_path"),
            target_repo=config.get("target_repo"),
            config=config,
        )

    @staticmethod
    def is_autonomous(conversation_id: str) -> bool:
        """Convenience shorthand for the most common check.

        Equivalent to: BugFixer.get_session_context(conv_id).is_autonomous
        """
        return BugFixer.get_session_context(conversation_id).is_autonomous

    @staticmethod
    def get_config() -> dict:
        """Return the plugin's config.json as a dict.

        Useful for consumers that need config values (protected_files,
        target_repo, etc.) without caring about a specific session.
        """
        workspace = _find_workspace_dir()
        if workspace is None:
            return {}
        return _load_config(workspace)
```

---

## Step 2: Add `record_dispatch()` to triage dispatcher

**File:** [`scripts/triage_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/triage_dispatcher.py)

### 2a. Add import (line 22)

After the existing `from liveness import agentapi_is_reachable`, add `record_dispatch`:

```python
from liveness import agentapi_is_reachable, record_dispatch
```

### 2b. Add DISPATCHES_DIR (after line 87)

After `TRIAGE_COOLDOWN_SECONDS = 3600`:

```python
DISPATCHES_DIR = _TRIAGE_STATE_DIR / "dispatches"
```

### 2c. Add record_dispatch call (after line 170)

After `log(f"Spawned triage conversation {conv_id}")`, add:

```python
    record_dispatch(DISPATCHES_DIR, conv_id, "triage", timeout_minutes=30)
```

---

## Step 3: Add `record_dispatch()` to trajectory dispatcher

**File:** [`scripts/trajectory_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/trajectory_dispatcher.py)

### 3a. Add import (line 22)

After `from liveness import agentapi_is_reachable`, add `record_dispatch`:

```python
from liveness import agentapi_is_reachable, record_dispatch
```

### 3b. Add DISPATCHES_DIR (after line 77, near WORKSPACE_TMP_DIR)

```python
DISPATCHES_DIR = CONFIG_FILE.parent / "dispatches"
```

### 3c. Add record_dispatch call (after line 204)

After `log(f"Spawned trajectory review conversation {conv_id}")`, add:

```python
    record_dispatch(DISPATCHES_DIR, conv_id, "trajectory", timeout_minutes=30)
```

---

## Step 4: Write tests

**New file: `tests/test_bugfixer_api.py`**

Test cases:

| Test | Validates |
|------|-----------|
| `test_no_conversation_id_returns_default` | Empty string → `is_autonomous=False` |
| `test_unknown_conversation_returns_not_autonomous` | Random UUID → `is_autonomous=False`, but `config` and `target_repo` populated |
| `test_matching_dispatch_returns_autonomous` | Write a dispatch file, look it up → full `SessionContext` |
| `test_different_session_types` | Verify `fresh_fix`, `reattach`, `pr_iteration`, `architect_review`, `triage`, `trajectory` |
| `test_convenience_is_autonomous` | `BugFixer.is_autonomous()` agrees with `get_session_context().is_autonomous` |
| `test_get_config_returns_config` | `BugFixer.get_config()` returns parsed config.json |
| `test_missing_workspace_returns_defaults` | No workspace dir → safe defaults, no crash |
| `test_corrupt_dispatch_file_skipped` | Malformed JSON → gracefully returns not autonomous |
| `test_discovery_via_env_var` | `BUG_FIXER_CONFIG` env var → correct workspace discovery |
| `test_discovery_via_script_location` | `__file__`-based discovery → correct workspace |

Test pattern (matching existing conventions from [`test_pre_tool_gate.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/tests/test_pre_tool_gate.py)):

```python
class TestBugFixerAPI(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.workspace = Path(self.tmp) / "bug-fixer-loop-workspace"
        self.workspace.mkdir()
        self.dispatches = self.workspace / "dispatches"
        self.dispatches.mkdir()
        # Write a test config
        (self.workspace / "config.json").write_text(json.dumps({
            "target_repo": "owner/test-repo",
            "protected_files": ["docs/spec.md"],
        }))

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def _write_dispatch(self, conv_id, dispatch_type, **kwargs):
        data = {"conversation_id": conv_id, "type": dispatch_type, **kwargs}
        (self.dispatches / f"{conv_id}.json").write_text(json.dumps(data))

    def test_matching_dispatch_returns_autonomous(self):
        self._write_dispatch("abc-123", "fresh_fix", issue_number=42)
        with patch("bugfixer_api._find_workspace_dir", return_value=self.workspace):
            ctx = BugFixer.get_session_context("abc-123")
        self.assertTrue(ctx.is_autonomous)
        self.assertEqual(ctx.session_type, "fresh_fix")
        self.assertEqual(ctx.issue_number, 42)
        self.assertEqual(ctx.target_repo, "owner/test-repo")
```

---

## Step 5: Documentation

### 5a. [`docs/plugin-architecture.md`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/docs/plugin-architecture.md)

Add a new section **"Consumer Hook API"** after the existing "Hooks" section (~L292). Content:

- What it is (stable API for consumer PreToolUse hooks)
- Import and usage example
- `SessionContext` fields table
- Discovery mechanism
- How to set up PYTHONPATH for consumer hooks

### 5b. [`README.md`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/README.md)

Add a subsection under an appropriate heading (e.g., after "Security & Boundary Defence" or as a new "Consumer Hook Integration" section). Content:

- One-paragraph description
- Quick usage example
- Link to full docs in plugin-architecture.md

---

## Verification Checklist

- [ ] `python -m pytest tests/test_bugfixer_api.py` — all pass
- [ ] `python -m pytest tests/` — no regressions
- [ ] `python -c "from scripts.bugfixer_api import BugFixer; print(BugFixer.get_config())"` — works from repo root
- [ ] Manual test: create a fake dispatch file, call `BugFixer.get_session_context()`, verify `is_autonomous=True`
- [ ] `test_import_hygiene.py` — passes (new module is importable)
- [ ] Triage dispatcher logs `record_dispatch` in sidecar output
- [ ] Trajectory dispatcher logs `record_dispatch` in sidecar output

---

## Files Summary

```
scripts/bugfixer_api.py              (NEW — ~170 lines)
scripts/triage_dispatcher.py         (MODIFY — 3 small additions)
scripts/trajectory_dispatcher.py     (MODIFY — 3 small additions)
tests/test_bugfixer_api.py           (NEW — ~120 lines)
docs/plugin-architecture.md          (MODIFY — new section ~40 lines)
README.md                            (MODIFY — new subsection ~15 lines)
```

**Zero changes to [`engineering_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py), [`architect_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py), [`liveness.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/liveness.py), or [`pre_tool_gate.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/pre_tool_gate.py).**
