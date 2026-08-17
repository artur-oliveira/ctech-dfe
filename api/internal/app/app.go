// Package app wires the application using Fx dependency injection.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"time"

	"github.com/gofiber/fiber/v3/middleware/cors"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	apiv1 "gopkg.aoctech.app/dfe/api/internal/api/v1"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/consumer"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"
	nfsesvc "gopkg.aoctech.app/dfe/api/internal/services/nfses"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/requestid"
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
		repositories.NewProductRepository,
		repositories.NewServiceRepository,
		repositories.NewTaxProfileRepository,
		repositories.NewOperationRepository,
		repositories.NewPaymentTermRepository,
		repositories.NewVehicleSetRepository,
		repositories.NewPersonRepository,
		repositories.NewVehicleRepository,
		repositories.NewNfeConfigRepository,
		repositories.NewNfceConfigRepository,
		repositories.NewCteConfigRepository,
		repositories.NewMdfeConfigRepository,
		repositories.NewNfseConfigRepository,
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
		services.NewMembershipService,
		services.NewInvitationService,
		newBillingClient,
		services.NewBillingService,
		newCertificateService,
		newProductService,
		newServiceService,
		newTaxProfileService,
		newOperationService,
		newPaymentTermService,
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
		newNFeService,
		newNFCeService,
		newMDFeService,
		newNfseService,
		newDistributionService,
		newResultsConsumer,
		services.NewAuditLogService,
	),
	fx.Invoke(seedRoles),
	fx.Invoke(registerRoutes),
	fx.Invoke(startResultsConsumer),
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
		TrustProxy:   cfg.TrustedProxies != nil && len(cfg.TrustedProxies) > 0,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: cfg.TrustedProxies,
		},
		ErrorHandler: errorHandler,
	}
	app := fiber.New(fibercfg)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CorsAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "Dfe-Organization-Pk"},
		AllowCredentials: true,
		MaxAge:           3600,
	}))
	app.Use(middleware.Recover())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","status":${status},"latency":"${latency}","method":"${method}","path":"${path}","request-id":"${request-id}"}` + "\n",
	}))
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

func newUserService(repo *repositories.UserRepository, c cache.Backend, cfg *config.Config, orgSvc *services.OrganizationService, memberSvc *services.MembershipService) *services.UserService {
	return services.NewUserService(repo, c, cfg.CtechURL, orgSvc, memberSvc)
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
	cfg *config.Config,
) *services.DistributionService {
	return services.NewDistributionService(
		orgRepo, certRepo,
		NfeConfig, NfceConfig, CteConfig, MdfeConfig, NfseConfig,
		nfeDist, cteDist, mdfeDist, nfseDist,
		clients,
		cfg.DistributionQueueURL,
		cfg.S3BucketDocuments,
		cfg.S3BucketCerts,
		cfg.SefazFunctionName,
	)
}

func newExternalService(certRepo *repositories.CertificateRepository, clients *awsclient.Clients, cfg *config.Config) *services.ExternalService {
	return services.NewExternalService(certRepo, clients, cfg.SefazFunctionName, cfg.S3BucketCerts)
}

func newWorkerService(clients *awsclient.Clients, cfg *config.Config) *services.WorkerService {
	return services.NewWorkerService(clients, cfg.WorkerTopicARN, cfg.TablePrefix)
}

func newNFeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfeConfigRepository,
	productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository,
	operationRepo *repositories.OperationRepository,
	paymentTermRepo *repositories.PaymentTermRepository,
	nfeRepo *repositories.NfeRepository,
	eventRepo *repositories.DocumentEventRepository,
	vehicleRepo *repositories.VehicleRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	billingSvc *services.BillingService,
	cfg *config.Config,
) *nfesvc.NfeService {
	return nfesvc.NewNfeService(
		orgRepo, certRepo, personRepo, configRepo, productRepo, taxProfileRepo, operationRepo, paymentTermRepo,
		nfeRepo, eventRepo, vehicleRepo, clients,
		workerSvc, extSvc, billingSvc, cfg.S3BucketDocuments,
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
	nfceRepo *repositories.NfceRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
	billingSvc *services.BillingService,
) *nfesvc.NfceService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "nfce")
	return nfesvc.NewNfceService(
		orgRepo, certRepo, personRepo, configRepo, productRepo, taxProfileRepo, operationRepo,
		nfceRepo, eventRepo, clients, workerSvc, billingSvc, cfg.S3BucketDocuments,
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
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
	billingSvc *services.BillingService,
) *mdfesvc.MdfeService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "mdfe")
	return mdfesvc.NewMdfeService(
		orgRepo, certRepo, configRepo, mdfeRepo, nfeRepo, cteRepo,
		eventRepo, vehicleRepo, personRepo, vehicleSetRepo, clients, workerSvc, billingSvc, cfg.S3BucketDocuments,
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
) *nfsesvc.NfseService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "nfse")
	return nfsesvc.NewNfseService(
		orgRepo, certRepo, personRepo, configRepo, serviceRepo,
		nfseRepo, eventRepo, distRepo, workerSvc, extSvc, billingSvc,
		clients, cacheBackend, cfg.S3BucketDocuments,
	)
}

// --- route registration ---

// Services bundles all service layer dependencies for the route layer.
type Services struct {
	fx.In

	OrgSvc       *services.OrganizationService
	UserSvc      *services.UserService
	MemberSvc    *services.MembershipService
	InvSvc       *services.InvitationService
	CertSvc      *services.CertificateService
	ProductSvc   *services.ProductService
	ServiceSvc   *services.ServiceService
	TaxProfSvc   *services.TaxProfileService
	OperationSvc *services.OperationService
	PayTermSvc   *services.PaymentTermService
	VehSetSvc    *services.VehicleSetService
	PersonSvc    *services.PersonService
	VehicleSvc   *services.VehicleService
	NfeSvc       *nfesvc.NfeService
	NfceSvc      *nfesvc.NfceService
	MdfeSvc      *mdfesvc.MdfeService
	NfseSvc      *nfsesvc.NfseService
	NfeConf      *services.NfeConfigService
	NfceConf     *services.NfceConfigService
	CteConf      *services.CteConfigService
	MdfeConf     *services.MdfeConfigService
	NfseConf     *services.NfseConfigService
	DistSvc      *services.DistributionService
	ExternalSvc  *services.ExternalService
	AuditLogSvc  *services.AuditLogService
	BillingSvc   *services.BillingService
	RoleRepo     *repositories.RoleRepository
	Cache        cache.Backend
	WSReg        ws.Registry
	Cfg          *config.Config
	AWS          *awsclient.Clients
}

func registerRoutes(app *fiber.App, svcs Services) {
	apiv1.Register(app, svcs.Cache, svcs.Cfg, svcs.WSReg, svcs.AWS, apiv1.Services{
		Org:          svcs.OrgSvc,
		User:         svcs.UserSvc,
		Member:       svcs.MemberSvc,
		Invitation:   svcs.InvSvc,
		Cert:         svcs.CertSvc,
		Product:      svcs.ProductSvc,
		Service:      svcs.ServiceSvc,
		TaxProfile:   svcs.TaxProfSvc,
		Operation:    svcs.OperationSvc,
		PaymentTerm:  svcs.PayTermSvc,
		VehicleSet:   svcs.VehSetSvc,
		Person:       svcs.PersonSvc,
		Vehicle:      svcs.VehicleSvc,
		NFe:          svcs.NfeSvc,
		NFCe:         svcs.NfceSvc,
		MDFe:         svcs.MdfeSvc,
		Nfse:         svcs.NfseSvc,
		NfeConfig:    svcs.NfeConf,
		NfceConfig:   svcs.NfceConf,
		CteConfig:    svcs.CteConf,
		MdfeConfig:   svcs.MdfeConf,
		NfseConfig:   svcs.NfseConf,
		Distribution: svcs.DistSvc,
		External:     svcs.ExternalSvc,
		AuditLog:     svcs.AuditLogSvc,
		Billing:      svcs.BillingSvc,
		RoleRepo:     svcs.RoleRepo,
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
	if f, ok := errors.AsType[*fiber.Error](err); ok {
		p := problem.FromFiber(f)
		return p.Send(c)
	}
	return problem.InternalServer(err.Error()).Send(c)
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
