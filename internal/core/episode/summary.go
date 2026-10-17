/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/episode/summary.go
 * Description: Produces deterministic, token-bounded Markdown and structured
 *              summaries of agent sessions from compacted episodes, flat JSON
 *              fixtures, or raw transcript streams.
 *
 * Exported Types / Functions:
 * - DefaultSummaryMaxTokens: Default token ceiling (400 tokens) for session summaries.
 * - SessionSummary:          Deterministic, token-bounded summary of a worker session.
 * - SummaryNoteItem:         Candidate knowledge item inside an inbox or evidence note.
 * - SummaryNote:             Stage 1 note loaded for session summarization.
 * - BuildSessionSummary():   Constructs a bounded SessionSummary from an Episode and optional notes.
 * - SummarizeSessionInput(): Resolves a session ID or transcript path and returns its SessionSummary.
 */

package episode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// DefaultSummaryMaxTokens is the default token ceiling for worker session summaries.
const DefaultSummaryMaxTokens = 400

// SessionSummary is a deterministic, token-bounded summary of a worker session.
type SessionSummary struct {
	SessionID           string   `json:"session_id"`           // Canonical session or episode identifier.
	Harness             string   `json:"harness,omitempty"`    // Agent harness that produced the session.
	Timestamp           string   `json:"timestamp,omitempty"`  // RFC3339 timestamp of session completion or start.
	FilesTouched        []string `json:"files_touched"`        // Ordered, deduplicated repository-relative file paths touched.
	NotesCaptured       []string `json:"notes_captured"`       // Formatted one-line summaries of notes linked to the session.
	FailuresEncountered []string `json:"failures_encountered"` // Formatted command or tool failures with exit codes and digests.
	CommandsRun         []string `json:"commands_run"`         // Ordered, deduplicated shell or test commands executed.
	VerificationOutcome string   `json:"verification_outcome"` // Terminal verification state ("verified", "completed (unverified)", "failed", or "incomplete").
	TokenCount          int      `json:"token_count"`          // Estimated token count of SummaryMarkdown.
	SummaryMarkdown     string   `json:"summary_markdown"`     // Token-bounded Markdown rendering of the session summary.
}

// SummaryNoteItem represents a candidate knowledge item inside an inbox note.
type SummaryNoteItem struct {
	Type    dossiers.EntryType `json:"type"`              // Dossier entry category (e.g., invariant, gotcha, decision).
	Title   string             `json:"title"`             // Concise headline of the knowledge item.
	Why     string             `json:"why,omitempty"`     // Rationale or root-cause explanation.
	Do      string             `json:"do,omitempty"`      // Actionable guidance or rule.
	Anchors []string           `json:"anchors,omitempty"` // File or symbol anchors associated with the item.
	Scope   string             `json:"scope,omitempty"`   // Repository scope path for the item.
}

// SummaryNote represents a Stage 1 note loaded for session summarization without
// creating an episode -> inbox -> delivery -> index -> episode import cycle.
type SummaryNote struct {
	ID             string            `json:"id"`                        // Stable ULID or content-hash identifier used for cross-directory deduplication.
	Source         string            `json:"source,omitempty"`          // Origin of the note (e.g., "memory_note", "distill").
	SessionID      string            `json:"session_id,omitempty"`      // Session ID associated with the note.
	ConversationID string            `json:"conversation_id,omitempty"` // Conversation UUID associated with the note.
	Kind           string            `json:"kind,omitempty"`            // Flat note kind when Items is unpopulated.
	Scope          string            `json:"scope,omitempty"`           // Default scope for the note.
	Statement      string            `json:"statement,omitempty"`       // Flat statement text when Items is unpopulated.
	Items          []SummaryNoteItem `json:"items,omitempty"`           // Structured knowledge items contained in the note.
}

// flatToolError represents a pre-extracted tool error record inside flat JSON episode fixtures.
type flatToolError struct {
	Tool         string `json:"tool"`          // Harness tool identifier (e.g. "run_command" or "edit_file") that produced the failure.
	Command      string `json:"command"`       // Command string executed by the tool.
	ExitCode     int    `json:"exit_code"`     // Non-zero process exit code.
	StderrDigest string `json:"stderr_digest"` // Short stderr excerpt describing the failure.
	Message      string `json:"message"`       // Fallback error message string.
}

// flatEpisodeJSON models both standard Episode JSON files and flat evaluation fixture JSON files.
type flatEpisodeJSON struct {
	SchemaVersion        string          `json:"schema_version"`        // Optional fixture schema version.
	ID                   string          `json:"id"`                    // Fallback episode or fixture ID.
	EpisodeID            string          `json:"episode_id"`            // Top-level ULID episode identifier in flat evaluation fixtures.
	SessionID            string          `json:"session_id"`            // Harness conversation UUID in flat evaluation fixtures.
	Harness              string          `json:"harness"`               // Agent harness adapter slug ("antigravity" or "claude-code").
	Timestamp            string          `json:"timestamp"`             // ISO-8601 session timestamp string.
	TouchedFiles         []string        `json:"touched_files"`         // Pre-extracted list of modified files.
	ToolSequence         []string        `json:"tool_sequence"`         // Pre-extracted sequence of tool commands.
	VerificationCommands []string        `json:"verification_commands"` // Pre-extracted verification commands.
	ToolErrors           []flatToolError `json:"tool_errors"`           // Pre-extracted tool failure records.
	NoteIDs              []string        `json:"note_ids"`              // Note IDs linked to the episode.
	TerminalSuccess      *bool           `json:"terminal_success"`      // Explicit terminal success override flag.
	Header               *Header         `json:"header"`                // Nested Episode Header when reading structured Episode JSON.
	Steps                []harness.Step  `json:"steps"`                 // Nested step slice when reading structured Episode JSON.
}

// BuildSessionSummary constructs a deterministic SessionSummary from an Episode and optional SummaryNotes,
// enforcing a strict token budget (defaulting to 400 tokens when maxTokens <= 0).
func BuildSessionSummary(ep *Episode, notes []*SummaryNote, maxTokens int) *SessionSummary {
	// --- Step 1: Initialize summary defaults and handle nil episode ---
	if maxTokens <= 0 {
		maxTokens = DefaultSummaryMaxTokens
	}

	summary := &SessionSummary{
		FilesTouched:        []string{},
		NotesCaptured:       []string{},
		FailuresEncountered: []string{},
		CommandsRun:         []string{},
		VerificationOutcome: "incomplete",
	}

	if ep == nil {
		summary.SummaryMarkdown = renderSummaryMarkdown(summary, 0, 0, 0, 0, 120, maxTokens)
		summary.TokenCount = dossiers.EstimateTokens(summary.SummaryMarkdown)
		return summary
	}

	// --- Step 2: Resolve session identity and timestamp metadata ---
	sessionID := strings.TrimSpace(ep.Header.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(ep.Header.EpisodeID)
	}
	if sessionID == "" {
		sessionID = "unknown"
	}
	summary.SessionID = sessionID
	summary.Harness = strings.TrimSpace(ep.Header.Harness)

	if !ep.Header.Ended.IsZero() {
		summary.Timestamp = ep.Header.Ended.UTC().Format(time.RFC3339)
	} else if !ep.Header.Started.IsZero() {
		summary.Timestamp = ep.Header.Started.UTC().Format(time.RFC3339)
	} else if ts, ok := metadataString(ep.Header.Metadata, "timestamp"); ok {
		summary.Timestamp = ts
	}

	repoRoot, _ := metadataString(ep.Header.Metadata, "repo_root")

	// --- Step 3: Collect and normalize touched file paths ---
	var filesTouched []string
	filesTouched = append(filesTouched, metadataStringSlice(ep.Header.Metadata, "touched_files")...)

	var editFiles []string
	var otherFiles []string
	for _, s := range ep.Steps {
		f := normalizeSummaryFilePath(s.File, repoRoot)
		if f == "" {
			continue
		}
		if s.Kind == harness.StepKindEdit || s.Kind == "" {
			editFiles = append(editFiles, f)
		} else {
			otherFiles = append(otherFiles, f)
		}
	}
	if len(editFiles) > 0 || len(filesTouched) > 0 {
		filesTouched = append(filesTouched, editFiles...)
	} else {
		filesTouched = append(filesTouched, otherFiles...)
	}
	summary.FilesTouched = dedupStringsPreserveOrder(filesTouched)

	// --- Step 4: Extract commands run, verification commands, and step failures ---
	var commandsRun []string
	var verificationCmds []string
	var failures []string

	commandsRun = append(commandsRun, metadataStringSlice(ep.Header.Metadata, "tool_sequence")...)
	verificationCmds = append(verificationCmds, metadataStringSlice(ep.Header.Metadata, "verification_commands")...)
	for _, cmd := range commandsRun {
		if isVerificationCommand(cmd) {
			verificationCmds = append(verificationCmds, cmd)
		}
	}
	failures = append(failures, metadataStringSlice(ep.Header.Metadata, "tool_errors_formatted")...)

	lastVerificationFailed := false
	hasStepActivity := len(ep.Steps) > 0 || len(summary.FilesTouched) > 0 || len(commandsRun) > 0

	for i := 0; i < len(ep.Steps); i++ {
		s := ep.Steps[i]
		cmd := strings.TrimSpace(s.Command)
		if cmd != "" {
			commandsRun = append(commandsRun, cmd)
			if isVerificationCommand(cmd) || s.Kind == harness.StepKindTest {
				verificationCmds = append(verificationCmds, cmd)
			}
		} else if s.Kind == harness.StepKindTest && strings.TrimSpace(s.Suite) != "" {
			suiteCmd := "test " + strings.TrimSpace(s.Suite)
			commandsRun = append(commandsRun, suiteCmd)
			verificationCmds = append(verificationCmds, suiteCmd)
		}

		stepFailed, exitCode, digest := inspectStepFailure(ep.Steps, i)
		if stepFailed {
			label := cmd
			if label == "" {
				label = strings.TrimSpace(s.Action)
			}
			if label == "" {
				label = string(s.Kind)
			}
			if label == "" {
				label = "command"
			}
			var formatted string
			if exitCode != nil {
				if digest != "" {
					formatted = fmt.Sprintf("%s (exit %d): %s", label, *exitCode, digest)
				} else {
					formatted = fmt.Sprintf("%s (exit %d)", label, *exitCode)
				}
			} else if digest != "" {
				formatted = fmt.Sprintf("%s: %s", label, digest)
			} else {
				formatted = fmt.Sprintf("%s (failed)", label)
			}
			failures = append(failures, formatted)
			if cmd != "" && (isVerificationCommand(cmd) || s.Kind == harness.StepKindTest) {
				lastVerificationFailed = true
			}
		} else if cmd != "" && (isVerificationCommand(cmd) || s.Kind == harness.StepKindTest) {
			lastVerificationFailed = false
		}
	}

	summary.CommandsRun = dedupStringsPreserveOrder(commandsRun)
	verificationCmds = dedupStringsPreserveOrder(verificationCmds)
	summary.FailuresEncountered = dedupStringsPreserveOrder(failures)

	// --- Step 5: Match and format captured notes associated with the session ---
	epNoteIDs := metadataStringSlice(ep.Header.Metadata, "note_ids")
	noteIDSet := make(map[string]bool, len(epNoteIDs))
	for _, id := range epNoteIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			noteIDSet[trimmed] = true
		}
	}

	matchedNoteIDs := make(map[string]bool)
	var notesCaptured []string
	for _, n := range notes {
		if n == nil {
			continue
		}
		nID := strings.TrimSpace(n.ID)
		convID := strings.TrimSpace(n.ConversationID)
		if convID == "" {
			convID = strings.TrimSpace(n.SessionID)
		}
		matchesSession := (convID != "" && (convID == summary.SessionID || convID == strings.TrimSpace(ep.Header.EpisodeID)))
		matchesID := (nID != "" && noteIDSet[nID])
		if !matchesSession && !matchesID {
			continue
		}
		if nID != "" {
			matchedNoteIDs[nID] = true
		}
		notesCaptured = append(notesCaptured, formatSummaryNoteLines(n)...)
	}
	for _, rawID := range epNoteIDs {
		trimmed := strings.TrimSpace(rawID)
		if trimmed != "" && !matchedNoteIDs[trimmed] {
			notesCaptured = append(notesCaptured, trimmed)
		}
	}
	summary.NotesCaptured = dedupStringsPreserveOrder(notesCaptured)

	// --- Step 6: Classify verification outcome and render token-bounded Markdown ---
	terminalSuccess := deriveTerminalSuccess(ep, hasStepActivity, len(summary.FailuresEncountered) > 0, len(verificationCmds) > 0, lastVerificationFailed)
	switch {
	case terminalSuccess && len(verificationCmds) > 0:
		summary.VerificationOutcome = "verified"
	case terminalSuccess && len(verificationCmds) == 0:
		summary.VerificationOutcome = "completed (unverified)"
	case !terminalSuccess && len(summary.FailuresEncountered) > 0:
		summary.VerificationOutcome = "failed"
	default:
		summary.VerificationOutcome = "incomplete"
	}

	summary.SummaryMarkdown = buildBoundedSummaryMarkdown(summary, maxTokens)
	summary.TokenCount = dossiers.EstimateTokens(summary.SummaryMarkdown)
	return summary
}

// deriveTerminalSuccess determines whether the episode completed without an unresolved terminal failure.
func deriveTerminalSuccess(ep *Episode, hasActivity, hasFailures, hasVerification, lastVerificationFailed bool) bool {
	// --- Step 1: Check explicit metadata override and header status ---
	if ep == nil {
		return false
	}
	if ep.Header.Metadata != nil {
		if v, ok := ep.Header.Metadata["terminal_success"].(bool); ok {
			return v
		}
	}
	status := strings.ToLower(strings.TrimSpace(ep.Header.Status))
	if status == "failed" || status == "error" || status == "incomplete" {
		return false
	}
	if !hasActivity {
		return false
	}
	if lastVerificationFailed {
		return false
	}

	// --- Step 2: Evaluate unverified failure states and trailing command outcome ---
	if hasFailures && !hasVerification && status != "completed" && status != "verified" && status != "parsed" && status != "captured" {
		return false
	}
	if hasFailures && !hasVerification {
		for i := len(ep.Steps) - 1; i >= 0; i-- {
			s := ep.Steps[i]
			if s.Kind == harness.StepKindCommand || s.Kind == harness.StepKindTest || s.Command != "" {
				failed, _, _ := inspectStepFailure(ep.Steps, i)
				if failed {
					return false
				}
				break
			}
		}
	}
	return true
}

// inspectStepFailure checks whether steps[idx] (or its paired tool-output follow-up step) failed,
// returning the failure flag, optional exit code, and a single-line error digest.
func inspectStepFailure(steps []harness.Step, idx int) (failed bool, exitCode *int, digest string) {
	// --- Step 1: Check direct exit code, test failure count, and outcome markers on the step ---
	s := steps[idx]
	if s.ExitCode != nil && *s.ExitCode != 0 {
		code := *s.ExitCode
		return true, &code, firstDigestLine(s.Excerpt, s.Text, s.Outcome)
	}
	if s.Failed > 0 {
		code := 1
		return true, &code, firstDigestLine(s.Excerpt, s.Text, s.Outcome)
	}
	outcomeLower := strings.ToLower(strings.TrimSpace(s.Outcome))
	if outcomeLower == "error" || outcomeLower == "failed" || outcomeLower == "failure" {
		return true, s.ExitCode, firstDigestLine(s.Excerpt, s.Text)
	}
	if looksLikeErrorText(s.Excerpt) {
		return true, extractExitCode(s.Excerpt), firstDigestLine(s.Excerpt)
	}

	// --- Step 2: Inspect the immediately following output step when command and output are split ---
	if s.Command != "" && s.Excerpt == "" && idx+1 < len(steps) {
		next := steps[idx+1]
		if next.Command == "" && next.Kind == harness.StepKindCommand {
			if next.ExitCode != nil && *next.ExitCode != 0 {
				code := *next.ExitCode
				return true, &code, firstDigestLine(next.Excerpt, next.Text, next.Outcome)
			}
			if looksLikeErrorText(next.Excerpt) {
				return true, extractExitCode(next.Excerpt), firstDigestLine(next.Excerpt)
			}
		}
	}
	return false, nil, ""
}

// looksLikeErrorText reports whether text contains common CLI or test runner failure markers.
func looksLikeErrorText(text string) bool {
	// --- Step 1: Trim and normalize text for case-insensitive matching ---
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)

	// --- Step 2: Check against known CLI, build, and test runner failure substrings ---
	for _, marker := range []string{
		"encountered error in tool execution",
		"exit status ",
		"exit code: ",
		"command failed",
		"build failed",
		"--- fail:",
		"fail\t",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// extractExitCode parses a non-zero integer exit code from command output text if present.
func extractExitCode(text string) *int {
	lower := strings.ToLower(text)
	for _, prefix := range []string{"exit status ", "exit code: ", "exit code "} {
		if pos := strings.Index(lower, prefix); pos >= 0 {
			rest := strings.TrimSpace(lower[pos+len(prefix):])
			var code int
			if n, err := fmt.Sscanf(rest, "%d", &code); n == 1 && err == nil && code != 0 {
				return &code
			}
		}
	}
	return nil
}

// firstDigestLine returns the first non-empty, non-timestamp line across candidates, truncated to 140 chars.
func firstDigestLine(candidates ...string) string {
	for _, c := range candidates {
		for _, line := range strings.Split(strings.TrimSpace(c), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "created at:") || strings.HasPrefix(strings.ToLower(trimmed), "completed at:") {
				continue
			}
			if len(trimmed) > 140 {
				trimmed = trimmed[:137] + "..."
			}
			return trimmed
		}
	}
	return ""
}

// isVerificationCommand reports whether cmd invokes a recognized build, test, or lint tool.
func isVerificationCommand(cmd string) bool {
	// --- Step 1: Normalize command string ---
	lower := strings.ToLower(strings.TrimSpace(cmd))
	if lower == "" {
		return false
	}

	// --- Step 2: Match against canonical test, build, and lint command prefixes ---
	for _, token := range []string{
		"go test",
		"go vet",
		"go build",
		"pytest",
		"npm test",
		"pnpm test",
		"yarn test",
		"cargo test",
		"cargo check",
		"make test",
		"make check",
		"jest",
		"vitest",
		"golangci-lint",
		"tsc --noemit",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// formatSummaryNoteLines formats a SummaryNote and its items into concise single-line bullet strings.
func formatSummaryNoteLines(n *SummaryNote) []string {
	// --- Step 1: Handle nil notes or flat single-statement notes ---
	if n == nil {
		return nil
	}
	id := strings.TrimSpace(n.ID)
	if id == "" {
		id = "note"
	}
	if len(n.Items) == 0 {
		if n.Kind != "" || n.Statement != "" {
			kind := strings.TrimSpace(n.Kind)
			if kind == "" {
				kind = "note"
			}
			scope := strings.TrimSpace(n.Scope)
			if scope == "" {
				scope = "repo"
			}
			return []string{fmt.Sprintf("%s [%s] (%s): %s", id, kind, scope, firstSingleLine(n.Statement))}
		}
		return []string{id}
	}

	// --- Step 2: Format each structured note item with kind, scope, and headline ---
	var out []string
	for _, item := range n.Items {
		kind := strings.TrimSpace(string(item.Type))
		if kind == "" {
			kind = strings.TrimSpace(n.Kind)
		}
		if kind == "" {
			kind = strings.TrimSpace(n.Source)
		}
		if kind == "" {
			kind = "note"
		}
		scope := strings.TrimSpace(item.Scope)
		if scope == "" {
			scope = strings.TrimSpace(n.Scope)
		}
		if scope == "" && len(item.Anchors) > 0 {
			scope = strings.TrimSpace(item.Anchors[0])
		}
		if scope == "" {
			scope = "repo"
		}
		stmt := firstSingleLine(item.Title)
		if stmt == "" {
			stmt = firstSingleLine(item.Why)
		}
		if stmt == "" {
			stmt = firstSingleLine(item.Do)
		}
		out = append(out, fmt.Sprintf("%s [%s] (%s): %s", id, kind, scope, stmt))
	}
	return out
}

// firstSingleLine returns the first non-empty trimmed line of s.
func firstSingleLine(s string) string {
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// buildBoundedSummaryMarkdown progressively tightens item and character limits until
// the rendered Markdown fits within maxTokens.
func buildBoundedSummaryMarkdown(s *SessionSummary, maxTokens int) string {
	// --- Step 1: Define progressive truncation tiers ---
	type budgetStep struct {
		maxFiles    int // Maximum number of touched files to list before summarizing remainder.
		maxNotes    int // Maximum number of captured notes to list.
		maxFailures int // Maximum number of failure items to list.
		maxCmds     int // Maximum number of commands to list.
		maxChars    int // Maximum character length per bullet item.
	}
	steps := []budgetStep{
		{15, 10, 8, 12, 140},
		{10, 8, 6, 8, 110},
		{6, 5, 4, 5, 90},
		{4, 3, 3, 3, 70},
		{2, 2, 2, 2, 50},
		{1, 1, 1, 1, 36},
		{0, 0, 0, 0, 24},
	}

	// --- Step 2: Render at each tier until token estimate is within budget ---
	var rendered string
	for _, st := range steps {
		rendered = renderSummaryMarkdown(s, st.maxFiles, st.maxNotes, st.maxFailures, st.maxCmds, st.maxChars, maxTokens)
		if dossiers.EstimateTokens(rendered) <= maxTokens {
			return rendered
		}
	}

	// --- Step 3: Enforce hard byte ceiling if even the minimal tier exceeds maxTokens ---
	maxBytes := maxTokens * 4
	if maxBytes > 0 && len(rendered) > maxBytes {
		rendered = strings.TrimSpace(rendered[:maxBytes])
	}
	return rendered
}

// renderSummaryMarkdown renders s as Markdown using the specified per-section item and character limits.
func renderSummaryMarkdown(s *SessionSummary, maxFiles, maxNotes, maxFailures, maxCmds, maxItemChars, maxTokens int) string {
	// --- Step 1: Write header metadata block ---
	var b strings.Builder
	sid := s.SessionID
	if sid == "" {
		sid = "unknown"
	}
	b.WriteString(fmt.Sprintf("## Session Summary: %s\n", sid))
	b.WriteString(fmt.Sprintf("- **Outcome:** %s\n", s.VerificationOutcome))
	if s.Harness != "" {
		b.WriteString(fmt.Sprintf("- **Harness:** %s\n", s.Harness))
	}
	if s.Timestamp != "" {
		b.WriteString(fmt.Sprintf("- **Timestamp:** %s\n", s.Timestamp))
	}

	// --- Step 2: Define bounded section writer helper ---
	writeSection := func(title string, items []string, limit int) {
		b.WriteString(fmt.Sprintf("\n### %s\n", title))
		if len(items) == 0 {
			b.WriteString("- (none)\n")
			return
		}
		if limit <= 0 {
			b.WriteString(fmt.Sprintf("- ... (+%d more)\n", len(items)))
			return
		}
		count := len(items)
		if count > limit {
			count = limit
		}
		for i := 0; i < count; i++ {
			item := items[i]
			if maxItemChars > 10 && len(item) > maxItemChars {
				item = item[:maxItemChars-3] + "..."
			}
			b.WriteString(fmt.Sprintf("- %s\n", item))
		}
		if len(items) > count {
			b.WriteString(fmt.Sprintf("- ... (+%d more)\n", len(items)-count))
		}
	}

	// --- Step 3: Render each summary section ---
	writeSection("Files Touched", s.FilesTouched, maxFiles)
	writeSection("Notes Captured", s.NotesCaptured, maxNotes)
	writeSection("Failures Encountered", s.FailuresEncountered, maxFailures)
	writeSection("Commands Run", s.CommandsRun, maxCmds)

	out := strings.TrimSpace(b.String()) + "\n"
	if maxTokens > 0 && dossiers.EstimateTokens(out) > maxTokens {
		return strings.TrimSpace(out)
	}
	return out
}

// SummarizeSessionInput resolves target as either a stored episode ID, an episode JSON/.jsonl.zst file,
// or a raw transcript file/directory on disk, pairs it with matching notes, and returns a SessionSummary.
func SummarizeSessionInput(dataDir, repoID, repoRoot, target string, maxTokens int) (*SessionSummary, error) {
	// --- Step 1: Validate target and resolve episode from store or filesystem ---
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, ErrEmptySessionID
	}

	ep, err := resolveEpisodeInput(dataDir, repoID, repoRoot, target)
	if err != nil {
		return nil, err
	}
	if ep.Header.Metadata == nil {
		ep.Header.Metadata = make(map[string]any)
	}
	if repoRoot != "" {
		ep.Header.Metadata["repo_root"] = repoRoot
	}

	// --- Step 2: Load candidate notes and build bounded session summary ---
	notes := loadCandidateNotes(dataDir, repoID, repoRoot)
	return BuildSessionSummary(ep, notes, maxTokens), nil
}

// resolveEpisodeInput locates and loads an Episode from the episode store, a JSON fixture,
// or a raw transcript path.
func resolveEpisodeInput(dataDir, repoID, repoRoot, target string) (*Episode, error) {
	// --- Step 1: Probe episode stores and evidence JSON directories when dataDir and repoID are set ---
	if dataDir != "" && repoID != "" {
		stores := []*Store{
			NewStore(filepath.Join(dataDir, "repos"), repoID),
			NewStore(dataDir, repoID),
		}
		for _, st := range stores {
			if ep, err := st.Read(target); err == nil && ep != nil {
				return ep, nil
			}
		}

		cleanBase := strings.TrimSuffix(filepath.Base(target), ".json")
		jsonCandidates := []string{
			filepath.Join(dataDir, "repos", repoID, "evidence", "episodes", cleanBase+".json"),
			filepath.Join(dataDir, "repos", repoID, "episodes", cleanBase+".json"),
			filepath.Join(dataDir, repoID, "evidence", "episodes", cleanBase+".json"),
			filepath.Join(dataDir, repoID, "episodes", cleanBase+".json"),
		}
		for _, cand := range jsonCandidates {
			if ep, ok := tryReadEpisodeJSONFile(cand, repoRoot); ok {
				return ep, nil
			}
		}

		for _, st := range stores {
			if eps, err := st.ListEpisodes(); err == nil {
				for _, ep := range eps {
					if ep != nil && (ep.Header.EpisodeID == target || ep.Header.SessionID == target) {
						return ep, nil
					}
				}
			}
		}
	}

	// --- Step 2: Check whether target is a direct file path (.jsonl.zst or .json) ---
	fi, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEpisodeNotFound, target)
	}

	if !fi.IsDir() {
		if strings.HasSuffix(strings.ToLower(target), ".jsonl.zst") {
			return NewStore(dataDir, repoID).ReadFromPath(target)
		}
		if strings.HasSuffix(strings.ToLower(target), ".json") {
			if ep, ok := tryReadEpisodeJSONFile(target, repoRoot); ok {
				return ep, nil
			}
		}
	}

	// --- Step 3: Fall back to parsing target as a raw transcript file or directory ---
	return parseRawTranscriptToEpisode(target, repoID, repoRoot)
}

// tryReadEpisodeJSONFile attempts to parse path as either a structured Episode JSON file
// or a flat evaluation fixture JSON file.
func tryReadEpisodeJSONFile(path, repoRoot string) (*Episode, bool) {
	// --- Step 1: Read and unmarshal JSON file into flatEpisodeJSON ---
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var flat flatEpisodeJSON
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, false
	}
	if flat.Header != nil && (flat.Header.SessionID != "" || flat.Header.EpisodeID != "" || len(flat.Steps) > 0) {
		ep := &Episode{
			Header: *flat.Header,
			Steps:  flat.Steps,
		}
		return ep, true
	}

	// --- Step 2: Validate flat fixture identity fields ---
	sid := strings.TrimSpace(flat.SessionID)
	if sid == "" {
		sid = strings.TrimSpace(flat.EpisodeID)
	}
	if sid == "" {
		sid = strings.TrimSpace(flat.ID)
	}
	if sid == "" && flat.SchemaVersion == "" && len(flat.TouchedFiles) == 0 && len(flat.ToolSequence) == 0 {
		return nil, false
	}
	if sid == "" {
		sid = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	// --- Step 3: Populate Header.Metadata from flat fixture fields ---
	meta := make(map[string]any)
	if repoRoot != "" {
		meta["repo_root"] = repoRoot
	}
	if flat.Timestamp != "" {
		meta["timestamp"] = flat.Timestamp
	}
	if len(flat.TouchedFiles) > 0 {
		meta["touched_files"] = flat.TouchedFiles
	}
	if len(flat.ToolSequence) > 0 {
		meta["tool_sequence"] = flat.ToolSequence
	}
	if len(flat.VerificationCommands) > 0 {
		meta["verification_commands"] = flat.VerificationCommands
	}
	if len(flat.NoteIDs) > 0 {
		meta["note_ids"] = flat.NoteIDs
	}
	if flat.TerminalSuccess != nil {
		meta["terminal_success"] = *flat.TerminalSuccess
	}
	if len(flat.ToolErrors) > 0 {
		var formattedErrs []string
		for _, te := range flat.ToolErrors {
			label := strings.TrimSpace(te.Command)
			if label == "" {
				label = strings.TrimSpace(te.Tool)
			}
			if label == "" {
				label = "tool"
			}
			digest := firstDigestLine(te.StderrDigest, te.Message)
			if te.ExitCode != 0 {
				if digest != "" {
					formattedErrs = append(formattedErrs, fmt.Sprintf("%s (exit %d): %s", label, te.ExitCode, digest))
				} else {
					formattedErrs = append(formattedErrs, fmt.Sprintf("%s (exit %d)", label, te.ExitCode))
				}
			} else if digest != "" {
				formattedErrs = append(formattedErrs, fmt.Sprintf("%s: %s", label, digest))
			} else {
				formattedErrs = append(formattedErrs, fmt.Sprintf("%s (error)", label))
			}
		}
		meta["tool_errors_formatted"] = formattedErrs
	}

	var started time.Time
	if flat.Timestamp != "" {
		started, _ = parseTimeFlexible(flat.Timestamp)
	}

	return &Episode{
		Header: Header{
			EpisodeID: sid,
			SessionID: sid,
			Harness:   flat.Harness,
			Started:   started,
			Ended:     started,
			Metadata:  meta,
		},
		Steps: flat.Steps,
	}, true
}

// parseRawTranscriptToEpisode streams a raw Antigravity, Claude Code, or compacted JSONL
// transcript from pathOrDir and normalizes it into an Episode.
func parseRawTranscriptToEpisode(pathOrDir, repoID, repoRoot string) (*Episode, error) {
	// --- Step 1: Open raw transcript stream and configure 50 MB line scanner ---
	rc, sessionIDHint, err := openRawTranscriptStream(pathOrDir)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024*50)

	var (
		steps       []harness.Step
		started     time.Time
		ended       time.Time
		harnessName string
		sessionID   = sessionIDHint
		stepIdx     int
		pendingCmd  int = -1
		claudeTools     = make(map[string]int)
	)

	// --- Step 2: Scan JSONL records across compacted, Antigravity, and Claude Code schemas ---
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || string(line) == "CHECKPOINT" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		// Handle compacted episode header line if present on line 1.
		if len(steps) == 0 {
			if _, hasEpID := raw["episode_id"]; hasEpID {
				var hdr Header
				if err := json.Unmarshal(line, &hdr); err == nil && (hdr.SessionID != "" || hdr.EpisodeID != "") {
					if hdr.SessionID != "" {
						sessionID = hdr.SessionID
					} else {
						sessionID = hdr.EpisodeID
					}
					harnessName = hdr.Harness
					started = hdr.Started
					ended = hdr.Ended
					continue
				}
			}
		}

		// Handle pre-normalized harness.Step lines.
		if kindVal, ok := raw["kind"].(string); ok && (raw["cmd"] != nil || raw["file"] != nil || raw["i"] != nil || raw["text"] != nil) {
			var st harness.Step
			if err := json.Unmarshal(line, &st); err == nil && st.Kind != "" {
				st.File = normalizeSummaryFilePath(st.File, repoRoot)
				steps = append(steps, st)
				_ = kindVal
				continue
			}
		}

		// Extract session ID and timestamp bounds.
		for _, k := range []string{"sessionId", "session_id", "conversation_id"} {
			if sv, ok := raw[k].(string); ok && strings.TrimSpace(sv) != "" {
				sessionID = strings.TrimSpace(sv)
			}
		}

		for _, k := range []string{"created_at", "timestamp", "time"} {
			if tv, ok := raw[k]; ok && tv != nil {
				if t, err := parseTimeFlexible(tv); err == nil && !t.IsZero() {
					if started.IsZero() || t.Before(started) {
						started = t
					}
					if ended.IsZero() || t.After(ended) {
						ended = t
					}
				}
			}
		}

		// Handle Antigravity tool_calls array.
		if tcRaw, hasTC := raw["tool_calls"]; hasTC && tcRaw != nil {
			if harnessName == "" {
				harnessName = HarnessAntigravity
			}
			tcBytes, _ := json.Marshal(tcRaw)
			var calls []struct {
				Name string         `json:"name"` // Tool name invoked in the step.
				Args map[string]any `json:"args"` // Tool arguments map.
			}
			if err := json.Unmarshal(tcBytes, &calls); err == nil && len(calls) > 0 {
				pendingCmd = -1
				for _, call := range calls {
					stepIdx++
					st := harness.Step{Index: stepIdx}
					switch call.Name {
					case "run_command":
						st.Kind = harness.StepKindCommand
						if cmdStr, ok := stringFromMap(call.Args, "CommandLine", "command", "cmd"); ok {
							st.Command = cmdStr
						}
						steps = append(steps, st)
						pendingCmd = len(steps) - 1
					case "replace_file_content", "write_to_file", "edit_file", "write_file", "multi_edit", "notebook_edit":
						st.Kind = harness.StepKindEdit
						if fStr, ok := stringFromMap(call.Args, "TargetFile", "Path", "AbsolutePath", "file_path", "NotebookPath"); ok {
							st.File = normalizeSummaryFilePath(fStr, repoRoot)
						}
						steps = append(steps, st)
					case "view_file", "read_file":
						st.Kind = harness.StepKindRead
						if fStr, ok := stringFromMap(call.Args, "AbsolutePath", "Path", "TargetFile", "file_path"); ok {
							st.File = normalizeSummaryFilePath(fStr, repoRoot)
						}
						steps = append(steps, st)
					default:
						st.Kind = harness.StepKindAgent
						st.Action = call.Name
						steps = append(steps, st)
					}
				}
				continue
			}
		}

		// Handle Antigravity source/content steps and pair tool output with pendingCmd.
		if src, ok := raw["source"].(string); ok {
			if harnessName == "" {
				harnessName = HarnessAntigravity
			}
			content, _ := raw["content"].(string)
			status, _ := raw["status"].(string)
			stepType, _ := raw["type"].(string)
			if pendingCmd >= 0 && (stepType == "GENERIC" || stepType == "TOOL_OUTPUT" || stepType == "COMMAND_OUTPUT" || strings.Contains(strings.ToLower(src), "tool") || status == "ERROR") {
				steps[pendingCmd].Excerpt = content
				if strings.EqualFold(status, "ERROR") {
					steps[pendingCmd].Outcome = "error"
				}
				pendingCmd = -1
				continue
			}
			pendingCmd = -1
			if content != "" {
				stepIdx++
				kind := harness.StepKindAgent
				if stepType == "USER_INPUT" || src == "USER_EXPLICIT" {
					kind = harness.StepKindUser
				}
				steps = append(steps, harness.Step{
					Index: stepIdx,
					Kind:  kind,
					Text:  content,
				})
			}
			continue
		}

		// Handle Claude Code message.content tool_use and tool_result blocks.
		if msgMap, ok := raw["message"].(map[string]any); ok {
			if harnessName == "" {
				harnessName = HarnessClaudeCode
			}
			if contentArr, ok := msgMap["content"].([]any); ok {
				for _, item := range contentArr {
					itemMap, ok := item.(map[string]any)
					if !ok {
						continue
					}
					itemType, _ := itemMap["type"].(string)
					switch itemType {
					case "tool_use":
						stepIdx++
						toolName, _ := itemMap["name"].(string)
						toolID, _ := itemMap["id"].(string)
						inputMap, _ := itemMap["input"].(map[string]any)
						st := harness.Step{Index: stepIdx}
						switch strings.ToLower(toolName) {
						case "bash", "run_command":
							st.Kind = harness.StepKindCommand
							if cmdStr, ok := stringFromMap(inputMap, "command", "CommandLine", "cmd"); ok {
								st.Command = cmdStr
							}
						case "edit", "multiedit", "write", "write_file", "edit_file", "replace_file_content":
							st.Kind = harness.StepKindEdit
							if fStr, ok := stringFromMap(inputMap, "file_path", "path", "TargetFile", "Path"); ok {
								st.File = normalizeSummaryFilePath(fStr, repoRoot)
							}
						case "read", "view_file", "read_file":
							st.Kind = harness.StepKindRead
							if fStr, ok := stringFromMap(inputMap, "file_path", "path", "AbsolutePath", "Path"); ok {
								st.File = normalizeSummaryFilePath(fStr, repoRoot)
							}
						default:
							st.Kind = harness.StepKindAgent
							st.Action = toolName
						}
						steps = append(steps, st)
						if toolID != "" {
							claudeTools[toolID] = len(steps) - 1
						}
					case "tool_result":
						toolUseID, _ := itemMap["tool_use_id"].(string)
						isErr, _ := itemMap["is_error"].(bool)
						resContent := extractClaudeToolResultContent(itemMap["content"])
						if idx, found := claudeTools[toolUseID]; found && idx >= 0 && idx < len(steps) {
							steps[idx].Excerpt = resContent
							if isErr {
								code := 1
								if parsedCode := extractExitCode(resContent); parsedCode != nil {
									code = *parsedCode
								}
								steps[idx].ExitCode = &code
								steps[idx].Outcome = "error"
							}
						} else if isErr {
							stepIdx++
							code := 1
							steps = append(steps, harness.Step{
								Index:    stepIdx,
								Kind:     harness.StepKindCommand,
								ExitCode: &code,
								Outcome:  "error",
								Excerpt:  resContent,
							})
						}
					}
				}
			}
		}
	}

	// --- Step 3: Validate scanner state and assemble Episode ---
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scanning transcript %s: %w", pathOrDir, err)
	}

	if sessionID == "" {
		sessionID = "unknown"
	}
	if harnessName == "" {
		harnessName = HarnessAntigravity
	}

	return &Episode{
		Header: Header{
			EpisodeID: sessionID,
			SessionID: sessionID,
			Harness:   harnessName,
			RepoID:    repoID,
			Started:   started,
			Ended:     ended,
			Source:    pathOrDir,
			Status:    "parsed",
			StepCount: len(steps),
			Metadata: map[string]any{
				"repo_root": repoRoot,
			},
		},
		Steps: steps,
	}, nil
}

// openRawTranscriptStream opens a transcript JSONL file from pathOrDir and derives a session ID hint.
func openRawTranscriptStream(pathOrDir string) (io.ReadCloser, string, error) {
	// --- Step 1: Handle direct file paths and infer session ID from parent directory ---
	fi, err := os.Stat(pathOrDir)
	if err != nil {
		return nil, "", fmt.Errorf("stat transcript %s: %w", pathOrDir, err)
	}
	if !fi.IsDir() {
		f, err := os.Open(pathOrDir)
		if err != nil {
			return nil, "", err
		}
		base := strings.TrimSuffix(filepath.Base(pathOrDir), filepath.Ext(pathOrDir))
		if base == "transcript" || base == "transcript_full" {
			parent := filepath.Base(filepath.Dir(pathOrDir))
			if parent == "logs" {
				grand := filepath.Dir(filepath.Dir(filepath.Dir(pathOrDir)))
				if gBase := filepath.Base(grand); gBase != "." && gBase != "/" {
					base = gBase
				}
			} else if parent != "." && parent != "/" {
				base = parent
			}
		}
		return f, base, nil
	}

	// --- Step 2: Probe standard conversation directory transcript locations ---
	sessionIDHint := filepath.Base(filepath.Clean(pathOrDir))
	if sessionIDHint == "logs" {
		grand := filepath.Dir(filepath.Dir(filepath.Clean(pathOrDir)))
		if gBase := filepath.Base(grand); gBase != "." && gBase != "/" {
			sessionIDHint = gBase
		}
	}

	candidates := []string{
		filepath.Join(pathOrDir, ".system_generated", "logs", "transcript_full.jsonl"),
		filepath.Join(pathOrDir, ".system_generated", "logs", "transcript.jsonl"),
		filepath.Join(pathOrDir, "transcript_full.jsonl"),
		filepath.Join(pathOrDir, "transcript.jsonl"),
	}
	for _, c := range candidates {
		if f, err := os.Open(c); err == nil {
			return f, sessionIDHint, nil
		}
	}

	// --- Step 3: Fall back to the first sorted .jsonl file in the directory ---
	entries, err := os.ReadDir(pathOrDir)
	if err == nil {
		var jsonlFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jsonl") {
				jsonlFiles = append(jsonlFiles, filepath.Join(pathOrDir, e.Name()))
			}
		}
		sort.Strings(jsonlFiles)
		if len(jsonlFiles) > 0 {
			f, err := os.Open(jsonlFiles[0])
			if err != nil {
				return nil, "", err
			}
			return f, sessionIDHint, nil
		}
	}

	return nil, "", fmt.Errorf("no transcript jsonl file found in %s", pathOrDir)
}

// extractClaudeToolResultContent flattens string or content-block array payloads from Claude tool_result items.
func extractClaudeToolResultContent(v any) string {
	// --- Step 1: Dispatch on raw string vs structured content-block array ---
	switch val := v.(type) {
	case string:
		return val
	case []any:
		// --- Step 2: Concatenate non-empty text blocks in order ---
		var parts []string
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// loadCandidateNotes loads deduplicated SummaryNote records from evidence/notes and .memory/inbox.
func loadCandidateNotes(dataDir, repoID, repoRoot string) []*SummaryNote {
	// --- Step 1: Build candidate note directories list ---
	var dirs []string
	if dataDir != "" && repoID != "" {
		dirs = append(dirs,
			filepath.Join(dataDir, "repos", repoID, "evidence", "notes"),
			filepath.Join(dataDir, repoID, "evidence", "notes"),
		)
	}
	if repoRoot != "" {
		dirs = append(dirs, filepath.Join(repoRoot, ".memory", "inbox"))
	}

	// --- Step 2: Scan JSON files across directories and deduplicate by note ID ---
	seen := make(map[string]bool)
	var out []*SummaryNote

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			fullPath := filepath.Join(dir, entry.Name())
			n, ok := parseNoteFileFlexible(fullPath)
			if !ok || n == nil {
				continue
			}
			key := n.ID
			if key == "" {
				key = fullPath
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, n)
		}
	}
	return out
}

// parseNoteFileFlexible unmarshals a JSON note file supporting both multi-item inbox notes
// and flat single-statement evidence notes.
func parseNoteFileFlexible(path string) (*SummaryNote, bool) {
	// --- Step 1: Read and unmarshal primary SummaryNote structure ---
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var n SummaryNote
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, false
	}

	// --- Step 2: Backfill flat note fields when Items slice is empty ---
	var flat struct {
		ID             string `json:"id"`              // Fallback note identifier.
		SessionID      string `json:"session_id"`      // Fallback session identifier.
		ConversationID string `json:"conversation_id"` // Fallback conversation UUID.
		Kind           string `json:"kind"`            // Fallback entry kind.
		Type           string `json:"type"`            // Alternate entry type key.
		Scope          string `json:"scope"`           // Fallback repository scope.
		Statement      string `json:"statement"`       // Primary flat statement text.
		Title          string `json:"title"`           // Alternate headline text.
	}
	_ = json.Unmarshal(data, &flat)
	if n.ID == "" {
		n.ID = strings.TrimSpace(flat.ID)
	}
	if n.ConversationID == "" {
		if flat.ConversationID != "" {
			n.ConversationID = strings.TrimSpace(flat.ConversationID)
		} else if flat.SessionID != "" {
			n.ConversationID = strings.TrimSpace(flat.SessionID)
		}
	}
	if len(n.Items) == 0 && (flat.Kind != "" || flat.Type != "" || flat.Statement != "" || flat.Title != "") {
		kind := flat.Kind
		if kind == "" {
			kind = flat.Type
		}
		stmt := flat.Statement
		if stmt == "" {
			stmt = flat.Title
		}
		n.Items = []SummaryNoteItem{
			{
				Type:  dossiers.EntryType(kind),
				Scope: flat.Scope,
				Title: stmt,
			},
		}
	}
	return &n, true
}

// normalizeSummaryFilePath converts p to a clean slash-separated path relative to repoRoot when possible.
func normalizeSummaryFilePath(p, repoRoot string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if repoRoot != "" && filepath.IsAbs(p) {
		if rel, err := filepath.Rel(repoRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// stringFromMap returns the first non-empty string value in m matching one of keys.
func stringFromMap(m map[string]any, keys ...string) (string, bool) {
	if m == nil {
		return "", false
	}
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// metadataString extracts a non-empty string value for key from metadata map m.
func metadataString(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v), true
	}
	return "", false
}

// metadataStringSlice extracts a slice of non-empty strings for key from metadata map m.
func metadataStringSlice(m map[string]any, key string) []string {
	// --- Step 1: Guard against nil map and dispatch on slice type ---
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		// --- Step 2: Coerce interface slice elements into trimmed non-empty strings ---
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

// dedupStringsPreserveOrder removes empty and duplicate strings from items while preserving first-seen order.
func dedupStringsPreserveOrder(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}
