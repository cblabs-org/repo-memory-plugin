package claims

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// GenerateID generates a new ULID prefixed with "clm-".
func GenerateID() string {
	t := time.Now()
	entropy := ulid.Monotonic(rand.Reader, 0)
	return "clm-" + ulid.MustNew(ulid.Timestamp(t), entropy).String()
}
