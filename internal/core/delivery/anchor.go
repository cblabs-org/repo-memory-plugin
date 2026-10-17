/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/anchor.go
 * Description: Verifies repository file and symbol anchors against the local
 *              workspace filesystem to prevent stale or phantom entries from
 *              being injected.
 *
 * Exported Types / Functions:
 * - VerifyAnchor():  Checks whether a single file or symbol anchor exists on disk.
 * - VerifyAnchors(): Partitions a slice of anchors into valid and suspect lists.
 */

package delivery

import (
	"os"
	"path/filepath"
	"strings"
)

// VerifyAnchor reports whether the file referenced by anchor (including symbol
// anchors of the form "file.go:FuncName") exists under workspaceRoot.
func VerifyAnchor(anchor string, workspaceRoot string) bool {
	if anchor == "" {
		return false
	}

	// Strip optional ":SymbolName" suffix to check the underlying file path on disk.
	parts := strings.SplitN(anchor, ":", 2)
	filePath := parts[0]

	absPath := filepath.Join(workspaceRoot, filePath)
	_, err := os.Stat(absPath)
	return err == nil
}

// VerifyAnchors partitions anchors into valid (existing under workspaceRoot)
// and suspect (missing on disk) slices.
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
