package repositories

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/oklog/ulid/v2"
)

// entropy is a process-wide, mutex-guarded monotonic source: two IDs generated
// in the same millisecond are still strictly increasing, which sort keys rely on.
var entropy = &ulid.LockedMonotonicReader{MonotonicReader: ulid.Monotonic(rand.Reader, 0)}

// GenerateID returns a new ULID string.
func GenerateID() string {
	// MustNew panics only if crypto/rand fails or the monotonic counter
	// overflows (>2^32 IDs in one millisecond) — both unrecoverable.
	return ulid.MustNew(ulid.Now(), entropy).String()
}

// ulidEntropyBytes is the entropy part of a ULID (80 bits).
const ulidEntropyBytes = 10

// WebhookEventID is a ULID whose time is when the thing happened and whose
// entropy is a hash of seed: the same event always gets the same id, so a
// redelivered queue message writes the same delivery rows (a put-if-absent no-op)
// instead of a second event the receiver cannot recognise as a duplicate, and ids
// still sort by time, which the delivery listing relies on.
func WebhookEventID(at time.Time, seed string) string {
	sum := sha256.Sum256([]byte(seed))
	id, err := ulid.New(ulid.Timestamp(at), bytes.NewReader(sum[:ulidEntropyBytes]))
	if err != nil {
		// Only a time past year 10889 or a short entropy reader fails; neither is reachable.
		panic("webhook event id: " + err.Error())
	}
	return id.String()
}
