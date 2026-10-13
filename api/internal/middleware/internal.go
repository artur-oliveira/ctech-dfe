package middleware

import (
	"slices"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// RequireInternalScope guards service-to-service routes; it runs after the auth
// middleware. Only client-credentials tokens pass: they carry no session (user
// and API-key tokens always do) and must hold scope. Same rule as
// ctech-account's middleware of the same name.
func RequireInternalScope(scope string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if GetSessionID(c) != "" || !slices.Contains(GetScopes(c), scope) {
			return problem.Forbidden("rota interna: token de serviço com o escopo exigido").Send(c)
		}
		return c.Next()
	}
}
