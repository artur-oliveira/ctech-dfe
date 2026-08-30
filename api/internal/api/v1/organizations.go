package v1

import (
	"encoding/json"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/validation"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// OrgHandlers bundles services needed by organization routes.
type OrgHandlers struct {
	OrgSvc      *services.OrganizationService
	CertSvc     *services.CertificateService
	NfeConfig   *services.NfeConfigService
	NfceConfig  *services.NfceConfigService
	CteConfig   *services.CteConfigService
	MdfeConfig  *services.MdfeConfigService
	NfseConfig  *services.NfseConfigService
	SerieClaims *repositories.SerieClaimRepository
	// LinkSvc adopts a company created in ctech-account (the handoff's return
	// leg). Nil where the ctech-account credential is not configured.
	LinkSvc    *services.LinkService
	UserSvc    *services.UserService
	MemberSvc  *services.MembershipService
	InvSvc     *services.InvitationService
	BillingSvc *services.BillingService
}

// RegisterOrganizations mounts all /organizations routes.
func RegisterOrganizations(router fiber.Router, h OrgHandlers, authMw fiber.Handler, perm *middleware.PermChecker) {
	orgs := router.Group("/organizations", authMw)

	// GET /organizations — list organizations the authenticated user belongs to
	orgs.Get("", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		memberships, err := h.MemberSvc.ListByUser(c.Context(), userID)
		if err != nil {
			return sendProblem(c, err)
		}
		result := make([]map[string]any, 0, len(memberships))
		for _, mem := range memberships {
			if mem.OrgPK == "" {
				continue
			}
			org, orgErr := h.OrgSvc.Get(c.Context(), mem.OrgPK)
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

	// GET /organizations/certificate-requirement?cpf_or_cnpj=... — whether the
	// caller must upload an A1 certificate to create this org (false when they
	// can inherit a matriz certificate for the same CNPJ root). Drives the UI.
	orgs.Get("/certificate-requirement", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		doc := c.Query("cpf_or_cnpj")
		if doc == "" {
			return sendProblem(c, problem.BadRequest("cpf_or_cnpj é obrigatório"))
		}
		required, err := h.OrgSvc.CertificateRequired(c.Context(), userID, doc)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"required": required})
	})

	// POST /organizations — create (no tenant context; user becomes OWNER).
	// multipart/form-data: `data` (JSON OrganizationCreateBody) + optional `file`
	// (A1 PFX) + `password`. KYC: a certificate is required unless the caller can
	// inherit a matriz certificate for the same CNPJ root (filial).
	orgs.Post("", func(c fiber.Ctx) error {
		var dto OrganizationCreateBody
		if err := json.Unmarshal([]byte(c.FormValue("data")), &dto); err != nil {
			return sendProblem(c, problem.BadRequest("campo 'data' inválido: "+err.Error()))
		}
		if p := validation.Struct(&dto); p != nil {
			return sendProblem(c, p)
		}
		if err := services.RequirePJFields(dto.CpfOrCnpj, dto.Person.Crt); err != nil {
			return sendProblem(c, err)
		}
		av, err := structToAV(dto)
		if err != nil {
			return sendProblem(c, err)
		}

		pfx, readErr := readOptionalUpload(c, "file")
		if readErr != nil {
			return sendProblem(c, readErr)
		}
		password := c.FormValue("password")

		userID, userName := resolveActor(c, h.UserSvc)
		// The company quota is checked here rather than in the subscription
		// middleware: there is no organization yet, so the gate has nothing to
		// resolve, and the limit belongs to the **caller's** account — the one
		// that is about to own this organization and pay for it.
		quotaGuard, err := h.BillingSvc.CompanyQuotaGuard(c.Context(), userID)
		if err != nil {
			return sendProblem(c, err)
		}
		var quotaTx []types.TransactWriteItem
		if quotaGuard != nil {
			quotaTx = append(quotaTx, *quotaGuard)
		}
		org, err := h.OrgSvc.CreateWithOwner(c.Context(), dto.CpfOrCnpj, userID, userName, av, pfx, password, quotaTx...)
		if err != nil {
			return sendProblem(c, err)
		}
		createdOrgPK, _ := repositories.ParseOrgPK(dto.CpfOrCnpj)
		if err := h.BillingSvc.ReportCompanyUsage(c.Context(), createdOrgPK); err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(org)
		if err != nil {
			return sendProblem(c, err)
		}
		// Refresh the caller's cached /auth/me + org list so the new org shows immediately.
		h.UserSvc.InvalidateCache(c.Context(), userID)
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// All routes below require org membership and per-route permission.
	scoped := orgs.Group("/:org_pk")

	RegisterOrganizationPlan(scoped, h.BillingSvc, perm)

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
			crt := dto.Person.Crt
			if dto.Person.Crt == nil {
				current, err := h.OrgSvc.Get(c.Context(), orgPK)
				if err != nil {
					return sendProblem(c, err)
				}
				currentMap, err := unmarshal(current)
				if err != nil {
					return sendProblem(c, err)
				}
				crt = extractCrt(currentMap)
			}
			if err := services.RequirePJFields(orgPK, crt); err != nil {
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

	// POST /organizations/link — the handoff's return leg.
	//
	// Outside the org-scoped group on purpose: there is no local organization
	// yet, so RBAC has nothing to resolve. Authorization is the reach check
	// against ctech-account, which is the only thing that knows whether this
	// person may act for the company they were just handed.
	router.Post("/organizations/link", authMw, func(c fiber.Ctx) error {
		if h.LinkSvc == nil || !h.LinkSvc.Enabled() {
			return sendProblem(c, problem.BadRequest("a integração com a conta CTech não está configurada nesta instalação"))
		}
		var body struct {
			OrganizationID string `json:"organization_id"`
			CompanyID      string `json:"company_id"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		org, err := h.LinkSvc.Link(c.Context(), body.OrganizationID, body.CompanyID, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(org)
		if err != nil {
			return sendProblem(c, err)
		}
		// 200 rather than 201: a refresh replays the same ids and finds the
		// company already linked, and answering "created" the second time would
		// be a lie the browser has no way to check.
		return c.JSON(m)
	})

	// ── Fiscal configs ───────────────────────────────────────────────────────
	//
	// The modelo on each is what the série claim is keyed by, and it is the
	// SEFAZ's code, not ours. NFS-e passes none: it is municipal, its numbering
	// is not keyed (CNPJ, modelo, série, número, ambiente), and claiming a
	// national série for it would refuse a collision that cannot happen.
	serieDeps := func(modelo string) fiscalConfigDeps {
		return fiscalConfigDeps{orgSvc: h.OrgSvc, claims: h.SerieClaims, modelo: modelo}
	}
	registerFiscalConfig(scoped, "/nfe-config",
		"get.organization_nfe_configs", "update.organization_nfe_configs",
		h.NfeConfig, perm, bindAVValidated[FiscalConfigBody], h.UserSvc, serieDeps(services.ModelNFe))
	registerFiscalConfig(scoped, "/nfce-config",
		"get.organization_nfce_configs", "update.organization_nfce_configs",
		h.NfceConfig, perm, bindAVValidated[NfceConfigBody], h.UserSvc, serieDeps(services.ModelNFCe))
	registerFiscalConfig(scoped, "/cte-config",
		"get.organization_cte_configs", "update.organization_cte_configs",
		h.CteConfig, perm, bindAVValidated[FiscalConfigBody], h.UserSvc, serieDeps(services.ModelCTe))
	registerFiscalConfig(scoped, "/mdfe-config",
		"get.organization_mdfe_configs", "update.organization_mdfe_configs",
		h.MdfeConfig, perm, bindAVValidated[MdfeConfigBody], h.UserSvc, serieDeps(services.ModelMDFe))
	registerFiscalConfig(scoped, "/nfse-config",
		"get.organization_nfse_configs", "update.organization_nfse_configs",
		h.NfseConfig, perm, bindAVValidated[NfseConfigBody], h.UserSvc, fiscalConfigDeps{})

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
		buf, err := readOptionalUpload(c, "file")
		if err != nil {
			return sendProblem(c, err)
		}
		if buf == nil {
			return sendProblem(c, problem.BadRequest("arquivo do certificado é obrigatório"))
		}
		password := c.FormValue("password")
		// The organization's own document, off the record: a certificate must
		// belong to the company it is uploaded for, and the key stopped
		// carrying a document (ctech-billing ADR 0022).
		company, err := h.OrgSvc.Company(c.Context(), orgPK)
		if err != nil {
			return sendProblem(c, err)
		}
		// IssuerDoc, not company.TaxID: a record that has not been through the
		// re-key carries none, and its key is still a legacy CNPJ_ one. Reading
		// the field alone would refuse every existing customer's upload on the
		// day this ships.
		expectedDoc := ""
		if company != nil {
			expectedDoc, _ = services.IssuerDoc(company.TaxID, company.TaxIDKind, orgPK)
		} else {
			expectedDoc, _ = services.IssuerDoc("", "", orgPK)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		result, err := h.CertSvc.Upload(c.Context(), orgPK, expectedDoc, buf, password, "", userID, userName)
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

	// ── Members & invitations ─────────────────────────────────────────────────
	registerMemberRoutes(scoped, h, perm)
}
