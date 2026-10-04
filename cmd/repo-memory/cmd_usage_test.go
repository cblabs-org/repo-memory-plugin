package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsageCommand_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"usage", "--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("usage --help failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Check available model quota") {
		t.Errorf("expected usage command description, got %s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("expected --json flag in help, got %s", out)
	}
}
