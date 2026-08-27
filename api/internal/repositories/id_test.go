package repositories

import (
	"sync"
	"testing"
)

// Sort keys depend on GenerateID never colliding, even when many goroutines
// generate inside the same millisecond.
func TestGenerateIDUniqueUnderConcurrency(t *testing.T) {
	const n = 10000
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) { defer wg.Done(); ids[i] = GenerateID() }(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for _, id := range ids {
		if len(id) != 26 {
			t.Fatalf("not a ULID: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate ULID: %q", id)
		}
		seen[id] = true
	}
}
