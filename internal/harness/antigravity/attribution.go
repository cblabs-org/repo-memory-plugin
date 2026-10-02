package antigravity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	_ "modernc.org/sqlite"
)

// ConversationSummary captures conversation metadata extracted from conversation_summaries.db.
type ConversationSummary struct {
	ConversationID       string    `json:"conversation_id"`
	Title                string    `json:"title"`
	Preview              string    `json:"preview"`
	StepCount            int       `json:"step_count"`
	LastModifiedTime     time.Time `json:"last_modified_time"`
	WorkspaceURIs        []string  `json:"workspace_uris"`
	WorkspacePaths       []string  `json:"workspace_paths"`
	ParentConversationID string    `json:"parent_conversation_id,omitempty"`
	NotFullyIdle         bool      `json:"not_fully_idle"`
	Killed               bool      `json:"killed"`
	SourceStore          string    `json:"source_store"` // "app", "cli", or "both"
	InAppStore           bool      `json:"in_app_store"`
	InCLIStore           bool      `json:"in_cli_store"`
	SubagentIDs          []string  `json:"subagent_ids,omitempty"`
}

// AttributedConversation captures the mapping between an Antigravity conversation and an enrolled repo.
type AttributedConversation struct {
	Summary              ConversationSummary `json:"summary"`
	ConversationID       string              `json:"conversation_id"`
	ParentConversationID string              `json:"parent_conversation_id,omitempty"`
	RepoID               string              `json:"repo_id"`
	RepoRoot             string              `json:"repo_root"`
	WorkspacePath        string              `json:"workspace_path"`
	TranscriptPath       string              `json:"transcript_path,omitempty"`
	Idle                 bool                `json:"idle"`
}

// AttributionOptions configures session discovery and database locations.
type AttributionOptions struct {
	AppDBPath    string
	CliDBPath    string
	AppBrainDir  string
	CliBrainDir  string
	IdleCutoff   time.Duration
	Now          time.Time
}

// AttributionOption allows overriding defaults for attribution options.
type AttributionOption func(*AttributionOptions)

// WithAppDBPath sets a custom path for the Antigravity app conversation_summaries.db.
func WithAppDBPath(p string) AttributionOption {
	return func(o *AttributionOptions) {
		o.AppDBPath = p
	}
}

// WithCliDBPath sets a custom path for the Antigravity CLI conversation_summaries.db.
func WithCliDBPath(p string) AttributionOption {
	return func(o *AttributionOptions) {
		o.CliDBPath = p
	}
}

// WithAppBrainDir sets a custom path for the Antigravity app brain directory.
func WithAppBrainDir(p string) AttributionOption {
	return func(o *AttributionOptions) {
		o.AppBrainDir = p
	}
}

// WithCliBrainDir sets a custom path for the Antigravity CLI brain directory.
func WithCliBrainDir(p string) AttributionOption {
	return func(o *AttributionOptions) {
		o.CliBrainDir = p
	}
}

// WithIdleCutoff sets the duration after which an unchanged session is considered idle.
func WithIdleCutoff(d time.Duration) AttributionOption {
	return func(o *AttributionOptions) {
		if d > 0 {
			o.IdleCutoff = d
		}
	}
}

// WithAttributionNow sets a frozen reference time for idle calculation (useful in testing).
func WithAttributionNow(t time.Time) AttributionOption {
	return func(o *AttributionOptions) {
		o.Now = t
	}
}

// SessionAttributor implements tick.SessionAttributor for Antigravity conversations.
type SessionAttributor struct {
	opts AttributionOptions
}

// NewSessionAttributor constructs a SessionAttributor with applied options and standard default paths.
func NewSessionAttributor(opts ...AttributionOption) *SessionAttributor {
	opt := AttributionOptions{
		IdleCutoff: 15 * time.Minute,
	}
	for _, o := range opts {
		o(&opt)
	}

	if opt.AppDBPath == "" || opt.CliDBPath == "" || opt.AppBrainDir == "" || opt.CliBrainDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			if opt.AppDBPath == "" {
				opt.AppDBPath = filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
			}
			if opt.CliDBPath == "" {
				opt.CliDBPath = filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
			}
			if opt.AppBrainDir == "" {
				opt.AppBrainDir = filepath.Join(home, ".gemini", "antigravity", "brain")
			}
			if opt.CliBrainDir == "" {
				opt.CliBrainDir = filepath.Join(home, ".gemini", "antigravity-cli", "brain")
			}
		}
	}

	return &SessionAttributor{opts: opt}
}

// AttributedSessions loads conversation summaries from both app and CLI stores, de-duplicates them,
// links subagents to parents, attributes them to enrolled repos, and returns normalized tick.AttributedSessions.
func (a *SessionAttributor) AttributedSessions(ctx context.Context, enrolledRepos map[string]config.RegistryEntry) ([]tick.AttributedSession, error) {
	appSummaries, err := LoadSummariesFromDB(a.opts.AppDBPath, "app")
	if err != nil {
		return nil, fmt.Errorf("loading app summaries: %w", err)
	}

	cliSummaries, err := LoadSummariesFromDB(a.opts.CliDBPath, "cli")
	if err != nil {
		return nil, fmt.Errorf("loading cli summaries: %w", err)
	}

	merged := DeduplicateSummaries(appSummaries, cliSummaries)
	LinkSubagents(merged)

	attributed := AttributeConversations(merged, enrolledRepos)

	now := a.opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	var sessions []tick.AttributedSession
	for _, attr := range attributed {
		transcriptPath := ResolveTranscriptPath(attr.ConversationID, a.opts.AppBrainDir, a.opts.CliBrainDir)
		idle := IsConversationIdle(attr.Summary, transcriptPath, now, a.opts.IdleCutoff)

		isHousekeeping := false
		for _, wsPath := range attr.Summary.WorkspacePaths {
			if strings.Contains(filepath.ToSlash(wsPath), "/runs/") {
				isHousekeeping = true
				break
			}
		}

		sessions = append(sessions, tick.AttributedSession{
			ConversationID:       attr.ConversationID,
			ParentConversationID: attr.ParentConversationID,
			RepoID:               attr.RepoID,
			RepoRoot:             attr.RepoRoot,
			TranscriptPath:       transcriptPath,
			LastModified:         attr.Summary.LastModifiedTime,
			Idle:                 idle,
			Source:               "antigravity",
			IsHousekeeping:       isHousekeeping,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ConversationID < sessions[j].ConversationID
	})

	return sessions, nil
}

// ParseWorkspaceURI extracts and normalizes a file path from a workspace URI string.
// Supports file:///path, file:/path, percent-encoded characters (%20), and Windows drives (/C:/...).
func ParseWorkspaceURI(uriStr string) (string, error) {
	uriStr = strings.TrimSpace(uriStr)
	if uriStr == "" {
		return "", fmt.Errorf("empty workspace URI")
	}

	raw := uriStr
	if strings.HasPrefix(raw, "file://") {
		raw = strings.TrimPrefix(raw, "file://")
	} else if strings.HasPrefix(raw, "file:") {
		raw = strings.TrimPrefix(raw, "file:")
	}

	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		unescaped = raw
	}

	// Handle Windows file:///C:/path or /C:/path
	if len(unescaped) >= 3 && unescaped[0] == '/' && isWindowsDrive(unescaped[1:3]) {
		unescaped = unescaped[1:]
	}

	clean := filepath.Clean(filepath.FromSlash(unescaped))
	return clean, nil
}

func isWindowsDrive(s string) bool {
	if len(s) != 2 {
		return false
	}
	isLetter := (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')
	return isLetter && s[1] == ':'
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

func parseFlexibleTime(val string) time.Time {
	val = strings.TrimSpace(val)
	if val == "" || val == "0001-01-01 00:00:00+00:00" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, val); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// LoadSummariesFromDB reads all conversation summaries from a SQLite database file.
// If the database file does not exist, an empty map is returned without error.
func LoadSummariesFromDB(dbPath, storeName string) (map[string]*ConversationSummary, error) {
	if dbPath == "" {
		return make(map[string]*ConversationSummary), nil
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*ConversationSummary), nil
		}
		return nil, fmt.Errorf("stat database %s: %w", dbPath, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&mode=ro", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	defer db.Close()

	query := `
SELECT
	conversation_id,
	title,
	preview,
	step_count,
	last_modified_time,
	workspace_uris,
	parent_conversation_id,
	not_fully_idle,
	killed
FROM conversation_summaries;
`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying conversation_summaries from %s: %w", dbPath, err)
	}
	defer rows.Close()

	results := make(map[string]*ConversationSummary)
	for rows.Next() {
		var convID, title, preview, timeStr, urisStr, parentID sql.NullString
		var stepCount sql.NullInt64
		var notIdle, killed sql.NullBool

		err := rows.Scan(
			&convID,
			&title,
			&preview,
			&stepCount,
			&timeStr,
			&urisStr,
			&parentID,
			&notIdle,
			&killed,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if !convID.Valid || convID.String == "" {
			continue
		}

		var uris []string
		if urisStr.Valid && urisStr.String != "" {
			_ = json.Unmarshal([]byte(urisStr.String), &uris)
		}

		var paths []string
		for _, u := range uris {
			if cleanPath, err := ParseWorkspaceURI(u); err == nil && cleanPath != "" {
				paths = append(paths, cleanPath)
			}
		}

		summary := &ConversationSummary{
			ConversationID:       convID.String,
			Title:                title.String,
			Preview:              preview.String,
			StepCount:            int(stepCount.Int64),
			LastModifiedTime:     parseFlexibleTime(timeStr.String),
			WorkspaceURIs:        uris,
			WorkspacePaths:       paths,
			ParentConversationID: strings.TrimSpace(parentID.String),
			NotFullyIdle:         notIdle.Bool,
			Killed:               killed.Bool,
			SourceStore:          storeName,
		}

		if storeName == "app" {
			summary.InAppStore = true
		} else if storeName == "cli" {
			summary.InCLIStore = true
		}

		results[convID.String] = summary
	}

	return results, rows.Err()
}

// DeduplicateSummaries merges conversation summaries from the app and CLI stores into a single map.
// For conversations present in both stores:
// - It marks InAppStore=true and InCLIStore=true, setting SourceStore="both".
// - If the CLI record is more recent, its timestamp, idle, and killed state are updated.
// - It retains the higher step count.
// - It preserves non-empty workspace paths and parent conversation IDs from either store.
func DeduplicateSummaries(appSummaries, cliSummaries map[string]*ConversationSummary) map[string]*ConversationSummary {
	merged := make(map[string]*ConversationSummary, len(appSummaries)+len(cliSummaries))

	for id, s := range appSummaries {
		copySummary := *s
		copySummary.InAppStore = true
		copySummary.SourceStore = "app"
		merged[id] = &copySummary
	}

	for id, cliSummary := range cliSummaries {
		existing, ok := merged[id]
		if !ok {
			copySummary := *cliSummary
			copySummary.InCLIStore = true
			copySummary.SourceStore = "cli"
			merged[id] = &copySummary
			continue
		}

		// Present in both stores: de-duplicate!
		existing.InCLIStore = true
		existing.SourceStore = "both"

		// Merge workspace paths if app was missing them
		if len(existing.WorkspacePaths) == 0 && len(cliSummary.WorkspacePaths) > 0 {
			existing.WorkspacePaths = cliSummary.WorkspacePaths
			existing.WorkspaceURIs = cliSummary.WorkspaceURIs
		}

		// Retain parent conversation ID if one had it
		if existing.ParentConversationID == "" && cliSummary.ParentConversationID != "" {
			existing.ParentConversationID = cliSummary.ParentConversationID
		}

		// Keep highest step count
		if cliSummary.StepCount > existing.StepCount {
			existing.StepCount = cliSummary.StepCount
		}

		// If CLI is more recently modified, update timestamp and status
		if cliSummary.LastModifiedTime.After(existing.LastModifiedTime) {
			existing.LastModifiedTime = cliSummary.LastModifiedTime
			existing.NotFullyIdle = cliSummary.NotFullyIdle
			existing.Killed = cliSummary.Killed
			if cliSummary.Title != "" {
				existing.Title = cliSummary.Title
			}
			if cliSummary.Preview != "" {
				existing.Preview = cliSummary.Preview
			}
		}
	}

	return merged
}

// LinkSubagents populates the SubagentIDs slice of each parent conversation based on parent_conversation_id.
func LinkSubagents(summaries map[string]*ConversationSummary) {
	for _, conv := range summaries {
		if conv.ParentConversationID == "" || conv.ParentConversationID == conv.ConversationID {
			continue
		}
		if parent, ok := summaries[conv.ParentConversationID]; ok {
			alreadyAdded := false
			for _, childID := range parent.SubagentIDs {
				if childID == conv.ConversationID {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				parent.SubagentIDs = append(parent.SubagentIDs, conv.ConversationID)
			}
		}
	}
}

// AttributeConversations maps conversation summaries to enrolled repositories.
// Conversations are mapped if any workspace path matches an enrolled repo.
// Subagents with no direct matching workspace inherit the repository attribution of their parent.
func AttributeConversations(
	summaries map[string]*ConversationSummary,
	enrolledRepos map[string]config.RegistryEntry,
) map[string]*AttributedConversation {
	attributed := make(map[string]*AttributedConversation)

	// Helper to check if a workspace path matches an enrolled repo
	matchRepo := func(wsPath string) *config.RegistryEntry {
		cleanWs := filepath.Clean(wsPath)
		evalWs, _ := filepath.EvalSymlinks(cleanWs)

		for root, entry := range enrolledRepos {
			cleanRoot := filepath.Clean(root)
			evalRoot, _ := filepath.EvalSymlinks(cleanRoot)

			if cleanWs == cleanRoot || (evalWs != "" && evalWs == cleanRoot) ||
				(evalRoot != "" && cleanWs == evalRoot) || (evalWs != "" && evalWs == evalRoot) {
				return &entry
			}

			if config.IsSubdir(cleanWs, cleanRoot) || config.IsSubdir(cleanRoot, cleanWs) {
				return &entry
			}
			if evalWs != "" && config.IsSubdir(evalWs, cleanRoot) {
				return &entry
			}
		}
		return nil
	}

	// 1. Direct attribution via workspace_paths
	for id, summary := range summaries {
		for _, wsPath := range summary.WorkspacePaths {
			if entry := matchRepo(wsPath); entry != nil {
				attributed[id] = &AttributedConversation{
					Summary:              *summary,
					ConversationID:       id,
					ParentConversationID: summary.ParentConversationID,
					RepoID:               entry.RepoID,
					RepoRoot:             entry.RootPath,
					WorkspacePath:        wsPath,
				}
				break
			}
		}
	}

	// 2. Multi-pass subagent inheritance: attribute subagents inheriting from parent
	changed := true
	for changed {
		changed = false
		for id, summary := range summaries {
			if _, alreadyAttributed := attributed[id]; alreadyAttributed {
				continue
			}
			if summary.ParentConversationID == "" {
				continue
			}

			if parentAttr, ok := attributed[summary.ParentConversationID]; ok {
				attributed[id] = &AttributedConversation{
					Summary:              *summary,
					ConversationID:       id,
					ParentConversationID: summary.ParentConversationID,
					RepoID:               parentAttr.RepoID,
					RepoRoot:             parentAttr.RepoRoot,
					WorkspacePath:        parentAttr.WorkspacePath,
				}
				changed = true
			}
		}
	}

	return attributed
}

// ResolveTranscriptPath searches candidate brain directories for a conversation's transcript source.
// Prioritizes appBrainDir, then cliBrainDir.
// Returns the resolved transcript directory or file, or "" if not found on disk.
func ResolveTranscriptPath(convID, appBrainDir, cliBrainDir string) string {
	candidates := []string{}
	if appBrainDir != "" {
		candidates = append(candidates, filepath.Join(appBrainDir, convID))
	}
	if cliBrainDir != "" {
		candidates = append(candidates, filepath.Join(cliBrainDir, convID))
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			if files, err := spool.ResolvePrimaryTranscript(cand); err == nil && len(files) > 0 {
				return cand
			}
		}
	}

	return ""
}

// IsConversationIdle checks if a conversation has been idle for the required duration (SPEC §6.2: 15 minutes).
// Killed sessions are considered immediately idle.
func IsConversationIdle(summary ConversationSummary, transcriptPath string, now time.Time, idleCutoff time.Duration) bool {
	if idleCutoff <= 0 {
		idleCutoff = 15 * time.Minute
	}
	if now.IsZero() {
		now = time.Now()
	}

	if summary.Killed {
		return true
	}

	if !summary.LastModifiedTime.IsZero() {
		if now.Sub(summary.LastModifiedTime) < idleCutoff {
			return false
		}
	}

	if summary.NotFullyIdle {
		return false
	}

	if transcriptPath != "" {
		if fi, err := os.Stat(transcriptPath); err == nil {
			if now.Sub(fi.ModTime()) < idleCutoff {
				return false
			}
		}
	}

	return true
}

// LookupConversationSummary loads and de-duplicates summaries from app and CLI stores,
// returning the summary for a single conversation ID, or nil if not found.
func LookupConversationSummary(convID string, opts ...AttributionOption) (*ConversationSummary, error) {
	attributor := NewSessionAttributor(opts...)
	appSummaries, err := LoadSummariesFromDB(attributor.opts.AppDBPath, "app")
	if err != nil {
		return nil, err
	}
	cliSummaries, err := LoadSummariesFromDB(attributor.opts.CliDBPath, "cli")
	if err != nil {
		return nil, err
	}

	merged := DeduplicateSummaries(appSummaries, cliSummaries)
	LinkSubagents(merged)

	if summary, ok := merged[convID]; ok {
		return summary, nil
	}
	return nil, nil
}
