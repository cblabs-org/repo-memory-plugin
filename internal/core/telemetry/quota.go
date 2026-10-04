package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

var (
	lookPath    = exec.LookPath
	execCommand = exec.CommandContext
)

// SetLookPathForTesting allows tests to override lookPath and returns a restore function.
func SetLookPathForTesting(fn func(string) (string, error)) func() {
	prev := lookPath
	lookPath = fn
	return func() { lookPath = prev }
}

// SetExecCommandForTesting allows tests to override execCommand and returns a restore function.
func SetExecCommandForTesting(fn func(context.Context, string, ...string) *exec.Cmd) func() {
	prev := execCommand
	execCommand = fn
	return func() { execCommand = prev }
}

// QuotaBucket represents an individual model or quota bucket.
type QuotaBucket struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Description       string     `json:"description,omitempty"`
	RemainingFraction float64    `json:"remaining_fraction"`
	ResetTime         *time.Time `json:"reset_time,omitempty"`
}

// QuotaGroup represents a logical group of quota buckets (e.g., "All Models").
type QuotaGroup struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Buckets     []QuotaBucket `json:"buckets"`
}

// QuotaStatus represents the parsed quota state returned by agy.
type QuotaStatus struct {
	Status      string        `json:"status"`
	Description string        `json:"description,omitempty"`
	Groups      []QuotaGroup  `json:"groups"`
	Buckets     []QuotaBucket `json:"buckets,omitempty"`
}

// GetBucket returns a bucket by its identifier.
func (q *QuotaStatus) GetBucket(id string) (QuotaBucket, bool) {
	for _, b := range q.Buckets {
		if b.ID == id {
			return b, true
		}
	}
	return QuotaBucket{}, false
}

// RemainingFraction returns the remaining fraction for a bucket by ID.
func (q *QuotaStatus) RemainingFraction(id string) (float64, bool) {
	b, ok := q.GetBucket(id)
	if !ok {
		return 0, false
	}
	return b.RemainingFraction, true
}

// rawAgyResponse captures the JSON output from `agy -p "/usage" --output-format json`.
type rawAgyResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
	Response       string `json:"response"`
	Command        struct {
		Name string `json:"name"`
		Data struct {
			Description string `json:"description"`
			Groups      []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Buckets     []struct {
					ID                string  `json:"id"`
					Name              string  `json:"name"`
					Description       string  `json:"description"`
					RemainingFraction float64 `json:"remaining_fraction"`
					ResetTime         string  `json:"reset_time"`
				} `json:"buckets"`
			} `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

// CheckQuota queries `agy -p "/usage" --output-format json` (zero tokens)
// and returns the current quota state across model pools.
// If agy is not in PATH, it returns an error.
func CheckQuota(ctx context.Context) (*QuotaStatus, error) {
	agyPath, err := lookPath("agy")
	if err != nil {
		return nil, fmt.Errorf("agy not found in PATH: %w", err)
	}

	cmd := execCommand(ctx, agyPath, "-p", "/usage", "--output-format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("executing agy usage: %w", err)
	}

	return ParseQuotaOutput(out)
}

// ParseQuotaOutput parses the raw JSON output from agy usage into a QuotaStatus.
func ParseQuotaOutput(data []byte) (*QuotaStatus, error) {
	var raw rawAgyResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decoding agy usage response: %w", err)
	}

	if raw.Status != "" && raw.Status != "SUCCESS" {
		return nil, fmt.Errorf("agy returned non-success status %q: %s", raw.Status, raw.Response)
	}

	qs := &QuotaStatus{
		Status:      raw.Status,
		Description: raw.Command.Data.Description,
		Groups:      make([]QuotaGroup, 0, len(raw.Command.Data.Groups)),
		Buckets:     make([]QuotaBucket, 0),
	}

	for _, g := range raw.Command.Data.Groups {
		group := QuotaGroup{
			Name:        g.Name,
			Description: g.Description,
			Buckets:     make([]QuotaBucket, 0, len(g.Buckets)),
		}

		for _, b := range g.Buckets {
			bucket := QuotaBucket{
				ID:                b.ID,
				Name:              b.Name,
				Description:       b.Description,
				RemainingFraction: b.RemainingFraction,
			}

			if b.ResetTime != "" {
				if t, err := time.Parse(time.RFC3339, b.ResetTime); err == nil {
					bucket.ResetTime = &t
				} else if t, err := time.Parse(time.RFC3339Nano, b.ResetTime); err == nil {
					bucket.ResetTime = &t
				}
			}

			group.Buckets = append(group.Buckets, bucket)
			qs.Buckets = append(qs.Buckets, bucket)
		}

		qs.Groups = append(qs.Groups, group)
	}

	return qs, nil
}
