/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/managed_block.go
 * Description: Management, validation, atomic insertion, and removal of the
 *              repo-memory v2 managed instruction block inside GEMINI.md.
 *
 * Exported Types / Functions:
 * - InstructionFilename:      Target instruction filename ("GEMINI.md").
 * - ManagedBlockBeginV2:      Opening HTML comment marker for the v2 managed block.
 * - ManagedBlockBeginPrefix:  Prefix used to detect any repo-memory begin marker.
 * - ManagedBlockEnd:          Closing HTML comment marker for the managed block.
 * - LegacyMapMention:         Legacy v1 map reference string replaced during upgrade.
 * - CanonicalManagedBlockV2(): Returns the canonical v2 managed instruction block for GEMINI.md.
 * - ValidateInstructionBlock(): Validates balanced and ordered managed block markers in GEMINI.md.
 * - EnsureInstructionBlock():  Inserts or updates the canonical v2 managed block in GEMINI.md.
 * - RemoveInstructionBlock():  Removes the managed instruction block from GEMINI.md on unenroll.
 */

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// InstructionFilename is the repository instruction file managed during enrolment.
	InstructionFilename = "GEMINI.md"
	// ManagedBlockBeginV2 is the opening HTML comment marker for the v2 managed block.
	ManagedBlockBeginV2 = "<!-- repo-memory:begin v2 — managed by repo-memory; edits inside are overwritten -->"
	// ManagedBlockBeginPrefix is the prefix used to match any version of the begin marker.
	ManagedBlockBeginPrefix = "<!-- repo-memory:begin"
	// ManagedBlockEnd is the closing HTML comment marker for the managed block.
	ManagedBlockEnd = "<!-- repo-memory:end -->"
	// LegacyMapMention is the v1 memory map include replaced by the v2 managed block.
	LegacyMapMention = "@[memory map](.memory/views/MAP.md)"
)

// CanonicalManagedBlockV2 returns the canonical v2 managed instruction block for GEMINI.md.
func CanonicalManagedBlockV2() string {
	return ManagedBlockBeginV2 + "\n" +
		"@[memory atlas](.memory/ATLAS.md)\n\n" +
		"## Repository Memory (`repo-memory`)\n" +
		"- **Pushed context (`[repo-memory ...]`):** Hooks automatically inject verified invariants, gotchas, failure signatures, and ruled-out approaches when you start a task or touch anchored files. Treat them as repository notes verified against the code; do not repeat or echo them as new discoveries.\n" +
		"- **Pull for depth (MCP):** Use `memory_brief` (module dossier), `memory_why` (entry/file/line rationale), `memory_history` (evolution), and `memory_search` (FTS5 search) when exploring unfamiliar subsystems.\n" +
		"- **Distillation:** End-of-turn capture happens automatically via the `Stop` hook on milestone turns (no action required). Use `memory_note` mid-turn only when you want to record a critical discovery or dead end immediately.\n" +
		ManagedBlockEnd
}

// ValidateInstructionBlock validates the repo-memory managed block markers in GEMINI.md content.
// It returns nil if no markers are present or if a single well-formed begin/end pair is present.
func ValidateInstructionBlock(content string) error {
	// --- Step 1: Count begin and end markers and verify cardinality ---
	beginCount := strings.Count(content, ManagedBlockBeginPrefix)
	endCount := strings.Count(content, ManagedBlockEnd)

	if beginCount == 0 && endCount == 0 {
		return nil
	}
	if beginCount > 1 {
		return fmt.Errorf("duplicate repo-memory:begin markers in GEMINI.md")
	}
	if endCount > 1 {
		return fmt.Errorf("duplicate repo-memory:end markers in GEMINI.md")
	}
	if beginCount != endCount {
		return fmt.Errorf("unbalanced repo-memory managed block markers in GEMINI.md")
	}

	// --- Step 2: Verify marker ordering and HTML comment closure ---
	beginIdx := strings.Index(content, ManagedBlockBeginPrefix)
	endIdx := strings.Index(content, ManagedBlockEnd)
	if endIdx < beginIdx {
		return fmt.Errorf("inverted repo-memory managed block markers in GEMINI.md")
	}

	// Ensure the begin HTML comment closes before the end marker.
	restAfterBegin := content[beginIdx+len(ManagedBlockBeginPrefix):]
	closeCommentRel := strings.Index(restAfterBegin, "-->")
	if closeCommentRel < 0 || beginIdx+len(ManagedBlockBeginPrefix)+closeCommentRel >= endIdx {
		return fmt.Errorf("unbalanced repo-memory managed block markers in GEMINI.md")
	}

	return nil
}

// EnsureInstructionBlock ensures GEMINI.md in repoRoot contains the canonical v2 managed instruction block.
// It returns createdFile=true if GEMINI.md did not previously exist and was created by this call.
func EnsureInstructionBlock(repoRoot string) (createdFile bool, err error) {
	// --- Step 1: Read existing GEMINI.md or create a new file with the canonical block ---
	path := filepath.Join(repoRoot, InstructionFilename)
	block := CanonicalManagedBlockV2()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := writeInstructionFileAtomic(path, block+"\n"); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	// --- Step 2: Validate existing managed block markers ---
	content := string(data)
	if err := ValidateInstructionBlock(content); err != nil {
		return false, err
	}

	// --- Step 3: Replace existing block, upgrade legacy mention, or append block ---
	var updated string
	beginIdx := strings.Index(content, ManagedBlockBeginPrefix)
	if beginIdx >= 0 {
		endIdx := strings.Index(content, ManagedBlockEnd)
		updated = content[:beginIdx] + block + content[endIdx+len(ManagedBlockEnd):]
	} else if strings.Contains(content, LegacyMapMention) {
		updated = strings.Replace(content, LegacyMapMention, block, 1)
	} else if content == "" {
		updated = block + "\n"
	} else if strings.HasSuffix(content, "\n\n") {
		updated = content + block + "\n"
	} else if strings.HasSuffix(content, "\n") {
		updated = content + "\n" + block + "\n"
	} else {
		updated = content + "\n\n" + block
	}

	// --- Step 4: Atomically persist changes when content differs ---
	if updated != content {
		if err := writeInstructionFileAtomic(path, updated); err != nil {
			return false, err
		}
	}

	return false, nil
}

// RemoveInstructionBlock removes the repo-memory managed instruction block (and any bare legacy map include)
// from GEMINI.md in repoRoot. If the remaining content is empty and createdByUs is true, GEMINI.md is deleted.
func RemoveInstructionBlock(repoRoot string, createdByUs bool) error {
	// --- Step 1: Read and validate GEMINI.md ---
	path := filepath.Join(repoRoot, InstructionFilename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	content := string(data)
	if err := ValidateInstructionBlock(content); err != nil {
		return err
	}

	// --- Step 2: Excise the managed block and normalize surrounding newlines ---
	remaining := content
	beginIdx := strings.Index(remaining, ManagedBlockBeginPrefix)
	if beginIdx >= 0 {
		endIdx := strings.Index(remaining, ManagedBlockEnd)
		before := remaining[:beginIdx]
		after := remaining[endIdx+len(ManagedBlockEnd):]

		switch {
		case before == "":
			if strings.HasPrefix(after, "\r\n") {
				after = after[2:]
			} else if strings.HasPrefix(after, "\n") {
				after = after[1:]
			}
			if strings.HasPrefix(after, "\r\n") {
				after = after[2:]
			} else if strings.HasPrefix(after, "\n") {
				after = after[1:]
			}
			remaining = after
		case after == "\n" && strings.HasSuffix(before, "\n\n"):
			remaining = before[:len(before)-1]
		case after == "\r\n" && strings.HasSuffix(before, "\r\n\r\n"):
			remaining = before[:len(before)-2]
		case after == "" && strings.HasSuffix(before, "\n\n"):
			remaining = before[:len(before)-2]
		case after == "" && strings.HasSuffix(before, "\r\n\r\n"):
			remaining = before[:len(before)-4]
		default:
			if strings.HasSuffix(before, "\n\n") && strings.HasPrefix(after, "\n") {
				remaining = before[:len(before)-1] + after
			} else {
				remaining = before + after
			}
		}
	}

	// --- Step 3: Strip any residual legacy map mentions ---
	if strings.Contains(remaining, LegacyMapMention) {
		lines := strings.Split(remaining, "\n")
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line) == LegacyMapMention {
				continue
			}
			filtered = append(filtered, line)
		}
		remaining = strings.Join(filtered, "\n")
	}

	// --- Step 4: Delete or atomically update GEMINI.md ---
	if strings.TrimSpace(remaining) == "" {
		if createdByUs {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing %s: %w", path, err)
			}
			return nil
		}
		return writeInstructionFileAtomic(path, "")
	}

	if remaining != content {
		return writeInstructionFileAtomic(path, remaining)
	}
	return nil
}

// writeInstructionFileAtomic writes content to a PID-scoped temporary file and
// renames it over path.
func writeInstructionFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	tmpFile := filepath.Join(dir, fmt.Sprintf(".gemini-md-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing temporary instruction file %s: %w", tmpFile, err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming temporary instruction file to %s: %w", path, err)
	}
	return nil
}
