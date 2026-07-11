//go:build integration

package mdfes

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

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

// newTestVehicleRepo connects to a local DynamoDB instance and returns a
// VehicleRepository backed by a freshly created organization_vehicles table
// with both plate-index and role-index, matching the production schema
// (cdk/lib/dynamodb-stack.ts). This targets DYNAMODB_ENDPOINT, matching the
// tests/integration harness convention. resolveVehicle/resolveTrailers are
// unexported, so this white-box test lives in package mdfes rather than
// tests/integration — that package can't reach them.
func newTestVehicleRepo(t *testing.T) *repositories.VehicleRepository {
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

	const tablePrefix = "testgating"
	const tableName = tablePrefix + "_organization_vehicles"
	_, err = db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
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
	})
	if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
		t.Skipf("no reachable DynamoDB Local at %s: %v", endpoint, err)
	}

	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(tableName)})
	})

	cfg := &config.Config{TablePrefix: tablePrefix, AWSRegion: "us-east-1"}
	return repositories.NewVehicleRepository(db, cfg)
}

func TestResolveVehicle_IncompleteRegisteredTractorReturnsBadRequest(t *testing.T) {
	repo := newTestVehicleRepo(t)
	svc := &MdfeService{vehicleRepo: repo}
	orgPK := "CNPJ_12345678000190"

	item, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "ABC1D23", PlateUF: "SP", Role: "tractor",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	_, err = svc.resolveVehicle(context.Background(), orgPK, MdfeVehicle{SK: &sk})
	if err == nil {
		t.Fatal("expected error for incomplete tractor, got nil")
	}
}

func TestResolveVehicle_CompleteRegisteredTractorSucceeds(t *testing.T) {
	repo := newTestVehicleRepo(t)
	svc := &MdfeService{vehicleRepo: repo}
	orgPK := "CNPJ_12345678000190"

	item, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "ABC1D23", PlateUF: "SP", Role: "tractor",
		Weight: 8000, Wheelset: "01", Bodywork: "00",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	rv, err := svc.resolveVehicle(context.Background(), orgPK, MdfeVehicle{SK: &sk})
	if err != nil {
		t.Fatalf("resolveVehicle: %v", err)
	}
	if rv.Placa != "ABC1D23" || rv.Tara != "8000" || rv.TpRod != "01" || rv.TpCar != "00" {
		t.Errorf("resolved = %+v, want placa=ABC1D23 tara=8000 tpRod=01 tpCar=00", rv)
	}
}

func TestResolveTrailers_IncompleteTrailerReturnsBadRequest(t *testing.T) {
	repo := newTestVehicleRepo(t)
	svc := &MdfeService{vehicleRepo: repo}
	orgPK := "CNPJ_12345678000190"

	item, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "XYZ1A23", PlateUF: "SP", Role: "trailer", Weight: 5000,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	_, err = svc.resolveTrailers(context.Background(), orgPK, []MdfeTrailer{{SK: sk}})
	if err == nil {
		t.Fatal("expected error for incomplete trailer, got nil")
	}
}

func TestResolveTrailers_CompleteTrailersSucceed(t *testing.T) {
	repo := newTestVehicleRepo(t)
	svc := &MdfeService{vehicleRepo: repo}
	orgPK := "CNPJ_12345678000190"

	item, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "XYZ1A23", PlateUF: "SP", Role: "trailer",
		Weight: 5000, CapKG: 9000, Bodywork: "01",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	resolved, err := svc.resolveTrailers(context.Background(), orgPK, []MdfeTrailer{{SK: sk}})
	if err != nil {
		t.Fatalf("resolveTrailers: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolveTrailers returned %d items, want 1", len(resolved))
	}
	if resolved[0].Placa != "XYZ1A23" || resolved[0].Tara != "5000" || resolved[0].CapKG != "9000" || resolved[0].TpCar != "01" {
		t.Errorf("resolved[0] = %+v, want placa=XYZ1A23 tara=5000 capKG=9000 tpCar=01", resolved[0])
	}
}

func TestResolveVehicle_BuildRodo_EndToEnd_IncludesTrailer(t *testing.T) {
	repo := newTestVehicleRepo(t)
	svc := &MdfeService{vehicleRepo: repo}
	orgPK := "CNPJ_12345678000190"

	tractorItem, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "ABC1D23", PlateUF: "SP", Role: "tractor",
		Weight: 8000, Wheelset: "01", Bodywork: "00",
	})
	if err != nil {
		t.Fatalf("Create tractor: %v", err)
	}
	tractorSK := tractorItem["sk"].(*types.AttributeValueMemberS).Value

	trailerItem, err := repo.Create(context.Background(), orgPK, repositories.VehicleFields{
		Plate: "XYZ1A23", PlateUF: "SP", Role: "trailer",
		Weight: 5000, CapKG: 9000, Bodywork: "01",
	})
	if err != nil {
		t.Fatalf("Create trailer: %v", err)
	}
	trailerSK := trailerItem["sk"].(*types.AttributeValueMemberS).Value

	vehicle, err := svc.resolveVehicle(context.Background(), orgPK, MdfeVehicle{SK: &tractorSK})
	if err != nil {
		t.Fatalf("resolveVehicle: %v", err)
	}
	trailers, err := svc.resolveTrailers(context.Background(), orgPK, []MdfeTrailer{{SK: trailerSK}})
	if err != nil {
		t.Fatalf("resolveTrailers: %v", err)
	}

	p := baseParams(nil)
	p.vehicle = vehicle
	p.trailers = trailers
	rodo := p.buildRodo()

	reboques, ok := rodo["veicReboque"].([]map[string]any)
	if !ok || len(reboques) != 1 {
		t.Fatalf("veicReboque = %v, want 1-item list", rodo["veicReboque"])
	}
	if reboques[0]["placa"] != "XYZ1A23" {
		t.Errorf("veicReboque[0].placa = %v, want XYZ1A23", reboques[0]["placa"])
	}
}
