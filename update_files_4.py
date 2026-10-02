import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 4 | `wo4-claim-store` | `pr-04-entities-claims/wo4-claim-store` | ☐ Pending',
                    '| 4 | `wo4-claim-store` | `pr-04-entities-claims/wo4-claim-store` | ☑ Complete')

proj += "- **WO4**: Claim store, month-sharded CRUD operations, and inverse relation index implemented.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO4/7 — In Progress', 'WO5/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
