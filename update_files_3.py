import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 3 | `wo3-claim-schema` | `pr-04-entities-claims/wo3-claim-schema` | ☐ Pending',
                    '| 3 | `wo3-claim-schema` | `pr-04-entities-claims/wo3-claim-schema` | ☑ Complete')

proj += "- **WO3**: Claim schema, ULID generation, and YAML+Markdown parser/writer implemented.\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO3/7 — In Progress', 'WO4/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
