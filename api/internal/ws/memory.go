package ws

import (
	"context"
	"log/slog"
	"sync"
)

const TextMessage = 1 // WebSocket text frame opcode

// MemoryRegistry is a single-instance registry.
// Does NOT fan out across replicas — use RedisRegistry in production.
type MemoryRegistry struct {
	mu    sync.RWMutex
	conns map[string]map[string]Conn // orgPK → connID → conn
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{conns: make(map[string]map[string]Conn)}
}

func (m *MemoryRegistry) Start(_ context.Context) error { return nil }
func (m *MemoryRegistry) Stop(_ context.Context) error  { return nil }

func (m *MemoryRegistry) Register(orgPK, connID string, conn Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conns[orgPK]; !ok {
		m.conns[orgPK] = make(map[string]Conn)
	}
	m.conns[orgPK][connID] = conn
	slog.Debug("ws registered", "org", orgPK, "conn", connID)
}

func (m *MemoryRegistry) Unregister(orgPK, connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if org, ok := m.conns[orgPK]; ok {
		delete(org, connID)
		if len(org) == 0 {
			delete(m.conns, orgPK)
		}
	}
	slog.Debug("ws unregistered", "org", orgPK, "conn", connID)
}

func (m *MemoryRegistry) Broadcast(_ context.Context, orgPK string, payload []byte) {
	m.mu.RLock()
	org, ok := m.conns[orgPK]
	if !ok {
		m.mu.RUnlock()
		return
	}
	snapshot := make(map[string]Conn, len(org))
	for id, c := range org {
		snapshot[id] = c
	}
	m.mu.RUnlock()

	var dead []string
	for id, c := range snapshot {
		if err := c.WriteMessage(TextMessage, payload); err != nil {
			slog.Warn("ws send failed", "org", orgPK, "conn", id, "err", err)
			dead = append(dead, id)
		}
	}
	for _, id := range dead {
		m.Unregister(orgPK, id)
	}
}
