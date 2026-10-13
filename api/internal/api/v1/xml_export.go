package v1

import (
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// HeaderNextCursor carries the cursor of the next export page.
const HeaderNextCursor = "X-Next-Cursor"

const contentTypeZip = "application/zip"

// RegisterXMLExport mounts the bulk XML export (org via header). Any member
// may export (plan ruling R12): the permission is the family's read
// permission (list.nfes, list.nfces, list.ctes, list.mdfes, list.nfses),
// which every role holds. An unknown doc_type fails RBAC with 403.
func RegisterXMLExport(router fiber.Router, svc *services.XMLExportService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/xml-export", authMw)
	g.Get("/:doc_type", perm.RequireDynamic("list.%ss", "doc_type"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		zipped, next, err := svc.Page(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		if n := buildNextCursor(next, cursor); n != nil {
			c.Set(HeaderNextCursor, *n)
		}
		return sendAttachment(c, zipped, contentTypeZip, c.Params("doc_type")+"-xml", ".zip")
	})
}
