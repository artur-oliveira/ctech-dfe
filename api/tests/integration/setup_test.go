//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfses"
)

const tablePrefix = "test"

var (
	db              *dynamodb.Client
	cfg             *config.Config
	orgRepo         *repositories.OrganizationRepository
	productRepo     *repositories.ProductRepository
	personRepo      *repositories.PersonRepository
	vehicleRepo     *repositories.VehicleRepository
	certRepo        *repositories.CertificateRepository
	auditRepo       *repositories.AuditLogRepository
	userRepo        *repositories.UserRepository
	roleRepo        *repositories.RoleRepository
	orgUserRepo     *repositories.OrgUserRepository
	invRepo         *repositories.OrgInvitationRepository
	nfeConfigRepo   *repositories.NfeConfigRepository
	nfceConfigRepo  *repositories.NfceConfigRepository
	cteConfigRepo   *repositories.CteConfigRepository
	mdfeConfigRepo  *repositories.MdfeConfigRepository
	nfseConfigRepo  *repositories.NfseConfigRepository
	orgSvc          *services.OrganizationService
	productSvc      *services.ProductService
	serviceSvc      *services.ServiceService
	taxProfileSvc   *services.TaxProfileService
	operationSvc    *services.OperationService
	paymentTermSvc  *services.PaymentTermService
	vehicleSetSvc   *services.VehicleSetService
	personSvc       *services.PersonService
	vehicleSvc      *services.VehicleService
	certSvc         *services.CertificateService
	memberSvc       *services.MembershipService
	invSvc          *services.InvitationService
	billingSvc      *services.BillingService
	nfeConfigSvc    *services.NfeConfigService
	nfceConfigSvc   *services.NfceConfigService
	cteConfigSvc    *services.CteConfigService
	mdfeConfigSvc   *services.MdfeConfigService
	nfseConfigSvc   *services.NfseConfigService
	serviceRepo     *repositories.ServiceRepository
	taxProfileRepo  *repositories.TaxProfileRepository
	operationRepo   *repositories.OperationRepository
	paymentTermRepo *repositories.PaymentTermRepository
	vehicleSetRepo  *repositories.VehicleSetRepository
	nfseRepo        *repositories.NfseRepository
	nfseSvc         *nfses.NfseService
	memCache        cache.Backend
)

func TestMain(m *testing.M) {
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		fmt.Println("skip: DYNAMODB_ENDPOINT not set — run: docker run -p 8000:8000 amazon/dynamodb-local")
		os.Exit(0)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("testing", "testing", ""),
		),
	)
	if err != nil {
		panic(err)
	}

	db = dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	cfg = &config.Config{
		TablePrefix: tablePrefix,
		AWSRegion:   "us-east-1",
	}

	if err := createTables(ctx, db); err != nil {
		panic(fmt.Sprintf("createTables: %v", err))
	}

	memCache = cache.NewMemoryBackend(1000)
	orgRepo = repositories.NewOrganizationRepository(db, cfg)
	productRepo = repositories.NewProductRepository(db, cfg)
	personRepo = repositories.NewPersonRepository(db, cfg)
	vehicleRepo = repositories.NewVehicleRepository(db, cfg)
	certRepo = repositories.NewCertificateRepository(db, cfg)
	auditRepo = repositories.NewAuditLogRepository(db, cfg)
	userRepo = repositories.NewUserRepository(db, cfg)
	roleRepo = repositories.NewRoleRepository(db, cfg)
	orgUserRepo = repositories.NewOrgUserRepository(db, cfg)
	invRepo = repositories.NewOrgInvitationRepository(db, cfg)
	nfeConfigRepo = repositories.NewNfeConfigRepository(db, cfg)
	nfceConfigRepo = repositories.NewNfceConfigRepository(db, cfg)
	cteConfigRepo = repositories.NewCteConfigRepository(db, cfg)
	mdfeConfigRepo = repositories.NewMdfeConfigRepository(db, cfg)
	nfseConfigRepo = repositories.NewNfseConfigRepository(db, cfg)

	// certSvc is Delete-only usable in this harness: Upload needs a real S3
	// client (no S3 test double exists in this codebase), so it's wired with a
	// zero-value *awsclient.Clients{} placeholder. Delete never touches
	// s.awsClients; Upload/StageUpload must not be exercised against it.
	certSvc = services.NewCertificateService(certRepo, auditRepo, &awsclient.Clients{}, "unused-test-bucket")
	memberSvc = services.NewMembershipService(orgUserRepo, auditRepo, roleRepo, memCache)
	orgSvc = services.NewOrganizationService(orgRepo, auditRepo, certRepo, orgUserRepo, certSvc, memberSvc, memCache)
	// A nil billing client is no-charge mode, which is what these tests want:
	// every quota check passes, so a membership test is testing memberships
	// rather than a subscription it never set up.
	billingSvc = services.NewBillingService(
		repositories.NewAccountBillingRepository(db, cfg), nil, nil, memberSvc, orgSvc, memCache)
	invSvc = services.NewInvitationService(invRepo, orgUserRepo, orgRepo, auditRepo, memberSvc, billingSvc)
	productSvc = services.NewProductService(productRepo, auditRepo, memCache)
	serviceRepo = repositories.NewServiceRepository(db, cfg)
	serviceSvc = services.NewServiceService(serviceRepo, auditRepo, memCache)
	taxProfileRepo = repositories.NewTaxProfileRepository(db, cfg)
	taxProfileSvc = services.NewTaxProfileService(taxProfileRepo, auditRepo, memCache)
	operationRepo = repositories.NewOperationRepository(db, cfg)
	operationSvc = services.NewOperationService(operationRepo, auditRepo, memCache)
	paymentTermRepo = repositories.NewPaymentTermRepository(db, cfg)
	paymentTermSvc = services.NewPaymentTermService(paymentTermRepo, auditRepo, memCache)
	vehicleSetRepo = repositories.NewVehicleSetRepository(db, cfg)
	vehicleSetSvc = services.NewVehicleSetService(vehicleSetRepo, vehicleRepo, auditRepo, memCache)
	personSvc = services.NewPersonService(personRepo, auditRepo, memCache)
	vehicleSvc = services.NewVehicleService(vehicleRepo, auditRepo, memCache)
	nfeConfigSvc = services.NewNfeConfigService(nfeConfigRepo, auditRepo)
	nfceConfigSvc = services.NewNfceConfigService(nfceConfigRepo, auditRepo)
	cteConfigSvc = services.NewCteConfigService(cteConfigRepo, auditRepo)
	mdfeConfigSvc = services.NewMdfeConfigService(mdfeConfigRepo, auditRepo)
	nfseConfigSvc = services.NewNfseConfigService(nfseConfigRepo, auditRepo)

	// NfseService roda inteiro contra o DynamoDB local: BuildOutboxTx só monta
	// item de transação e PublishWorkerEvent é no-op com topicARN vazio. As
	// operações que dependem de S3 ou do go-dfe (XML, DANFSE, parâmetros
	// municipais sem cache) não são exercidas aqui.
	nfseRepo = repositories.NewNfseRepository(db, cfg)
	nfseSvc = nfses.NewNfseService(
		orgRepo, certRepo, personRepo, nfseConfigRepo, serviceRepo, nfseRepo,
		repositories.NewDocumentEventRepository(db, cfg, nfses.DocTypeNfse),
		repositories.NewNfseDistributionRepository(db, cfg),
		services.NewWorkerService(&awsclient.Clients{}, "", tablePrefix),
		services.NewExternalService(certRepo, &awsclient.Clients{}, "", "unused-test-bucket"),
		billingSvc,
		&awsclient.Clients{}, memCache, "unused-test-bucket",
	)

	code := m.Run()
	dropTables(ctx, db)
	os.Exit(code)
}

func createTables(ctx context.Context, db *dynamodb.Client) error {
	definitions := []dynamodb.CreateTableInput{
		{
			TableName:   aws.String(tablePrefix + "_organizations"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_products"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("description"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("code"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("description-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("description"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("code-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("code"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_services"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("description"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("code"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("description-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("description"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("code-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("code"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_persons"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("name"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("org-name-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("name"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_vehicles"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("plate"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("role"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("plate-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("plate"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("role-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("role"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_certificates"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_nfe_configs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_nfce_configs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_cte_configs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_mdfe_configs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_nfse_configs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			// nfses: a SK é o id_dps. A access-key-index existe porque a chave
			// de acesso de 50 dígitos só chega na resposta do fisco.
			TableName:   aws.String(tablePrefix + "_nfses"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("access_key"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("access-key-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("access_key"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_nfse_events"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_worker_outbox"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_audit_logs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("org-time-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("user-id-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_users"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_roles"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_users"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("user-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("sk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			// One partition key and no index: the snapshot is read by account and
			// the webhook marker by event id, and nothing else asks anything else.
			TableName:   aws.String(tablePrefix + "_account_billing"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			TableName:   aws.String(tablePrefix + "_organization_invitations"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("org_pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("org-invite-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("org_pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
	}

	// Cadastros reutilizáveis: mesma forma (pk/sk + name-index) para os quatro.
	for _, name := range orgEntityTables {
		definitions = append(definitions, dynamodb.CreateTableInput{
			TableName:   aws.String(tablePrefix + "_" + name),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("name"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String(repositories.OrgEntityNameIndex),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("name"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		})
	}

	for _, def := range definitions {
		_, err := db.CreateTable(ctx, &def)
		if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
			return fmt.Errorf("create table %s: %w", *def.TableName, err)
		}
	}
	return nil
}

// orgEntityTables são os cadastros reutilizáveis, todos com a mesma forma.
var orgEntityTables = []string{
	repositories.TableTaxProfiles,
	repositories.TableOperations,
	repositories.TablePaymentTerms,
	repositories.TableVehicleSets,
}

func dropTables(ctx context.Context, db *dynamodb.Client) {
	tables := []string{
		tablePrefix + "_organizations",
		tablePrefix + "_organization_products",
		tablePrefix + "_organization_services",
		tablePrefix + "_" + repositories.TableTaxProfiles,
		tablePrefix + "_" + repositories.TableOperations,
		tablePrefix + "_" + repositories.TablePaymentTerms,
		tablePrefix + "_" + repositories.TableVehicleSets,
		tablePrefix + "_organization_persons",
		tablePrefix + "_organization_vehicles",
		tablePrefix + "_organization_certificates",
		tablePrefix + "_organization_nfe_configs",
		tablePrefix + "_organization_nfce_configs",
		tablePrefix + "_organization_cte_configs",
		tablePrefix + "_organization_mdfe_configs",
		tablePrefix + "_organization_nfse_configs",
		tablePrefix + "_nfses",
		tablePrefix + "_nfse_events",
		tablePrefix + "_worker_outbox",
		tablePrefix + "_audit_logs",
		tablePrefix + "_users",
		tablePrefix + "_roles",
		tablePrefix + "_organization_users",
		tablePrefix + "_organization_invitations",
		tablePrefix + "_account_billing",
	}
	for _, t := range tables {
		_, _ = db.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(t)})
	}
}

// randomCNPJ generates a structurally valid CNPJ (check digits correct).
func randomCNPJ() string {
	digits := make([]int, 12)
	for {
		for i := range digits {
			digits[i] = rand.Intn(10)
		}
		allSame := true
		for i := 1; i < 12; i++ {
			if digits[i] != digits[0] {
				allSame = false
				break
			}
		}
		if !allSame {
			break
		}
	}
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	s1 := 0
	for i, w := range w1 {
		s1 += digits[i] * w
	}
	d1 := 11 - (s1 % 11)
	if d1 > 9 {
		d1 = 0
	}

	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3}
	s2 := 0
	for i, w := range w2 {
		s2 += digits[i] * w
	}
	s2 += d1 * 2
	d2 := 11 - (s2 % 11)
	if d2 > 9 {
		d2 = 0
	}

	digits = append(digits, d1, d2)
	b := strings.Builder{}
	for _, d := range digits {
		b.WriteByte(byte('0' + d))
	}
	return b.String()
}
