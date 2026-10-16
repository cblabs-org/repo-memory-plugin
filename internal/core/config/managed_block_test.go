package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedBlock_CreateAndDeleteWhenCreatedByUs(t *testing.T) {
	repoDir := t.TempDir()
	geminiPath := filepath.Join(repoDir, InstructionFilename)

	created, err := EnsureInstructionBlock(repoDir)
	if err != nil {
		t.Fatalf("EnsureInstructionBlock failed: %v", err)
	}
	if !created {
		t.Errorf("expected createdFile=true for fresh repo without GEMINI.md")
	}

	data, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatalf("reading GEMINI.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, ManagedBlockBeginV2) || !strings.Contains(content, "@[memory atlas](.memory/ATLAS.md)") || !strings.Contains(content, ManagedBlockEnd) {
		t.Errorf("unexpected GEMINI.md content:\n%s", content)
	}

	// Re-running is idempotent and returns created=false
	created2, err := EnsureInstructionBlock(repoDir)
	if err != nil {
		t.Fatalf("second EnsureInstructionBlock failed: %v", err)
	}
	if created2 {
		t.Errorf("expected createdFile=false on second EnsureInstructionBlock")
	}
	data2, _ := os.ReadFile(geminiPath)
	if string(data2) != content {
		t.Errorf("expected idempotent content on second run")
	}

	// Remove with createdByUs=true deletes the file
	if err := RemoveInstructionBlock(repoDir, true); err != nil {
		t.Fatalf("RemoveInstructionBlock failed: %v", err)
	}
	if _, err := os.Stat(geminiPath); !os.IsNotExist(err) {
		t.Errorf("expected GEMINI.md to be deleted when createdByUs=true and no user content was added")
	}
}

func TestManagedBlock_AppendAndPreserveUserContentByteForByte(t *testing.T) {
	for _, userContent := range []string{
		"# Project Rules\n\nAlways run `go test ./...` before committing.\n",
		"# Project Rules without trailing newline",
	} {
		repoDir := t.TempDir()
		geminiPath := filepath.Join(repoDir, InstructionFilename)
		if err := os.WriteFile(geminiPath, []byte(userContent), 0644); err != nil {
			t.Fatal(err)
		}

		created, err := EnsureInstructionBlock(repoDir)
		if err != nil {
			t.Fatalf("EnsureInstructionBlock failed: %v", err)
		}
		if created {
			t.Errorf("expected createdFile=false when GEMINI.md already existed")
		}

		// Re-run to verify idempotency
		if _, err := EnsureInstructionBlock(repoDir); err != nil {
			t.Fatalf("second EnsureInstructionBlock failed: %v", err)
		}
		dataAfterEnsure, err := os.ReadFile(geminiPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(dataAfterEnsure), ManagedBlockBeginPrefix) != 1 {
			t.Errorf("expected exactly 1 managed block begin marker, got:\n%s", string(dataAfterEnsure))
		}

		// Remove block and verify original user content is preserved byte-for-byte
		if err := RemoveInstructionBlock(repoDir, false); err != nil {
			t.Fatalf("RemoveInstructionBlock failed: %v", err)
		}
		restored, err := os.ReadFile(geminiPath)
		if err != nil {
			t.Fatalf("expected GEMINI.md to still exist after RemoveInstructionBlock(createdByUs=false): %v", err)
		}
		if string(restored) != userContent {
			t.Errorf("user content not preserved byte-for-byte:\ngot:  %q\nwant: %q", string(restored), userContent)
		}
	}
}

func TestManagedBlock_UpgradeV1InPlace(t *testing.T) {
	repoDir := t.TempDir()
	geminiPath := filepath.Join(repoDir, InstructionFilename)

	before := "# Team Header\n\nIntro paragraph.\n\n"
	v1Block := "<!-- repo-memory:begin v1 -->\n@[memory map](.memory/views/MAP.md)\n<!-- repo-memory:end -->"
	after := "\n\n## Footer Section\nKeep this intact.\n"

	if err := os.WriteFile(geminiPath, []byte(before+v1Block+after), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureInstructionBlock(repoDir)
	if err != nil {
		t.Fatalf("EnsureInstructionBlock failed: %v", err)
	}
	if created {
		t.Errorf("expected createdFile=false when upgrading existing GEMINI.md")
	}

	updated, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatal(err)
	}
	want := before + CanonicalManagedBlockV2() + after
	if string(updated) != want {
		t.Errorf("v1 upgrade mismatch:\ngot:\n%s\nwant:\n%s", string(updated), want)
	}
}

func TestManagedBlock_RejectInvalidMarkers(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "duplicate begin",
			content: "<!-- repo-memory:begin v2 -->\nfoo\n<!-- repo-memory:begin v2 -->\nbar\n<!-- repo-memory:end -->\n",
			wantErr: "duplicate repo-memory:begin",
		},
		{
			name: "duplicate end",
			content: "<!-- repo-memory:begin v2 -->\nfoo\n<!-- repo-memory:end -->\nbar\n<!-- repo-memory:end -->\n",
			wantErr: "duplicate repo-memory:end",
		},
		{
			name: "unbalanced only begin",
			content: "# Doc\n<!-- repo-memory:begin v2 -->\nfoo\n",
			wantErr: "unbalanced",
		},
		{
			name: "unbalanced only end",
			content: "# Doc\n<!-- repo-memory:end -->\n",
			wantErr: "unbalanced",
		},
		{
			name: "inverted markers",
			content: "<!-- repo-memory:end -->\nfoo\n<!-- repo-memory:begin v2 -->\n",
			wantErr: "inverted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInstructionBlock(tc.content)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateInstructionBlock(%s) error = %v, want substring %q", tc.name, err, tc.wantErr)
			}

			repoDir := t.TempDir()
			geminiPath := filepath.Join(repoDir, InstructionFilename)
			if err := os.WriteFile(geminiPath, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			if _, err := EnsureInstructionBlock(repoDir); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("EnsureInstructionBlock(%s) error = %v, want substring %q", tc.name, err, tc.wantErr)
			}

			// Verify file was not modified when validation failed
			after, _ := os.ReadFile(geminiPath)
			if string(after) != tc.content {
				t.Errorf("expected GEMINI.md to remain untouched on validation failure")
			}
		})
	}
}
