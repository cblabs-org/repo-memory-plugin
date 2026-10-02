package views

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
)

type Generator struct {
	store  *store.Store
	outDir string
}

func NewGenerator(claimsDir, outDir string) (*Generator, error) {
	s, err := store.New(claimsDir)
	if err != nil {
		return nil, err
	}
	return &Generator{
		store:  s,
		outDir: outDir,
	}, nil
}

func (g *Generator) Generate() error {
	allClaims, err := g.store.ListAll()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(g.outDir, 0755); err != nil {
		return err
	}

	var signatures []*claims.Claim
	var decisions []*claims.Claim
	subsystems := make(map[string][]*claims.Claim)

	for _, c := range allClaims {
		if c.Type == claims.TypeFailureSignature {
			signatures = append(signatures, c)
		} else if c.Type == claims.TypeDecision {
			decisions = append(decisions, c)
		}

		if sub, ok := c.Scope["subsystem"]; ok {
			subsystems[sub] = append(subsystems[sub], c)
		}
	}

	if err := g.generateMap(subsystems); err != nil {
		return err
	}
	if err := g.generateSignatures(signatures); err != nil {
		return err
	}
	if err := g.generateDecisions(decisions); err != nil {
		return err
	}
	for sub, subClaims := range subsystems {
		if err := g.generateSubsystem(sub, subClaims); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateMap(subsystems map[string][]*claims.Claim) error {
	path := filepath.Join(g.outDir, "MAP.md")
	var b strings.Builder
	b.WriteString("# Repository Map\n\n")
	b.WriteString("## Subsystems\n\n")

	var keys []string
	for k := range subsystems {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, sub := range keys {
		b.WriteString(fmt.Sprintf("- [%s](%s.md)\n", sub, sub))
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func (g *Generator) generateSignatures(claimsList []*claims.Claim) error {
	path := filepath.Join(g.outDir, "SIGNATURES.md")
	var b strings.Builder
	b.WriteString("# Failure Signatures\n\n")
	for _, c := range claimsList {
		b.WriteString(fmt.Sprintf("## %s\n%s\n\n", c.Title, c.Body))
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func (g *Generator) generateDecisions(claimsList []*claims.Claim) error {
	path := filepath.Join(g.outDir, "DECISIONS.md")
	var b strings.Builder
	b.WriteString("# Decisions\n\n")
	for _, c := range claimsList {
		b.WriteString(fmt.Sprintf("## %s\n%s\n\n", c.Title, c.Body))
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func (g *Generator) generateSubsystem(subsystem string, claimsList []*claims.Claim) error {
	path := filepath.Join(g.outDir, fmt.Sprintf("%s.md", subsystem))
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Subsystem: %s\n\n", subsystem))
	for _, c := range claimsList {
		b.WriteString(fmt.Sprintf("## [%s] %s\n%s\n\n", c.Type, c.Title, c.Body))
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
