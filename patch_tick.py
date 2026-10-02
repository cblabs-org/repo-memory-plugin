with open("internal/core/tick/tick.go", "r") as f:
    code = f.read()

import re

# Add harness to imports if not there
if '"github.com/cblabs-org/repo-memory-plugin/internal/harness"' not in code:
    code = code.replace('"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"',
                        '"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"\n\t"github.com/cblabs-org/repo-memory-plugin/internal/harness"')

# Add hk to Ticker
code = code.replace("copiers     []TranscriptCopier\n}", "copiers     []TranscriptCopier\n\thk          harness.Housekeeper\n}")

# Add WithHousekeeper
with_hk = """
// WithHousekeeper registers the LLM harness for pipeline processing.
func WithHousekeeper(hk harness.Housekeeper) TickerOption {
	return func(t *Ticker) {
		t.hk = hk
	}
}
"""
code = code.replace("func NewTicker", with_hk + "\nfunc NewTicker")

with open("internal/core/tick/tick.go", "w") as f:
    f.write(code)
