package v1

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/ws"

	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

const wsPingInterval = 30 * time.Second

const wsAuthTimeout = 5 * time.Second

// readAuthToken reads the first WebSocket frame after the upgrade and extracts
// the bearer JWT. The client sends it as {"token":"..."} (or a raw token) once;
// a missing or unreadable frame fails closed so no connection hangs open.
func readAuthToken(conn *fws.Conn) (string, bool) {
	_ = conn.SetReadDeadline(time.Now().Add(wsAuthTimeout))
	defer conn.SetReadDeadline(time.Time{})
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

var wsUpgrader = fws.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *fasthttp.RequestCtx) bool { return true },
}

// RegisterWS registers GET /ws WebSocket upgrade endpoint.
// Auth: the JWT is sent as the first post-upgrade text frame (M3 — it must not
// travel in the ?token= query string, which leaks into LB/CF logs); org_pk stays
// in the query string as it is a non-secret org identifier.
func RegisterWS(router fiber.Router, verifier *middleware.Verifier, memberSvc *services.MembershipService, reg ws.Registry) {
	router.Get("/ws", func(c fiber.Ctx) error {
		orgPKRaw := c.Query("org_pk")
		if orgPKRaw == "" {
			return sendProblem(c, problem.BadRequest("org_pk obrigatório"))
		}

		return wsUpgrader.Upgrade(c.RequestCtx(), func(conn *fws.Conn) {
			ctx := c.Context()
			send := func(msg any) {
				data, _ := json.Marshal(msg)
				_ = conn.WriteMessage(fws.TextMessage, data)
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
			sub, _, err := verifier.Verify(ctx, token)
			if err != nil || sub == "" {
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
			m, err := memberSvc.Get(ctx, orgPK, sub)
			if err != nil {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Usuário não encontrado"})
				return
			}
			if m == nil {
				send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso negado a esta organização"})
				return
			}

			connID := uuid.NewString()
			reg.Register(orgPK, connID, &wsConnAdapter{conn: conn})
			defer reg.Unregister(orgPK, connID)

			send(map[string]any{"type": "connected", "org_pk": orgPK, "conn_id": connID})
			slog.Info("ws connected", "conn", connID, "org", orgPK)

			// Ping loop. Also re-checks membership each tick (cached read, ~zero
			// cost) so a member removed after the socket opened stops receiving
			// events instead of staying subscribed forever.
			done := make(chan struct{})
			go func() {
				t := time.NewTicker(wsPingInterval)
				defer t.Stop()
				for {
					select {
					case <-t.C:
						if still, e := memberSvc.Get(ctx, orgPK, sub); e == nil && still == nil {
							send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso revogado"})
							_ = conn.Close()
							return
						}
						if e := conn.WriteMessage(fws.TextMessage, []byte(`{"type":"ping"}`)); e != nil {
							return
						}
					case <-done:
						return
					}
				}
			}()

			// Read loop — detect disconnect, accept pong
			for {
				_, _, e := conn.ReadMessage()
				if e != nil {
					break
				}
			}
			close(done)
			slog.Info("ws disconnected", "conn", connID, "org", orgPK)
		})
	})
}

// wsConnAdapter adapts fasthttp/websocket.Conn to ws.Conn.
type wsConnAdapter struct {
	conn *fws.Conn
}

func (w *wsConnAdapter) WriteMessage(messageType int, data []byte) error {
	return w.conn.WriteMessage(messageType, data)
}
