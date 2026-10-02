package extractor

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

var funcRegex = regexp.MustCompile(`^[\w\s\*&]+\s+(\w+)\s*\([^)]*\)\s*\{?$`)

func ExtractSymbols(path string) ([]entities.Entity, error) {
	ext := filepath.Ext(path)
	if ext != ".c" && ext != ".cpp" && ext != ".h" && ext != ".hpp" {
		return nil, nil
	}

	// Try ctags first
	if ctagsPath, err := exec.LookPath("ctags"); err == nil {
		cmd := exec.Command(ctagsPath, "-x", "--c-kinds=fp", path)
		out, err := cmd.Output()
		if err == nil {
			var ext []entities.Entity
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				parts := strings.Fields(scanner.Text())
				if len(parts) > 0 {
					ext = append(ext, entities.Entity{
						ID:   "symbol:" + parts[0],
						Kind: entities.KindSymbol,
						Name: parts[0],
					})
				}
			}
			return ext, nil
		}
	}

	// Fallback to naive brace-aware scanner
	return fallbackCScanner(path)
}

func fallbackCScanner(path string) ([]entities.Entity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var extracted []entities.Entity
	scanner := bufio.NewScanner(f)
	braceLevel := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// adjust brace level
		braceLevel += strings.Count(line, "{")
		braceLevel -= strings.Count(line, "}")

		if braceLevel == 0 || (braceLevel == 1 && strings.HasSuffix(line, "{")) {
			// Look for function signatures
			if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
				name := matches[1]
				if name != "if" && name != "for" && name != "while" && name != "switch" {
					extracted = append(extracted, entities.Entity{
						ID:   "symbol:" + name,
						Kind: entities.KindSymbol,
						Name: name,
					})
				}
			}
		}
	}
	return extracted, nil
}
