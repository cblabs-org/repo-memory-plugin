package delivery

import (
	"os"
	"path/filepath"
	"strings"
)

func VerifyAnchor(anchor string, workspaceRoot string) bool {
	if anchor == "" {
		return false
	}
	
	// Symbol anchors (e.g. file.go:FuncName)
	parts := strings.SplitN(anchor, ":", 2)
	filePath := parts[0]
	
	absPath := filepath.Join(workspaceRoot, filePath)
	_, err := os.Stat(absPath)
	return err == nil
}

func VerifyAnchors(anchors []string, workspaceRoot string) (valid []string, suspect []string) {
	for _, a := range anchors {
		if VerifyAnchor(a, workspaceRoot) {
			valid = append(valid, a)
		} else {
			suspect = append(suspect, a)
		}
	}
	return valid, suspect
}
