import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 1 | `wo1-entity-types` | `pr-04-entities-claims/wo1-entity-types` | ☐ Pending',
                    '| 1 | `wo1-entity-types` | `pr-04-entities-claims/wo1-entity-types` | ☑ Complete')

proj += "\n- **WO1**: Entity Types complete. Entities, Relations, and Registry interface defined.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO1/7 — In Progress', 'WO2/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
