package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestCompaction(t *testing.T) {
	compactor := NewCompactor()
	var steps []harness.Step

	// Create long noisy output
	longOutput := strings.Repeat("this is some normal command output\n", 50) +
		"Error: something failed\n" +
		strings.Repeat("more normal output\n", 50) +
		"Summary: 1 failed\n"
		
	exitCode := 1

	// Create steps
	steps = append(steps, harness.Step{
		Index: 1, Kind: harness.StepKindUser, Text: "<SYSTEM_MESSAGE> agentapi report",
	})
	steps = append(steps, harness.Step{
		Index: 2, Kind: harness.StepKindUser, Text: "Can you fix the tests?",
	})
	steps = append(steps, harness.Step{
		Index: 3, Kind: harness.StepKindCommand, Command: "run tests", Excerpt: longOutput, ExitCode: &exitCode,
	})
	steps = append(steps, harness.Step{
		Index: 4, Kind: harness.StepKindRead, File: "main.go", Text: strings.Repeat("func main() {}\n", 100),
	})
	steps = append(steps, harness.Step{
		Index: 5, Kind: harness.StepKindEdit, File: "main.go", Text: strings.Repeat("func main() { fmt.Println() }\n", 100),
	})
	steps = append(steps, harness.Step{
		Index: 6, Kind: harness.StepKindExposure, Via: "read", Text: strings.Repeat("Memory content clm-123", 50), Claims: []string{"clm-123"},
	})
	
	// device outputs that repeat
	deviceOut := "Booting...\nInit ok"
	steps = append(steps, harness.Step{
		Index: 7, Kind: harness.StepKindDevice, Excerpt: deviceOut, Action: "monitor",
	})
	steps = append(steps, harness.Step{
		Index: 8, Kind: harness.StepKindDevice, Excerpt: deviceOut, Action: "monitor",
	})
	steps = append(steps, harness.Step{
		Index: 9, Kind: harness.StepKindDevice, Excerpt: deviceOut, Action: "monitor",
	})

	// Secret scrubber check
	steps = append(steps, harness.Step{
		Index: 10, Kind: harness.StepKindAgent, Text: "Found token Bearer abcdefghijklmnopqrstuvwxyz12345 and AWS key AKIAIOSFODNN7EXAMPLE",
	})

	raw, _ := json.Marshal(steps)
	rawSize := len(raw)

	compacted := compactor.Compact(steps)
	
	comp, _ := json.Marshal(compacted)
	compSize := len(comp)

	if compSize * 4 > rawSize {
		t.Errorf("Compaction did not achieve 4x reduction. Raw: %d, Compacted: %d (%.2fx)", rawSize, compSize, float64(rawSize)/float64(compSize))
	}

	if compacted[0].Author != "automation" {
		t.Errorf("Expected step 1 author to be automation, got %s", compacted[0].Author)
	}
	if compacted[1].Author != "human" {
		t.Errorf("Expected step 2 author to be human, got %s", compacted[1].Author)
	}

	// Verify command truncation
	if len(compacted[2].Excerpt) > 100 {
		t.Errorf("Command excerpt too long after compaction: %d bytes", len(compacted[2].Excerpt))
	}

	// Verify file read/edit dropping
	if compacted[3].Text != "" || compacted[4].Text != "" {
		t.Errorf("Read/Edit steps did not drop content")
	}

	// Verify exposure marker dropping
	if compacted[5].Text != "" {
		t.Errorf("Exposure step did not drop content")
	}
	
	// Verify device repeat collapse
	// 6 + 1 (collapsed device) + 1 (secret) = 8 steps total
	if len(compacted) != 8 {
		t.Errorf("Expected 8 steps after compaction, got %d", len(compacted))
	}

	// Verify secret scrubbing
	agentText := compacted[7].Text
	if !strings.Contains(agentText, "«secret:token»") || !strings.Contains(agentText, "«secret:aws»") {
		t.Errorf("Secrets not scrubbed correctly: %s", agentText)
	}
	if strings.Contains(agentText, "AKIA") {
		t.Errorf("AWS Key was not fully scrubbed")
	}
}
