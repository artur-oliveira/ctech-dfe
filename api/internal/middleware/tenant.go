package middleware

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"

	"github.com/gofiber/fiber/v3"
)

// GetOrgPK returns the authenticated org PK from Fiber locals.
func GetOrgPK(c fiber.Ctx) string {
	v, _ := c.Locals(OrgPKKey).(string)
	return v
}

// ParseOrgPK validates and returns an org PK string.
// Org PKs must start with CNPJ_ or CPF_.
func ParseOrgPK(raw string) (string, error) {
	if len(raw) > 0 && (len(raw) >= 5 && (raw[:5] == "CNPJ_" || raw[:4] == "CPF_")) {
		return raw, nil
	}
	return "", problem.BadRequest("org_pk inválido: deve começar com CNPJ_ ou CPF_")
}
