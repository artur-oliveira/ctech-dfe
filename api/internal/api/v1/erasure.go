package v1

import (
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
)

// ScopeErasureEligibility is carried by the token ctech-account mints to ask
// this service for account-deletion blockers (saga protocol §4.1).
const ScopeErasureEligibility = "internal:dfe:erasure-eligibility"

// RegisterErasure mounts the saga's eligibility route.
func RegisterErasure(router fiber.Router, authMw fiber.Handler) {
	router.Get("/internal/erasure/eligibility/:sub", authMw,
		middleware.RequireInternalScope(ScopeErasureEligibility), eligibility)
}

// eligibility has no blocker to report (plan ruling R2): ownership of shared
// organizations is ctech-account's check, and a locked user cannot start
// an emission.
func eligibility(c fiber.Ctx) error {
	return c.JSON(erasure.NewEligibility())
}
