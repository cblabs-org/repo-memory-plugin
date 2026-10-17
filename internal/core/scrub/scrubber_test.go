package scrub

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScrub_Layer1_Structural(t *testing.T) {
	fakeGCPKey := "AI" + "za" + strings.Repeat("B", 35)
	fakeAWSKey := "AK" + "IAIOSFODNN7EXAMPLE"
	fakePrivKey := "-----BEGIN " + "RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0123456789abcdef\n-----END " + "RSA PRIVATE KEY-----"

	input := strings.Join([]string{
		fakePrivKey,
		"postgres://admin:s3cr3tPassw0rd@db.internal:5432/prod",
		"Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"aws_id=" + fakeAWSKey,
		"gh=ghp_1234567890abcdef1234567890abcdef1234",
		"slack=xoxb-123456789012-1234567890123-abcdefABCDEF1234567890",
		"gcp=" + fakeGCPKey,
		"openai=sk-ant-api03-1234567890abcdef1234567890abcdef",
	}, "\n")

	res, err := Scrub(input, Context{Boundary: "test"})
	if err != nil {
		t.Fatalf("Scrub failed: %v", err)
	}

	checks := []struct {
		mustNotContain string
		mustContain    string
		countKey       string
	}{
		{"MIIEpAIBAAKCAQEA0123456789abcdef", "«secret:private_key»", "private_key"},
		{"s3cr3tP", "postgres://admin:«secret:url_cred»@db.internal:5432/prod", "url_cred"},
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "«secret:token»", "token"},
		{"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "AWS_SECRET_ACCESS_KEY=«secret:env»", "env"},
		{fakeAWSKey, "«secret:aws»", "aws"},
		{"ghp_1234567890abcdef1234567890abcdef1234", "«secret:github»", "github"},
		{"xoxb-123456789012", "«secret:slack»", "slack"},
		{fakeGCPKey, "«secret:gcp_api_key»", "gcp_api_key"},
		{"sk-ant-api03-1234567890abcdef1234567890abcdef", "«secret:api_key»", "api_key"},
	}

	for _, tc := range checks {
		if strings.Contains(res.Text, tc.mustNotContain) {
			t.Errorf("expected %q to be redacted, got:\n%s", tc.mustNotContain, res.Text)
		}
		if !strings.Contains(res.Text, tc.mustContain) {
			t.Errorf("expected %q in output, got:\n%s", tc.mustContain, res.Text)
		}
		if res.Counts[tc.countKey] < 1 {
			t.Errorf("expected Counts[%q] >= 1, got %d", tc.countKey, res.Counts[tc.countKey])
		}
	}
}

func TestScrub_Layer2_IdentifiersAndHex(t *testing.T) {
	// Set up a temporary git repo with a real commit to test `git cat-file -e` SHA preservation.
	repoDir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@users.noreply.github.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@users.noreply.github.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial")
	realCommitSHA := runGit("rev-parse", "HEAD")

	allowedConvUUID := "3ce5d33d-ed0b-4d4a-8f21-5c33e7ce80c5"
	benignUnlistedUUID := "123e4567-e89b-12d3-a456-426614174000"
	secretLabelledUUID := "550e8400-e29b-41d4-a716-446655440000"
	allowedSHA := "aaaa1111bbbb2222cccc3333dddd4444eeee5555"
	contextSHA := "1111222233334444555566667777888899990000"
	unlabelledSecretHex := "deadbeefcafebabe0123456789abcdefdeadbeefcafebabe"
	ulidVal := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	input := strings.Join([]string{
		"conversation_id: " + allowedConvUUID,
		"session ref: " + benignUnlistedUUID,
		"api_key: " + secretLabelledUUID,
		"entries: inv-01abc, got-99xyz, ulid=" + ulidVal,
		"allowed sha: " + allowedSHA,
		"commit " + contextSHA + " fixed the race",
		"verified against " + realCommitSHA + " in repo",
		"raw dump: " + unlabelledSecretHex,
	}, "\n")

	ctx := Context{
		Boundary: "note",
		RepoRoot: repoDir,
		AllowedIDs: map[string]bool{
			allowedConvUUID: true,
			allowedSHA:      true,
		},
	}

	res, err := Scrub(input, ctx)
	if err != nil {
		t.Fatalf("Scrub failed: %v", err)
	}

	for _, preserved := range []string{
		allowedConvUUID,
		benignUnlistedUUID,
		"inv-01abc",
		"got-99xyz",
		ulidVal,
		allowedSHA,
		contextSHA,
		realCommitSHA,
	} {
		if !strings.Contains(res.Text, preserved) {
			t.Errorf("expected identifier %q to be preserved, got:\n%s", preserved, res.Text)
		}
	}

	if strings.Contains(res.Text, secretLabelledUUID) || !strings.Contains(res.Text, "api_key: «secret:uuid»") {
		t.Errorf("expected secret-labelled UUID to be scrubbed to «secret:uuid», got:\n%s", res.Text)
	}
	if strings.Contains(res.Text, unlabelledSecretHex) || !strings.Contains(res.Text, "raw dump: «secret:hex»") {
		t.Errorf("expected unlabelled 48-char hex secret to be scrubbed to «secret:hex», got:\n%s", res.Text)
	}
}

func TestScrub_Layer3_CustomRulesAndFailClosed(t *testing.T) {
	t.Run("custom rules from .memory/scrub.yaml and scrub.local.yaml are applied", func(t *testing.T) {
		repoRoot := t.TempDir()
		dataDir := t.TempDir()
		repoID := "repo-test"

		if err := os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755); err != nil {
			t.Fatal(err)
		}
		sharedYAML := `version: 1
rules:
  - name: corp_ticket
    pattern: 'CORPSECRET-[0-9]{6}'
    replace: '«secret:corp_ticket»'
`
		if err := os.WriteFile(filepath.Join(repoRoot, ".memory", "scrub.yaml"), []byte(sharedYAML), 0644); err != nil {
			t.Fatal(err)
		}

		if err := os.MkdirAll(filepath.Join(dataDir, repoID), 0755); err != nil {
			t.Fatal(err)
		}
		localYAML := `version: 1
rules:
  - name: local_box
    pattern: 'BOXKEY_[A-Z0-9]{8}'
    replace: '«secret:local_box»'
`
		if err := os.WriteFile(filepath.Join(dataDir, repoID, "scrub.local.yaml"), []byte(localYAML), 0644); err != nil {
			t.Fatal(err)
		}

		res, err := Scrub("found CORPSECRET-123456 and BOXKEY_ABCD1234 in log", Context{
			Boundary: "note",
			RepoID:   repoID,
			RepoRoot: repoRoot,
			DataDir:  dataDir,
		})
		if err != nil {
			t.Fatalf("Scrub failed: %v", err)
		}
		if strings.Contains(res.Text, "CORPSECRET-123456") || !strings.Contains(res.Text, "«secret:corp_ticket»") {
			t.Errorf("expected shared rule to scrub CORPSECRET, got %q", res.Text)
		}
		if strings.Contains(res.Text, "BOXKEY_ABCD1234") || !strings.Contains(res.Text, "«secret:local_box»") {
			t.Errorf("expected local rule to scrub BOXKEY, got %q", res.Text)
		}
		if res.Counts["custom:corp_ticket"] != 1 || res.Counts["custom:local_box"] != 1 {
			t.Errorf("unexpected custom counts: %+v", res.Counts)
		}
	})

	t.Run("fail closed on invalid regex in scrub.yaml", func(t *testing.T) {
		repoRoot := t.TempDir()
		_ = os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)
		badYAML := `version: 1
rules:
  - name: broken
    pattern: '[unclosed-group'
`
		_ = os.WriteFile(filepath.Join(repoRoot, ".memory", "scrub.yaml"), []byte(badYAML), 0644)

		_, err := Scrub("hello", Context{RepoRoot: repoRoot})
		if err == nil {
			t.Fatal("expected non-nil error for invalid regex in scrub.yaml, got nil")
		}
	})

	t.Run("fail closed on allow/disable/whitelist keys in scrub.local.yaml", func(t *testing.T) {
		for _, forbiddenKey := range []string{"allow", "disable", "whitelist"} {
			dataDir := t.TempDir()
			repoID := "repo-1"
			_ = os.MkdirAll(filepath.Join(dataDir, repoID), 0755)
			content := "version: 1\n" + forbiddenKey + ":\n  - aws\nrules: []\n"
			_ = os.WriteFile(filepath.Join(dataDir, repoID, "scrub.local.yaml"), []byte(content), 0644)

			_, err := Scrub("some text", Context{DataDir: dataDir, RepoID: repoID})
			if err == nil {
				t.Fatalf("expected non-nil error when scrub.local.yaml contains %q key", forbiddenKey)
			}
		}
	})

	t.Run("QuarantinePayload writes 0600 file with reason header", func(t *testing.T) {
		dataDir := t.TempDir()
		repoID := "repo-q"
		raw := []byte("sensitive un-scrubbed note payload")

		qPath, err := QuarantinePayload(dataDir, repoID, "note", "invalid regex in scrub.local.yaml", raw)
		if err != nil {
			t.Fatalf("QuarantinePayload failed: %v", err)
		}
		info, err := os.Stat(qPath)
		if err != nil {
			t.Fatalf("Stat quarantine file: %v", err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
		}
		saved, err := os.ReadFile(qPath)
		if err != nil {
			t.Fatalf("ReadFile quarantine: %v", err)
		}
		expectedPrefix := "# reason: invalid regex in scrub.local.yaml\n"
		if !strings.HasPrefix(string(saved), expectedPrefix) {
			t.Errorf("expected prefix %q, got %q", expectedPrefix, string(saved))
		}
		if !strings.HasSuffix(string(saved), string(raw)) {
			t.Errorf("expected raw payload suffix, got %q", string(saved))
		}
	})
}

func TestScrub_Layer4_Blobs(t *testing.T) {
	dataURI := "data:image/png;base64," + strings.Repeat("QUJD", 80) // 320 chars > 256
	longBlob := strings.Repeat("AbCdEf0123456789+/", 60)             // 1080 chars > 1024

	input := "inline image: " + dataURI + "\nraw dump: " + longBlob
	res, err := Scrub(input, Context{Boundary: "capture"})
	if err != nil {
		t.Fatalf("Scrub failed: %v", err)
	}
	if strings.Contains(res.Text, "QUJDQUJD") {
		t.Errorf("expected data URI to be stripped, got %q", res.Text)
	}
	if strings.Contains(res.Text, "AbCdEf0123456789+/") {
		t.Errorf("expected >1KiB blob to be stripped, got %q", res.Text)
	}
	if strings.Count(res.Text, "«blob:stripped»") != 2 {
		t.Errorf("expected 2 «blob:stripped» placeholders, got %q", res.Text)
	}
	if res.Counts["blob"] != 2 {
		t.Errorf("expected Counts[blob] == 2, got %d", res.Counts["blob"])
	}
}
