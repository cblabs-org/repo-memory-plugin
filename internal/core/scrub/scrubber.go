package scrub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Context provides boundary and repository metadata for a Scrub operation.
type Context struct {
	Boundary   string          // e.g. "capture", "note", "sidecar", "publish"
	RepoID     string
	RepoRoot   string
	DataDir    string
	AllowedIDs map[string]bool // known conversation UUIDs, entry IDs, ULIDs, commit SHAs
}

// Result contains the scrubbed text and counts of redactions by category.
type Result struct {
	Text   string
	Counts map[string]int
}

// CustomRule defines a single repo-specific or local custom regex scrubbing rule.
type CustomRule struct {
	Name    string `yaml:"name" json:"name"`
	Pattern string `yaml:"pattern" json:"pattern"`
	Replace string `yaml:"replace" json:"replace"`
}

// ScrubRulesConfig represents the schema of .memory/scrub.yaml and scrub.local.yaml.
type ScrubRulesConfig struct {
	Version int          `yaml:"version" json:"version"`
	Rules   []CustomRule `yaml:"rules" json:"rules"`
}

var (
	// Layer 1 structural regexes.
	privateKeyBlockRe  = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	privateKeyHeaderRe = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	credURLRe          = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:)([^@\s/]+)(@)`)
	authTokenRe        = regexp.MustCompile(`(?i)\b(authorization:\s*(?:bearer|token|basic)\s+|(?:bearer|token)\s+)([A-Za-z0-9\-._~+/]{20,}=*)`)
	envSecretRe        = regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API_KEY|PRIVATE_KEY|ACCESS_KEY|CLIENT_SECRET)[A-Z0-9_]*\s*[:=]\s*["']?)([^\s"']{8,})(["']?)`)

	awsAccessKeyRe = regexp.MustCompile(`(?:AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`)
	githubTokenRe  = regexp.MustCompile(`(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}`)
	slackTokenRe   = regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)
	gcpAPIKeyRe    = regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)
	openAIAnthKey  = regexp.MustCompile(`sk-(?:ant-)?[A-Za-z0-9\-_]{20,}`)

	// Layer 2 identifier preservation & entropy/hex regexes.
	uuidRe          = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	uuidExactRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	entryIDExactRe  = regexp.MustCompile(`^(?:dec|inv|got|fail|rul|pat|pit|fix|not|doc|sem|prc|epi)-[0-9a-zA-Z]+$`)
	ulidExactRe     = regexp.MustCompile(`^01[0-9A-HJKMNP-TV-Z]{24}$`)
	secretLabelRe   = regexp.MustCompile(`(?i)(?:^|[^a-z])(secret|token|key|password|passwd|apikey|api_key|bearer|auth)(?:[^a-z]|$)`)
	hexCandidateRe   = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
	gitContextWordRe = regexp.MustCompile(`(?i)\b(?:commit|sha|rev|head|base|cherry-pick|merge|tag|object|blob|tree|parent)\b`)
	entryPrefixRe    = regexp.MustCompile(`(?i)(?:dec|inv|got|fail|rul|pat|pit|fix|not|doc|sem|prc|epi|note)-$`)

	// Layer 4 blob stripping regexes (note: Go regexp caps {n,m} quantifiers at 1000).
	dataURIBlobRe    = regexp.MustCompile(`data:[a-zA-Z0-9/+.-]+;base64,[A-Za-z0-9+/=]{256,}`)
	contiguousBlobRe = regexp.MustCompile(`[A-Za-z0-9+/=]{512,}[A-Za-z0-9+/=]{512,}`)
)

type compiledCustomRule struct {
	name    string
	re      *regexp.Regexp
	replace string
}

// Scrub applies the 4-layer secret and blob scrubber to text under the given Context.
// If any custom rule file is malformed, contains forbidden weakening keys (allow/disable/whitelist),
// or has an invalid regular expression, Scrub fails closed and returns a non-nil error.
func Scrub(text string, ctx Context) (Result, error) {
	counts := make(map[string]int)

	// Load and validate Layer 3 custom rules up front so invalid configs always fail closed.
	customRules, err := loadCustomRules(ctx)
	if err != nil {
		return Result{}, err
	}

	if text == "" {
		return Result{Text: "", Counts: counts}, nil
	}

	out := text

	// Layer 1: Structural rules (always on, cannot be disabled).
	out = replaceAndCount(out, privateKeyBlockRe, "«secret:private_key»", counts, "private_key")
	out = replaceAndCount(out, privateKeyHeaderRe, "«secret:private_key»", counts, "private_key")

	out = credURLRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := credURLRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		if strings.HasPrefix(sub[2], "«secret:") {
			return m
		}
		counts["url_cred"]++
		return sub[1] + "«secret:url_cred»" + sub[3]
	})

	out = authTokenRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := authTokenRe.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		val := sub[2]
		if isAllowedID(val, ctx.AllowedIDs) || entryIDExactRe.MatchString(val) || ulidExactRe.MatchString(val) {
			return m
		}
		counts["token"]++
		return sub[1] + "«secret:token»"
	})

	// Known provider token prefixes.
	out = replaceAndCount(out, awsAccessKeyRe, "«secret:aws»", counts, "aws")
	out = replaceAndCount(out, githubTokenRe, "«secret:github»", counts, "github")
	out = replaceAndCount(out, slackTokenRe, "«secret:slack»", counts, "slack")
	out = replaceAndCount(out, gcpAPIKeyRe, "«secret:gcp_api_key»", counts, "gcp_api_key")
	out = replaceAndCount(out, openAIAnthKey, "«secret:api_key»", counts, "api_key")

	// Env assignments with secret names.
	out = envSecretRe.ReplaceAllStringFunc(out, func(m string) string {
		if strings.Contains(m, "«") || strings.Contains(m, "»") {
			return m
		}
		sub := envSecretRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		val := sub[2]
		// Defer UUIDs to Layer 2 so AllowedIDs are preserved and secret-labelled UUIDs become «secret:uuid».
		if uuidExactRe.MatchString(val) {
			return m
		}
		if isAllowedID(val, ctx.AllowedIDs) || entryIDExactRe.MatchString(val) || ulidExactRe.MatchString(val) {
			return m
		}
		counts["env"]++
		return sub[1] + "«secret:env»" + sub[3]
	})

	// Layer 2: Identifier preservation & Entropy/Hex.
	out = scrubUUIDs(out, ctx, counts)
	out = scrubHexStrings(out, ctx, counts)

	// Layer 3: Repo-specific custom regex rules.
	for _, rule := range customRules {
		matches := rule.re.FindAllStringIndex(out, -1)
		if len(matches) > 0 {
			counts["custom:"+rule.name] += len(matches)
			out = rule.re.ReplaceAllString(out, rule.replace)
		}
	}

	// Layer 4: Blob stripping.
	out = replaceAndCount(out, dataURIBlobRe, "«blob:stripped»", counts, "blob")
	out = replaceAndCount(out, contiguousBlobRe, "«blob:stripped»", counts, "blob")

	return Result{
		Text:   out,
		Counts: counts,
	}, nil
}

func replaceAndCount(s string, re *regexp.Regexp, repl string, counts map[string]int, key string) string {
	matches := re.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	counts[key] += len(matches)
	return re.ReplaceAllString(s, repl)
}

func isAllowedID(id string, allowed map[string]bool) bool {
	if len(allowed) == 0 || id == "" {
		return false
	}
	if allowed[id] || allowed[strings.ToLower(id)] {
		return true
	}
	return false
}

func scrubUUIDs(s string, ctx Context, counts map[string]int) string {
	matches := uuidRe.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s
	}

	var b strings.Builder
	lastIdx := 0
	for _, loc := range matches {
		start, end := loc[0], loc[1]
		b.WriteString(s[lastIdx:start])
		candidate := s[start:end]

		// Defer >= 1024-byte contiguous blobs to Layer 4.
		if len(candidate) >= 1024 {
			b.WriteString(candidate)
			lastIdx = end
			continue
		}

		if isAllowedID(candidate, ctx.AllowedIDs) {
			b.WriteString(candidate)
		} else {
			lineStart := strings.LastIndexByte(s[:start], '\n') + 1
			winStart := lineStart
			if start-winStart > 40 {
				winStart = start - 40
			}
			preceding := s[winStart:start]
			if secretLabelRe.MatchString(preceding) {
				counts["uuid"]++
				b.WriteString("«secret:uuid»")
			} else {
				b.WriteString(candidate)
			}
		}
		lastIdx = end
	}
	b.WriteString(s[lastIdx:])
	return b.String()
}

func scrubHexStrings(s string, ctx Context, counts map[string]int) string {
	matches := hexCandidateRe.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s
	}

	gitVerifiedCache := make(map[string]bool)

	var b strings.Builder
	lastIdx := 0
	for _, loc := range matches {
		start, end := loc[0], loc[1]
		b.WriteString(s[lastIdx:start])
		candidate := s[start:end]

		// Defer >= 1024-byte contiguous blobs to Layer 4.
		if len(candidate) >= 1024 {
			b.WriteString(candidate)
			lastIdx = end
			continue
		}

		// Preserve if part of an entry ID prefix or ULID or in AllowedIDs.
		prefixStart := start - 8
		if prefixStart < 0 {
			prefixStart = 0
		}
		if isAllowedID(candidate, ctx.AllowedIDs) ||
			ulidExactRe.MatchString(candidate) ||
			entryPrefixRe.MatchString(s[prefixStart:start]) {
			b.WriteString(candidate)
			lastIdx = end
			continue
		}

		// Preserve 40-char git commit SHAs if in git context on the same line or valid git object in RepoRoot.
		if len(candidate) == 40 {
			lineStart := strings.LastIndexByte(s[:start], '\n') + 1
			lineEnd := strings.IndexByte(s[end:], '\n')
			if lineEnd < 0 {
				lineEnd = len(s)
			} else {
				lineEnd = end + lineEnd
			}
			lineText := s[lineStart:lineEnd]
			if gitContextWordRe.MatchString(lineText) {
				b.WriteString(candidate)
				lastIdx = end
				continue
			}

			lowerSHA := strings.ToLower(candidate)
			validObj, checked := gitVerifiedCache[lowerSHA]
			if !checked {
				validObj = isGitObject(ctx.RepoRoot, lowerSHA)
				gitVerifiedCache[lowerSHA] = validObj
			}
			if validObj {
				b.WriteString(candidate)
				lastIdx = end
				continue
			}
		}

		counts["hex"]++
		b.WriteString("«secret:hex»")
		lastIdx = end
	}
	b.WriteString(s[lastIdx:])
	return b.String()
}

func isGitObject(repoRoot, sha string) bool {
	if strings.TrimSpace(repoRoot) == "" || len(sha) != 40 {
		return false
	}
	cctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(cctx, "git", "-C", repoRoot, "cat-file", "-e", sha+"^{object}")
	return cmd.Run() == nil
}

func loadCustomRules(ctx Context) ([]compiledCustomRule, error) {
	var paths []string
	if strings.TrimSpace(ctx.RepoRoot) != "" {
		paths = append(paths, filepath.Join(ctx.RepoRoot, ".memory", "scrub.yaml"))
	}
	if strings.TrimSpace(ctx.DataDir) != "" {
		if strings.TrimSpace(ctx.RepoID) != "" {
			paths = append(paths, filepath.Join(ctx.DataDir, ctx.RepoID, "scrub.local.yaml"))
		} else {
			paths = append(paths, filepath.Join(ctx.DataDir, "scrub.local.yaml"))
		}
	}

	var compiled []compiledCustomRule
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading scrub config %s: %w", p, err)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			continue
		}

		var rawMap map[string]any
		if err := yaml.Unmarshal(data, &rawMap); err != nil {
			return nil, fmt.Errorf("parsing scrub config %s: %w", p, err)
		}
		if err := checkForbiddenKeys(rawMap); err != nil {
			return nil, fmt.Errorf("invalid scrub config %s: %w", p, err)
		}

		var cfg ScrubRulesConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshaling scrub rules %s: %w", p, err)
		}

		for _, r := range cfg.Rules {
			name := strings.TrimSpace(r.Name)
			if name == "" {
				name = "unnamed"
			}
			if strings.TrimSpace(r.Pattern) == "" {
				return nil, fmt.Errorf("scrub rule %q in %s has empty pattern", name, p)
			}
			re, err := regexp.Compile(r.Pattern)
			if err != nil {
				return nil, fmt.Errorf("compiling regex for rule %q in %s: %w", name, p, err)
			}
			repl := r.Replace
			if repl == "" {
				repl = "«secret:" + name + "»"
			}
			compiled = append(compiled, compiledCustomRule{
				name:    name,
				re:      re,
				replace: repl,
			})
		}
	}

	return compiled, nil
}

func checkForbiddenKeys(v any) error {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			lower := strings.ToLower(strings.TrimSpace(k))
			switch lower {
			case "allow", "allows", "allowlist", "disable", "disabled", "whitelist":
				return fmt.Errorf("forbidden key %q (custom configs may only add rules)", k)
			}
			if err := checkForbiddenKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range val {
			if err := checkForbiddenKeys(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// QuarantinePayload writes an un-scrubbable payload to <dataDir>/<repoID>/quarantine/<timestamp>-<boundary>.quarantine
// with permissions 0600 and a header line "# reason: <reason>" followed by raw.
func QuarantinePayload(dataDir, repoID, boundary, reason string, raw []byte) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("dataDir is required for quarantine")
	}
	cleanBoundary := strings.TrimSpace(boundary)
	if cleanBoundary == "" {
		cleanBoundary = "unknown"
	}
	cleanBoundary = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, cleanBoundary)

	qDir := filepath.Join(dataDir, repoID, "quarantine")
	if err := os.MkdirAll(qDir, 0700); err != nil {
		return "", fmt.Errorf("creating quarantine dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405.000000000Z")
	filename := fmt.Sprintf("%s-%s.quarantine", ts, cleanBoundary)
	fullPath := filepath.Join(qDir, filename)

	cleanReason := strings.ReplaceAll(strings.TrimSpace(reason), "\n", " ")
	header := fmt.Sprintf("# reason: %s\n", cleanReason)
	buf := make([]byte, 0, len(header)+len(raw))
	buf = append(buf, []byte(header)...)
	buf = append(buf, raw...)

	if err := os.WriteFile(fullPath, buf, 0600); err != nil {
		return "", fmt.Errorf("writing quarantine file: %w", err)
	}
	_ = os.Chmod(fullPath, 0600)
	return fullPath, nil
}
