package middleware

import (
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/gofiber/fiber/v3"
)

// GetOrgPK returns the authenticated org PK from Fiber locals.
func GetOrgPK(c fiber.Ctx) string {
	v, _ := c.Locals(OrgPKKey).(string)
	return v
}

// ParseOrgPK validates a tenant key from the WebSocket handshake, where there is
// no repository to hand.
//
// It delegates rather than checking the shape itself. The version this replaced
// tested for the CNPJ_/CPF_ prefixes by hand — a second spelling of a rule that
// already existed, and one that would have rejected every platform company id
// while the HTTP path accepted them.
func ParseOrgPK(raw string) (string, error) {
	pk, err := repositories.ParseOrgPK(raw)
	if err != nil {
		return "", problem.BadRequest("organização inválida")
	}
	return pk, nil
}
