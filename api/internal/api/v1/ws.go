package v1

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
	"github.com/artur-oliveira/ctech-dfe/api/internal/ws"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	fws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

const wsPingInterval = 30 * time.Second

var wsUpgrader = fws.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *fasthttp.RequestCtx) bool { return true },
}

// RegisterWS registers GET /ws WebSocket upgrade endpoint.
// Auth via query params: ?token=<jwt>&org_pk=<pk>
func RegisterWS(router fiber.Router, verifier *middleware.Verifier, userSvc *services.UserService, reg ws.Registry) {
	router.Get("/ws", func(c fiber.Ctx) error {
		token := c.Query("token")
		orgPKRaw := c.Query("org_pk")
		if token == "" || orgPKRaw == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"detail": "token e org_pk obrigatórios",
			})
		}

		return wsUpgrader.Upgrade(c.RequestCtx(), func(conn *fws.Conn) {
			ctx := c.Context()
			send := func(msg any) {
				data, _ := json.Marshal(msg)
				_ = conn.WriteMessage(fws.TextMessage, data)
			}

			// Validate JWT
			sub, err := verifier.Verify(ctx, token)
			if err != nil || sub == "" {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Token inválido ou expirado"})
				return
			}

			// Parse and validate org PK format
			orgPK, err := middleware.ParseOrgPK(orgPKRaw)
			if err != nil {
				send(map[string]any{"type": "error", "code": "bad_request", "message": "org_pk inválido"})
				return
			}

			// Verify user belongs to org
			user, err := userSvc.GetMe(ctx, sub)
			if err != nil || user == nil {
				send(map[string]any{"type": "error", "code": "unauthorized", "message": "Usuário não encontrado"})
				return
			}
			if !wsUserBelongsToOrg(user, orgPK) {
				send(map[string]any{"type": "error", "code": "forbidden", "message": "Acesso negado a esta organização"})
				return
			}

			connID := uuid.NewString()
			reg.Register(orgPK, connID, &wsConnAdapter{conn: conn})
			defer reg.Unregister(orgPK, connID)

			send(map[string]any{"type": "connected", "org_pk": orgPK, "conn_id": connID})
			slog.Info("ws connected", "conn", connID, "org", orgPK)

			// Ping loop
			done := make(chan struct{})
			go func() {
				t := time.NewTicker(wsPingInterval)
				defer t.Stop()
				for {
					select {
					case <-t.C:
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

// wsUserBelongsToOrg checks if the DynamoDB user record includes orgPK in its organizations list.
func wsUserBelongsToOrg(user map[string]types.AttributeValue, orgPK string) bool {
	orgsAttr, ok := user["organizations"].(*types.AttributeValueMemberL)
	if !ok {
		return false
	}
	for _, item := range orgsAttr.Value {
		if m, ok := item.(*types.AttributeValueMemberM); ok {
			if pk, ok := m.Value["pk"].(*types.AttributeValueMemberS); ok && pk.Value == orgPK {
				return true
			}
			if pk, ok := m.Value["org_pk"].(*types.AttributeValueMemberS); ok && pk.Value == orgPK {
				return true
			}
		}
	}
	return false
}
