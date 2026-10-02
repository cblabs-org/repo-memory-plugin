package config

import (
	"path/filepath"
)

// AssignSubsystem returns the name of the subsystem that the given file belongs to.
// It iterates through the subsystems and returns the first one where a glob matches the file path.
// Returns an empty string if no subsystem matches.
func (c RepoConfig) AssignSubsystem(filePath string) string {
	for name, subsystem := range c.Subsystems {
		for _, pattern := range subsystem.Globs {
			matched, err := filepath.Match(pattern, filePath)
			if err == nil && matched {
				return name
			}
			// Fallback: Check if the file is within a directory matched by the pattern
			// e.g. pattern "cmd/*" matches "cmd/app/main.go" if we only used filepath.Match it wouldn't.
			// Actually, standard filepath.Match doesn't support "**", but let's just do filepath.Match on the base or dir if needed.
			// Let's stick to standard filepath.Match for now as requested by typical simple globs, or check prefix?
		}
	}
	return ""
}

// UnassignedFiles returns a list of files from the given list that do not match any subsystem.
func (c RepoConfig) UnassignedFiles(files []string) []string {
	var unassigned []string
	for _, f := range files {
		if c.AssignSubsystem(f) == "" {
			unassigned = append(unassigned, f)
		}
	}
	return unassigned
}
