import re

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'r') as f:
    proj = f.read()

proj = proj.replace('| 2 | `wo2-entity-extractor` | `pr-04-entities-claims/wo2-entity-extractor` | ☐ Pending',
                    '| 2 | `wo2-entity-extractor` | `pr-04-entities-claims/wo2-entity-extractor` | ☑ Complete')

proj += "- **WO2**: No-LLM entity extractor implemented successfully (file tree walk, config parsing, C++ symbol scanner, rule-based extraction).\n"

with open('.agents/skills/principal-architect-workspace/pr-04-entities-claims/PROJECT.md', 'w') as f:
    f.write(proj)

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'r') as f:
    mem = f.read()

mem = mem.replace('WO2/7 — In Progress', 'WO3/7 — In Progress')

with open('.agents/skills/principal-architect-workspace/memory/MEMORY.md', 'w') as f:
    f.write(mem)
