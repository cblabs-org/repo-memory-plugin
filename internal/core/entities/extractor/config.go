package extractor

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// ParseConfig parses config files and lockfiles for config entities.
func ParseConfig(path string) ([]entities.Entity, error) {
	var extracted []entities.Entity
	base := filepath.Base(path)

	// Check if it's a known config file
	isConfig := false
	if base == "sdkconfig" || base == "Kconfig" || base == "platformio.ini" || strings.HasSuffix(base, ".lock") || base == "go.mod" {
		isConfig = true
	}
	if !isConfig {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// For sdkconfig/Kconfig/platformio: usually KEY=VALUE or KEY = VALUE
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key != "" {
				extracted = append(extracted, entities.Entity{
					ID:   "config:" + key,
					Kind: entities.KindConfig,
					Name: key,
				})
			}
		} else if strings.HasPrefix(line, "require ") { // very naive go.mod support
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				extracted = append(extracted, entities.Entity{
					ID:   "config:" + parts[1],
					Kind: entities.KindConfig,
					Name: parts[1],
				})
			}
		}
	}
	return extracted, nil
}
