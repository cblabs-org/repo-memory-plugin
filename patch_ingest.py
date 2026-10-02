with open("internal/core/tick/tick.go", "r") as f:
    code = f.read()

import re

# Add Payload: string(item.Raw) to job creation
new_job = """
		job := &jobs.Job{
			Kind:           jobs.Kind(item.Entry.Kind),
			IdempotencyKey: item.Entry.ID,
			Priority:       10,
			Payload:        string(item.Raw),
		}
"""
code = re.sub(r'job := &jobs.Job\{\s*Kind:.*?Priority:\s*10,\s*\}', new_job.strip(), code, flags=re.DOTALL)

with open("internal/core/tick/tick.go", "w") as f:
    f.write(code)
