// Package middleware provides Fiber middleware for JWT RS256 auth, tenant resolution,
// and request recovery — mirroring api/app/dependencies/auth.py and security.py.
package middleware

import (
	"strings"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/jwtverify"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

const (
	firstPartyDfeClientID = "dfe"

	UserIDKey    = "user_id"
	SessionIDKey = "session_id"
	// ScopesKey stores the token's OAuth scopes (from the space-delimited `scope`
	// claim) in Fiber locals, for the RBAC layer to intersect with the role.
	ScopesKey = "token_scopes"
	AzpKey    = "auth_azp"
)

// Verifier validates RS256 access tokens issued by ctech-account against its
// JWKS. The JWKS-fetch and claims-parsing mechanics live in the shared
// gopkg.aoctech.app/api-commons/jwtverify package; this wrapper only adds the
// Fiber-facing bits (locals wiring, RFC 7807 error responses) specific to dfe.
type Verifier struct {
	*jwtverify.Verifier
	blocked services.BlockedFunc
	// afterAuth runs once the token is verified and the locals are set; it must
	// call c.Next() itself (the rate limiter).
	afterAuth fiber.Handler
}

// NewVerifier also turns on the account-deletion revocation list: ctech-account
// writes ctech:jwt:revoked_sub:{sub} in Valkey DB 0, which is this service's
// cache backend (VALKEY_URL has no DB suffix). Fail open when Valkey is down;
// the local lock (WithLockCheck) is the independent second layer.
func NewVerifier(jwksURL, audience, issuer string, cacheBackend cache.Backend) *Verifier {
	v := jwtverify.NewVerifier(jwksURL, audience, issuer, cacheBackend)
	v.WithRevocation(cacheBackend)
	return &Verifier{Verifier: v}
}

// WithLockCheck refuses state-changing requests of a sub the deletion saga
// locked or erased. This middleware is the one choke point every user route
// goes through, so no write path can forget the check.
func (v *Verifier) WithLockCheck(blocked services.BlockedFunc) *Verifier {
	v.blocked = blocked
	return v
}

// WithAfterAuth runs h for every authenticated request, after the lock check.
func (v *Verifier) WithAfterAuth(h fiber.Handler) *Verifier {
	v.afterAuth = h
	return v
}

// Middleware returns Fiber middleware that validates the Bearer token on each request.
func (v *Verifier) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("missing bearer token"))
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := v.VerifyClaims(c.Context(), tokenStr)
		if err != nil || claims.Sub == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("invalid credentials"))
		}

		c.Locals(UserIDKey, claims.Sub)
		c.Locals(SessionIDKey, claims.SID)
		c.Locals(ScopesKey, claims.Scopes())
		c.Locals(AzpKey, claims.AZP)
		if v.blocked != nil && isMutating(c.Method()) {
			locked, err := v.blocked(c.Context(), claims.Sub)
			if err != nil {
				return problem.InternalServer("estado da conta indisponível").WithCause(err).Send(c)
			}
			if locked {
				return problem.AccountPendingDeletion("Conta em processo de exclusão; operações de escrita estão bloqueadas.").Send(c)
			}
		}
		if v.afterAuth != nil {
			return v.afterAuth(c)
		}
		return c.Next()
	}
}

// GetUserID returns the authenticated user's subject from Fiber locals.
func GetUserID(c fiber.Ctx) string {
	v, _ := c.Locals(UserIDKey).(string)
	return v
}

// GetScopes returns the token's OAuth scopes from Fiber locals (nil if none).
func GetScopes(c fiber.Ctx) []string {
	v, _ := c.Locals(ScopesKey).([]string)
	return v
}

// GetAZP returns Authorized Party of the current token
func GetAZP(c fiber.Ctx) string {
	v, _ := c.Locals(AzpKey).(string)
	return v
}

// GetSessionID returns the current logged user session
func GetSessionID(c fiber.Ctx) string {
	v, _ := c.Locals(SessionIDKey).(string)
	return v
}

func IsFirstPartyDfeSession(ctx fiber.Ctx) bool {
	return GetSessionID(ctx) != "" && GetAZP(ctx) == firstPartyDfeClientID
}
