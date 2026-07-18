//go:build integration

package nfes

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// newTestNfeRepos connects to a local DynamoDB instance and returns fresh
// organization_persons/organizations tables, matching the production schema
// closely enough for appendDeliveryLocation/appendPickupLocation (both are
// unexported, so this white-box test lives in package nfes rather than
// tests/integration — that package can't reach them). Mirrors the harness
// pattern from internal/services/mdfes/vehicle_gating_test.go.
func newTestNfeRepos(t *testing.T) (*repositories.PersonRepository, *repositories.OrganizationRepository) {
	t.Helper()

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT not set — run: docker run -p 8000:8000 amazon/dynamodb-local")
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("testing", "testing", ""),
		),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	db := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	const tablePrefix = "testlocpersist"
	personsTable := tablePrefix + "_organization_persons"
	orgsTable := tablePrefix + "_organizations"

	_, err = db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(personsTable),
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
	})
	if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
		t.Skipf("no reachable DynamoDB Local at %s: %v", endpoint, err)
	}

	_, err = db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(orgsTable),
		BillingMode: types.BillingModePayPerRequest,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
	})
	if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
		t.Skipf("no reachable DynamoDB Local at %s: %v", endpoint, err)
	}

	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(personsTable)})
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(orgsTable)})
	})

	cfg := &config.Config{TablePrefix: tablePrefix, AWSRegion: "us-east-1"}
	return repositories.NewPersonRepository(db, cfg), repositories.NewOrganizationRepository(db, cfg)
}

func TestAppendDeliveryLocation_PersistsOnPersonRecord(t *testing.T) {
	personRepo, orgRepo := newTestNfeRepos(t)
	svc := &NfeService{personRepo: personRepo, orgRepo: orgRepo}
	ctx := context.Background()
	orgPK := "CNPJ_12345678000190"

	created, err := personRepo.Create(ctx, orgPK, "CPF_11122233344", map[string]any{"name": "Destinatario"})
	if err != nil {
		t.Fatalf("Create person: %v", err)
	}
	sk := created["sk"].(*types.AttributeValueMemberS).Value

	loc := &NfeLocalBody{XLgr: "Rua Entrega", Nro: "42", XBairro: "Centro", CMun: "3550308", XMun: "São Paulo", UF: "SP"}
	if err := svc.appendDeliveryLocation(ctx, orgPK, sk, loc); err != nil {
		t.Fatalf("appendDeliveryLocation: %v", err)
	}

	got, err := personRepo.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	plain, err := unmarshalToAny(got)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	locs, _ := plain["delivery_locations"].([]any)
	if len(locs) != 1 {
		t.Fatalf("expected 1 saved delivery location, got %+v", locs)
	}
	first := locs[0].(map[string]any)
	if first["x_lgr"] != "Rua Entrega" {
		t.Errorf("x_lgr = %v, want Rua Entrega", first["x_lgr"])
	}
}

func TestAppendPickupLocation_PersistsOnOrganizationRecord(t *testing.T) {
	personRepo, orgRepo := newTestNfeRepos(t)
	svc := &NfeService{personRepo: personRepo, orgRepo: orgRepo}
	ctx := context.Background()

	if err := orgRepo.CreateOrganization(ctx, "12345678000190", map[string]types.AttributeValue{
		"name": &types.AttributeValueMemberS{Value: "Org Teste"},
	}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}

	loc := &NfeLocalBody{XLgr: "Rua Retirada", Nro: "7", XBairro: "Industrial", CMun: "3550308", XMun: "São Paulo", UF: "SP"}
	if err := svc.appendPickupLocation(ctx, "CNPJ_12345678000190", loc); err != nil {
		t.Fatalf("appendPickupLocation: %v", err)
	}

	got, err := orgRepo.GetOrganization(ctx, "CNPJ_12345678000190")
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	plain, err := unmarshalToAny(got)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	locs, _ := plain["pickup_locations"].([]any)
	if len(locs) != 1 {
		t.Fatalf("expected 1 saved pickup location, got %+v", locs)
	}
	first := locs[0].(map[string]any)
	if first["x_lgr"] != "Rua Retirada" {
		t.Errorf("x_lgr = %v, want Rua Retirada", first["x_lgr"])
	}
}
