package v1

import (
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"

	"github.com/gofiber/fiber/v3"
)

// Services bundles all service-layer dependencies for route registration.
type Services struct {
	Org          *services.OrganizationService
	User         *services.UserService
	Member       *services.MembershipService
	Invitation   *services.InvitationService
	Cert         *services.CertificateService
	Product      *services.ProductService
	Person       *services.PersonService
	Vehicle      *services.VehicleService
	NFe          *nfesvc.NfeService
	NFCe         *nfesvc.NfceService
	MDFe         *mdfesvc.MdfeService
	NfeConfig    *services.NfeConfigService
	NfceConfig   *services.NfceConfigService
	CteConfig    *services.CteConfigService
	MdfeConfig   *services.MdfeConfigService
	Distribution *services.DistributionService
	External     *services.ExternalService
	AuditLog     *services.AuditLogService
	RoleRepo     *repositories.RoleRepository
}

// Register mounts all /v1.0 routes onto the Fiber app.
func Register(app *fiber.App, cacheBackend cache.Backend, cfg *config.Config, wsReg ws.Registry, awsClients *awsclient.Clients, svcs Services) {
	// The issuer is ctech-account's public base URL — the iss claim it signs into tokens.
	verifier := middleware.NewVerifier(cfg.CtechJWKSURL, cfg.ServiceAudience, cfg.CtechURL, cacheBackend)
	authMw := verifier.Middleware()
	perm := middleware.NewPermChecker(svcs.Member, svcs.RoleRepo, cacheBackend)

	v1 := app.Group("/v1.0")
	RegisterHealth(v1, cacheBackend, awsClients, cfg)
	RegisterAuth(v1, svcs.User, svcs.Org, svcs.RoleRepo, authMw)
	RegisterOrganizations(v1, OrgHandlers{
		OrgSvc:     svcs.Org,
		CertSvc:    svcs.Cert,
		NfeConfig:  svcs.NfeConfig,
		NfceConfig: svcs.NfceConfig,
		CteConfig:  svcs.CteConfig,
		MdfeConfig: svcs.MdfeConfig,
		UserSvc:    svcs.User,
		MemberSvc:  svcs.Member,
		InvSvc:     svcs.Invitation,
	}, authMw, perm)
	RegisterInvitations(v1, svcs.Invitation, svcs.User, authMw)
	RegisterProducts(v1, svcs.Product, svcs.User, authMw, perm)
	RegisterPersons(v1, svcs.Person, svcs.User, authMw, perm)
	RegisterVehicles(v1, svcs.Vehicle, svcs.User, authMw, perm)
	RegisterNFes(v1, svcs.NFe, svcs.External, svcs.User, authMw, perm)
	RegisterNFCes(v1, svcs.NFCe, svcs.External, svcs.User, authMw, perm)
	RegisterMDFes(v1, svcs.MDFe, svcs.External, svcs.User, authMw, perm)
	RegisterDistributions(v1, svcs.Distribution, authMw, perm)
	RegisterExternal(v1, svcs.External, authMw, perm)
	RegisterAuditLogs(v1, svcs.AuditLog, authMw, perm)
	RegisterWS(v1, verifier, svcs.Member, wsReg, cfg.CorsAllowedOrigins)
}
