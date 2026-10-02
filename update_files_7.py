import re
from datetime import datetime

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 7 | `wo7-events-store` | `pr-04-entities-claims/wo7-events-store` | ☐ Pending',
                    '| 7 | `wo7-events-store` | `pr-04-entities-claims/wo7-events-store` | ☑ Complete')
proj = proj.replace('> **Status:** In Progress', '> **Status:** Complete')

proj += "- **WO7**: Events store for human decisions implemented, with replay order documented.\n"
proj += "### Project Wrap-Up\n"
proj += "The PR-04 data model layer has been fully implemented across 7 WOs. All exit criteria are met, and tests pass.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

# Remove PR-04 from Active Projects
mem = re.sub(r'\| pr-04-entities-claims \| WO7/7 — In Progress \| Entities, Claims, Index, Events \| \[PROJECT\.md\]\(\.\./pr-04-entities-claims/PROJECT\.md\) \|\n', '', mem)
# Add to Recently Completed
recent_entry = "| pr-04-entities-claims | 2026-10-02 | Entities, Claims, Index, Events | Data model layer built across 7 WOs. |\n"
# Find the start of the recently completed table
mem = mem.replace('|---|---|---|---|\n', '|---|---|---|---|\n' + recent_entry)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
