import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 5 | `wo5-sqlite-index` | `pr-04-entities-claims/wo5-sqlite-index` | ☐ Pending',
                    '| 5 | `wo5-sqlite-index` | `pr-04-entities-claims/wo5-sqlite-index` | ☑ Complete')

proj += "- **WO5**: SQLite index schema, WAL concurrency, write transactions, and the rebuild mechanism added. Added `reindex` CLI command.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO5/7 — In Progress', 'WO6/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
