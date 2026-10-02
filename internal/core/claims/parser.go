package claims

import (
	"bytes"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidFormat = errors.New("invalid claim file format")
)

// Parse reads a markdown file with YAML frontmatter into a Claim.
func Parse(data []byte) (*Claim, error) {
	strData := string(data)
	if !strings.HasPrefix(strData, "---\n") && !strings.HasPrefix(strData, "---\r\n") {
		return nil, ErrInvalidFormat
	}

	endIdx := strings.Index(strData[4:], "\n---")
	if endIdx == -1 {
		return nil, ErrInvalidFormat
	}
	
	endIdx += 4 // adjust for the slice offset

	frontmatter := strData[4:endIdx]

	bodyStartIdx := endIdx + 4
	if bodyStartIdx < len(strData) && strData[bodyStartIdx] == '\r' {
		bodyStartIdx++
	}
	if bodyStartIdx < len(strData) && strData[bodyStartIdx] == '\n' {
		bodyStartIdx++
	}

	body := strData[bodyStartIdx:]

	var claim Claim
	if err := yaml.Unmarshal([]byte(frontmatter), &claim); err != nil {
		return nil, err
	}

	claim.Body = strings.TrimSpace(body)

	return &claim, nil
}

// Write serializes a Claim into a markdown file with YAML frontmatter.
func Write(claim *Claim) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(claim); err != nil {
		return nil, err
	}
	encoder.Close()

	buf.WriteString("---\n\n")
	buf.WriteString(claim.Body)
	buf.WriteString("\n")

	return buf.Bytes(), nil
}
