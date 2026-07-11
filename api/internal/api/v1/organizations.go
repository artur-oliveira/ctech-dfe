package v1

import (
	"mime/multipart"

	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// OrgHandlers bundles services needed by organization routes.
type OrgHandlers struct {
	OrgSvc     *services.OrganizationService
	CertSvc    *services.CertificateService
	NfeConfig  *services.NfeConfigService
	NfceConfig *services.NfceConfigService
	CteConfig  *services.CteConfigService
	MdfeConfig *services.MdfeConfigService
	UserSvc    *services.UserService
}

// RegisterOrganizations mounts all /organizations routes.
func RegisterOrganizations(router fiber.Router, h OrgHandlers, authMw fiber.Handler, perm *middleware.PermChecker) {
	orgs := router.Group("/organizations", authMw)

	// GET /organizations — list organizations the authenticated user belongs to
	orgs.Get("", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		user, err := h.UserSvc.GetMe(c.Context(), userID)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(user)
		if err != nil {
			return sendProblem(c, err)
		}
		orgsRaw, _ := m["organizations"].([]any)
		result := make([]map[string]any, 0, len(orgsRaw))
		for _, entry := range orgsRaw {
			ref, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			pk, _ := ref["pk"].(string)
			if pk == "" {
				continue
			}
			org, orgErr := h.OrgSvc.Get(c.Context(), pk)
			if orgErr != nil || org == nil {
				continue
			}
			orgMap, umErr := unmarshal(org)
			if umErr != nil {
				continue
			}
			result = append(result, orgMap)
		}
		return c.JSON(result)
	})

	// POST /organizations — create (no tenant context; user becomes OWNER after creation)
	orgs.Post("", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		var dto OrganizationCreateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		if err := services.RequirePJFields(dto.CpfOrCnpj, dto.Person.Crt); err != nil {
			return sendProblem(c, err)
		}
		if err := services.RequireOrgIE(dto.CpfOrCnpj, toStateRegEntries(dto.Person.StateRegistrations)); err != nil {
			return sendProblem(c, err)
		}
		av, err := structToAV(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		org, err := h.OrgSvc.Create(c.Context(), dto.CpfOrCnpj, av)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(org)
		if err != nil {
			return sendProblem(c, err)
		}
		if orgPK, ok := m["pk"].(string); ok && orgPK != "" {
			if err := h.UserSvc.AttachToOrg(c.Context(), userID, orgPK, "OWNER", repositories.AllPermissions); err != nil {
				return sendProblem(c, err)
			}
			h.UserSvc.InvalidateCache(c.Context(), userID)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// All routes below require org membership and per-route permission.
	scoped := orgs.Group("/:org_pk")

	// GET /organizations/:org_pk
	scoped.Get("", perm.Require("get.organizations"), func(c fiber.Ctx) error {
		org, err := h.OrgSvc.Get(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})

	// PUT /organizations/:org_pk
	scoped.Put("", perm.Require("update.organizations"), func(c fiber.Ctx) error {
		var dto OrganizationUpdateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		if dto.Person != nil {
			orgPK := middleware.GetOrgPK(c)
			crt, regs := dto.Person.Crt, toStateRegEntries(dto.Person.StateRegistrations)
			if dto.Person.Crt == nil || dto.Person.StateRegistrations == nil {
				current, err := h.OrgSvc.Get(c.Context(), orgPK)
				if err != nil {
					return sendProblem(c, err)
				}
				currentMap, err := unmarshal(current)
				if err != nil {
					return sendProblem(c, err)
				}
				currentCrt, currentRegs := extractCrtAndRegs(currentMap)
				if dto.Person.Crt == nil {
					crt = currentCrt
				}
				if dto.Person.StateRegistrations == nil {
					regs = currentRegs
				}
			}
			if err := services.RequirePJFields(orgPK, crt); err != nil {
				return sendProblem(c, err)
			}
			if err := services.RequireOrgIE(orgPK, regs); err != nil {
				return sendProblem(c, err)
			}
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		org, err := h.OrgSvc.Update(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})

	// ── Authorized XML viewers (SEFAZ autXML) ──────────────────────────────────

	scoped.Post("/authorized-viewers", perm.Require("update.organizations"), func(c fiber.Ctx) error {
		var dto AuthorizedViewerBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		entry := services.AuthorizedViewerEntry{CpfOrCnpj: dto.CpfOrCnpj, Name: dto.Name}
		org, err := h.OrgSvc.AddAuthorizedViewer(c.Context(), middleware.GetOrgPK(c), entry, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})

	scoped.Delete("/authorized-viewers/:cpf_cnpj", perm.Require("update.organizations"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, h.UserSvc)
		org, err := h.OrgSvc.RemoveAuthorizedViewer(c.Context(), middleware.GetOrgPK(c), c.Params("cpf_cnpj"), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})

	// ── Fiscal configs ───────────────────────────────────────────────────────
	registerFiscalConfig(scoped, "/nfe-config",
		"get.organization_nfe_configs", "update.organization_nfe_configs",
		h.NfeConfig, perm, bindAVValidated[FiscalConfigBody], h.UserSvc)
	registerFiscalConfig(scoped, "/nfce-config",
		"get.organization_nfce_configs", "update.organization_nfce_configs",
		h.NfceConfig, perm, bindAVValidated[NfceConfigBody], h.UserSvc)
	registerFiscalConfig(scoped, "/cte-config",
		"get.organization_cte_configs", "update.organization_cte_configs",
		h.CteConfig, perm, bindAVValidated[FiscalConfigBody], h.UserSvc)
	registerFiscalConfig(scoped, "/mdfe-config",
		"get.organization_mdfe_configs", "update.organization_mdfe_configs",
		h.MdfeConfig, perm, bindAVValidated[FiscalConfigBody], h.UserSvc)

	// ── Certificates ────────────────────────────────────────────────────────

	scoped.Get("/certificates", perm.Require("list.organization_certificates"), func(c fiber.Ctx) error {
		items, err := h.CertSvc.List(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(items)
	})

	scoped.Post("/certificates", perm.Require("create.organization_certificates"), func(c fiber.Ctx) error {
		orgPK := middleware.GetOrgPK(c)
		file, err := c.FormFile("file")
		if err != nil {
			return sendProblem(c, err)
		}
		password := c.FormValue("password")
		f, err := file.Open()
		if err != nil {
			return sendProblem(c, err)
		}
		defer func(f multipart.File) {
			err := f.Close()
			if err != nil {
			}
		}(f)

		buf := make([]byte, file.Size)
		if _, err := f.Read(buf); err != nil {
			return sendProblem(c, err)
		}

		userID, userName := resolveActor(c, h.UserSvc)
		result, err := h.CertSvc.Upload(c.Context(), orgPK, buf, password, "", userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})

	scoped.Delete("/certificates/:md5", perm.Require("delete.organization_certificates"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, h.UserSvc)
		if err := h.CertSvc.Delete(c.Context(), middleware.GetOrgPK(c), c.Params("md5"), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
