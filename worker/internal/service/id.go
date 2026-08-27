package service

import (
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// entropy is a process-wide, mutex-guarded monotonic source: two IDs generated
// in the same millisecond are still strictly increasing, which sort keys rely on.
var entropy = &ulid.LockedMonotonicReader{MonotonicReader: ulid.Monotonic(rand.Reader, 0)}

// genULID returns a new ULID string.
func genULID() string {
	// MustNew panics only if crypto/rand fails or the monotonic counter
	// overflows (>2^32 IDs in one millisecond) — both unrecoverable.
	return ulid.MustNew(ulid.Now(), entropy).String()
}
