package v1

import (
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"

	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

const wsPingInterval = 30 * time.Second

const wsAuthTimeout = 5 * time.Second

// wsPongWait is the read deadline after the last native pong — a standard
// margin (15s) over wsPingInterval so one slow tick doesn't false-positive.
const wsPongWait = wsPingInterval + 15*time.Second

const wsWriteWait = 5 * time.Second

// readAuthToken reads the first WebSocket frame after the upgrade and extracts
// the bearer JWT. The client sends it as {"token":"..."} (or a raw token) once;
// a missing or unreadable frame fails closed so no connection hangs open.
func readAuthToken(conn *fws.Conn) (string, bool) {
	_ = conn.SetReadDeadline(time.Now().Add(wsAuthTimeout))
	defer func(conn *fws.Conn, t time.Time) {
		err := conn.SetReadDeadline(t)
		if err != nil {

		}
	}(conn, time.Time{})
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return "", false
	}
	return extractBearer(msg), true
}

// extractBearer returns the token from a {"token":"..."} JSON frame, falling
// back to the raw trimmed frame body when it isn't JSON.
func extractBearer(msg []byte) string {
	var p struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(msg, &p) == nil && p.Token != "" {
		return p.Token
	}
	return strings.TrimSpace(string(msg))
}

func wsAllowedOrigin(ctx *fasthttp.RequestCtx, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return true
	}
	return slices.Contains(allowed, origin)
}

// RegisterWS registers GET /ws WebSocket upgrade endpoint.
// Auth: the JWT is sent as the first post-upgrade text frame (M3 — it must not
// travel in the ?token= query string, which leaks into LB/CF logs); org_pk stays
// in the query string as it is a non-secret org identifier.
func RegisterWS(router fiber.Router, verifier *middleware.Verifier, memberSvc *services.MembershipService, reg ws.Registry, allowedOrigins []string) {
	wsUpgrader := fws.FastHTTPUpgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(ctx *fasthttp.RequestCtx) bool { return wsAllowedOrigin(ctx, allowedOrigins) }}

	router.Get("/ws", func(c fiber.Ctx) error {
		orgPKRaw := c.Query("org_pk")
		if orgPKRaw == "" {
			return sendProblem(c, problem.BadRequest("org_pk obrigatório"))
		}

		return wsUpgrader.Upgrade(c.RequestCtx(), func(conn *fws.Conn) {
			// Post-upgrade the handler runs on a hijacked goroutine outside
			// Fiber's recover middleware — an unrecovered panic here kills the
			// whole process, not just this connection.
			defer func() {
				if r := recover(); r != nil {
					slog.Error("ws handler panic", "panic", r)
					_ = conn.Close()
				}
			}()
			ctx := c.Context()
			// Single adapter shared by this handler, the heartbeat goroutine
			// and the fan-out registry: its mutex is the only thing
			// serializing data-frame writes (fasthttp/websocket panics on
			// concurrent writes).
			safeConn := &wsConnAdapter{conn: conn}
			send := func(msg any) {
				data, _ := json.Marshal(msg)
				_ = safeConn.WriteMessage(fws.TextMessage, data)
			}

			// M3: auth moved off the ?token= query string. The client sends the
			// JWT as the first text frame after the upgrade; read exactly one frame
			// under a short deadline, then clear it.
			token, ok := readAuthToken(conn)
			if !ok {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Token ausente ou inválido"})
				_ = conn.Close()
				return
			}

			// Validate JWT (scopes don't gate the realtime channel — membership does)
			claims, err := verifier.VerifyClaims(ctx, token)
			if err != nil || claims.Sub == "" {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Token inválido ou expirado"})
				_ = conn.Close()
				return
			}

			// Parse and validate org PK format
			orgPK, err := middleware.ParseOrgPK(orgPKRaw)
			if err != nil {
				send(map[string]any{"type": "error", "code": "bad_request", "message": "org_pk inválido"})
				return
			}

			// Verify user belongs to org
			m, err := memberSvc.Get(ctx, orgPK, claims.Sub)
			if err != nil {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Usuário não encontrado"})
				return
			}
			if m == nil {
				send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso negado a esta organização"})
				return
			}

			connID := uuid.NewString()
			reg.Register(orgPK, connID, safeConn)
			defer reg.Unregister(orgPK, connID)

			send(map[string]any{"type": "connected", "org_pk": orgPK, "conn_id": connID})
			slog.Info("ws connected", "conn", connID, "org", orgPK)

			// Heartbeat: native ping/pong frames (the browser answers these
			// transparently — no client code involved) plus a re-check of
			// membership each tick (cached read, ~zero cost) so a member removed
			// after the socket opened stops receiving events instead of staying
			// subscribed forever.
			done := make(chan struct{})
			checkAlive := func() bool {
				still, e := memberSvc.Get(ctx, orgPK, claims.Sub)
				if e == nil && still == nil {
					send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso revogado"})
					return false
				}
				return true
			}
			go startHeartbeat(conn, done, wsPingInterval, wsPongWait, checkAlive)

			// Read loop — detects a dead connection via the heartbeat's read
			// deadline, and replies to the client's own app-level {"type":"ping"}
			// heartbeat (the client can't send native ping frames — WHATWG gives
			// browsers no API for that).
			for {
				_, msg, e := conn.ReadMessage()
				if e != nil {
					break
				}
				if isClientPing(msg) {
					send(map[string]any{"type": "pong"})
				}
			}
			close(done)
			slog.Info("ws disconnected", "conn", connID, "org", orgPK)
		})
	})
}

// startHeartbeat sends a native ping every pingInterval and arms/resets a read
// deadline on every native pong, so a half-open connection (no pong within
// pongWait) breaks the caller's blocking ReadMessage() instead of lingering
// forever. checkAlive lets the caller veto the next ping (e.g. revoked
// membership) — returning false closes the connection immediately.
func startHeartbeat(conn *fws.Conn, done <-chan struct{}, pingInterval, pongWait time.Duration, checkAlive func() bool) {
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))

	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if checkAlive != nil && !checkAlive() {
				_ = conn.Close()
				return
			}
			if e := conn.WriteControl(fws.PingMessage, nil, time.Now().Add(wsWriteWait)); e != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// isClientPing reports whether msg is the client's app-level heartbeat frame
// (the client can't send a native WS ping — see startHeartbeat).
func isClientPing(msg []byte) bool {
	var p struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(msg, &p) == nil && p.Type == "ping"
}

// wsConnAdapter adapts fasthttp/websocket.Conn to ws.Conn, serializing
// writes: the registry broadcasts from other goroutines while the read
// loop and heartbeat reply inline, and fasthttp/websocket allows only one
// concurrent data-frame writer per conn.
type wsConnAdapter struct {
	mu   sync.Mutex
	conn *fws.Conn
}

func (w *wsConnAdapter) WriteMessage(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(messageType, data)
}
