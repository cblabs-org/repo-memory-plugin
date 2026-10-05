package antigravity

import (
	"os"
	"strings"
	"testing"
)

func TestV2SpikePreInvocation(t *testing.T) {
	os.Setenv("REPO_MEMORY_V2_SPIKE", "1")
	defer os.Unsetenv("REPO_MEMORY_V2_SPIKE")

	tests := []struct {
		name     string
		payload  []byte
		envValue string
		want     string
	}{
		{
			name:     "invocation 0 returns systemMessage",
			payload:  []byte(`{"conversationId": "test-123", "invocationNum": 0}`),
			envValue: "1",
			want:     `"systemMessage":"[X5-CHANNEL-TEST conv=test-123]`,
		},
		{
			name:     "invocation > 0 returns empty",
			payload:  []byte(`{"conversationId": "test-123", "invocationNum": 1}`),
			envValue: "1",
			want:     `{}`,
		},
		{
			name:     "disabled by env flag",
			payload:  []byte(`{"conversationId": "test-123", "invocationNum": 0}`),
			envValue: "0",
			want:     `{}`,
		},
		{
			name:     "malformed payload returns systemMessage (defaults 0)",
			payload:  []byte(`malformed`),
			envValue: "1",
			want:     `"systemMessage":`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("REPO_MEMORY_V2_SPIKE", tc.envValue)
			out, err := V2SpikePreInvocation(tc.payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("got %q, want to contain %q", string(out), tc.want)
			}
		})
	}
}
