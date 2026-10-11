# Manual Patches — Copy & Paste into Terminal

All three files are protected from agent modification (by design). Run these commands in your terminal.

---

## 1. Fix `injection_scanner.py` — move `import random` to module level

```bash
cd ~/Documents/Projects/bug-fixer-loop-plugin

# Add `import random` at module level (after `import hashlib` on line 8)
sed -i '' '8 a\
import random
' daemon/workspace-plugin/bug-fixer/scripts/injection_scanner.py

# Remove the function-scoped `import random` (now line 83 after the insert above)
sed -i '' '83d' daemon/workspace-plugin/bug-fixer/scripts/injection_scanner.py
```

## 2. Fix `pre_tool_gate.py` — whitelist daemon trusted scripts

Two changes needed:

### 2a. Add `DAEMON_TRUSTED_SCRIPTS` constant (after line 167)

```bash
sed -i '' '167 a\
# Scripts in the daemon scripts/ dir that agents may execute (but not write).\
DAEMON_TRUSTED_SCRIPTS = frozenset({\
    "manage.py", "tick.py",\
})
' daemon/workspace-plugin/bug-fixer/scripts/pre_tool_gate.py
```

### 2b. Update `_is_prefix_protected` to allow trusted daemon scripts

After the 2a patch, the method `_is_prefix_protected` will be around **line 971** (was 963, +8 from the insert). We need to add an early-return before `return True` on what is now the last line of the if-block.

```bash
# Find the exact line number of "return True           # protected" inside _is_prefix_protected
LINE=$(grep -n 'return True           # protected' daemon/workspace-plugin/bug-fixer/scripts/pre_tool_gate.py | head -1 | cut -d: -f1)

# Insert the trusted-scripts exemption before that line
sed -i '' "${LINE}i\\
\\                # Allow execution of trusted daemon scripts (manage.py, tick.py)\\
\\                if len(parts) == 2 and parts[0] == \"scripts\" and parts[1] in DAEMON_TRUSTED_SCRIPTS:\\
\\                    return False      # allow execution
" daemon/workspace-plugin/bug-fixer/scripts/pre_tool_gate.py
```

## 3. Verify & deploy

```bash
# Run the import hygiene test that was failing
python3 -m pytest tests/test_import_hygiene.py -v

# Run the full test suite  
python3 -m pytest tests/ -v

# Re-install the plugin
./scripts/install.sh
```

## 4. Commit everything

```bash
git add -A
git commit -m "feat: add bug-fixer-manage skill, manage.py CLI, fix import hygiene

- manage.py: 8 subcommands with HITL gates for mutable operations
- SKILL.md: thin skill mapping user intent to manage.py via ask_question
- install.sh: auto-refresh sidecars on upgrade (no re-enrollment needed)
- pre_tool_gate.py: whitelist daemon trusted scripts for execution
- injection_scanner.py: move import random to module level (PLC0415)"
```

---

> [!IMPORTANT]
> After deploying, test manage.py from an agent session:
> ```
> python3 ~/.gemini/config/plugins/bug-fixer-daemon/scripts/manage.py status
> ```
> This should now be **allowed** by the pre-tool gate.
