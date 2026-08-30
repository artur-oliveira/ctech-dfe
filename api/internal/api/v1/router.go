package v1

import (
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/oauthresource"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"
	nfsesvc "gopkg.aoctech.app/dfe/api/internal/services/nfses"

	"github.com/gofiber/fiber/v3"
)

// Services bundles all service-layer dependencies for route registration.
type Services struct {
	Org             *services.OrganizationService
	User            *services.UserService
	Member          *services.MembershipService
	Invitation      *services.InvitationService
	Cert            *services.CertificateService
	Product         *services.ProductService
	Service         *services.ServiceService
	TaxProfile      *services.TaxProfileService
	Operation       *services.OperationService
	PaymentTerm     *services.PaymentTermService
	PaymentTerminal *services.PaymentTerminalService
	TollProvider    *services.TollProviderService
	CargoUnit       *services.CargoUnitService
	ImportDI        *services.ImportDeclarationService
	InsurancePolicy *services.InsurancePolicyService
	ProductLot      *services.ProductLotService
	FuelPump        *services.FuelPumpService
	VehicleSet      *services.VehicleSetService
	Person          *services.PersonService
	Vehicle         *services.VehicleService
	NFe             *nfesvc.NfeService
	NFCe            *nfesvc.NfceService
	MDFe            *mdfesvc.MdfeService
	Nfse            *nfsesvc.NfseService
	NfeConfig       *services.NfeConfigService
	NfceConfig      *services.NfceConfigService
	CteConfig       *services.CteConfigService
	MdfeConfig      *services.MdfeConfigService
	NfseConfig      *services.NfseConfigService
	// SerieClaims enforces ADR 0022's rule that two companies sharing a CNPJ
	// must not emit on the same série. A repository rather than a service: the
	// rule is one conditional write and has nothing else to decide.
	SerieClaims *repositories.SerieClaimRepository
	// Reach answers whether a person may act for a company, from ctech-account.
	// Nil keeps the product's own row as the access record.
	Reach        *services.ReachService
	Distribution *services.DistributionService
	External     *services.ExternalService
	AuditLog     *services.AuditLogService
	Billing      *services.BillingService
	RoleRepo     *repositories.RoleRepository
}

// Register mounts all /v1.0 routes onto the Fiber app.
func Register(app *fiber.App, cacheBackend cache.Backend, cfg *config.Config, wsReg ws.Registry, awsClients *awsclient.Clients, svcs Services) {
	// The issuer is ctech-account's public base URL — the iss claim it signs into tokens.
	verifier := middleware.NewVerifier(cfg.CtechJWKSURL, cfg.ServiceAudience, cfg.CtechIssuerURL, cacheBackend)
	authMw := verifier.Middleware()
	perm := middleware.NewPermChecker(svcs.Member, svcs.RoleRepo, cacheBackend)
	// The flip. Off unless ctech-account issued this service a credential:
	// reach is an authorization check, and an unconfigured one must leave the
	// previous behaviour in place rather than refuse everybody.
	if svcs.Reach != nil {
		perm = perm.WithReach(svcs.Reach)
	}

	RegisterDocs(app)
	oauthresource.Register(app, cfg.ServiceAudience, cfg.CtechIssuerURL)

	v1 := app.Group("/v1.0")
	// The subscription gate, mounted once on the whole group rather than added to
	// each write route. It is default-deny by shape: a route added tomorrow is
	// gated without anybody remembering, and the exemptions live in one named list
	// in middleware/subscription.go instead of being scattered per handler.
	v1.Use(middleware.RequireActiveSubscription(svcs.Billing))
	RegisterHealth(v1, cacheBackend, awsClients, cfg)
	RegisterAuth(v1, svcs.User, svcs.Org, svcs.RoleRepo, authMw)
	RegisterOrganizations(v1, OrgHandlers{
		OrgSvc:      svcs.Org,
		CertSvc:     svcs.Cert,
		NfeConfig:   svcs.NfeConfig,
		NfceConfig:  svcs.NfceConfig,
		CteConfig:   svcs.CteConfig,
		MdfeConfig:  svcs.MdfeConfig,
		NfseConfig:  svcs.NfseConfig,
		SerieClaims: svcs.SerieClaims,
		UserSvc:     svcs.User,
		MemberSvc:   svcs.Member,
		InvSvc:      svcs.Invitation,
		BillingSvc:  svcs.Billing,
	}, authMw, perm)
	RegisterBilling(v1, app, svcs.Billing, cfg.BillingWebhookSecret, authMw)
	RegisterInvitations(v1, svcs.Invitation, svcs.User, authMw)
	RegisterProducts(v1, svcs.Product, svcs.User, authMw, perm)
	RegisterServices(v1, svcs.Service, svcs.User, authMw, perm)
	RegisterTaxProfiles(v1, svcs.TaxProfile, svcs.User, authMw, perm)
	RegisterTaxTables(v1, authMw)
	RegisterOperations(v1, svcs.Operation, svcs.User, authMw, perm)
	RegisterPaymentTerms(v1, svcs.PaymentTerm, svcs.User, authMw, perm)
	RegisterPaymentTerminals(v1, svcs.PaymentTerminal, svcs.User, authMw, perm)
	RegisterTollProviders(v1, svcs.TollProvider, svcs.User, authMw, perm)
	RegisterCargoUnits(v1, svcs.CargoUnit, svcs.User, authMw, perm)
	RegisterImportDeclarations(v1, svcs.ImportDI, svcs.User, authMw, perm)
	RegisterInsurancePolicies(v1, svcs.InsurancePolicy, svcs.User, authMw, perm)
	RegisterProductLots(v1, svcs.ProductLot, svcs.User, authMw, perm)
	RegisterFuelPumps(v1, svcs.FuelPump, svcs.User, authMw, perm)
	RegisterVehicleSets(v1, svcs.VehicleSet, svcs.User, authMw, perm)
	RegisterPersons(v1, svcs.Person, svcs.User, authMw, perm)
	RegisterVehicles(v1, svcs.Vehicle, svcs.User, authMw, perm)
	RegisterNFes(v1, svcs.NFe, svcs.User, authMw, perm)
	RegisterNFCes(v1, svcs.NFCe, svcs.User, authMw, perm)
	RegisterMDFes(v1, svcs.MDFe, svcs.User, authMw, perm)
	RegisterNfses(v1, svcs.Nfse, svcs.User, authMw, perm)
	RegisterDistributions(v1, svcs.Distribution, authMw, perm)
	RegisterExternal(v1, svcs.External, authMw, perm)
	RegisterAuditLogs(v1, svcs.AuditLog, authMw, perm)
	RegisterWS(v1, verifier, svcs.Member, wsReg, cfg.CorsAllowedOrigins)
}
