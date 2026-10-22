/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package spool

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyIncremental_InitialCopyAndAppend(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "transcript.jsonl")
	dstPath := filepath.Join(tempDir, "queue", "c1.jsonl")
	statePath := filepath.Join(tempDir, "state", "cursor_c1.json")

	part1 := "{\"step_index\":0,\"content\":\"hello world\"}\n"
	if err := os.WriteFile(srcPath, []byte(part1), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Initial copy
	res1, err := CopyFileIncremental(srcPath, dstPath, statePath, "c1")
	if err != nil {
		t.Fatalf("initial copy failed: %v", err)
	}

	if !res1.FullCopy {
		t.Error("expected initial copy to be full copy")
	}
	if res1.BytesCopied != int64(len(part1)) {
		t.Errorf("expected %d bytes copied, got %d", len(part1), res1.BytesCopied)
	}

	dstData1, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData1) != part1 {
		t.Errorf("destination content mismatch: got %q, want %q", string(dstData1), part1)
	}

	// 2. Append new bytes
	part2 := "{\"step_index\":1,\"content\":\"second turn\"}\n"
	f, err := os.OpenFile(srcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(part2); err != nil {
		t.Fatal(err)
	}
	f.Close()

	res2, err := CopyFileIncremental(srcPath, dstPath, statePath, "c1")
	if err != nil {
		t.Fatalf("incremental copy failed: %v", err)
	}

	if res2.FullCopy {
		t.Error("expected incremental copy, got full copy")
	}
	if res2.BytesCopied != int64(len(part2)) {
		t.Errorf("expected %d bytes copied incrementally, got %d", len(part2), res2.BytesCopied)
	}
	if res2.TotalBytes != int64(len(part1)+len(part2)) {
		t.Errorf("expected total bytes %d, got %d", len(part1)+len(part2), res2.TotalBytes)
	}

	dstData2, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData2) != part1+part2 {
		t.Errorf("destination content mismatch after append: got %q, want %q", string(dstData2), part1+part2)
	}

	// 3. No changes (idempotent)
	res3, err := CopyFileIncremental(srcPath, dstPath, statePath, "c1")
	if err != nil {
		t.Fatalf("idempotent copy failed: %v", err)
	}
	if res3.FullCopy {
		t.Error("expected FullCopy=false for unchanged file")
	}
	if res3.BytesCopied != 0 {
		t.Errorf("expected 0 bytes copied, got %d", res3.BytesCopied)
	}
}

func TestCopyIncremental_PrefixMismatchTriggersFullRecopy(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "transcript.jsonl")
	dstPath := filepath.Join(tempDir, "queue", "c2.jsonl")
	statePath := filepath.Join(tempDir, "state", "cursor_c2.json")

	initial := "{\"step_index\":0,\"content\":\"initial prompt\"}\n"
	if err := os.WriteFile(srcPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Initial copy
	if _, err := CopyFileIncremental(srcPath, dstPath, statePath, "c2"); err != nil {
		t.Fatal(err)
	}

	// Now rewrite the source file with completely different content (simulating branch reset)
	rewritten := "{\"step_index\":0,\"content\":\"reset session prompt\"}\n{\"step_index\":1,\"content\":\"more\"}\n"
	if err := os.WriteFile(srcPath, []byte(rewritten), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := CopyFileIncremental(srcPath, dstPath, statePath, "c2")
	if err != nil {
		t.Fatalf("copy after prefix mismatch failed: %v", err)
	}

	if !res.FullCopy {
		t.Error("expected FullCopy=true after prefix mismatch")
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData) != rewritten {
		t.Errorf("destination content mismatch after rewrite: got %q, want %q", string(dstData), rewritten)
	}

	// Verify state file matches new content
	state, err := LoadCopyState(statePath)
	if err != nil || state == nil {
		t.Fatalf("failed to load state: %v", err)
	}
	expectedHash := md5Hex([]byte(rewritten))
	if state.PrefixHash != expectedHash {
		t.Errorf("expected state prefix hash %s, got %s", expectedHash, state.PrefixHash)
	}
}

func TestCopyIncremental_TruncationTriggersFullRecopy(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "transcript.jsonl")
	dstPath := filepath.Join(tempDir, "queue", "c3.jsonl")
	statePath := filepath.Join(tempDir, "state", "cursor_c3.json")

	longContent := "{\"step_index\":0,\"content\":\"line 1\"}\n{\"step_index\":1,\"content\":\"line 2\"}\n"
	if err := os.WriteFile(srcPath, []byte(longContent), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := CopyFileIncremental(srcPath, dstPath, statePath, "c3"); err != nil {
		t.Fatal(err)
	}

	// Truncate source file
	shortContent := "{\"step\":0}\n"
	if err := os.WriteFile(srcPath, []byte(shortContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := CopyFileIncremental(srcPath, dstPath, statePath, "c3")
	if err != nil {
		t.Fatalf("copy after truncation failed: %v", err)
	}

	if !res.FullCopy {
		t.Error("expected FullCopy=true after truncation")
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData) != shortContent {
		t.Errorf("destination mismatch: got %q, want %q", string(dstData), shortContent)
	}
}

func TestCopyIncremental_DestinationDeletedTriggersFullRecopy(t *testing.T) {
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "transcript.jsonl")
	dstPath := filepath.Join(tempDir, "queue", "c4.jsonl")
	statePath := filepath.Join(tempDir, "state", "cursor_c4.json")

	content := "{\"step_index\":0,\"content\":\"important\"}\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := CopyFileIncremental(srcPath, dstPath, statePath, "c4"); err != nil {
		t.Fatal(err)
	}

	// Remove destination
	if err := os.Remove(dstPath); err != nil {
		t.Fatal(err)
	}

	res, err := CopyFileIncremental(srcPath, dstPath, statePath, "c4")
	if err != nil {
		t.Fatalf("copy after destination deletion failed: %v", err)
	}

	if !res.FullCopy {
		t.Error("expected FullCopy=true when destination was missing")
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData) != content {
		t.Errorf("destination mismatch: got %q, want %q", string(dstData), content)
	}
}

func TestResolvePrimaryTranscript(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1")

	// Full normal monolithic
	fullNormalDir := filepath.Join(fixturesDir, "full_normal")
	files, err := ResolvePrimaryTranscript(fullNormalDir)
	if err != nil {
		t.Fatalf("resolve full_normal failed: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "transcript_full.jsonl" {
		t.Errorf("expected transcript_full.jsonl, got %v", files)
	}

	// Full chunked
	chunkedDir := filepath.Join(fixturesDir, "full_chunked")
	chunks, err := ResolvePrimaryTranscript(chunkedDir)
	if err != nil {
		t.Fatalf("resolve full_chunked failed: %v", err)
	}
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunk files, got %d (%v)", len(chunks), chunks)
	}

	// Compact fallback
	compactDir := filepath.Join(fixturesDir, "compact")
	compactFiles, err := ResolvePrimaryTranscript(compactDir)
	if err != nil {
		t.Fatalf("resolve compact failed: %v", err)
	}
	if len(compactFiles) != 1 || filepath.Base(compactFiles[0]) != "transcript.jsonl" {
		t.Errorf("expected transcript.jsonl, got %v", compactFiles)
	}

	// Pass transcript.jsonl when transcript_full.jsonl exists in the same dir
	fullDirect := filepath.Join(fullNormalDir, "transcript.jsonl")
	// Write dummy transcript.jsonl alongside transcript_full.jsonl in a temp dir to test upgrade
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "transcript.jsonl"), []byte("compact"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "transcript_full.jsonl"), []byte("full"), 0644)

	upgraded, err := ResolvePrimaryTranscript(filepath.Join(tempDir, "transcript.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(upgraded) != 1 || filepath.Base(upgraded[0]) != "transcript_full.jsonl" {
		t.Errorf("expected upgrade to transcript_full.jsonl, got %v", upgraded)
	}
	_ = fullDirect
}

func TestCopyTranscriptIncremental_Integration(t *testing.T) {
	tempDir := t.TempDir()
	queueDir := filepath.Join(tempDir, "queue")
	stateDir := filepath.Join(tempDir, "state")

	chunkedDir := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1", "full_chunked")
	res, err := CopyTranscriptIncremental("conv-chunked", chunkedDir, queueDir, stateDir)
	if err != nil {
		t.Fatalf("CopyTranscriptIncremental failed: %v", err)
	}

	if !res.FullCopy {
		t.Error("expected FullCopy=true for first chunked copy")
	}

	dstFile := filepath.Join(queueDir, "conv-chunked.jsonl")
	dstData, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(dstData) != int(res.TotalBytes) {
		t.Errorf("expected destination size %d, got %d", res.TotalBytes, len(dstData))
	}
}

func md5Hex(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}
