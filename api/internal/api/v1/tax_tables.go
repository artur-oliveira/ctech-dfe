package v1

import (
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"

	"gopkg.aoctech.app/dfe/api/internal/problem"

	"github.com/gofiber/fiber/v3"
)

// RegisterTaxTables mounts GET /v1.0/tax-tables/icms-aliq — a stateless
// preview of the ICMS/FCP rate the backend would resolve for emit_uf/dest_uf/
// ncm without any override, used by the frontend's aliquot-divergence
// warning (design spec 2026-08-09-tax-config-redesign §Modelo de dados 6).
// Calls nfesvc.PreviewICMSAliq directly instead of going through a service
// struct: internal/services already imports internal/services/nfes for the
// NFe/NFCe services, so a services.TaxTableService wrapping nfesvc would
// create an import cycle for no behavioral gain.
func RegisterTaxTables(v1 fiber.Router, authMw fiber.Handler) {
	g := v1.Group("/tax-tables", authMw)
	g.Get("/icms-aliq", func(c fiber.Ctx) error {
		emitUF := c.Query("emit_uf")
		destUF := c.Query("dest_uf")
		ncm := c.Query("ncm")
		if emitUF == "" || destUF == "" {
			return sendProblem(c, problem.BadRequest("emit_uf e dest_uf são obrigatórios"))
		}
		icmsAliq, fcpAliq := nfesvc.PreviewICMSAliq(emitUF, destUF, ncm)
		return c.JSON(fiber.Map{"icms_aliq": icmsAliq, "fcp_aliq": fcpAliq})
	})
}
