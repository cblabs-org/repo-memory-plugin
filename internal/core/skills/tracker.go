package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

var (
	inlineCodeSpanRe = regexp.MustCompile("`([^`\\n]+)`")
	stepPrefixRe     = regexp.MustCompile(`^(?:[-*+]\s+|\d+[.)]\s+|(?i:step\s+\d+\s*[:.-]\s*)|(?i:command\s*:\s*)|(?i:cmd\s*:\s*))+`)
	whitespaceRunRe  = regexp.MustCompile(`\s+`)
	compoundSplitRe  = regexp.MustCompile(`\s*(?:&&|\|\||;|\n)\s*`)
)

var commonCLIVerbs = map[string]bool{
	"go":         true,
	"git":        true,
	"pio":        true,
	"platformio": true,
	"make":       true,
	"gmake":      true,
	"cmake":      true,
	"ctest":      true,
	"ninja":      true,
	"npm":        true,
	"pnpm":       true,
	"yarn":       true,
	"npx":        true,
	"bun":        true,
	"cargo":      true,
	"rustc":      true,
	"pytest":     true,
	"python":     true,
	"python3":    true,
	"pip":        true,
	"pip3":       true,
	"uv":         true,
	"poetry":     true,
	"bash":       true,
	"sh":         true,
	"zsh":        true,
	"task":       true,
	"just":       true,
	"docker":     true,
	"kubectl":    true,
	"helm":       true,
	"terraform":  true,
	"curl":       true,
	"wget":       true,
	"idf.py":     true,
	"esptool.py": true,
	"gradle":     true,
	"mvn":        true,
	"bundle":     true,
	"rake":       true,
	"dotnet":     true,
}

// EntryOutcome tracks per-entry execution outcome history across conversations and build manifest snapshots.
type EntryOutcome struct {
	SucceededConversations         []string          `json:"succeeded_conversations,omitempty"`
	ConsecutiveFailedConversations []string          `json:"consecutive_failed_conversations,omitempty"`
	LastFailedConversation         string            `json:"last_failed_conversation,omitempty"`
	ManifestBlobSHAs               map[string]string `json:"manifest_blob_shas,omitempty"`
}

// OutcomeState persists procedural outcome tracking across invocations at <dataDir>/<repoID>/state/procedural_outcomes.json.
type OutcomeState struct {
	Entries map[string]*EntryOutcome `json:"entries"`
}

// OutcomesPath returns the file path for procedural_outcomes.json.
func OutcomesPath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "procedural_outcomes.json")
}

// LoadOutcomeState reads OutcomeState from disk, returning an initialized empty state if missing.
func LoadOutcomeState(dataDir, repoID string) (*OutcomeState, error) {
	st := &OutcomeState{
		Entries: make(map[string]*EntryOutcome),
	}
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(repoID) == "" {
		return st, nil
	}
	p := OutcomesPath(dataDir, repoID)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, fmt.Errorf("reading procedural outcomes %s: %w", p, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(data, st); err != nil {
		return &OutcomeState{Entries: make(map[string]*EntryOutcome)}, fmt.Errorf("unmarshaling procedural outcomes %s: %w", p, err)
	}
	if st.Entries == nil {
		st.Entries = make(map[string]*EntryOutcome)
	}
	return st, nil
}

// SaveOutcomeState writes OutcomeState atomically to <dataDir>/<repoID>/state/procedural_outcomes.json.
func SaveOutcomeState(dataDir, repoID string, st *OutcomeState) error {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(repoID) == "" || st == nil {
		return nil
	}
	if st.Entries == nil {
		st.Entries = make(map[string]*EntryOutcome)
	}
	p := OutcomesPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling procedural outcomes: %w", err)
	}
	data = append(data, '\n')
	tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing procedural outcomes tmp: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming procedural outcomes: %w", err)
	}
	return nil
}

func (st *OutcomeState) getOrCreate(entryID string) *EntryOutcome {
	if st.Entries == nil {
		st.Entries = make(map[string]*EntryOutcome)
	}
	eo, ok := st.Entries[entryID]
	if !ok || eo == nil {
		eo = &EntryOutcome{
			ManifestBlobSHAs: make(map[string]string),
		}
		st.Entries[entryID] = eo
	}
	if eo.ManifestBlobSHAs == nil {
		eo.ManifestBlobSHAs = make(map[string]string)
	}
	return eo
}

// NormalizeCommand trims whitespace, strips a leading "$ " prompt prefix, and collapses internal whitespace.
func NormalizeCommand(cmd string) string {
	s := strings.TrimSpace(cmd)
	for strings.HasPrefix(s, "$") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	}
	s = whitespaceRunRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func startsWithCLIVerb(cmd string) bool {
	s := NormalizeCommand(cmd)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if commonCLIVerbs[first] {
		return true
	}
	base := path.Base(first)
	if strings.Contains(first, "/") && commonCLIVerbs[base] {
		return true
	}
	return false
}

func looksLikeInlineCommand(rawSpan string) bool {
	trimmed := strings.TrimSpace(rawSpan)
	if trimmed == "" || strings.Contains(trimmed, " · ") {
		return false
	}
	if strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") {
		return true
	}
	norm := NormalizeCommand(trimmed)
	if startsWithCLIVerb(norm) {
		return true
	}
	lower := strings.ToLower(norm)
	for _, metaPrefix := range []string{"anchors:", "kind:", "evidence:", "source_doc:", "verified:", "closes_when:"} {
		if strings.HasPrefix(lower, metaPrefix) {
			return false
		}
	}
	// Multi-token span starting with a command-like identifier (e.g. "repo-memory promote-skills --dry-run").
	fields := strings.Fields(norm)
	if len(fields) >= 2 {
		first := fields[0]
		if strings.HasPrefix(first, "-") || strings.HasSuffix(first, ":") {
			return false
		}
		for _, r := range first {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '/') {
				return false
			}
		}
		return true
	}
	return false
}

// ExtractRecipeCommands extracts shell commands from fenced code blocks, inline code spans,
// or explicit command lines in a recipe/procedure entry text.
func ExtractRecipeCommands(text string) []string {
	var commands []string
	seen := make(map[string]bool)
	addCmd := func(raw string) {
		norm := NormalizeCommand(raw)
		if norm == "" || seen[norm] {
			return
		}
		seen[norm] = true
		commands = append(commands, norm)
	}

	lines := strings.Split(text, "\n")
	inFence := false

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
				continue
			}
			addCmd(trimmed)
			continue
		}

		if trimmed == "" {
			continue
		}

		// 1. Check inline code spans on this line.
		matches := inlineCodeSpanRe.FindAllStringSubmatch(rawLine, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				if looksLikeInlineCommand(m[1]) {
					addCmd(m[1])
				}
			}
			continue
		}

		// 2. Check explicit command lines ($ ... or starting with common CLI verbs).
		stripped := strings.TrimSpace(stepPrefixRe.ReplaceAllString(trimmed, ""))
		if strings.HasPrefix(stripped, "$ ") || strings.HasPrefix(stripped, "$") {
			norm := NormalizeCommand(stripped)
			if norm != "" {
				addCmd(norm)
			}
			continue
		}
		if startsWithCLIVerb(stripped) {
			addCmd(stripped)
		}
	}

	if commands == nil {
		return []string{}
	}
	return commands
}

// CommandMatchesRecipe returns true when NormalizeCommand(executedCmd) equals a normalized
// recipe command, starts with "<recipeCmd> ", or invokes <recipeCmd> as a segment of a
// compound shell command (&&, ;, ||).
func CommandMatchesRecipe(executedCmd string, recipeCmds []string) bool {
	normExec := NormalizeCommand(executedCmd)
	if normExec == "" || len(recipeCmds) == 0 {
		return false
	}

	rawSegments := compoundSplitRe.Split(executedCmd, -1)
	segments := make([]string, 0, len(rawSegments)+1)
	segments = append(segments, normExec)
	for _, seg := range rawSegments {
		n := NormalizeCommand(seg)
		if n != "" && n != normExec {
			segments = append(segments, n)
		}
	}

	matchesSingle := func(candidate, target string) bool {
		if candidate == "" || target == "" {
			return false
		}
		return candidate == target || strings.HasPrefix(candidate, target+" ")
	}

	for _, rc := range recipeCmds {
		normRC := NormalizeCommand(rc)
		if normRC == "" {
			continue
		}
		for _, seg := range segments {
			if matchesSingle(seg, normRC) {
				return true
			}
		}
		// If the recipe command itself is a compound command (e.g. "pio run && pio test"),
		// also check non-trivial sub-segments of the recipe command.
		if strings.Contains(normRC, "&&") || strings.Contains(normRC, ";") || strings.Contains(normRC, "||") {
			for _, rcSeg := range compoundSplitRe.Split(normRC, -1) {
				normRCSeg := NormalizeCommand(rcSeg)
				if normRCSeg == "" || strings.HasPrefix(normRCSeg, "cd ") || normRCSeg == "true" {
					continue
				}
				for _, seg := range segments {
					if matchesSingle(seg, normRCSeg) {
						return true
					}
				}
			}
		}
	}

	return false
}

func stripAnchorSymbol(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "#"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx > 0 {
		if !(idx == 1 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))) {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// IsBuildManifestPath returns true if relPath refers to a tracked build or environment manifest:
// Makefile, CMakeLists.txt, platformio.ini, sdkconfig*, go.mod, package.json, Taskfile.yml, Taskfile.yaml,
// or .github/workflows/*.yml / .github/workflows/*.yaml.
func IsBuildManifestPath(relPath string) bool {
	stripped := stripAnchorSymbol(strings.TrimSpace(relPath))
	if stripped == "" {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(stripped))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")

	if strings.HasPrefix(clean, ".github/workflows/") {
		lower := strings.ToLower(clean)
		if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
			return true
		}
	}

	base := path.Base(clean)
	switch {
	case base == "Makefile" || base == "makefile" || base == "GNUmakefile":
		return true
	case strings.EqualFold(base, "CMakeLists.txt"):
		return true
	case base == "platformio.ini":
		return true
	case strings.HasPrefix(base, "sdkconfig"):
		return true
	case base == "go.mod":
		return true
	case base == "package.json":
		return true
	case strings.EqualFold(base, "Taskfile.yml") || strings.EqualFold(base, "Taskfile.yaml"):
		return true
	default:
		return false
	}
}

func gitBlobSHA(ctx context.Context, repoRoot, relPath string) string {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(relPath) == "" {
		return ""
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "hash-object", "--", filepath.FromSlash(relPath))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func loadActiveProceduralEntries(ctx context.Context, repoRoot, dataDir, repoID string, idx *index.Index) ([]dossiers.Entry, map[string]bool, error) {
	inOps := make(map[string]bool)
	retiredInOps := make(map[string]bool)
	byID := make(map[string]dossiers.Entry)

	if strings.TrimSpace(repoRoot) != "" {
		opsDir := filepath.Join(repoRoot, ".memory", "ops")
		if info, err := os.Stat(opsDir); err == nil && info.IsDir() {
			projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("projecting ops: %w", err)
			}
			for _, d := range projected {
				if d == nil {
					continue
				}
				for _, h := range d.History {
					retiredInOps[h.ID] = true
				}
				for _, e := range d.Entries {
					if e.Retired {
						retiredInOps[e.ID] = true
						continue
					}
					inOps[e.ID] = true
					if e.Type == dossiers.EntryTypeRecipe || e.Type == dossiers.EntryTypeProcedure {
						byID[e.ID] = e
					}
				}
			}
		}
	}

	var openedIdx *index.Index
	targetIdx := idx
	if targetIdx == nil && strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		idxPath := index.IndexPath(dataDir, repoID)
		if _, err := os.Stat(idxPath); err == nil {
			if i, openErr := index.NewIndex(idxPath); openErr == nil {
				openedIdx = i
				targetIdx = i
			}
		}
	}
	if openedIdx != nil {
		defer openedIdx.Close()
	}

	if targetIdx != nil {
		idxEntries, err := targetIdx.ListProceduralEntries(ctx)
		if err == nil {
			for _, ie := range idxEntries {
				if retiredInOps[ie.ID] {
					continue
				}
				if ie.Type != dossiers.EntryTypeRecipe && ie.Type != dossiers.EntryTypeProcedure {
					continue
				}
				if existing, ok := byID[ie.ID]; ok {
					// Preserve Evidence from ops while taking any fresher counters from index if higher.
					if len(ie.Anchors) > 0 && len(existing.Anchors) == 0 {
						existing.Anchors = ie.Anchors
					}
					byID[ie.ID] = existing
				} else {
					byID[ie.ID] = ie
				}
			}
		}
	}

	out := make([]dossiers.Entry, 0, len(byID))
	for _, e := range byID {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, inOps, nil
}

func appendUniqueString(slice []string, val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return slice
	}
	for _, item := range slice {
		if item == val {
			return slice
		}
	}
	return append(slice, val)
}

// RecordCommandOutcome updates procedural outcome statistics and stale_suspect flags for all
// active recipe/procedure entries whose extracted commands match command.
func RecordCommandOutcome(ctx context.Context, dataDir, repoID, repoRoot, convID, command string, succeeded bool) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if NormalizeCommand(command) == "" {
		return nil, nil
	}

	entries, inOps, err := loadActiveProceduralEntries(ctx, repoRoot, dataDir, repoID, nil)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	outcomeState, _ := LoadOutcomeState(dataDir, repoID)
	now := time.Now().UTC()
	sessionToken := strings.TrimSpace(convID)
	if sessionToken == "" {
		sessionToken = fmt.Sprintf("anon-%d", now.UnixNano())
	}

	var matchedEntries []dossiers.Entry
	var matchedIDs []string
	var batchOps []ops.Op

	for _, entry := range entries {
		recipeCmds := ExtractRecipeCommands(entry.Text)
		if !CommandMatchesRecipe(command, recipeCmds) {
			continue
		}

		eo := outcomeState.getOrCreate(entry.ID)
		if succeeded {
			entry.SuccessCount++
			entry.FailureCount = 0
			entry.StaleSuspect = false
			entry.LastSucceededAt = now.Format(time.RFC3339)

			eo.SucceededConversations = appendUniqueString(eo.SucceededConversations, sessionToken)
			eo.ConsecutiveFailedConversations = nil
			eo.LastFailedConversation = ""

			for _, a := range entry.Anchors {
				relPath := stripAnchorSymbol(a)
				if IsBuildManifestPath(relPath) && strings.TrimSpace(repoRoot) != "" {
					if sha := gitBlobSHA(ctx, repoRoot, relPath); sha != "" {
						eo.ManifestBlobSHAs[relPath] = sha
					}
				}
			}

			if strings.TrimSpace(convID) != "" {
				entry.Evidence = appendUniqueString(entry.Evidence, "conv:"+strings.TrimSpace(convID))
			}
		} else {
			entry.FailureCount++
			eo.LastFailedConversation = sessionToken
			eo.ConsecutiveFailedConversations = appendUniqueString(eo.ConsecutiveFailedConversations, sessionToken)
			if len(eo.ConsecutiveFailedConversations) >= 2 {
				entry.StaleSuspect = true
			}
		}

		matchedEntries = append(matchedEntries, entry)
		matchedIDs = append(matchedIDs, entry.ID)

		opType := ops.OpAmend
		fields := map[string]any{
			"success_count":     entry.SuccessCount,
			"failure_count":     entry.FailureCount,
			"last_succeeded_at": entry.LastSucceededAt,
			"stale_suspect":     entry.StaleSuspect,
			"evidence":          entry.Evidence,
		}
		if !inOps[entry.ID] {
			// Ensure an entry seeded directly into index.db is also materialized in .memory/ops/ so OpAmend projects cleanly.
			batchOps = append(batchOps, ops.Op{
				Type:         ops.OpAdd,
				EntryID:      entry.ID,
				DossierScope: entry.DossierScope,
				Timestamp:    now,
				Source:        firstNonEmpty(entry.Source, ops.SourceTurnNote),
				SessionID:    strings.TrimSpace(convID),
				Fields: map[string]any{
					"entry_type":        string(entry.Type),
					"kind":              string(entry.EffectiveKind()),
					"text":              entry.Text,
					"anchors":           entry.Anchors,
					"evidence":          entry.Evidence,
					"verified":          entry.Verified,
					"render_as_skill":   entry.RenderAsSkill,
					"success_count":     entry.SuccessCount,
					"failure_count":     entry.FailureCount,
					"last_succeeded_at": entry.LastSucceededAt,
					"stale_suspect":     entry.StaleSuspect,
				},
			})
		}
		batchOps = append(batchOps, ops.Op{
			Type:         opType,
			EntryID:      entry.ID,
			DossierScope: entry.DossierScope,
			Timestamp:    now,
			Source:       firstNonEmpty(entry.Source, ops.SourceTurnNote),
			SessionID:    strings.TrimSpace(convID),
			Fields:       fields,
		})
	}

	if len(matchedEntries) == 0 {
		return nil, nil
	}

	if err := SaveOutcomeState(dataDir, repoID, outcomeState); err != nil {
		return matchedIDs, err
	}

	if strings.TrimSpace(repoRoot) != "" && len(batchOps) > 0 {
		opsDir := filepath.Join(repoRoot, ".memory", "ops")
		if err := os.MkdirAll(opsDir, 0o755); err == nil {
			batch := ops.NewBatchWithTime(batchOps, now)
			_ = ops.NewWriter(opsDir).Write(batch)
			if projected, projErr := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx); projErr == nil {
				_ = dossiers.NewWriter(filepath.Join(repoRoot, ".memory")).WriteAll(projected)
			}
		}
	}

	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		idxPath := index.IndexPath(dataDir, repoID)
		if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err == nil {
			if idx, idxErr := index.NewIndex(idxPath); idxErr == nil {
				for _, entry := range matchedEntries {
					_ = idx.UpsertEntry(ctx, entry)
				}
				_ = idx.Close()
			}
		}
	}

	if strings.TrimSpace(repoRoot) != "" {
		_, _ = SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, false)
	}

	return matchedIDs, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseFlexibleTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func isManifestModifiedSinceVerification(ctx context.Context, repoRoot, relPath string, entry dossiers.Entry, eo *EntryOutcome) bool {
	if strings.TrimSpace(repoRoot) == "" || !IsBuildManifestPath(relPath) {
		return false
	}
	cleanRel := filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(filepath.Clean(stripAnchorSymbol(relPath))), "./"))

	// 1. Check uncommitted changes in working tree or index (`git status --porcelain -- <path>`).
	statusCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain", "--", cleanRel)
	if out, err := statusCmd.Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "??") {
				return true
			}
		}
	}

	// 2. Compare current git blob SHA against recorded ManifestBlobSHAs[path].
	currentSHA := gitBlobSHA(ctx, repoRoot, cleanRel)
	var recordedSHA string
	if eo != nil && eo.ManifestBlobSHAs != nil {
		recordedSHA = eo.ManifestBlobSHAs[cleanRel]
		if recordedSHA == "" {
			recordedSHA = eo.ManifestBlobSHAs[relPath]
		}
	}
	if recordedSHA != "" {
		return currentSHA != "" && currentSHA != recordedSHA
	}

	// 3. No recorded ManifestBlobSHA: compare file's latest git commit timestamp against entry's LastSucceededAt (or UpdatedAt/CreatedAt).
	logCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "log", "-1", "--format=%cI", "--", cleanRel)
	out, err := logCmd.Output()
	if err != nil {
		return false
	}
	commitTime := parseFlexibleTime(string(out))
	if commitTime.IsZero() {
		return false
	}

	refTime := parseFlexibleTime(entry.LastSucceededAt)
	if refTime.IsZero() {
		if !entry.UpdatedAt.IsZero() {
			refTime = entry.UpdatedAt.UTC()
		} else if !entry.CreatedAt.IsZero() {
			refTime = entry.CreatedAt.UTC()
		}
	}
	if refTime.IsZero() {
		return false
	}

	if commitTime.After(refTime) {
		return true
	}

	// Handle sub-second test commits where entry.UpdatedAt/CreatedAt has nanosecond precision
	// while git commit timestamps (%cI) have 1-second resolution.
	if commitTime.Truncate(time.Second).Equal(refTime.Truncate(time.Second)) && refTime.Nanosecond() > 0 {
		fullPath := filepath.Join(repoRoot, cleanRel)
		if fi, statErr := os.Stat(fullPath); statErr == nil && fi.ModTime().UTC().After(refTime) {
			return true
		}
	}

	return false
}

// CheckBuildManifestFreshness inspects active recipe/procedure entries with build-manifest anchors
// and flags affected entries stale_suspect: true when any anchored build manifest has changed.
func CheckBuildManifestFreshness(ctx context.Context, repoRoot, dataDir, repoID string, idx *index.Index) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(repoRoot) == "" {
		return nil, nil
	}

	entries, inOps, err := loadActiveProceduralEntries(ctx, repoRoot, dataDir, repoID, idx)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	outcomeState, _ := LoadOutcomeState(dataDir, repoID)
	now := time.Now().UTC()

	var flaggedEntries []dossiers.Entry
	var flaggedIDs []string
	var batchOps []ops.Op

	for _, entry := range entries {
		if entry.Retired || entry.StaleSuspect {
			continue
		}
		var eo *EntryOutcome
		if outcomeState != nil && outcomeState.Entries != nil {
			eo = outcomeState.Entries[entry.ID]
		}

		modified := false
		for _, a := range entry.Anchors {
			relPath := stripAnchorSymbol(a)
			if IsBuildManifestPath(relPath) && isManifestModifiedSinceVerification(ctx, repoRoot, relPath, entry, eo) {
				modified = true
				break
			}
		}
		if !modified {
			continue
		}

		entry.StaleSuspect = true
		entry.UpdatedAt = now
		flaggedEntries = append(flaggedEntries, entry)
		flaggedIDs = append(flaggedIDs, entry.ID)

		if !inOps[entry.ID] {
			batchOps = append(batchOps, ops.Op{
				Type:         ops.OpAdd,
				EntryID:      entry.ID,
				DossierScope: entry.DossierScope,
				Timestamp:    now,
				Source:       firstNonEmpty(entry.Source, ops.SourceConsolidation),
				Fields: map[string]any{
					"entry_type":        string(entry.Type),
					"kind":              string(entry.EffectiveKind()),
					"text":              entry.Text,
					"anchors":           entry.Anchors,
					"evidence":          entry.Evidence,
					"verified":          entry.Verified,
					"render_as_skill":   entry.RenderAsSkill,
					"success_count":     entry.SuccessCount,
					"failure_count":     entry.FailureCount,
					"last_succeeded_at": entry.LastSucceededAt,
					"stale_suspect":     true,
				},
			})
		}
		batchOps = append(batchOps, ops.Op{
			Type:         ops.OpAmend,
			EntryID:      entry.ID,
			DossierScope: entry.DossierScope,
			Timestamp:    now,
			Source:       firstNonEmpty(entry.Source, ops.SourceConsolidation),
			Fields: map[string]any{
				"stale_suspect": true,
			},
		})
	}

	if len(flaggedEntries) == 0 {
		return nil, nil
	}

	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := os.MkdirAll(opsDir, 0o755); err == nil && len(batchOps) > 0 {
		batch := ops.NewBatchWithTime(batchOps, now)
		if writeErr := ops.NewWriter(opsDir).Write(batch); writeErr != nil {
			return flaggedIDs, fmt.Errorf("writing stale_suspect op batch: %w", writeErr)
		}
		if projected, projErr := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx); projErr == nil {
			_ = dossiers.NewWriter(filepath.Join(repoRoot, ".memory")).WriteAll(projected)
		}
	}

	if idx != nil {
		for _, entry := range flaggedEntries {
			_ = idx.UpsertEntry(ctx, entry)
		}
	} else if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		idxPath := index.IndexPath(dataDir, repoID)
		if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err == nil {
			if openedIdx, idxErr := index.NewIndex(idxPath); idxErr == nil {
				for _, entry := range flaggedEntries {
					_ = openedIdx.UpsertEntry(ctx, entry)
				}
				_ = openedIdx.Close()
			}
		}
	}

	_, _ = SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, false)

	return flaggedIDs, nil
}
