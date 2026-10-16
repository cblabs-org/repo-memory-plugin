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
	SessionID           string   `json:"session_id"`
	Harness             string   `json:"harness,omitempty"`
	Timestamp           string   `json:"timestamp,omitempty"`
	FilesTouched        []string `json:"files_touched"`
	NotesCaptured       []string `json:"notes_captured"`
	FailuresEncountered []string `json:"failures_encountered"`
	CommandsRun         []string `json:"commands_run"`
	VerificationOutcome string   `json:"verification_outcome"`
	TokenCount          int      `json:"token_count"`
	SummaryMarkdown     string   `json:"summary_markdown"`
}

// SummaryNoteItem represents a candidate knowledge item inside an inbox note.
type SummaryNoteItem struct {
	Type    dossiers.EntryType `json:"type"`
	Title   string             `json:"title"`
	Why     string             `json:"why,omitempty"`
	Do      string             `json:"do,omitempty"`
	Anchors []string           `json:"anchors,omitempty"`
	Scope   string             `json:"scope,omitempty"`
}

// SummaryNote represents a Stage 1 note loaded for session summarization without
// creating an episode -> inbox -> delivery -> index -> episode import cycle.
type SummaryNote struct {
	ID             string            `json:"id"`
	Source         string            `json:"source,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	ConversationID string            `json:"conversation_id,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	Scope          string            `json:"scope,omitempty"`
	Statement      string            `json:"statement,omitempty"`
	Items          []SummaryNoteItem `json:"items,omitempty"`
}

type flatToolError struct {
	Tool         string `json:"tool"`
	Command      string `json:"command"`
	ExitCode     int    `json:"exit_code"`
	StderrDigest string `json:"stderr_digest"`
	Message      string `json:"message"`
}

type flatEpisodeJSON struct {
	SchemaVersion        string          `json:"schema_version"`
	ID                   string          `json:"id"`
	EpisodeID            string          `json:"episode_id"`
	SessionID            string          `json:"session_id"`
	Harness              string          `json:"harness"`
	Timestamp            string          `json:"timestamp"`
	TouchedFiles         []string        `json:"touched_files"`
	ToolSequence         []string        `json:"tool_sequence"`
	VerificationCommands []string        `json:"verification_commands"`
	ToolErrors           []flatToolError `json:"tool_errors"`
	NoteIDs              []string        `json:"note_ids"`
	TerminalSuccess      *bool           `json:"terminal_success"`
	Header               *Header         `json:"header"`
	Steps                []harness.Step  `json:"steps"`
}

// BuildSessionSummary constructs a deterministic SessionSummary from an Episode and optional SummaryNotes,
// enforcing a strict token budget (defaulting to 400 tokens when maxTokens <= 0).
func BuildSessionSummary(ep *Episode, notes []*SummaryNote, maxTokens int) *SessionSummary {
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

	// 1. FilesTouched
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

	// 2. CommandsRun, VerificationCommands, FailuresEncountered
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

	// 3. NotesCaptured
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

	// 4. VerificationOutcome
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

	// 5. Render SummaryMarkdown within maxTokens
	summary.SummaryMarkdown = buildBoundedSummaryMarkdown(summary, maxTokens)
	summary.TokenCount = dossiers.EstimateTokens(summary.SummaryMarkdown)
	return summary
}

func deriveTerminalSuccess(ep *Episode, hasActivity, hasFailures, hasVerification, lastVerificationFailed bool) bool {
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

func inspectStepFailure(steps []harness.Step, idx int) (failed bool, exitCode *int, digest string) {
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

func looksLikeErrorText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
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

func isVerificationCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	if lower == "" {
		return false
	}
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

func formatSummaryNoteLines(n *SummaryNote) []string {
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

func firstSingleLine(s string) string {
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildBoundedSummaryMarkdown(s *SessionSummary, maxTokens int) string {
	type budgetStep struct {
		maxFiles    int
		maxNotes    int
		maxFailures int
		maxCmds     int
		maxChars    int
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

	var rendered string
	for _, st := range steps {
		rendered = renderSummaryMarkdown(s, st.maxFiles, st.maxNotes, st.maxFailures, st.maxCmds, st.maxChars, maxTokens)
		if dossiers.EstimateTokens(rendered) <= maxTokens {
			return rendered
		}
	}

	maxBytes := maxTokens * 4
	if maxBytes > 0 && len(rendered) > maxBytes {
		rendered = strings.TrimSpace(rendered[:maxBytes])
	}
	return rendered
}

func renderSummaryMarkdown(s *SessionSummary, maxFiles, maxNotes, maxFailures, maxCmds, maxItemChars, maxTokens int) string {
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

	notes := loadCandidateNotes(dataDir, repoID, repoRoot)
	return BuildSessionSummary(ep, notes, maxTokens), nil
}

func resolveEpisodeInput(dataDir, repoID, repoRoot, target string) (*Episode, error) {
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

	return parseRawTranscriptToEpisode(target, repoID, repoRoot)
}

func tryReadEpisodeJSONFile(path, repoRoot string) (*Episode, bool) {
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

func parseRawTranscriptToEpisode(pathOrDir, repoID, repoRoot string) (*Episode, error) {
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

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || string(line) == "CHECKPOINT" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

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

		if kindVal, ok := raw["kind"].(string); ok && (raw["cmd"] != nil || raw["file"] != nil || raw["i"] != nil || raw["text"] != nil) {
			var st harness.Step
			if err := json.Unmarshal(line, &st); err == nil && st.Kind != "" {
				st.File = normalizeSummaryFilePath(st.File, repoRoot)
				steps = append(steps, st)
				_ = kindVal
				continue
			}
		}

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

		if tcRaw, hasTC := raw["tool_calls"]; hasTC && tcRaw != nil {
			if harnessName == "" {
				harnessName = HarnessAntigravity
			}
			tcBytes, _ := json.Marshal(tcRaw)
			var calls []struct {
				Name string         `json:"name"`
				Args map[string]any `json:"args"`
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

func openRawTranscriptStream(pathOrDir string) (io.ReadCloser, string, error) {
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

func extractClaudeToolResultContent(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
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

func loadCandidateNotes(dataDir, repoID, repoRoot string) []*SummaryNote {
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

func parseNoteFileFlexible(path string) (*SummaryNote, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var n SummaryNote
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, false
	}
	var flat struct {
		ID             string `json:"id"`
		SessionID      string `json:"session_id"`
		ConversationID string `json:"conversation_id"`
		Kind           string `json:"kind"`
		Type           string `json:"type"`
		Scope          string `json:"scope"`
		Statement      string `json:"statement"`
		Title          string `json:"title"`
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

func metadataString(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v), true
	}
	return "", false
}

func metadataStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
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
