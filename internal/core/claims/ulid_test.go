package claims

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	if !strings.HasPrefix(id1, "clm-") {
		t.Errorf("expected ID to start with clm-, got %s", id1)
	}

	id2 := GenerateID()
	if id1 == id2 {
		t.Errorf("expected generated IDs to be unique")
	}
}
