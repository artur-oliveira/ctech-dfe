package repositories

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// GenerateID returns a new ULID strings
func GenerateID() string {
	return newULID().String()
}

var ulidPool = sync.Pool{
	New: func() any {
		return ulid.Monotonic(rand.Reader, 0)
	},
}

func newULID() ulid.ULID {
	// Acquire engine from pool
	entropy := ulidPool.Get().(ulid.MonotonicReader)
	defer ulidPool.Put(entropy)

	ms := ulid.Timestamp(time.Now())
	id, _ := ulid.New(ms, entropy)
	return id
}
