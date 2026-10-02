import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 6 | `wo6-subsystem-config` | `pr-04-entities-claims/wo6-subsystem-config` | ☐ Pending',
                    '| 6 | `wo6-subsystem-config` | `pr-04-entities-claims/wo6-subsystem-config` | ☑ Complete')

proj += "- **WO6**: Subsystem configuration glob matching and unassigned file detection implemented.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO6/7 — In Progress', 'WO7/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
