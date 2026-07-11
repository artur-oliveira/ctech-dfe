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

	"github.com/artur-oliveira/ctech-dfe/api/internal/awsclient"
	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
)

const tablePrefix = "test"

var (
	db          *dynamodb.Client
	cfg         *config.Config
	orgRepo        *repositories.OrganizationRepository
	productRepo    *repositories.ProductRepository
	personRepo     *repositories.PersonRepository
	vehicleRepo    *repositories.VehicleRepository
	certRepo       *repositories.CertificateRepository
	auditRepo      *repositories.AuditLogRepository
	nfeConfigRepo  *repositories.NfeConfigRepository
	nfceConfigRepo *repositories.NfceConfigRepository
	cteConfigRepo  *repositories.CteConfigRepository
	mdfeConfigRepo *repositories.MdfeConfigRepository
	orgSvc         *services.OrganizationService
	productSvc     *services.ProductService
	personSvc      *services.PersonService
	vehicleSvc     *services.VehicleService
	certSvc        *services.CertificateService
	nfeConfigSvc   *services.NfeConfigService
	nfceConfigSvc  *services.NfceConfigService
	cteConfigSvc   *services.CteConfigService
	mdfeConfigSvc  *services.MdfeConfigService
	memCache       cache.Backend
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
	nfeConfigRepo = repositories.NewNfeConfigRepository(db, cfg)
	nfceConfigRepo = repositories.NewNfceConfigRepository(db, cfg)
	cteConfigRepo = repositories.NewCteConfigRepository(db, cfg)
	mdfeConfigRepo = repositories.NewMdfeConfigRepository(db, cfg)

	orgSvc = services.NewOrganizationService(orgRepo, auditRepo, memCache)
	productSvc = services.NewProductService(productRepo, auditRepo, memCache)
	personSvc = services.NewPersonService(personRepo, auditRepo, memCache)
	vehicleSvc = services.NewVehicleService(vehicleRepo, auditRepo, memCache)
	nfeConfigSvc = services.NewNfeConfigService(nfeConfigRepo, auditRepo)
	nfceConfigSvc = services.NewNfceConfigService(nfceConfigRepo, auditRepo)
	cteConfigSvc = services.NewCteConfigService(cteConfigRepo, auditRepo)
	mdfeConfigSvc = services.NewMdfeConfigService(mdfeConfigRepo, auditRepo)
	// certSvc is Delete-only usable in this harness: Upload needs a real S3
	// client (no S3 test double exists in this codebase — see task-10-report.md
	// for the accepted scope boundary), so it's wired with a zero-value
	// *awsclient.Clients{} placeholder. Delete never touches s.awsClients, so
	// this is safe for the Delete integration test but Upload must not be
	// exercised against this instance.
	certSvc = services.NewCertificateService(certRepo, auditRepo, &awsclient.Clients{}, "unused-test-bucket")

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
	}

	for _, def := range definitions {
		_, err := db.CreateTable(ctx, &def)
		if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
			return fmt.Errorf("create table %s: %w", *def.TableName, err)
		}
	}
	return nil
}

func dropTables(ctx context.Context, db *dynamodb.Client) {
	tables := []string{
		tablePrefix + "_organizations",
		tablePrefix + "_organization_products",
		tablePrefix + "_organization_persons",
		tablePrefix + "_organization_vehicles",
		tablePrefix + "_organization_certificates",
		tablePrefix + "_organization_nfe_configs",
		tablePrefix + "_organization_nfce_configs",
		tablePrefix + "_organization_cte_configs",
		tablePrefix + "_organization_mdfe_configs",
		tablePrefix + "_audit_logs",
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
