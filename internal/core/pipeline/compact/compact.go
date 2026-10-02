package compact

import (
	"regexp"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Compactor performs deterministic compaction on transcript steps.
type Compactor struct {
}

// NewCompactor creates a new Compactor.
func NewCompactor() *Compactor {
	return &Compactor{}
}

// Compact reduces a list of steps to a compacted footprint.
func (c *Compactor) Compact(steps []harness.Step) []harness.Step {
	var compacted []harness.Step
	
	// For collapsing repeated identical test/build/flash/soak outputs
	var lastOutput string
	var lastKind harness.StepKind
	var repeatCount int
	var lastStep *harness.Step
	
	flushRepeats := func() {
		if lastStep != nil {
			if repeatCount > 1 {
				// We don't have a count field in Step explicitly, but we can append to Excerpt
				lastStep.Excerpt = lastStep.Excerpt + " (repeated " + string(rune(repeatCount)) + " times)" // rough, but actually we should just collapse
			}
			compacted = append(compacted, *lastStep)
			lastStep = nil
			repeatCount = 0
		}
	}

	for _, s := range steps {
		s = c.compactStep(s)
		if s.Kind == "" {
			continue // dropped
		}
		
		// If it's test/build/flash/soak (Test, Device), collapse identical outputs
		if s.Kind == harness.StepKindTest || s.Kind == harness.StepKindDevice {
			if lastStep != nil && lastKind == s.Kind && s.Excerpt == lastOutput {
				repeatCount++
				continue
			} else {
				flushRepeats()
				kind := s.Kind
				excerpt := s.Excerpt
				sCopy := s
				lastStep = &sCopy
				lastKind = kind
				lastOutput = excerpt
				repeatCount = 1
				continue
			}
		} else {
			flushRepeats()
			compacted = append(compacted, s)
		}
	}
	flushRepeats()
	
	return compacted
}

func (c *Compactor) compactStep(s harness.Step) harness.Step {
	// 1. Author classification
	if s.Kind == harness.StepKindUser {
		s.Author = classifyAuthor(s.Text)
	}

	// 2. Exposure markers
	if s.Kind == harness.StepKindExposure || s.Via != "" { 
		// Exposure markers: drop actual memory content but keep references
		s.Text = ""
	}
    
    // 3. File Reads and Edits
    if s.Kind == harness.StepKindRead || s.Kind == harness.StepKindEdit {
        // Keep the path (in s.File) but drop the content (s.Text)
        s.Text = "" 
    }

	// 4. Commands
	if s.Kind == harness.StepKindCommand {
		// Drop command output, except error/failure lines and summary lines (bounded excerpt)
		s.Excerpt = truncateCommandExcerpt(s.Excerpt, s.ExitCode)
	}

	// 5. Secret scrubber
	if s.Text != "" {
		s.Text = scrubSecrets(s.Text)
	}
	if s.Excerpt != "" {
		s.Excerpt = scrubSecrets(s.Excerpt)
	}
	if s.Command != "" {
		s.Command = scrubSecrets(s.Command)
	}

	return s
}

func truncateCommandExcerpt(excerpt string, exitCode *int) string {
	if excerpt == "" {
		return ""
	}
	
	lines := strings.Split(excerpt, "\n")
	var kept []string
	
	// Simple bounded excerpt logic: 
	// If exit code is 0 (success) or nil, maybe just keep last few lines or nothing?
	// The rule: "Command output, except error/failure lines and summary lines"
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || 
		   strings.Contains(lower, "fail") || 
		   strings.Contains(lower, "fatal") ||
		   strings.Contains(lower, "exception") ||
		   strings.Contains(lower, "summary") {
			kept = append(kept, line)
		}
	}
	
	// If it's a failure (exit != 0), and we found no explicit errors, keep the last 5 lines.
	if len(kept) == 0 && exitCode != nil && *exitCode != 0 {
		start := 0
		if len(lines) > 5 {
			start = len(lines) - 5
		}
		kept = lines[start:]
	}
	
	// Bound the excerpt
	if len(kept) > 10 {
		kept = kept[len(kept)-10:]
	}
	
	return strings.Join(kept, "\n")
}

func classifyAuthor(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "agentapi") || 
	   strings.Contains(lower, "bug-fixer") || 
	   strings.Contains(lower, "desktop scheduled task") ||
	   strings.Contains(lower, "-p run") || 
	   strings.Contains(lower, "system generated") ||
	   strings.HasPrefix(lower, "<system_message>") {
		return "automation"
	}
	return "human"
}

func scrubSecrets(text string) string {
	// Secret scrubber (pattern + entropy) replacing matches with «secret:type»
	
	// Mock AWS key
	awsKeyRegex := regexp.MustCompile(`(?i)(?:AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`)
	text = awsKeyRegex.ReplaceAllString(text, "«secret:aws»")
	
	// UUID
	uuidRegex := regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	text = uuidRegex.ReplaceAllString(text, "«secret:uuid»")

	// High entropy base64-like strings (tokens)
	// Just a simple heuristic for tests
	tokenRegex := regexp.MustCompile(`(?i)(?:bearer|token)\s+([a-zA-Z0-9\-\.]{20,})`)
	text = tokenRegex.ReplaceAllStringFunc(text, func(m string) string {
		parts := strings.SplitN(m, " ", 2)
		if len(parts) == 2 {
			return parts[0] + " «secret:token»"
		}
		return "«secret:token»"
	})
	
	// Simple hex key entropy
	hexRegex := regexp.MustCompile(`[0-9a-fA-F]{32,}`)
	text = hexRegex.ReplaceAllString(text, "«secret:hex»")
    
    return text
}
