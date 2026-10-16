package consolidate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

var (
	leadingHeadingRe   = regexp.MustCompile(`^#{1,6}\s*`)
	leadingBracketIDRe = regexp.MustCompile(`^\[[a-zA-Z0-9_-]+\]\s*`)
	inlineEntryIDTagRe = regexp.MustCompile(`(?i)\[(inv|con|rul|fail|err|got|dec|proc|rec|iface|issue|iss|docseed|seed)-[a-z0-9_-]+\]`)
	punctuationReplacer = strings.NewReplacer(
		"-", " ",
		"_", " ",
		":", " ",
		".", " ",
		",", " ",
		"/", " ",
		"`", " ",
		"'", " ",
		"\"", " ",
		"(", " ",
		")", " ",
	)
	inlineWhyRe = regexp.MustCompile(`(?i)\bwhy:\s*`)
	inlineDoRe  = regexp.MustCompile(`(?i)\bdo:\s*`)
)

// EchoFilterResult holds the partitioned outcome of running the deterministic P13 Echo Filter.
type EchoFilterResult struct {
	Kept                 []inbox.NoteItem
	Suppressed           []inbox.NoteItem
	Reconfirmed          []inbox.NoteItem
	EchoSuppressionCount int
}

// CorroborationResult holds the outcome of running the Pre-Consolidation Corroboration Gate (VD18).
type CorroborationResult struct {
	Items                       []inbox.NoteItem
	Downgraded                  []inbox.NoteItem
	CorroborationDowngradeCount int
}

// NormalizeTitle lowercases s, strips markdown heading prefixes and bracketed entry ID tags,
// replaces punctuation with spaces, and collapses whitespace into single spaces.
func NormalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = leadingHeadingRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = leadingBracketIDRe.ReplaceAllString(s, "")
	s = inlineEntryIDTagRe.ReplaceAllString(s, " ")
	s = strings.ToLower(s)
	s = punctuationReplacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// ExtractEntryTitle extracts the title from the first line of entry.Text, stripping
// any ### [<id>] or [<id>] prefix and stopping before inline Why:/Do: clauses if present.
func ExtractEntryTitle(entryText string) string {
	lines := strings.Split(strings.TrimSpace(entryText), "\n")
	var firstLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "`") && strings.HasSuffix(trimmed, "`") && strings.Contains(trimmed, " · ") {
			continue
		}
		firstLine = trimmed
		break
	}
	if firstLine == "" {
		return ""
	}

	firstLine = leadingHeadingRe.ReplaceAllString(firstLine, "")
	firstLine = strings.TrimSpace(firstLine)
	firstLine = leadingBracketIDRe.ReplaceAllString(firstLine, "")
	firstLine = strings.TrimSpace(firstLine)

	cutIdx := len(firstLine)
	if loc := inlineWhyRe.FindStringIndex(firstLine); loc != nil && loc[0] > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}
	if loc := inlineDoRe.FindStringIndex(firstLine); loc != nil && loc[0] > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}
	return strings.TrimSpace(firstLine[:cutIdx])
}

// ExtractEntryDo extracts the Do: line content from entry.Text if present.
func ExtractEntryDo(entryText string) string {
	lines := strings.Split(entryText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimSpace(trimmed)
		if len(trimmed) >= 3 && strings.EqualFold(trimmed[:3], "do:") {
			return strings.TrimSpace(trimmed[3:])
		}
	}
	if loc := inlineDoRe.FindStringIndex(entryText); loc != nil {
		rest := entryText[loc[1]:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[:nl]
		}
		if whyLoc := inlineWhyRe.FindStringIndex(rest); whyLoc != nil {
			rest = rest[:whyLoc[0]]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

// StripAnchorSymbol strips file: prefixes and #symbol or :line/:symbol suffixes,
// normalizes slashes (\ -> /), and strips leading ./ or /.
// Returns "" for bare symbol anchors (e.g. "symbol:Foo" or "#Foo").
func StripAnchorSymbol(anchor string) string {
	s := strings.TrimSpace(anchor)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "symbol:") || strings.HasPrefix(s, "#") {
		return ""
	}
	if strings.HasPrefix(lower, "file:") {
		s = strings.TrimSpace(s[5:])
	}
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.IndexByte(s, ':'); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "/")
	for strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") {
		if strings.HasPrefix(s, "./") {
			s = strings.TrimPrefix(s, "./")
		} else {
			s = strings.TrimPrefix(s, "/")
		}
	}
	s = strings.TrimRight(s, "/")
	if s == "." || s == ".." {
		return ""
	}
	return s
}

// extractAnchorSymbol extracts a non-numeric symbol suffix from an anchor (e.g. "path/file.go#MyFunc"
// or "path/file.go:MyFunc"), returning "" if no symbol suffix is present or if the suffix is a line number.
func extractAnchorSymbol(anchor string) string {
	s := strings.TrimSpace(anchor)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(s), "file:") {
		s = strings.TrimSpace(s[5:])
	}
	var sym string
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		sym = strings.TrimSpace(s[idx+1:])
	} else if idx := strings.IndexByte(s, ':'); idx >= 0 {
		sym = strings.TrimSpace(s[idx+1:])
	}
	if sym == "" {
		return ""
	}
	// Check if sym is purely numeric or a numeric line range like "42" or "42-50".
	linePart := sym
	if dash := strings.IndexByte(linePart, '-'); dash >= 0 {
		linePart = linePart[:dash]
	}
	if _, err := strconv.Atoi(strings.TrimSpace(linePart)); err == nil {
		return ""
	}
	return sym
}

// AnchorsOverlap returns true if a and b share at least one normalized file path or raw symbol anchor,
// or if one anchor path is a directory prefix of the other, or if both slices are empty.
func AnchorsOverlap(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	for _, rawA := range a {
		trimA := strings.TrimSpace(rawA)
		pathA := StripAnchorSymbol(trimA)
		for _, rawB := range b {
			trimB := strings.TrimSpace(rawB)
			pathB := StripAnchorSymbol(trimB)
			if trimA != "" && strings.EqualFold(trimA, trimB) {
				return true
			}
			if pathA != "" && pathB != "" {
				if pathA == pathB || strings.HasPrefix(pathA, pathB+"/") || strings.HasPrefix(pathB, pathA+"/") {
					return true
				}
			}
		}
	}
	return false
}

// FlattenActiveEntries collects all non-retired, non-superseded entries across all dossiers
// in deterministic sorted order (by scope, then entry ID).
func FlattenActiveEntries(dossiersMap map[string]*dossiers.Dossier) []dossiers.Entry {
	if len(dossiersMap) == 0 {
		return nil
	}
	scopes := make([]string, 0, len(dossiersMap))
	for scope := range dossiersMap {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	var active []dossiers.Entry
	seen := make(map[string]bool)
	for _, scope := range scopes {
		d := dossiersMap[scope]
		if d == nil {
			continue
		}
		entries := make([]dossiers.Entry, 0, len(d.Entries))
		for _, e := range d.Entries {
			if e.Retired || e.SupersededBy != "" {
				continue
			}
			if e.DossierScope == "" {
				e.DossierScope = scope
			}
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ID < entries[j].ID
		})
		for _, e := range entries {
			if e.ID != "" && seen[e.ID] {
				continue
			}
			if e.ID != "" {
				seen[e.ID] = true
			}
			active = append(active, e)
		}
	}
	return active
}

// IsUnknownExposureNote reports whether a note originated from an external capture source
// without an injection ledger exposure list (Addendum A §11.2).
func IsUnknownExposureNote(note *inbox.Note) bool {
	if note == nil || len(note.ExposureList) > 0 {
		return false
	}
	src := strings.ToLower(strings.TrimSpace(string(note.Source)))
	dedup := strings.ToLower(strings.TrimSpace(note.DedupKey))
	return strings.HasPrefix(src, "capture:") ||
		strings.EqualFold(src, "external") ||
		strings.EqualFold(src, "unknown") ||
		strings.Contains(dedup, "exposure:unknown")
}

// FilterEchoes implements the deterministic P13 Echo Filter (SPEC §6.5 step 1, design-v2 §6 step 1).
// Any note item repeating an entry in the note's ExposureList (or any active entry when exposure is unknown)
// is suppressed unless the source episode modified one of the entry's anchored files and subsequently
// passed a build or test step.
func FilterEchoes(note *inbox.Note, activeEntries []dossiers.Entry, ep *episode.Episode) EchoFilterResult {
	var res EchoFilterResult
	if note == nil || len(note.Items) == 0 {
		return res
	}

	entryByID := make(map[string]dossiers.Entry, len(activeEntries))
	filteredActive := make([]dossiers.Entry, 0, len(activeEntries))
	for _, e := range activeEntries {
		if e.Retired || e.SupersededBy != "" {
			continue
		}
		filteredActive = append(filteredActive, e)
		if id := strings.TrimSpace(e.ID); id != "" {
			entryByID[id] = e
		}
	}

	exposedIDs := make(map[string]bool)
	exposedEntries := make([]dossiers.Entry, 0)
	seenExposedEntry := make(map[string]bool)

	for _, rec := range note.ExposureList {
		id := strings.TrimSpace(rec.EntryID)
		if id == "" {
			continue
		}
		exposedIDs[id] = true
		if e, ok := entryByID[id]; ok && !seenExposedEntry[id] {
			seenExposedEntry[id] = true
			exposedEntries = append(exposedEntries, e)
		}
	}

	if IsUnknownExposureNote(note) {
		for _, e := range filteredActive {
			id := strings.TrimSpace(e.ID)
			if id != "" {
				exposedIDs[id] = true
				if !seenExposedEntry[id] {
					seenExposedEntry[id] = true
					exposedEntries = append(exposedEntries, e)
				}
			} else {
				exposedEntries = append(exposedEntries, e)
			}
		}
	}

	sortedExposedIDs := make([]string, 0, len(exposedIDs))
	for id := range exposedIDs {
		sortedExposedIDs = append(sortedExposedIDs, id)
	}
	sort.Strings(sortedExposedIDs)

	for _, item := range note.Items {
		var matchedEntry *dossiers.Entry
		isEcho := false

		// 1. Explicit EntryID match in Title, Evidence, Why, or Do.
		for _, expID := range sortedExposedIDs {
			if containsEntryIDToken(expID, item.Title, item.Evidence, item.Why, item.Do) {
				isEcho = true
				if e, ok := entryByID[expID]; ok {
					eCopy := e
					matchedEntry = &eCopy
				} else {
					matchedEntry = &dossiers.Entry{
						ID:      expID,
						Anchors: item.Anchors,
					}
				}
				break
			}
		}

		// 2. Semantic Title/Do + Anchor overlap match.
		if !isEcho {
			normItemTitle := NormalizeTitle(item.Title)
			normItemDo := NormalizeTitle(item.Do)
			for _, e := range exposedEntries {
				normEntryTitle := NormalizeTitle(ExtractEntryTitle(e.Text))
				normEntryText := NormalizeTitle(e.Text)
				normEntryDo := NormalizeTitle(ExtractEntryDo(e.Text))

				titleMatches := normItemTitle != "" && (normItemTitle == normEntryTitle ||
					(len(normItemTitle) >= 12 && strings.Contains(normEntryText, normItemTitle)) ||
					(len(normEntryTitle) >= 12 && strings.Contains(normItemTitle, normEntryTitle)))
				doMatches := normItemDo != "" && normEntryDo != "" && normItemDo == normEntryDo

				if (titleMatches || doMatches) && AnchorsOverlap(item.Anchors, e.Anchors) {
					isEcho = true
					eCopy := e
					matchedEntry = &eCopy
					break
				}
			}
		}

		if !isEcho {
			res.Kept = append(res.Kept, item)
			continue
		}

		combinedAnchors := combineAnchors(matchedEntry.Anchors, item.Anchors)
		if IsEchoReconfirmed(ep, combinedAnchors) {
			if !IsVerifiedStamp(item.Verified) {
				item.Verified = formatVerifiedStamp(note, ep, "reconfirmed")
			}
			res.Kept = append(res.Kept, item)
			res.Reconfirmed = append(res.Reconfirmed, item)
		} else {
			res.Suppressed = append(res.Suppressed, item)
			res.EchoSuppressionCount++
		}
	}

	return res
}

func containsEntryIDToken(entryID string, fields ...string) bool {
	target := strings.ToLower(strings.TrimSpace(entryID))
	if target == "" {
		return false
	}
	for _, f := range fields {
		if f == "" {
			continue
		}
		if strings.Contains(strings.ToLower(f), target) {
			return true
		}
	}
	return false
}

func combineAnchors(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	return out
}

// IsEchoReconfirmed returns true if and only if ep is non-nil, combinedAnchors is non-empty,
// and HasPassingBuildOrTestAfterEdit(ep, combinedAnchors) is true.
func IsEchoReconfirmed(ep *episode.Episode, combinedAnchors []string) bool {
	return ep != nil && len(combinedAnchors) > 0 && HasPassingBuildOrTestAfterEdit(ep, combinedAnchors)
}

// HasPassingBuildOrTestAfterEdit returns true if the episode contains an edit step to at least one
// of the anchored files/directories followed by a subsequent passing build or test step.
func HasPassingBuildOrTestAfterEdit(ep *episode.Episode, anchors []string) bool {
	if ep == nil || len(ep.Steps) == 0 || len(anchors) == 0 {
		return false
	}

	anchorPaths := make([]string, 0, len(anchors))
	for _, a := range anchors {
		if p := StripAnchorSymbol(a); p != "" {
			anchorPaths = append(anchorPaths, p)
		}
	}
	if len(anchorPaths) == 0 {
		return false
	}

	editPos := -1
	editStepIdx := -1
	for pos, step := range ep.Steps {
		if IsEditToAnchoredFile(step, anchorPaths) {
			editPos = pos
			editStepIdx = step.Index
			break
		}
	}
	if editPos < 0 {
		return false
	}

	for pos, step := range ep.Steps {
		if pos <= editPos && (step.Index <= editStepIdx || editStepIdx <= 0) {
			continue
		}
		if IsPassingBuildOrTestStep(step) {
			return true
		}
	}
	return false
}

// IsEditToAnchoredFile reports whether step is an edit/write step targeting one of anchorPaths.
func IsEditToAnchoredFile(step harness.Step, anchorPaths []string) bool {
	isEdit := step.Kind == harness.StepKindEdit ||
		strings.EqualFold(step.Action, "edit") ||
		strings.EqualFold(step.Action, "write")
	if !isEdit {
		return false
	}

	rawFile := strings.TrimSpace(step.File)
	if rawFile == "" {
		return false
	}
	normFile := StripAnchorSymbol(rawFile)
	slashFile := strings.TrimRight(strings.ReplaceAll(rawFile, "\\", "/"), "/")

	for _, ap := range anchorPaths {
		if ap == "" {
			continue
		}
		if normFile == ap || strings.HasPrefix(normFile, ap+"/") {
			return true
		}
		if strings.HasSuffix(slashFile, "/"+ap) || strings.Contains(slashFile, "/"+ap+"/") {
			return true
		}
	}
	return false
}

// IsPassingBuildOrTestStep reports whether step is a passing test step or a passing build/test command step.
func IsPassingBuildOrTestStep(step harness.Step) bool {
	switch step.Kind {
	case harness.StepKindTest:
		if step.Failed > 0 || isFailOutcome(step.Outcome) {
			return false
		}
		if step.ExitCode != nil {
			return *step.ExitCode == 0
		}
		return step.Passed > 0 || isPassOutcome(step.Outcome)
	case harness.StepKindCommand:
		if !IsBuildOrTestCommand(step.Command) || isFailOutcome(step.Outcome) {
			return false
		}
		if step.ExitCode != nil {
			return *step.ExitCode == 0
		}
		return isPassOutcome(step.Outcome)
	default:
		return false
	}
}

func isPassOutcome(outcome string) bool {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "pass", "passed", "ok", "success", "succeeded", "0":
		return true
	default:
		return false
	}
}

func isFailOutcome(outcome string) bool {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "fail", "failed", "failure", "error", "errored", "timeout", "timed_out":
		return true
	default:
		return false
	}
}

var buildOrTestTokenRe = regexp.MustCompile(`(?i)(?:^|[;&|()\s/])(pytest|py\.test|make|cmake|ninja|gradle|gradlew|ctest|jest|vitest|golangci-lint)(?:$|[;&|()\s])`)

// IsBuildOrTestCommand reports whether cmd invokes a build, test, lint, or verification tool.
func IsBuildOrTestCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	if lower == "" {
		return false
	}
	phrases := []string{
		"go test",
		"go build",
		"go vet",
		"cargo test",
		"cargo build",
		"cargo check",
		"cargo clippy",
		"npm test",
		"npm run test",
		"npm run build",
		"npm run check",
		"npm run lint",
		"pnpm test",
		"pnpm build",
		"pnpm run test",
		"pnpm run build",
		"yarn test",
		"yarn build",
		"yarn run test",
		"yarn run build",
		"idf.py build",
		"idf.py test",
		"pio run",
		"pio test",
		"mvn test",
		"mvn compile",
		"mvn package",
		"mvn verify",
		"python -m pytest",
		"python3 -m pytest",
		"python -m unittest",
		"python3 -m unittest",
		"bazel test",
		"bazel build",
	}
	for _, p := range phrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return buildOrTestTokenRe.MatchString(lower)
}

// RequiresCorroboration reports whether a candidate NoteItem is a semantic or procedural rule
// (or requests an edit gate) that must pass the VD18 Pre-Consolidation Corroboration Gate.
func RequiresCorroboration(item inbox.NoteItem) bool {
	switch item.Type {
	case dossiers.EntryTypeInvariant,
		dossiers.EntryTypeContract,
		dossiers.EntryTypeConvention,
		dossiers.EntryTypeRecipe,
		dossiers.EntryTypeProcedure:
		return true
	}
	if strings.Contains(strings.ToLower(item.Do), "gate: edit") ||
		strings.Contains(strings.ToLower(item.Why), "gate: edit") ||
		strings.Contains(strings.ToLower(item.Verified), "gate: edit") ||
		strings.Contains(strings.ToLower(item.Title), "gate: edit") {
		return true
	}
	return false
}

// IsVerifiedStamp reports whether v represents a positive verification stamp rather than
// an empty, null, or unverified placeholder.
func IsVerifiedStamp(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	switch s {
	case "", "null", "nil", "unverified", "none", "false", "no", "pending", "hypothesis":
		return false
	default:
		return true
	}
}

// EnforceCorroborationGate implements the VD18 Pre-Consolidation Corroboration Gate
// (SPEC §6.4 step 3, design-v2 §6 step 1). Any semantic or procedural rule arriving without a
// positive verification stamp must be corroborated by (1) a passing build/test after edit in ep,
// (2) a verified static code/symbol anchor in HEAD/repoRoot, or (3) explicit human USER_INPUT
// confirmation in ep. Uncorroborated rules are downgraded to open-issue with a deterministic closes_when.
func EnforceCorroborationGate(note *inbox.Note, items []inbox.NoteItem, ep *episode.Episode, repoRoot string) CorroborationResult {
	var res CorroborationResult
	if len(items) == 0 {
		return res
	}

	for _, item := range items {
		if !RequiresCorroboration(item) {
			res.Items = append(res.Items, item)
			continue
		}

		if note != nil && (note.Source == inbox.SourceDocSeed || note.Source == inbox.SourcePA) {
			if !IsVerifiedStamp(item.Verified) {
				item.Verified = formatVerifiedStamp(note, ep, string(note.Source))
			}
			res.Items = append(res.Items, item)
			continue
		}

		if IsVerifiedStamp(item.Verified) {
			res.Items = append(res.Items, item)
			continue
		}

		// Check 1: Passing build/test step after edit in ep (or passing command for recipe/procedure).
		corroboratedByTest := HasPassingBuildOrTestAfterEdit(ep, item.Anchors)
		if !corroboratedByTest && (item.Type == dossiers.EntryTypeRecipe || item.Type == dossiers.EntryTypeProcedure) && len(item.Commands) > 0 {
			corroboratedByTest = hasPassingCommandPrefix(ep, item.Commands)
		}
		if corroboratedByTest {
			item.Verified = formatVerifiedStamp(note, ep, "test-pass")
			res.Items = append(res.Items, item)
			continue
		}

		// Check 2: Verified static code anchor present in HEAD / repoRoot.
		if HasVerifiedStaticAnchor(note, item, repoRoot) {
			item.Verified = formatVerifiedStamp(note, ep, "anchor-verified")
			res.Items = append(res.Items, item)
			continue
		}

		// Check 3: Explicit human USER_INPUT confirmation in ep.
		if HasExplicitUserConfirmation(ep, item) {
			item.Verified = formatVerifiedStamp(note, ep, "user-confirmed")
			res.Items = append(res.Items, item)
			continue
		}

		// Downgrade uncorroborated rule to open-issue.
		item.Type = dossiers.EntryTypeOpenIssue
		item.Verified = "unverified"
		if strings.TrimSpace(item.ClosesWhen) == "" {
			if len(item.Anchors) > 0 && StripAnchorSymbol(item.Anchors[0]) != "" {
				item.ClosesWhen = "path_exists:" + StripAnchorSymbol(item.Anchors[0])
			} else if len(item.Commands) > 0 && strings.TrimSpace(item.Commands[0]) != "" {
				item.ClosesWhen = "command_passes:" + strings.TrimSpace(item.Commands[0])
			} else {
				item.ClosesWhen = "outcome:build_or_test_pass"
			}
		}
		res.Items = append(res.Items, item)
		res.Downgraded = append(res.Downgraded, item)
		res.CorroborationDowngradeCount++
	}

	return res
}

func hasPassingCommandPrefix(ep *episode.Episode, commands []string) bool {
	if ep == nil || len(ep.Steps) == 0 || len(commands) == 0 {
		return false
	}
	for _, step := range ep.Steps {
		if step.Kind != harness.StepKindCommand && step.Kind != harness.StepKindTest {
			continue
		}
		if step.Failed > 0 || isFailOutcome(step.Outcome) {
			continue
		}
		passed := false
		if step.ExitCode != nil {
			passed = *step.ExitCode == 0
		} else {
			passed = step.Passed > 0 || isPassOutcome(step.Outcome)
		}
		if !passed {
			continue
		}
		stepCmd := strings.ToLower(strings.TrimSpace(step.Command))
		if stepCmd == "" {
			continue
		}
		for _, rawPrefix := range commands {
			prefix := strings.ToLower(strings.TrimSpace(rawPrefix))
			if prefix != "" && (stepCmd == prefix || strings.HasPrefix(stepCmd, prefix+" ") || strings.Contains(stepCmd, prefix)) {
				return true
			}
		}
	}
	return false
}

// HasVerifiedStaticAnchor reports whether item has at least one static file (and optional symbol)
// anchor verified to exist via item.AnchorChecks, note.AnchorChecks, or live repoRoot inspection.
func HasVerifiedStaticAnchor(note *inbox.Note, item inbox.NoteItem, repoRoot string) bool {
	if len(item.Anchors) == 0 {
		return false
	}

	checkMap := make(map[string]bool)
	recordCheck := func(ac inbox.AnchorCheck) {
		if !ac.Exists {
			return
		}
		trimmed := strings.TrimSpace(ac.Anchor)
		if trimmed != "" {
			checkMap[trimmed] = true
		}
		if norm := StripAnchorSymbol(trimmed); norm != "" {
			checkMap[norm] = true
		}
	}

	for _, ac := range item.AnchorChecks {
		recordCheck(ac)
	}
	if note != nil {
		for _, ac := range note.AnchorChecks {
			recordCheck(ac)
		}
	}
	if repoRoot != "" && len(checkMap) == 0 {
		for _, ac := range inbox.CheckAnchors(repoRoot, item.Anchors) {
			recordCheck(ac)
		}
	}

	for _, rawAnchor := range item.Anchors {
		trimmed := strings.TrimSpace(rawAnchor)
		normPath := StripAnchorSymbol(trimmed)
		if normPath == "" {
			continue
		}
		exists := checkMap[trimmed] || checkMap[normPath]
		if !exists && repoRoot != "" {
			for _, ac := range inbox.CheckAnchors(repoRoot, []string{trimmed}) {
				if ac.Exists {
					exists = true
					break
				}
			}
		}
		if !exists {
			continue
		}

		symbol := extractAnchorSymbol(trimmed)
		if symbol != "" && repoRoot != "" {
			fullPath := filepath.Join(repoRoot, filepath.FromSlash(normPath))
			info, err := os.Stat(fullPath)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			data, err := os.ReadFile(fullPath)
			if err != nil || !strings.Contains(string(data), symbol) {
				continue
			}
		}
		return true
	}

	return false
}

var userConfirmPhrases = []string{
	"confirm",
	"approved",
	"yes",
	"correct",
	"must",
	"always",
	"never",
	"invariant",
	"rule",
	"contract",
}

// HasExplicitUserConfirmation reports whether ep contains a genuine human StepKindUser step
// (excluding SYSTEM_SDK and [repo-memory ...] injections) that confirms item either by step citation
// or by confirmation phrasing + shared significant token/anchor.
func HasExplicitUserConfirmation(ep *episode.Episode, item inbox.NoteItem) bool {
	if ep == nil || len(ep.Steps) == 0 {
		return false
	}

	citedSteps := make(map[int]bool, len(item.StepCitations))
	for _, idx := range item.StepCitations {
		citedSteps[idx] = true
	}

	itemTokens := extractSignificantTokens(item.Title + " " + item.Do)
	anchorPaths := make([]string, 0, len(item.Anchors))
	for _, a := range item.Anchors {
		if p := StripAnchorSymbol(a); p != "" {
			anchorPaths = append(anchorPaths, strings.ToLower(p))
			base := strings.ToLower(filepath.Base(p))
			if len(base) >= 4 {
				itemTokens[base] = true
			}
		}
	}

	for _, step := range ep.Steps {
		if step.Kind != harness.StepKindUser {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(step.Author), "SYSTEM_SDK") {
			continue
		}
		trimmedText := strings.TrimSpace(step.Text)
		if trimmedText == "" || strings.HasPrefix(trimmedText, "[repo-memory") {
			continue
		}

		if len(citedSteps) > 0 && citedSteps[step.Index] {
			return true
		}

		lowerText := strings.ToLower(trimmedText)
		hasPhrase := false
		for _, phrase := range userConfirmPhrases {
			if strings.Contains(lowerText, phrase) {
				hasPhrase = true
				break
			}
		}
		if !hasPhrase {
			continue
		}

		for _, ap := range anchorPaths {
			if ap != "" && strings.Contains(lowerText, ap) {
				return true
			}
		}

		stepTokens := extractSignificantTokens(lowerText)
		for tok := range itemTokens {
			if stepTokens[tok] {
				return true
			}
		}
	}

	return false
}

var confirmStopwords = map[string]bool{
	"confirm":   true,
	"confirmed": true,
	"approved":  true,
	"correct":   true,
	"must":      true,
	"always":    true,
	"never":     true,
	"invariant": true,
	"rule":      true,
	"contract":  true,
	"that":      true,
	"this":      true,
	"with":      true,
	"from":      true,
	"when":      true,
	"only":      true,
	"have":      true,
	"should":    true,
}

func extractSignificantTokens(s string) map[string]bool {
	norm := NormalizeTitle(s)
	fields := strings.Fields(norm)
	out := make(map[string]bool, len(fields))
	for _, f := range fields {
		if len(f) >= 4 && !confirmStopwords[f] {
			out[f] = true
		}
	}
	return out
}

func formatVerifiedStamp(note *inbox.Note, ep *episode.Episode, tag string) string {
	dateStr := time.Now().UTC().Format("2006-01-02")
	commitStr := ""
	if note != nil {
		if !note.CreatedAt.IsZero() {
			dateStr = note.CreatedAt.UTC().Format("2006-01-02")
		}
		commitStr = strings.TrimSpace(note.HeadCommit)
	}
	if ep != nil {
		if (note == nil || note.CreatedAt.IsZero()) && !ep.Header.Ended.IsZero() {
			dateStr = ep.Header.Ended.UTC().Format("2006-01-02")
		} else if (note == nil || note.CreatedAt.IsZero()) && !ep.Header.Started.IsZero() {
			dateStr = ep.Header.Started.UTC().Format("2006-01-02")
		}
		if commitStr == "" {
			if ep.Header.HeadEnd != "" {
				commitStr = strings.TrimSpace(ep.Header.HeadEnd)
			} else if ep.Header.HeadStart != "" {
				commitStr = strings.TrimSpace(ep.Header.HeadStart)
			}
		}
	}
	if len(commitStr) > 7 {
		commitStr = commitStr[:7]
	}
	if commitStr != "" && tag != "" {
		return fmt.Sprintf("%s (%s, %s)", dateStr, commitStr, tag)
	}
	if tag != "" {
		return fmt.Sprintf("%s (%s)", dateStr, tag)
	}
	if commitStr != "" {
		return fmt.Sprintf("%s (%s)", dateStr, commitStr)
	}
	return dateStr
}
