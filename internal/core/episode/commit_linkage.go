package episode

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// ErrNotGitRepository is returned when the given directory is not a git repository.
var ErrNotGitRepository = errors.New("not a git repository")

// GitRunner abstracts git command execution for commit linkage and testing.
type GitRunner interface {
	RunGit(repoRoot string, args ...string) (string, error)
}

// ExecGitRunner executes real git commands on the system.
type ExecGitRunner struct{}

// RunGit executes a git command in repoRoot.
func (e *ExecGitRunner) RunGit(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(out))
	if err != nil {
		return outputStr, fmt.Errorf("git %s: %w (output: %s)", strings.Join(args, " "), err, outputStr)
	}
	return outputStr, nil
}

// CommitLinker links commits from git log and reflog to session episodes (§6.2).
type CommitLinker struct {
	git        GitRunner
	timeMargin time.Duration
}

// LinkerOption configures a CommitLinker.
type LinkerOption func(*CommitLinker)

// WithGitRunner sets a custom GitRunner.
func WithGitRunner(runner GitRunner) LinkerOption {
	return func(l *CommitLinker) {
		if runner != nil {
			l.git = runner
		}
	}
}

// WithTimeMargin sets the grace window margin when querying git log/reflog around session start/end.
func WithTimeMargin(d time.Duration) LinkerOption {
	return func(l *CommitLinker) {
		l.timeMargin = d
	}
}

// NewCommitLinker creates a new CommitLinker with default settings.
func NewCommitLinker(opts ...LinkerOption) *CommitLinker {
	l := &CommitLinker{
		git:        &ExecGitRunner{},
		timeMargin: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// CandidateCommit represents a commit found in git log or reflog within the session window.
type CandidateCommit struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Timestamp time.Time `json:"timestamp"`
	Subject   string    `json:"subject"`
	Source    string    `json:"source"` // "log" or "reflog"
	Order     int       `json:"order"`
}

// TranscriptCommit represents a commit command detected in transcript steps.
type TranscriptCommit struct {
	StepIndex int
	Command   string
	ExitCode  *int
	Excerpt   string
	Message   string   // Extracted commit message from -m / --message
	Hashes    []string // Explicit hashes found in command output
}

var (
	gitCommitCmdRegex     = regexp.MustCompile(`(?i)(?:^|[\s;&|` + "`" + `])git(?:\.exe)?(?:\s+-[^\s]+|\s+--[^\s]+)*\s+commit\b`)
	msgQuotedRegex        = regexp.MustCompile(`(?s)(?:-m|-am|--message(?:=|\s+))\s*["']([^"']+)["']`)
	msgUnquotedRegex      = regexp.MustCompile(`--message=([^\s]+)`)
	gitCommitBracketRegex = regexp.MustCompile(`\[[^\]]*\b([0-9a-fA-F]{7,40})\b\]`)
	fullShaRegex          = regexp.MustCompile(`\b([0-9a-fA-F]{40})\b`)
)

// ExtractTranscriptCommits scans steps for git commit commands and outcomes.
func ExtractTranscriptCommits(steps []harness.Step) []TranscriptCommit {
	var results []TranscriptCommit
	for _, s := range steps {
		if s.Commit != "" {
			results = append(results, TranscriptCommit{
				StepIndex: s.Index,
				Hashes:    []string{s.Commit},
			})
		}

		cmdText := s.Command
		if cmdText == "" && s.Kind == harness.StepKindCommand {
			cmdText = s.Text
		}

		if isGitCommitCommand(cmdText) {
			tc := TranscriptCommit{
				StepIndex: s.Index,
				Command:   cmdText,
				ExitCode:  s.ExitCode,
				Excerpt:   s.Excerpt,
				Message:   extractCommitMessage(cmdText),
				Hashes:    extractHashesFromOutput(s.Excerpt),
			}
			if s.Text != "" && s.Text != cmdText {
				textHashes := extractHashesFromOutput(s.Text)
				tc.Hashes = append(tc.Hashes, textHashes...)
			}
			results = append(results, tc)
		}
	}
	return results
}

func isGitCommitCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	return gitCommitCmdRegex.MatchString(cmd)
}

func extractCommitMessage(cmd string) string {
	if m := msgQuotedRegex.FindStringSubmatch(cmd); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := msgUnquotedRegex.FindStringSubmatch(cmd); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractHashesFromOutput(text string) []string {
	if text == "" {
		return nil
	}
	var hashes []string
	seen := make(map[string]bool)

	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		if len(h) >= 7 && !seen[h] {
			seen[h] = true
			hashes = append(hashes, h)
		}
	}

	for _, m := range gitCommitBracketRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range fullShaRegex.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}

	return hashes
}

// Link links commits on the session's branch between start and end times
// confirmed against commit commands in the transcript, and records them in header.Commits.
func (l *CommitLinker) Link(repoRoot string, header *Header, steps []harness.Step) ([]string, error) {
	if header == nil {
		return nil, ErrNilEpisode
	}

	// 1. Identify branch
	branch := strings.TrimSpace(header.Branch)
	if branch == "" || branch == "HEAD" {
		if currentBranch, err := l.git.RunGit(repoRoot, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && currentBranch != "" {
			branch = strings.TrimSpace(currentBranch)
			if header.Branch == "" && branch != "HEAD" {
				header.Branch = branch
			}
		}
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")

	// 2. Determine time window
	since := header.Started
	until := header.Ended
	if until.IsZero() {
		until = time.Now().UTC()
	}

	// 3. Query git log
	logCommits, err := l.queryGitLog(repoRoot, branch, since, until)
	if err != nil {
		if isNoCommitsError(err) {
			return nil, nil
		}
		if isNotGitRepoError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotGitRepository, repoRoot)
		}
		return nil, fmt.Errorf("querying git log for branch %s: %w", branch, err)
	}

	// 4. Query git reflog
	reflogCommits, _ := l.queryGitReflog(repoRoot, branch, since, until)

	// Combine and deduplicate candidates by Hash
	var candidates []CandidateCommit
	seen := make(map[string]bool)
	for _, c := range logCommits {
		if !seen[c.Hash] {
			seen[c.Hash] = true
			candidates = append(candidates, c)
		}
	}
	for _, c := range reflogCommits {
		if !seen[c.Hash] {
			seen[c.Hash] = true
			candidates = append(candidates, c)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// 5. Extract commit commands from transcript
	transcriptCommits := ExtractTranscriptCommits(steps)
	if len(transcriptCommits) == 0 {
		// Commits were made on branch but NOT by any commit command in the transcript
		return nil, nil
	}

	// 6. Confirm candidates against transcript commit commands
	confirmed := l.confirmCommits(candidates, transcriptCommits)
	if len(confirmed) == 0 {
		return nil, nil
	}

	// 7. Sort confirmed commits chronologically by timestamp (oldest first).
	// If timestamps match (e.g. commits made within the same second), preserve chronological git order.
	sort.SliceStable(confirmed, func(i, j int) bool {
		if !confirmed[i].Timestamp.Equal(confirmed[j].Timestamp) {
			return confirmed[i].Timestamp.Before(confirmed[j].Timestamp)
		}
		return confirmed[i].Order < confirmed[j].Order
	})

	var confirmedHashes []string
	for _, c := range confirmed {
		confirmedHashes = append(confirmedHashes, c.Hash)
	}

	// 8. Record in episode header
	header.RecordCommits(confirmedHashes)

	if header.HeadEnd == "" && len(confirmedHashes) > 0 {
		header.HeadEnd = confirmedHashes[len(confirmedHashes)-1]
	}

	return confirmedHashes, nil
}

// LinkEpisode links commits for an episode, records them in header.Commits,
// and resolves edit steps to the matching commit where possible.
func (l *CommitLinker) LinkEpisode(repoRoot string, ep *Episode) error {
	if ep == nil {
		return ErrNilEpisode
	}

	confirmedHashes, err := l.Link(repoRoot, &ep.Header, ep.Steps)
	if err != nil {
		return err
	}

	if len(confirmedHashes) == 0 {
		return nil
	}

	// Resolve edit steps to commits (§5.1, §6.2)
	l.resolveEditSteps(repoRoot, ep, confirmedHashes)
	return nil
}

func (l *CommitLinker) resolveEditSteps(repoRoot string, ep *Episode, commitHashes []string) {
	// Build map of commit -> touched files
	commitFiles := make(map[string]map[string]bool)
	for _, hash := range commitHashes {
		files, err := l.getFilesForCommit(repoRoot, hash)
		if err == nil && len(files) > 0 {
			fileSet := make(map[string]bool, len(files))
			for _, f := range files {
				fileSet[f] = true
			}
			commitFiles[hash] = fileSet
		}
	}

	for i := range ep.Steps {
		step := &ep.Steps[i]
		if step.Kind == harness.StepKindEdit && step.Commit == "" && step.File != "" {
			for _, hash := range commitHashes {
				if commitFiles[hash] != nil && commitFiles[hash][step.File] {
					step.Commit = hash
					break
				}
			}
		}
	}
}

func (l *CommitLinker) getFilesForCommit(repoRoot, hash string) ([]string, error) {
	output, err := l.git.RunGit(repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", hash)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(output, "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func (l *CommitLinker) queryGitLog(repoRoot, branch string, since, until time.Time) ([]CandidateCommit, error) {
	args := []string{"log"}
	if branch != "" && branch != "HEAD" {
		args = append(args, branch)
	} else {
		args = append(args, "HEAD")
	}

	args = append(args, "--format=%H\x1f%h\x1f%cI\x1f%s")

	if !since.IsZero() {
		args = append(args, "--since="+since.Add(-l.timeMargin).Format(time.RFC3339))
	}
	if !until.IsZero() {
		args = append(args, "--until="+until.Add(l.timeMargin).Format(time.RFC3339))
	}

	output, err := l.git.RunGit(repoRoot, args...)
	if err != nil {
		return nil, err
	}

	return parseGitLogOutput(output, "log", since, until, l.timeMargin), nil
}

func (l *CommitLinker) queryGitReflog(repoRoot, branch string, since, until time.Time) ([]CandidateCommit, error) {
	targets := []string{}
	if branch != "" && branch != "HEAD" {
		targets = append(targets, branch)
	}
	targets = append(targets, "HEAD")

	var all []CandidateCommit
	seen := make(map[string]bool)

	for _, target := range targets {
		args := []string{"log", "-g", target, "--format=%H\x1f%h\x1f%cI\x1f%gs"}
		if !since.IsZero() {
			args = append(args, "--since="+since.Add(-l.timeMargin).Format(time.RFC3339))
		}
		if !until.IsZero() {
			args = append(args, "--until="+until.Add(l.timeMargin).Format(time.RFC3339))
		}

		output, err := l.git.RunGit(repoRoot, args...)
		if err != nil {
			continue
		}

		parsed := parseGitReflogOutput(output, since, until, l.timeMargin)
		for _, c := range parsed {
			if !seen[c.Hash] {
				seen[c.Hash] = true
				all = append(all, c)
			}
		}
		if len(all) > 0 {
			break
		}
	}

	return all, nil
}

func parseGitLogOutput(output, source string, since, until time.Time, margin time.Duration) []CandidateCommit {
	var candidates []CandidateCommit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) < 4 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		shortHash := strings.TrimSpace(parts[1])
		timeStr := strings.TrimSpace(parts[2])
		subject := strings.TrimSpace(parts[3])

		ts, _ := time.Parse(time.RFC3339, timeStr)
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339Nano, timeStr)
		}

		if !since.IsZero() && !ts.IsZero() && ts.Before(since.Add(-margin)) {
			continue
		}
		if !until.IsZero() && !ts.IsZero() && ts.After(until.Add(margin)) {
			continue
		}

		candidates = append(candidates, CandidateCommit{
			Hash:      hash,
			ShortHash: shortHash,
			Timestamp: ts,
			Subject:   subject,
			Source:    source,
		})
	}

	// git log returns newest first; reverse so candidates are chronological (oldest first)
	for i, j := 0, len(candidates)-1; i < j; i, j = i+1, j-1 {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}
	for i := range candidates {
		candidates[i].Order = i
	}

	return candidates
}

func parseGitReflogOutput(output string, since, until time.Time, margin time.Duration) []CandidateCommit {
	var candidates []CandidateCommit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) < 4 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		shortHash := strings.TrimSpace(parts[1])
		timeStr := strings.TrimSpace(parts[2])
		action := strings.TrimSpace(parts[3])

		if !strings.HasPrefix(strings.ToLower(action), "commit") {
			continue
		}

		subject := action
		if idx := strings.Index(action, ": "); idx != -1 {
			subject = strings.TrimSpace(action[idx+2:])
		}

		ts, _ := time.Parse(time.RFC3339, timeStr)
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339Nano, timeStr)
		}

		if !since.IsZero() && !ts.IsZero() && ts.Before(since.Add(-margin)) {
			continue
		}
		if !until.IsZero() && !ts.IsZero() && ts.After(until.Add(margin)) {
			continue
		}

		candidates = append(candidates, CandidateCommit{
			Hash:      hash,
			ShortHash: shortHash,
			Timestamp: ts,
			Subject:   subject,
			Source:    "reflog",
		})
	}

	// git reflog returns newest first; reverse so candidates are chronological (oldest first)
	for i, j := 0, len(candidates)-1; i < j; i, j = i+1, j-1 {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}
	for i := range candidates {
		candidates[i].Order = i
	}

	return candidates
}

func (l *CommitLinker) confirmCommits(candidates []CandidateCommit, transcriptCommits []TranscriptCommit) []CandidateCommit {
	var confirmed []CandidateCommit
	claimedCmds := make(map[int]bool)

	// Filter successful transcript commits
	var successfulCmds []TranscriptCommit
	for _, tc := range transcriptCommits {
		if tc.ExitCode != nil && *tc.ExitCode != 0 {
			continue
		}
		lowerExcerpt := strings.ToLower(tc.Excerpt)
		if strings.Contains(lowerExcerpt, "nothing to commit") || strings.Contains(lowerExcerpt, "fatal:") {
			continue
		}
		successfulCmds = append(successfulCmds, tc)
	}

	if len(successfulCmds) == 0 {
		return nil
	}

	for _, cand := range candidates {
		isConfirmed := false

		// 1. Direct hash match in excerpt / text / explicit commit
		for i, tc := range successfulCmds {
			for _, h := range tc.Hashes {
				if strings.EqualFold(h, cand.Hash) ||
					(len(h) >= 7 && strings.HasPrefix(strings.ToLower(cand.Hash), strings.ToLower(h))) ||
					(len(cand.ShortHash) >= 7 && strings.EqualFold(cand.ShortHash, h)) {
					isConfirmed = true
					claimedCmds[i] = true
					break
				}
			}
			if isConfirmed {
				break
			}
		}

		// 2. Commit message / subject match
		if !isConfirmed && cand.Subject != "" {
			candSubLower := strings.ToLower(cand.Subject)
			for i, tc := range successfulCmds {
				if tc.Message != "" {
					tcMsgLower := strings.ToLower(tc.Message)
					if tcMsgLower == candSubLower || strings.Contains(tcMsgLower, candSubLower) || strings.Contains(candSubLower, tcMsgLower) {
						isConfirmed = true
						claimedCmds[i] = true
						break
					}
				}
				if tc.Command != "" && strings.Contains(strings.ToLower(tc.Command), candSubLower) {
					isConfirmed = true
					claimedCmds[i] = true
					break
				}
				if tc.Excerpt != "" && strings.Contains(strings.ToLower(tc.Excerpt), candSubLower) {
					isConfirmed = true
					claimedCmds[i] = true
					break
				}
			}
		}

		// 3. Correlation with unclaimed successful commit command
		if !isConfirmed {
			for i, tc := range successfulCmds {
				if !claimedCmds[i] {
					isConfirmed = true
					claimedCmds[i] = true
					_ = tc
					break
				}
			}
		}

		if isConfirmed {
			confirmed = append(confirmed, cand)
		}
	}

	return confirmed
}

func isNoCommitsError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "does not have any commits yet") ||
		strings.Contains(s, "fatal: bad default revision 'head'") ||
		strings.Contains(s, "unknown revision or path")
}

func isNotGitRepoError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not a git repository")
}

// LinkCommits is a package-level helper that links commits for an episode.
func LinkCommits(repoRoot string, ep *Episode) error {
	return NewCommitLinker().LinkEpisode(repoRoot, ep)
}

// LinkHeaderCommits is a package-level helper that links commits for a header and steps.
func LinkHeaderCommits(repoRoot string, header *Header, steps []harness.Step) ([]string, error) {
	return NewCommitLinker().Link(repoRoot, header, steps)
}
