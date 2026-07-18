//go:build integration

package repositories

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
)

// newTestEventRepo connects to a local DynamoDB instance and returns a
// DocumentEventRepository backed by a freshly created nfe_events table.
//
// Unlike this package's other tests — which are pure builder tests with no
// DynamoDB client at all (see TestBase_BuildPutTxItem in base_test.go and
// TestCertificateRepository_BuildCreateTxItem in certificates_test.go, both
// of which only exercise non-executing Build*TxItem helpers) —
// CreateEvent has no Build+Execute split: it calls PutItem inline. There is
// no interface seam to mock (Base.db is a concrete *dynamodb.Client), so
// exercising CreateEvent end to end — as api/CLAUDE.md's testing table
// requires for repository changes ("Repository | Integration test
// (DynamoDB)") — needs a live table.
//
// This targets DYNAMODB_ENDPOINT, matching the tests/integration harness
// convention (see tests/integration/setup_test.go's TestMain). If it isn't
// set, the test skips cleanly rather than guessing a port — there is no
// default DynamoDB Local instance this repo starts on its own.
func newTestEventRepo(t *testing.T) *DocumentEventRepository {
	t.Helper()

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT not set — run: docker compose -f docker-compose.test.yml up -d")
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

	const tableName = "test_nfe_events"
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
		},
	})
	if err != nil && !strings.Contains(err.Error(), "ResourceInUseException") {
		t.Skipf("no reachable DynamoDB Local at %s: %v", endpoint, err)
	}

	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(tableName)})
	})

	cfg := &config.Config{TablePrefix: "test", AWSRegion: "us-east-1"}
	return NewDocumentEventRepository(db, cfg, "nfe")
}

func TestDocumentEventRepository_CreateEvent_StampsActor(t *testing.T) {
	r := newTestEventRepo(t) // reuse whatever local-DynamoDB harness this file's other tests use
	item, err := r.CreateEvent(context.Background(), "43210000000000000000000000000000000000000000", "210200", 1, "pending", nil, nil, nil, "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if item["user_id"].(*types.AttributeValueMemberS).Value != "user-1" {
		t.Errorf("user_id = %v, want user-1", item["user_id"])
	}
	if item["user_name"].(*types.AttributeValueMemberS).Value != "Jane Doe" {
		t.Errorf("user_name = %v, want Jane Doe", item["user_name"])
	}
}
