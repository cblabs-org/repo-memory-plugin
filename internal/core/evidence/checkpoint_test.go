package evidence

import (
	"testing"
)

func TestParseCheckpointSections_CanonicalNumbered(t *testing.T) {
	content := `### 1. Outstanding User Requests
- Fix capture to evidence pipeline
- Ensure unit tests pass

### 2. User Knowledge
- Principal Architect requested no external tool modifications.

### 3. Work Accomplished
- Inspected codebase and identified broken link in convertCapturesToTriage.

### 4. Model Knowledge
- Packages under internal/core must not import internal/harness/antigravity.

### 5. Files and Code
### Edited Files
- internal/core/tick/pipeline.go
### Viewed Files
- internal/core/episode/store.go

### 6. Current Work and Next Steps
- Write tests and verify compilation.

# Conversation Logs
- User prompt...
`

	sections := ParseCheckpointSections(content)
	if len(sections) == 0 {
		t.Fatal("expected parsed sections, got empty map")
	}

	expected := map[string]string{
		SectionOutstandingUserRequests: "- Fix capture to evidence pipeline\n- Ensure unit tests pass",
		SectionUserKnowledge:           "- Principal Architect requested no external tool modifications.",
		SectionWorkAccomplished:         "- Inspected codebase and identified broken link in convertCapturesToTriage.",
		SectionModelKnowledge:          "- Packages under internal/core must not import internal/harness/antigravity.",
		SectionFilesAndCode:            "### Edited Files\n- internal/core/tick/pipeline.go\n### Viewed Files\n- internal/core/episode/store.go",
		SectionCurrentWork:             "- Write tests and verify compilation.",
	}

	for k, want := range expected {
		got, ok := sections[k]
		if !ok {
			t.Errorf("missing expected section %q", k)
			continue
		}
		if got != want {
			t.Errorf("section %q:\ngot:\n%s\nwant:\n%s", k, got, want)
		}
	}
}

func TestParseCheckpointSections_SimpleHeaders(t *testing.T) {
	content := `## User Knowledge
The user prefers concise output.

## Model Knowledge
Go packages in internal/core cannot import adapters.

## Work Accomplished
Implemented pipeline.

## Current Work
Running tests.
`

	sections := ParseCheckpointSections(content)
	if sections[SectionUserKnowledge] != "The user prefers concise output." {
		t.Errorf("User Knowledge: got %q", sections[SectionUserKnowledge])
	}
	if sections[SectionModelKnowledge] != "Go packages in internal/core cannot import adapters." {
		t.Errorf("Model Knowledge: got %q", sections[SectionModelKnowledge])
	}
	if sections[SectionWorkAccomplished] != "Implemented pipeline." {
		t.Errorf("Work Accomplished: got %q", sections[SectionWorkAccomplished])
	}
	if sections[SectionCurrentWork] != "Running tests." {
		t.Errorf("Current Work: got %q", sections[SectionCurrentWork])
	}
}

func TestParseCheckpointSections_Empty(t *testing.T) {
	if sections := ParseCheckpointSections(""); len(sections) != 0 {
		t.Errorf("expected nil/empty for empty input, got %v", sections)
	}
	if sections := ParseCheckpointSections("   \n\n  "); len(sections) != 0 {
		t.Errorf("expected nil/empty for whitespace input, got %v", sections)
	}
}
