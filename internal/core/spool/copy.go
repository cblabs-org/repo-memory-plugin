/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/spool/copy.go
 * Description: Implements incremental, prefix-verified transcript copying into
 *              the spool queue directory with persistent cursor state.
 *
 * Exported Types / Functions:
 * - CopyState:                   Progress and MD5 prefix cursor persisted in state/.
 * - CopyResult:                  Summary metrics of an incremental or full transcript copy.
 * - LoadCopyState():             Reads a persisted CopyState cursor from disk.
 * - SaveCopyState():             Atomically persists a CopyState cursor to disk.
 * - ResolvePrimaryTranscript():  Resolves ordered source files for a transcript path or directory.
 * - CopyFileIncremental():       Incrementally copies a single source file into dstPath.
 * - CopyTranscriptIncremental(): Resolves and incrementally copies a conversation transcript into queueDir.
 * - CopyFilesIncremental():      Incrementally copies one or more ordered source files with prefix MD5 verification.
 */

package spool

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CopyState stores the progress and integrity cursor for a transcript copy in state/.
type CopyState struct {
	ConversationID string    `json:"conversation_id"` // Identifier of the conversation being copied.
	SourcePath     string    `json:"source_path"`     // Comma-joined list of resolved source file paths.
	ByteOffset     int64     `json:"byte_offset"`     // Number of bytes copied so far.
	PrefixHash     string    `json:"prefix_hash"`     // MD5 hex digest of bytes [0:ByteOffset].
	CopiedAt       time.Time `json:"copied_at"`       // UTC timestamp when the cursor was last updated.
}

// CopyResult holds the outcome of an incremental or full transcript copy.
type CopyResult struct {
	ConversationID string `json:"conversation_id"` // Identifier of the copied conversation.
	SourcePath     string `json:"source_path"`     // Comma-joined list of source file paths read.
	DestPath       string `json:"dest_path"`       // Destination file path in the queue directory.
	BytesCopied    int64  `json:"bytes_copied"`    // Number of bytes written during this copy pass.
	TotalBytes     int64  `json:"total_bytes"`     // Total byte size of the destination file after copy.
	FullCopy       bool   `json:"full_copy"`       // True if a full copy was performed instead of an incremental append.
	PrefixHash     string `json:"prefix_hash"`     // Updated MD5 hex digest of the full destination file.
}

// LoadCopyState reads the copy state cursor from disk.
// It returns nil, nil if the state file does not exist.
func LoadCopyState(stateFilePath string) (*CopyState, error) {
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading copy state %s: %w", stateFilePath, err)
	}

	var state CopyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing copy state %s: %w", stateFilePath, err)
	}

	return &state, nil
}

// SaveCopyState atomically writes the copy state cursor to disk.
func SaveCopyState(stateFilePath string, state *CopyState) error {
	// --- Step 1: Ensure state directory exists and marshal state JSON ---
	dir := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling copy state: %w", err)
	}

	// --- Step 2: Write and flush state payload to a sibling temporary file ---
	tmpFile, err := os.CreateTemp(dir, ".state_tmp_*")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing state data: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing state data: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing state file: %w", err)
	}

	// --- Step 3: Atomically rename temporary state file into place ---
	if err := atomicRename(tmpPath, stateFilePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp state file to %s: %w", stateFilePath, err)
	}

	return nil
}

// ResolvePrimaryTranscript inspects a transcript file or conversation directory and returns
// the slice of files representing the primary transcript source.
// It prioritizes transcript_full.jsonl, then chunks, then transcript.jsonl.
func ResolvePrimaryTranscript(srcDirOrFile string) ([]string, error) {
	// --- Step 1: Stat input path and upgrade compact file path to full transcript if sibling exists ---
	fi, err := os.Stat(srcDirOrFile)
	if err != nil {
		return nil, fmt.Errorf("stat transcript path %s: %w", srcDirOrFile, err)
	}

	if !fi.IsDir() {
		dir := filepath.Dir(srcDirOrFile)
		base := filepath.Base(srcDirOrFile)
		if base == "transcript.jsonl" {
			fullPath := filepath.Join(dir, "transcript_full.jsonl")
			if _, err := os.Stat(fullPath); err == nil {
				return []string{fullPath}, nil
			}
		}
		return []string{srcDirOrFile}, nil
	}

	// --- Step 2: Descend into .system_generated/logs if present ---
	sysLogs := filepath.Join(srcDirOrFile, ".system_generated", "logs")
	if sysStat, err := os.Stat(sysLogs); err == nil && sysStat.IsDir() {
		return ResolvePrimaryTranscript(sysLogs)
	}

	// --- Step 3: Check for monolithic transcript_full.jsonl ---
	fullPath := filepath.Join(srcDirOrFile, "transcript_full.jsonl")
	if _, err := os.Stat(fullPath); err == nil {
		return []string{fullPath}, nil
	}

	// --- Step 4: Check for ordered chunk files under chunks/transcript_full/ ---
	chunksDir := filepath.Join(srcDirOrFile, "chunks", "transcript_full")
	if entries, err := os.ReadDir(chunksDir); err == nil {
		var chunks []string
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
				chunks = append(chunks, filepath.Join(chunksDir, entry.Name()))
			}
		}
		if len(chunks) > 0 {
			sort.Strings(chunks)
			return chunks, nil
		}
	}

	// --- Step 5: Fall back to compact transcript.jsonl ---
	compactPath := filepath.Join(srcDirOrFile, "transcript.jsonl")
	if _, err := os.Stat(compactPath); err == nil {
		return []string{compactPath}, nil
	}

	return nil, fmt.Errorf("no transcript source found in %s", srcDirOrFile)
}

// CopyFileIncremental performs an incremental or full copy of a single source file to dstPath,
// storing cursor state in stateFilePath.
func CopyFileIncremental(srcPath, dstPath, stateFilePath, conversationID string) (*CopyResult, error) {
	return CopyFilesIncremental([]string{srcPath}, dstPath, stateFilePath, conversationID)
}

// CopyTranscriptIncremental resolves the primary transcript for conversationID from srcDirOrFile,
// and incrementally copies it into queueDir (<queueDir>/<conversationID>.jsonl),
// tracking progress in stateDir (<stateDir>/cursor_<conversationID>.json).
func CopyTranscriptIncremental(conversationID, srcDirOrFile, queueDir, stateDir string) (*CopyResult, error) {
	if conversationID == "" {
		return nil, errors.New("conversationID cannot be empty")
	}
	srcFiles, err := ResolvePrimaryTranscript(srcDirOrFile)
	if err != nil {
		return nil, err
	}
	dstPath := filepath.Join(queueDir, fmt.Sprintf("%s.jsonl", conversationID))
	stateFilePath := filepath.Join(stateDir, fmt.Sprintf("cursor_%s.json", conversationID))
	return CopyFilesIncremental(srcFiles, dstPath, stateFilePath, conversationID)
}

// CopyFilesIncremental performs an incremental copy across one or more source files (e.g. chunks)
// into dstPath. It verifies the prefix MD5 hash of already copied bytes:
//   - If the prefix hash matches, only newly appended bytes are appended to dstPath.
//   - If the prefix hash mismatches, or the destination file is missing/corrupt, a full re-copy is performed.
func CopyFilesIncremental(srcFiles []string, dstPath, stateFilePath, conversationID string) (*CopyResult, error) {
	// --- Step 1: Compute total source size and load existing copy cursor ---
	if len(srcFiles) == 0 {
		return nil, errors.New("no source files provided")
	}

	var totalSize int64
	for _, f := range srcFiles {
		fi, err := os.Stat(f)
		if err != nil {
			return nil, fmt.Errorf("stat source file %s: %w", f, err)
		}
		totalSize += fi.Size()
	}

	existingState, err := LoadCopyState(stateFilePath)
	if err != nil {
		return nil, err
	}

	dstFi, dstErr := os.Stat(dstPath)

	// --- Step 2: Determine whether a full copy is required or prefix hash can be verified ---
	needsFullCopy := false
	if existingState == nil {
		needsFullCopy = true
	} else if dstErr != nil || dstFi.Size() < existingState.ByteOffset {
		needsFullCopy = true
	} else if totalSize < existingState.ByteOffset {
		needsFullCopy = true
	} else if existingState.ByteOffset <= 0 {
		needsFullCopy = true
	}

	sourceIdentifier := strings.Join(srcFiles, ",")

	var closers []io.ReadCloser
	var reader io.Reader
	hasher := md5.New()

	if !needsFullCopy {
		closers, reader, err = openReaders(srcFiles)
		if err != nil {
			return nil, fmt.Errorf("opening source files: %w", err)
		}

		n, err := io.CopyN(hasher, reader, existingState.ByteOffset)
		if err != nil || n != existingState.ByteOffset {
			needsFullCopy = true
			closeAll(closers)
		} else {
			prefixMD5 := hex.EncodeToString(hasher.Sum(nil))
			if prefixMD5 != existingState.PrefixHash {
				// Prefix hash mismatch: session was reset, branched, or rewritten.
				// Trigger full re-copy per SPEC §6.2.
				needsFullCopy = true
				closeAll(closers)
			}
		}
	}

	// --- Step 3: Execute full copy if cursor is absent, truncated, or prefix-mismatched ---
	if needsFullCopy {
		closers, reader, err = openReaders(srcFiles)
		if err != nil {
			return nil, fmt.Errorf("opening source files for full copy: %w", err)
		}
		defer closeAll(closers)

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return nil, fmt.Errorf("creating destination directory: %w", err)
		}

		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening destination file: %w", err)
		}

		hasher = md5.New()
		tee := io.TeeReader(reader, hasher)
		copied, err := io.CopyN(dstFile, tee, totalSize)
		if err != nil && !errors.Is(err, io.EOF) {
			dstFile.Close()
			return nil, fmt.Errorf("copying source to destination: %w", err)
		}

		if err := dstFile.Sync(); err != nil {
			dstFile.Close()
			return nil, fmt.Errorf("syncing destination file: %w", err)
		}
		if err := dstFile.Close(); err != nil {
			return nil, fmt.Errorf("closing destination file: %w", err)
		}

		newHash := hex.EncodeToString(hasher.Sum(nil))
		newState := &CopyState{
			ConversationID: conversationID,
			SourcePath:     sourceIdentifier,
			ByteOffset:     copied,
			PrefixHash:     newHash,
			CopiedAt:       time.Now().UTC(),
		}

		if err := SaveCopyState(stateFilePath, newState); err != nil {
			return nil, err
		}

		return &CopyResult{
			ConversationID: conversationID,
			SourcePath:     sourceIdentifier,
			DestPath:       dstPath,
			BytesCopied:    copied,
			TotalBytes:     copied,
			FullCopy:       true,
			PrefixHash:     newHash,
		}, nil
	}

	// --- Step 4: Append only newly written suffix bytes and update cursor ---
	defer closeAll(closers)

	bytesToCopy := totalSize - existingState.ByteOffset
	if bytesToCopy == 0 {
		return &CopyResult{
			ConversationID: conversationID,
			SourcePath:     sourceIdentifier,
			DestPath:       dstPath,
			BytesCopied:    0,
			TotalBytes:     totalSize,
			FullCopy:       false,
			PrefixHash:     existingState.PrefixHash,
		}, nil
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening destination file for append: %w", err)
	}

	// hasher already holds prefix bytes [0:existingState.ByteOffset].
	// tee feeds [existingState.ByteOffset:totalSize] into the same hasher,
	// resulting in hasher.Sum(nil) being the MD5 of the entire file.
	tee := io.TeeReader(reader, hasher)
	copied, err := io.CopyN(dstFile, tee, bytesToCopy)
	if err != nil && !errors.Is(err, io.EOF) {
		dstFile.Close()
		return nil, fmt.Errorf("appending new bytes to destination: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		dstFile.Close()
		return nil, fmt.Errorf("syncing destination file: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		return nil, fmt.Errorf("closing destination file: %w", err)
	}

	newTotalBytes := existingState.ByteOffset + copied
	newHash := hex.EncodeToString(hasher.Sum(nil))

	newState := &CopyState{
		ConversationID: conversationID,
		SourcePath:     sourceIdentifier,
		ByteOffset:     newTotalBytes,
		PrefixHash:     newHash,
		CopiedAt:       time.Now().UTC(),
	}

	if err := SaveCopyState(stateFilePath, newState); err != nil {
		return nil, err
	}

	return &CopyResult{
		ConversationID: conversationID,
		SourcePath:     sourceIdentifier,
		DestPath:       dstPath,
		BytesCopied:    copied,
		TotalBytes:     newTotalBytes,
		FullCopy:       false,
		PrefixHash:     newHash,
	}, nil
}

// openReaders opens all files in order and returns their closers and a combined io.MultiReader.
func openReaders(files []string) ([]io.ReadCloser, io.Reader, error) {
	// --- Step 1: Allocate closer and reader slices ---
	closers := make([]io.ReadCloser, len(files))
	readers := make([]io.Reader, len(files))

	// --- Step 2: Open each file sequentially, cleaning up prior handles on error ---
	for i, file := range files {
		f, err := os.Open(file)
		if err != nil {
			for j := 0; j < i; j++ {
				_ = closers[j].Close()
			}
			return nil, nil, err
		}
		closers[i] = f
		readers[i] = f
	}
	return closers, io.MultiReader(readers...), nil
}

// closeAll closes all non-nil io.ReadCloser handles in closers.
func closeAll(closers []io.ReadCloser) {
	for _, c := range closers {
		if c != nil {
			_ = c.Close()
		}
	}
}
