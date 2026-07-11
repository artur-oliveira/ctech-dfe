// Package ws implements the WebSocket connection registry with tenant isolation,
// mirroring api/app/core/ws_registry.py.
//
// Fan-out pattern:
//   - Each API instance holds a local map[orgPK → []conn].
//   - Worker publishes to Redis channel "ws:{orgPK}".
//   - All instances subscribed to that channel receive and push to local connections.
//   - No sticky sessions required.
package ws

import "context"

// Conn is a minimal WebSocket connection abstraction.
type Conn interface {
	WriteMessage(messageType int, data []byte) error
}

// Registry fans out payloads to WebSocket connections keyed by org_pk.
type Registry interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Register(orgPK, connID string, conn Conn)
	Unregister(orgPK, connID string)
	Broadcast(ctx context.Context, orgPK string, payload []byte)
}
