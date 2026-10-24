// Package app wires the application using Fx dependency injection.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/middleware/cors"

	"gopkg.aoctech.app/api-commons/accountorgs"
	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/dataexport"
	"gopkg.aoctech.app/api-commons/deadline"
	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/api-commons/oauth2client"
	fiberobs "gopkg.aoctech.app/api-commons/observability/fiber"
	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/api-commons/resilient"
	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	apiv1 "gopkg.aoctech.app/dfe/api/internal/api/v1"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/consumer"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"
	nfsesvc "gopkg.aoctech.app/dfe/api/internal/services/nfses"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.uber.org/fx"
)

// Module is the root Fx module for the application.
var Module = fx.Options(
	fx.Provide(
		// Infrastructure
		config.Load,
		newAWSClients,
		newCacheBackend,
		newWSRegistry,
		newFiberApp,
		newErasureStore,
		// Repositories
		newDynamoDBClient,
		repositories.NewOrganizationRepository,
		repositories.NewCertificateRepository,
		repositories.NewAuditLogRepository,
		repositories.NewUserRepository,
		repositories.NewRoleRepository,
		repositories.NewOrgUserRepository,
		repositories.NewOrgInvitationRepository,
		repositories.NewAccountBillingRepository,
		repositories.NewErasureRepository,
		repositories.NewProductRepository,
		repositories.NewServiceRepository,
		repositories.NewTaxProfileRepository,
		repositories.NewOperationRepository,
		repositories.NewPaymentTermRepository,
		repositories.NewPaymentTerminalRepository,
		repositories.NewTollProviderRepository,
		repositories.NewCargoUnitRepository,
		repositories.NewImportDeclarationRepository,
		repositories.NewInsurancePolicyRepository,
		repositories.NewProductLotRepository,
		repositories.NewServiceLocationRepository,
		repositories.NewReferenceDocumentRepository,
		repositories.NewFuelPumpRepository,
		repositories.NewVehicleSetRepository,
		repositories.NewPersonRepository,
		repositories.NewVehicleRepository,
		repositories.NewNfeConfigRepository,
		repositories.NewNfceConfigRepository,
		repositories.NewCteConfigRepository,
		repositories.NewMdfeConfigRepository,
		repositories.NewNfseConfigRepository,
		repositories.NewSerieClaimRepository,
		repositories.NewNfeRepository,
		repositories.NewNfceRepository,
		repositories.NewCteRepository,
		repositories.NewMdfeRepository,
		repositories.NewNfseRepository,
		repositories.NewNfseDistributionRepository,
		newNfeEventRepository,
		repositories.NewNfeDistributionRepository,
		repositories.NewCteDistributionRepository,
		repositories.NewMdfeDistributionRepository,
		// Services
		newOrganizationService,
		newUserService,
		newErasureService,
		newXMLExportService,
		services.NewMembershipService,
		services.NewInvitationService,
		newBillingClient,
		newReachService,
		newWorkspaceRoleService,
		newLinkService,
		newBillingService,
		newCertificateService,
		newProductService,
		newServiceService,
		newTaxProfileService,
		newOperationService,
		newPaymentTermService,
		newPaymentTerminalService,
		newTollProviderService,
		newCargoUnitService,
		newImportDeclarationService,
		newInsurancePolicyService,
		newProductLotService,
		newServiceLocationService,
		newReferenceDocumentService,
		newFuelPumpService,
		newVehicleSetService,
		newPersonService,
		newVehicleService,
		services.NewNfeConfigService,
		services.NewNfceConfigService,
		services.NewCteConfigService,
		services.NewMdfeConfigService,
		services.NewNfseConfigService,
		newExternalService,
		newWorkerService,
		newAuxiliaryDocumentService,
		newNFeService,
		newNFCeService,
		newMDFeService,
		newNfseService,
		newDistributionService,
		newResultsConsumer,
		newLevelReporter,
		services.NewAuditLogService,
	),
	fx.Invoke(seedRoles),
	fx.Invoke(registerRoutes),
	fx.Invoke(startResultsConsumer),
	fx.Invoke(startErasureConsumer),
	fx.Invoke(startLevelSweeper),
	fx.Invoke(startServer),
)

// seedRoles upserts the built-in RBAC roles on startup. Without it the roles
// table is empty and any USER/VIEWER member is denied everything (OWNER/ADMIN
// work regardless, since they bypass the permission-string check). Upsert is a
// full PutItem, so it is idempotent — replicas booting in parallel write the
// same payload. A failure here fails the boot: running without roles would
// silently break every non-admin member.
func seedRoles(lc fx.Lifecycle, roleRepo *repositories.RoleRepository) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var lastErr error
			for _, r := range repositories.SystemRoles() {
				for attempt := 0; attempt < 3; attempt++ {
					if _, err := roleRepo.Upsert(ctx, r.Name, r.Description, r.Permissions); err != nil {
						lastErr = err
						continue
					}
					lastErr = nil
					break
				}
				if lastErr != nil {
					return fmt.Errorf("seed role %s: %w", r.Name, lastErr)
				}
			}
			slog.Info("seeded RBAC roles", "count", len(repositories.SystemRoles()))
			return nil
		},
	})
}

func newDynamoDBClient(clients *awsclient.Clients) *dynamodb.Client {
	return clients.DynamoDB
}

func newAWSClients(cfg *config.Config) (*awsclient.Clients, error) {
	return awsclient.New(context.Background(), cfg)
}

func newCacheBackend(lc fx.Lifecycle, cfg *config.Config) cache.Backend {
	if cfg.RedisURL == "" {
		slog.Warn("VALKEY_URL not set — using in-memory cache (not shared across replicas)")
		return cache.NewMemoryBackend(1000)
	}
	rb, err := cache.NewRedisBackend(cfg.RedisURL)
	if err != nil {
		slog.Warn("redis connection failed, falling back to in-memory", "err", err)
		return cache.NewMemoryBackend(1000)
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return rb.Ping(ctx) },
		OnStop: func(ctx context.Context) error {
			rb.Client().Close() // valkey.Client.Close() has no return value, unlike go-redis
			return nil
		},
	})
	slog.Info("cache: Redis backend active", "url", cfg.RedisURL)
	return rb
}

func newWSRegistry(lc fx.Lifecycle, cacheBackend cache.Backend) ws.Registry {
	var registry ws.Registry
	if rb, ok := cacheBackend.(*cache.RedisBackend); ok {
		registry = ws.NewRedisRegistry(rb.Client())
		slog.Info("ws: Redis registry active")
	} else {
		registry = ws.NewMemoryRegistry()
		slog.Warn("ws: in-memory registry active (not shared across replicas)")
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return registry.Start(ctx) },
		OnStop:  func(ctx context.Context) error { return registry.Stop(ctx) },
	})
	return registry
}

func newFiberApp(cfg *config.Config) *fiber.App {
	fibercfg := fiber.Config{
		AppName:      "ctech-dfe-api",
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
		ProxyHeader:  fiber.HeaderXForwardedFor,
		TrustProxy:   len(cfg.TrustedProxies) > 0,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: cfg.TrustedProxies,
		},
		ErrorHandler: errorHandler,
	}
	app := fiber.New(fibercfg)
	app.Use(fiberobs.RequestID())
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CorsAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "Dfe-Organization-Pk", "Idempotency-Key"},
		ExposeHeaders:    []string{fiberobs.RequestIDHeader, apiv1.HeaderNextCursor, fiber.HeaderRetryAfter},
		AllowCredentials: true,
		MaxAge:           3600,
	}))
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","status":${status},"latency":"${latency}","method":"${method}",` +
			`"path":"${path}","request_id":"${respHeader:X-Request-Id}"}` + "\n",
		DisableColors: true,
		Skip: func(c fiber.Ctx) bool {
			return c.Path() == "/v1.0/health" || c.Path() == "/v1.0/health-check"
		},
	}))
	// A time budget for every request, so a DynamoDB, S3, Lambda or outbound call
	// ends with it instead of outliving the client. Websocket upgrades are exempt
	// (done by the middleware). No route is skipped: the slowest, the Lambda-egress
	// lookups, already hit WriteTimeout at 10 s.
	if cfg.RequestTimeout > 0 {
		app.Use(deadline.New(cfg.RequestTimeout))
	}
	return app
}

// --- repository factories that need extra args ---

func newNfeEventRepository(db *dynamodb.Client, cfg *config.Config) *repositories.DocumentEventRepository {
	return repositories.NewDocumentEventRepository(db, cfg, "nfe")
}

// --- service factories ---

func newOrganizationService(
	repo *repositories.OrganizationRepository,
	auditRepo *repositories.AuditLogRepository,
	certRepo *repositories.CertificateRepository,
	orgUserRepo *repositories.OrgUserRepository,
	certSvc *services.CertificateService,
	memberSvc *services.MembershipService,
	c cache.Backend,
) *services.OrganizationService {
	return services.NewOrganizationService(repo, auditRepo, certRepo, orgUserRepo, certSvc, memberSvc, c)
}

func newUserService(repo *repositories.UserRepository, c cache.Backend, cfg *config.Config, orgSvc *services.OrganizationService, memberSvc *services.MembershipService, store *erasure.Store) *services.UserService {
	svc := services.NewUserService(repo, c, cfg.CtechURL, orgSvc, memberSvc)
	if blocked := blockedOf(store); blocked != nil {
		svc = svc.WithErasureLock(blocked)
	}
	return svc
}

func newCertificateService(repo *repositories.CertificateRepository, auditRepo *repositories.AuditLogRepository, clients *awsclient.Clients, cfg *config.Config) *services.CertificateService {
	return services.NewCertificateService(repo, auditRepo, clients, cfg.S3BucketCerts)
}

func newProductService(repo *repositories.ProductRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ProductService {
	return services.NewProductService(repo, auditRepo, c)
}

func newServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ServiceService {
	return services.NewServiceService(repo, auditRepo, c)
}

func newTaxProfileService(repo *repositories.TaxProfileRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.TaxProfileService {
	return services.NewTaxProfileService(repo, auditRepo, c)
}

func newOperationService(repo *repositories.OperationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.OperationService {
	return services.NewOperationService(repo, auditRepo, c)
}

func newPaymentTerminalService(repo *repositories.PaymentTerminalRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.PaymentTerminalService {
	return services.NewPaymentTerminalService(repo, auditRepo, c)
}

func newTollProviderService(repo *repositories.TollProviderRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.TollProviderService {
	return services.NewTollProviderService(repo, auditRepo, c)
}

func newCargoUnitService(repo *repositories.CargoUnitRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.CargoUnitService {
	return services.NewCargoUnitService(repo, auditRepo, c)
}

func newImportDeclarationService(repo *repositories.ImportDeclarationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ImportDeclarationService {
	return services.NewImportDeclarationService(repo, auditRepo, c)
}

func newInsurancePolicyService(repo *repositories.InsurancePolicyRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.InsurancePolicyService {
	return services.NewInsurancePolicyService(repo, auditRepo, c)
}

func newProductLotService(repo *repositories.ProductLotRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ProductLotService {
	return services.NewProductLotService(repo, auditRepo, c)
}

func newServiceLocationService(repo *repositories.ServiceLocationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ServiceLocationService {
	return services.NewServiceLocationService(repo, auditRepo, c)
}

func newReferenceDocumentService(repo *repositories.ReferenceDocumentRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ReferenceDocumentService {
	return services.NewReferenceDocumentService(repo, auditRepo, c)
}

func newFuelPumpService(repo *repositories.FuelPumpRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.FuelPumpService {
	return services.NewFuelPumpService(repo, auditRepo, c)
}

func newPaymentTermService(repo *repositories.PaymentTermRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.PaymentTermService {
	return services.NewPaymentTermService(repo, auditRepo, c)
}

func newVehicleSetService(
	repo *repositories.VehicleSetRepository, vehicleRepo *repositories.VehicleRepository,
	auditRepo *repositories.AuditLogRepository, c cache.Backend,
) *services.VehicleSetService {
	return services.NewVehicleSetService(repo, vehicleRepo, auditRepo, c)
}

func newPersonService(repo *repositories.PersonRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.PersonService {
	return services.NewPersonService(repo, auditRepo, c)
}

func newVehicleService(repo *repositories.VehicleRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.VehicleService {
	return services.NewVehicleService(repo, auditRepo, c)
}

func newDistributionService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	NfeConfig *repositories.NfeConfigRepository,
	NfceConfig *repositories.NfceConfigRepository,
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	NfseConfig *repositories.NfseConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	nfseDist *repositories.NfseDistributionRepository,
	clients *awsclient.Clients,
	documentSvc *documents.Service,
	cfg *config.Config,
) *services.DistributionService {
	return services.NewDistributionService(
		orgRepo, certRepo,
		NfeConfig, NfceConfig, CteConfig, MdfeConfig, NfseConfig,
		nfeDist, cteDist, mdfeDist, nfseDist,
		clients, documentSvc,
		cfg.DistributionQueueURL,
		cfg.S3BucketDocuments,
		cfg.S3BucketCerts,
		cfg.SefazFunctionName,
	)
}

func newExternalService(certRepo *repositories.CertificateRepository, orgRepo *repositories.OrganizationRepository, clients *awsclient.Clients, cfg *config.Config) *services.ExternalService {
	return services.NewExternalService(certRepo, orgRepo, clients, cfg.SefazFunctionName, cfg.S3BucketCerts)
}

func newWorkerService(clients *awsclient.Clients, cfg *config.Config) *services.WorkerService {
	return services.NewWorkerService(clients, cfg.WorkerTopicARN, cfg.TablePrefix)
}

func newAuxiliaryDocumentService(clients *awsclient.Clients, cfg *config.Config) (*documents.Service, error) {
	return documents.NewService(clients, cfg.S3BucketDocuments)
}

func newNFeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfeConfigRepository,
	productRepo *repositories.ProductRepository,
	serviceRepo *repositories.ServiceRepository,
	importDIRepo *repositories.ImportDeclarationRepository,
	productLotRepo *repositories.ProductLotRepository,
	fuelPumpRepo *repositories.FuelPumpRepository,
	taxProfileRepo *repositories.TaxProfileRepository,
	operationRepo *repositories.OperationRepository,
	paymentTermRepo *repositories.PaymentTermRepository,
	paymentTerminalRepo *repositories.PaymentTerminalRepository,
	nfeRepo *repositories.NfeRepository,
	eventRepo *repositories.DocumentEventRepository,
	vehicleRepo *repositories.VehicleRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
	cfg *config.Config,
) *nfesvc.NfeService {
	return nfesvc.NewNfeService(
		orgRepo, certRepo, personRepo, configRepo, productRepo, serviceRepo, importDIRepo, productLotRepo, fuelPumpRepo,
		taxProfileRepo, operationRepo, paymentTermRepo,
		paymentTerminalRepo, nfeRepo, eventRepo, vehicleRepo, clients,
		workerSvc, extSvc, billingSvc, documentSvc, cfg.S3BucketDocuments,
		nfesvc.TechData{
			CNPJ:    cfg.TechnicalCNPJ,
			Name:    cfg.TechnicalName,
			Email:   cfg.TechnicalEmail,
			Phone:   cfg.TechnicalPhone,
			Version: cfg.AppVersion,
		},
	)
}

func newNFCeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfceConfigRepository,
	productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository,
	operationRepo *repositories.OperationRepository,
	paymentTerminalRepo *repositories.PaymentTerminalRepository,
	fuelPumpRepo *repositories.FuelPumpRepository,
	nfceRepo *repositories.NfceRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
) *nfesvc.NfceService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "nfce")
	return nfesvc.NewNfceService(
		orgRepo, certRepo, personRepo, configRepo, productRepo, taxProfileRepo, operationRepo,
		paymentTerminalRepo, fuelPumpRepo, nfceRepo, eventRepo, clients, workerSvc, billingSvc, documentSvc, cfg.S3BucketDocuments,
		nfesvc.TechData{
			CNPJ:    cfg.TechnicalCNPJ,
			Name:    cfg.TechnicalName,
			Email:   cfg.TechnicalEmail,
			Phone:   cfg.TechnicalPhone,
			Version: cfg.AppVersion,
		},
	)
}

func newMDFeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	configRepo *repositories.MdfeConfigRepository,
	mdfeRepo *repositories.MdfeRepository,
	nfeRepo *repositories.NfeRepository,
	cteRepo *repositories.CteRepository,
	vehicleRepo *repositories.VehicleRepository,
	personRepo *repositories.PersonRepository,
	vehicleSetRepo *repositories.VehicleSetRepository,
	tollProviderRepo *repositories.TollProviderRepository,
	productRepo *repositories.ProductRepository,
	cargoUnitRepo *repositories.CargoUnitRepository,
	insurancePolicyRepo *repositories.InsurancePolicyRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
) *mdfesvc.MdfeService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "mdfe")
	return mdfesvc.NewMdfeService(
		orgRepo, certRepo, configRepo, mdfeRepo, nfeRepo, cteRepo,
		eventRepo, vehicleRepo, personRepo, vehicleSetRepo, tollProviderRepo, productRepo, cargoUnitRepo,
		insurancePolicyRepo,
		clients, workerSvc, billingSvc, documentSvc, cfg.S3BucketDocuments,
		mdfesvc.TechData{
			CNPJ:    cfg.TechnicalCNPJ,
			Name:    cfg.TechnicalName,
			Email:   cfg.TechnicalEmail,
			Phone:   cfg.TechnicalPhone,
			Version: cfg.AppVersion,
		},
	)
}

// newNfseService monta o serviço de NFS-e. O repositório de eventos é criado
// aqui (e não como provider) porque NewDocumentEventRepository recebe o docType
// — mesmo padrão de newNFCeService/newMDFeService.
func newNfseService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfseConfigRepository,
	serviceRepo *repositories.ServiceRepository,
	nfseRepo *repositories.NfseRepository,
	distRepo *repositories.NfseDistributionRepository,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	clients *awsclient.Clients,
	cacheBackend cache.Backend,
	db *dynamodb.Client,
	cfg *config.Config,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
) *nfsesvc.NfseService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "nfse")
	return nfsesvc.NewNfseService(
		orgRepo, certRepo, personRepo, configRepo, serviceRepo,
		nfseRepo, eventRepo, distRepo, workerSvc, extSvc, billingSvc,
		documentSvc, clients, cacheBackend, cfg.S3BucketDocuments,
	)
}

// --- route registration ---

// Services bundles all service layer dependencies for the route layer.
type Services struct {
	fx.In

	OrgSvc          *services.OrganizationService
	UserSvc         *services.UserService
	MemberSvc       *services.MembershipService
	InvSvc          *services.InvitationService
	CertSvc         *services.CertificateService
	ProductSvc      *services.ProductService
	ServiceSvc      *services.ServiceService
	TaxProfSvc      *services.TaxProfileService
	OperationSvc    *services.OperationService
	PayTermSvc      *services.PaymentTermService
	PayTerminalSvc  *services.PaymentTerminalService
	TollProviderSvc *services.TollProviderService
	CargoUnitSvc    *services.CargoUnitService
	ImportDISvc     *services.ImportDeclarationService
	InsurancePolSvc *services.InsurancePolicyService
	ProductLotSvc   *services.ProductLotService
	SvcLocationSvc  *services.ServiceLocationService
	RefDocSvc       *services.ReferenceDocumentService
	FuelPumpSvc     *services.FuelPumpService
	VehSetSvc       *services.VehicleSetService
	PersonSvc       *services.PersonService
	VehicleSvc      *services.VehicleService
	NfeSvc          *nfesvc.NfeService
	NfceSvc         *nfesvc.NfceService
	MdfeSvc         *mdfesvc.MdfeService
	NfseSvc         *nfsesvc.NfseService
	NfeConf         *services.NfeConfigService
	NfceConf        *services.NfceConfigService
	CteConf         *services.CteConfigService
	MdfeConf        *services.MdfeConfigService
	NfseConf        *services.NfseConfigService
	SerieClaims     *repositories.SerieClaimRepository
	ReachSvc        *services.ReachService
	LinkSvc         *services.LinkService
	DistSvc         *services.DistributionService
	ExternalSvc     *services.ExternalService
	AuditLogSvc     *services.AuditLogService
	BillingSvc      *services.BillingService
	LevelReporter   *services.LevelReporter
	RoleRepo        *repositories.RoleRepository
	Cache           cache.Backend
	WSReg           ws.Registry
	Cfg             *config.Config
	AWS             *awsclient.Clients
	ErasureStore    *erasure.Store
	XMLExport       *services.XMLExportService
}

func registerRoutes(app *fiber.App, svcs Services) {
	apiv1.Register(app, svcs.Cache, svcs.Cfg, svcs.WSReg, svcs.AWS, apiv1.Services{
		Org:             svcs.OrgSvc,
		User:            svcs.UserSvc,
		Member:          svcs.MemberSvc,
		Invitation:      svcs.InvSvc,
		Cert:            svcs.CertSvc,
		Product:         svcs.ProductSvc,
		Service:         svcs.ServiceSvc,
		TaxProfile:      svcs.TaxProfSvc,
		Operation:       svcs.OperationSvc,
		PaymentTerm:     svcs.PayTermSvc,
		PaymentTerminal: svcs.PayTerminalSvc,
		TollProvider:    svcs.TollProviderSvc,
		CargoUnit:       svcs.CargoUnitSvc,
		ImportDI:        svcs.ImportDISvc,
		InsurancePolicy: svcs.InsurancePolSvc,
		ProductLot:      svcs.ProductLotSvc,
		ServiceLocation: svcs.SvcLocationSvc,
		ReferenceDoc:    svcs.RefDocSvc,
		FuelPump:        svcs.FuelPumpSvc,
		VehicleSet:      svcs.VehSetSvc,
		Person:          svcs.PersonSvc,
		Vehicle:         svcs.VehicleSvc,
		NFe:             svcs.NfeSvc,
		NFCe:            svcs.NfceSvc,
		MDFe:            svcs.MdfeSvc,
		Nfse:            svcs.NfseSvc,
		NfeConfig:       svcs.NfeConf,
		NfceConfig:      svcs.NfceConf,
		CteConfig:       svcs.CteConf,
		MdfeConfig:      svcs.MdfeConf,
		NfseConfig:      svcs.NfseConf,
		SerieClaims:     svcs.SerieClaims,
		Reach:           svcs.ReachSvc,
		Link:            svcs.LinkSvc,
		Distribution:    svcs.DistSvc,
		External:        svcs.ExternalSvc,
		AuditLog:        svcs.AuditLogSvc,
		Billing:         svcs.BillingSvc,
		Levels:          svcs.LevelReporter,
		RoleRepo:        svcs.RoleRepo,
		ErasureBlocked:  blockedOf(svcs.ErasureStore),
		XMLExport:       svcs.XMLExport,
	})
}

func startServer(lc fx.Lifecycle, app *fiber.App, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			addr := fmt.Sprintf(":%d", cfg.Port)
			slog.Info("starting ctech-dfe-api", "addr", addr, "env", cfg.Env)
			go func() {
				if err := app.Listen(addr); err != nil {
					slog.Error("server error", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("shutting down server")
			return app.ShutdownWithContext(ctx)
		},
	})
}

func newResultsConsumer(clients *awsclient.Clients, cfg *config.Config, reg ws.Registry, c cache.Backend, billing *services.BillingService) *consumer.ResultsConsumer {
	return consumer.NewResultsConsumer(clients.SQS, cfg.ResultsQueueURL, reg, c, billing)
}

// newLevelReporter delivers companies levels to billing. Billing off → a nil
// sink and a no-op reporter.
func newLevelReporter(repo *repositories.AccountBillingRepository, client *billingclient.Client, billing *services.BillingService) *services.LevelReporter {
	if client == nil {
		return services.NewLevelReporter(repo, nil, billing)
	}
	r := services.NewLevelReporter(repo, client, billing)
	billing.WithLevelFlush(r.Flush)
	return r
}

// startLevelSweeper runs the sweeper for the life of the process.
func startLevelSweeper(lc fx.Lifecycle, r *services.LevelReporter) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { go r.Run(ctx, services.LevelSweepInterval); return nil },
		OnStop:  func(context.Context) error { cancel(); return nil },
	})
}

func startResultsConsumer(lc fx.Lifecycle, rc *consumer.ResultsConsumer) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go rc.Start(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}

func errorHandler(c fiber.Ctx, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return problem.GatewayTimeout("a requisição demorou demais; tente novamente").WithCause(err).Send(c)
	}
	if f, ok := errors.AsType[*fiber.Error](err); ok {
		return fiberErrorProblem(f).Send(c)
	}
	return problem.InternalServer("erro interno").WithCause(err).Send(c)
}

// fiberErrorProblem keeps the API's historical mapping of framework errors: 404
// and 504 keep their meaning, every other fiber error answers a generic 500.
func fiberErrorProblem(err *fiber.Error) *problem.Problem {
	switch err.Code {
	case http.StatusNotFound:
		return problem.NotFound(err.Error()).WithCause(err)
	case http.StatusGatewayTimeout:
		return problem.GatewayTimeout("a requisição demorou demais; tente novamente").WithCause(err)
	default:
		return problem.InternalServer("erro interno").WithCause(err)
	}
}

// newBillingClient builds the ctech-billing client, or nil when this deployment
// does not charge.
//
// Nil is returned rather than an error, and the warning is loud rather than
// fatal: a dev environment with no billing credentials must still start, and a
// production one that lost them must be obvious in the first lines of the log
// rather than at the first subscription attempt.
//
// The token endpoint is derived from CTECH_URL rather than configured
// separately. It is ctech-account's, this service already holds that base URL
// for userinfo, and a second env var naming the same host is a second thing that
// can point somewhere else.
// newLinkService builds the handoff's return leg, or nil when ctech-account has
// not issued this service a credential.
//
// Nil means the link route answers "not configured" rather than linking on
// trust: the ids arrive on a URL the person controls, and without the reach
// check anybody could adopt any company id they can type.
func newLinkService(
	cfg *config.Config,
	orgRepo *repositories.OrganizationRepository,
	orgUserRepo *repositories.OrgUserRepository,
	reach *services.ReachService,
	userSvc *services.UserService,
	c cache.Backend,
) *services.LinkService {
	if reach == nil {
		slog.Warn("the ctech-account link route is off — a company created in the account cannot be adopted here")
		return nil
	}
	client := accountclient.New(accountclient.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.AccountClientID,
		ClientSecret: cfg.AccountClientSecret,
		Cache:        c,
	})
	if client == nil {
		return nil
	}
	return services.NewLinkService(orgRepo, orgUserRepo, reach, client, userSvc)
}

// newBillingService builds the billing service with the company quota counting
// what is ENABLED rather than what is owned (ctech-billing ADR 0021).
//
// A wrapper rather than a longer constructor: the enablement is one concern with
// one wiring, and threading five config repositories through
// services.NewBillingService would put "what enabled means" in its signature.
func newBillingService(
	repo *repositories.AccountBillingRepository,
	client *billingclient.Client,
	users *services.UserService,
	orgs *services.OrganizationService,
	nfe *repositories.NfeConfigRepository,
	nfce *repositories.NfceConfigRepository,
	cte *repositories.CteConfigRepository,
	mdfe *repositories.MdfeConfigRepository,
	nfse *repositories.NfseConfigRepository,
	c cache.Backend,
	roles *services.WorkspaceRoleService,
) *services.BillingService {
	// A nil *WorkspaceRoleService refuses every management attempt (fail
	// closed); it is passed as a typed nil on purpose, which its methods handle.
	return services.NewBillingService(repo, client, users, orgs, c).
		WithEnablement(services.NewFiscalConfigEnablement(nfe, nfce, cte, mdfe, nfse)).
		WithWorkspaceRoles(roles)
}

// newReachService builds the reach check, or nil when ctech-account has not
// issued this service a credential for it.
//
// Nil means OFF: the product's own membership row stays the access record, which
// is the pre-flip behaviour. That default is deliberate — turning reach on is a
// live authorization change with no dry run, and an unconfigured deployment must
// leave the previous behaviour in place rather than refuse everybody.
//
// The token endpoint is derived from CTECH_URL, like billing's, for the same
// reason: a second env var naming the same host is a second thing that can point
// somewhere else.
func newReachService(cfg *config.Config, c cache.Backend) *services.ReachService {
	client := accountclient.New(accountclient.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.AccountClientID,
		ClientSecret: cfg.AccountClientSecret,
		Cache:        c,
	})
	if client == nil {
		slog.Warn("the ctech-account reach check is not configured — the product's own membership row remains the access record",
			"account_client_id_set", cfg.AccountClientID != "",
			"account_client_secret_set", cfg.AccountClientSecret != "")
		return nil
	}
	slog.Info("the ctech-account reach check is ON — a membership row with no company edge grants nothing")
	return services.NewReachService(client, c)
}

// newWorkspaceRoleService builds the role check for managing an organization's
// subscription. Nil when ctech-account has not issued the credential: the
// service then answers every management attempt with 403, and reading the plan
// keeps working.
func newWorkspaceRoleService(cfg *config.Config, c cache.Backend) *services.WorkspaceRoleService {
	client := accountorgs.New(accountorgs.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.AccountWorkspaceClientID,
		ClientSecret: cfg.AccountWorkspaceClientSecret,
		Cache:        c,
	})
	if client == nil {
		slog.Warn("the ctech-account workspace credential is not configured — nobody can manage a DF-e subscription")
		return nil
	}
	return services.NewWorkspaceRoleService(client, c)
}

func newBillingClient(cfg *config.Config, c cache.Backend) *billingclient.Client {
	client := billingclient.New(billingclient.Config{
		BaseURL:      cfg.BillingAPIURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.BillingClientID,
		ClientSecret: cfg.BillingClientSecret,
		Cache:        c,
	})
	if client == nil {
		slog.Warn("billing is not configured — running in NO-CHARGE mode, every account is unlimited",
			"billing_api_url_set", cfg.BillingAPIURL != "",
			"client_id_set", cfg.BillingClientID != "",
			"client_secret_set", cfg.BillingClientSecret != "")
		return nil
	}
	if cfg.BillingWebhookSecret == "" {
		// The client works without it — it signs nothing — but billing's
		// notify-back cannot be verified, so that route is not mounted and every
		// subscription change waits on the 60s snapshot TTL instead of arriving.
		slog.Warn("BILLING_WEBHOOK_SECRET is unset — billing's webhook route will not be mounted")
	}
	return client
}

const erasureAckPath = "/v1.0/internal/erasure/ack"

const exportAckPath = "/v1.0/internal/export/ack"

// newErasureStore binds {prefix}_erasure_state, or nil (participant off) when
// ERASURE_QUEUE_URL is unset. Tombstones are kept forever (erasedTTL 0): dfe
// retains nothing, and the record is what rejects a late message.
func newErasureStore(clients *awsclient.Clients, cfg *config.Config) *erasure.Store {
	if cfg.ErasureQueueURL == "" {
		slog.Warn("ERASURE_QUEUE_URL is unset: the account-deletion lock and purge are OFF")
		return nil
	}
	return erasure.NewStore(clients.DynamoDB, cfg.TablePrefix, 0)
}

func blockedOf(store *erasure.Store) services.BlockedFunc {
	if store == nil {
		return nil
	}
	return store.Blocked
}

func newErasureService(
	repo *repositories.ErasureRepository,
	certs *repositories.CertificateRepository,
	audit *repositories.AuditLogRepository,
	claims *repositories.SerieClaimRepository,
	store *erasure.Store,
	clients *awsclient.Clients,
	cfg *config.Config,
	c cache.Backend,
) *services.ErasureService {
	if store == nil {
		return nil
	}
	return services.NewErasureService(repo, certs, audit, claims, store, clients.S3,
		cfg.S3BucketDocuments, cfg.S3BucketCerts, newErasureSubject(cfg, c).CPF, c)
}

// newErasureSubject builds the CPF lookup from the dedicated erasure client
// (ruling R11), or nil when that credential is incomplete.
func newErasureSubject(cfg *config.Config, c cache.Backend) *accountclient.SubjectClient {
	return accountclient.NewSubject(accountclient.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.ErasureClientID,
		ClientSecret: cfg.ErasureClientSecret,
		Cache:        c,
	})
}

// startErasureConsumer runs ctech-go-common's erasure.Consumer in the
// background, like the results consumer (plan ruling R1). It needs the
// dedicated erasure credential to ack (ruling R11); without it the purge
// cannot finish, so it does not start and says so.
func startErasureConsumer(lc fx.Lifecycle, cfg *config.Config, clients *awsclient.Clients, store *erasure.Store, svc *services.ErasureService, c cache.Backend) {
	if svc == nil {
		return
	}
	if cfg.ErasureClientID == "" || cfg.ErasureClientSecret == "" {
		slog.Error("erasure consumer NOT started: ERASURE_CLIENT_ID/ERASURE_CLIENT_SECRET are unset, so acks cannot be sent")
		return
	}
	httpClient := resilient.Client(10*time.Second, resilient.Config{}) // the account: acks and token; a down account is refused at once
	tokens := oauth2client.New(httpClient, c, billingclient.TokenURLFor(cfg.CtechURL),
		cfg.ErasureClientID, cfg.ErasureClientSecret, accountclient.ScopeErasureAck)
	acks := erasure.NewAckClient(httpClient, strings.TrimSuffix(cfg.CtechURL, "/")+erasureAckPath, tokens)
	// The personal-data export rides the same queue (ctech-account
	// docs/specs/2026-10-17-data-export.md): the part is PUT to a presigned S3 URL
	// and acked, with a longer timeout than the saga's because it is data.
	uploads := &http.Client{Timeout: 60 * time.Second}
	exportTokens := oauth2client.New(httpClient, c, billingclient.TokenURLFor(cfg.CtechURL),
		cfg.ErasureClientID, cfg.ErasureClientSecret, accountclient.ScopeExportAck)
	exporter := dataexport.NewStreamHandler(services.ErasureServiceName, svc.DataExporter().Write, uploads,
		dataexport.NewAckClient(uploads, strings.TrimSuffix(cfg.CtechURL, "/")+exportAckPath, exportTokens))
	consumer := erasure.NewConsumer(clients.SQS, cfg.ErasureQueueURL, services.ErasureServiceName, store,
		services.AlertingPurge(svc.Purge, newAlerts(cfg)), acks).
		WithExporter(exporter.Handle).WithVisibilityHeartbeat(cfg.ErasureQueueVisibility)
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() { _ = consumer.Run(ctx) }()
			slog.Info("erasure consumer started", "queue", cfg.ErasureQueueURL)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

// newAlerts builds the operator-alert publisher; unconfigured or unavailable
// degrades to alerts.Nop (logged), never to a startup error.
func newAlerts(cfg *config.Config) alerts.Publisher {
	p, err := alerts.New(context.Background(), cfg.AWSRegion, cfg.AlertsTopicARN, "dfe", cfg.Env)
	if err != nil {
		slog.Warn("alerts: publisher unavailable, operator alerts are log-only", "err", err)
		return alerts.Nop{}
	}
	return p
}

func newXMLExportService(
	nfe *repositories.NfeRepository, nfce *repositories.NfceRepository, cte *repositories.CteRepository,
	mdfe *repositories.MdfeRepository, nfse *repositories.NfseRepository,
	db *dynamodb.Client, clients *awsclient.Clients, cfg *config.Config,
) *services.XMLExportService {
	docs := map[string]*repositories.DocumentRepository{
		"nfe": &nfe.DocumentRepository, "nfce": &nfce.DocumentRepository, "cte": &cte.DocumentRepository,
		"mdfe": &mdfe.DocumentRepository, "nfse": &nfse.DocumentRepository,
	}
	events := make(map[string]*repositories.DocumentEventRepository, len(docs))
	for docType := range docs {
		events[docType] = repositories.NewDocumentEventRepository(db, cfg, docType)
	}
	return services.NewXMLExportService(docs, events, clients.S3, cfg.S3BucketDocuments)
}
