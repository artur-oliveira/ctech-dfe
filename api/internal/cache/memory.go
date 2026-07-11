package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type memEntry struct {
	value     []byte
	expiresAt time.Time
}

// MemoryBackend is a single-instance in-memory cache.
// Not shared across replicas — use RedisBackend in multi-instance deployments.
type MemoryBackend struct {
	mu      sync.Mutex
	entries map[string]memEntry
	maxSize int
}

func NewMemoryBackend(maxSize int) *MemoryBackend {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MemoryBackend{entries: make(map[string]memEntry, maxSize), maxSize: maxSize}
}

func (m *MemoryBackend) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(e.expiresAt) {
		delete(m.entries, key)
		return nil, false, nil
	}
	return e.value, true, nil
}

func (m *MemoryBackend) Set(_ context.Context, key string, value []byte, ttlSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) >= m.maxSize {
		// evict one expired entry (simple strategy)
		for k, e := range m.entries {
			if time.Now().After(e.expiresAt) {
				delete(m.entries, k)
				break
			}
		}
	}
	m.entries[key] = memEntry{
		value:     value,
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	}
	return nil
}

func (m *MemoryBackend) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *MemoryBackend) DeletePrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.entries {
		if strings.HasPrefix(k, prefix) {
			delete(m.entries, k)
		}
	}
	return nil
}

func (m *MemoryBackend) Ping(_ context.Context) error { return nil }
