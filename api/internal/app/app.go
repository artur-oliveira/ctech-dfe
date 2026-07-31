// Package app wires the application using Fx dependency injection.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	apiv1 "gopkg.aoctech.app/dfe/api/internal/api/v1"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/consumer"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"

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
		repositories.NewProductRepository,
		repositories.NewPersonRepository,
		repositories.NewVehicleRepository,
		repositories.NewNfeConfigRepository,
		repositories.NewNfceConfigRepository,
		repositories.NewCteConfigRepository,
		repositories.NewMdfeConfigRepository,
		repositories.NewNfeRepository,
		repositories.NewNfceRepository,
		repositories.NewCteRepository,
		repositories.NewMdfeRepository,
		newNfeEventRepository,
		repositories.NewNfeDistributionRepository,
		repositories.NewCteDistributionRepository,
		repositories.NewMdfeDistributionRepository,
		// Services
		newOrganizationService,
		newUserService,
		services.NewMembershipService,
		services.NewInvitationService,
		newCertificateService,
		newProductService,
		newPersonService,
		newVehicleService,
		services.NewNfeConfigService,
		services.NewNfceConfigService,
		services.NewCteConfigService,
		services.NewMdfeConfigService,
		newExternalService,
		newWorkerService,
		newNFeService,
		newNFCeService,
		newMDFeService,
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
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	clients *awsclient.Clients,
	cfg *config.Config,
) *services.DistributionService {
	return services.NewDistributionService(
		orgRepo, certRepo,
		NfeConfig, CteConfig, MdfeConfig,
		nfeDist, cteDist, mdfeDist,
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
	nfeRepo *repositories.NfeRepository,
	eventRepo *repositories.DocumentEventRepository,
	vehicleRepo *repositories.VehicleRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	cfg *config.Config,
) *nfesvc.NfeService {
	return nfesvc.NewNfeService(
		orgRepo, certRepo, personRepo, configRepo, productRepo,
		nfeRepo, eventRepo, vehicleRepo, clients,
		workerSvc, extSvc, cfg.S3BucketDocuments,
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
	nfceRepo *repositories.NfceRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
) *nfesvc.NfceService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "nfce")
	return nfesvc.NewNfceService(
		orgRepo, certRepo, personRepo, configRepo, productRepo,
		nfceRepo, eventRepo, clients, workerSvc, cfg.S3BucketDocuments,
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
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	db *dynamodb.Client,
	cfg *config.Config,
) *mdfesvc.MdfeService {
	eventRepo := repositories.NewDocumentEventRepository(db, cfg, "mdfe")
	return mdfesvc.NewMdfeService(
		orgRepo, certRepo, configRepo, mdfeRepo, nfeRepo, cteRepo,
		eventRepo, vehicleRepo, clients, workerSvc, cfg.S3BucketDocuments,
		mdfesvc.TechData{
			CNPJ:    cfg.TechnicalCNPJ,
			Name:    cfg.TechnicalName,
			Email:   cfg.TechnicalEmail,
			Phone:   cfg.TechnicalPhone,
			Version: cfg.AppVersion,
		},
	)
}

// --- route registration ---

// Services bundles all service layer dependencies for the route layer.
type Services struct {
	fx.In

	OrgSvc      *services.OrganizationService
	UserSvc     *services.UserService
	MemberSvc   *services.MembershipService
	InvSvc      *services.InvitationService
	CertSvc     *services.CertificateService
	ProductSvc  *services.ProductService
	PersonSvc   *services.PersonService
	VehicleSvc  *services.VehicleService
	NfeSvc      *nfesvc.NfeService
	NfceSvc     *nfesvc.NfceService
	MdfeSvc     *mdfesvc.MdfeService
	NfeConf     *services.NfeConfigService
	NfceConf    *services.NfceConfigService
	CteConf     *services.CteConfigService
	MdfeConf    *services.MdfeConfigService
	DistSvc     *services.DistributionService
	ExternalSvc *services.ExternalService
	AuditLogSvc *services.AuditLogService
	RoleRepo    *repositories.RoleRepository
	Cache       cache.Backend
	WSReg       ws.Registry
	Cfg         *config.Config
	AWS         *awsclient.Clients
}

func registerRoutes(app *fiber.App, svcs Services) {
	apiv1.Register(app, svcs.Cache, svcs.Cfg, svcs.WSReg, svcs.AWS, apiv1.Services{
		Org:          svcs.OrgSvc,
		User:         svcs.UserSvc,
		Member:       svcs.MemberSvc,
		Invitation:   svcs.InvSvc,
		Cert:         svcs.CertSvc,
		Product:      svcs.ProductSvc,
		Person:       svcs.PersonSvc,
		Vehicle:      svcs.VehicleSvc,
		NFe:          svcs.NfeSvc,
		NFCe:         svcs.NfceSvc,
		MDFe:         svcs.MdfeSvc,
		NfeConfig:    svcs.NfeConf,
		NfceConfig:   svcs.NfceConf,
		CteConfig:    svcs.CteConf,
		MdfeConfig:   svcs.MdfeConf,
		Distribution: svcs.DistSvc,
		External:     svcs.ExternalSvc,
		AuditLog:     svcs.AuditLogSvc,
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

func newResultsConsumer(clients *awsclient.Clients, cfg *config.Config, reg ws.Registry, c cache.Backend) *consumer.ResultsConsumer {
	return consumer.NewResultsConsumer(clients.SQS, cfg.ResultsQueueURL, reg, c)
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
