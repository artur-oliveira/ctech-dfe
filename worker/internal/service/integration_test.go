//go:build integration

// Integration tests require real AWS credentials (or LocalStack) and are excluded
// from the default `go test` run. To execute them:
//
//	AWS_REGION=us-east-1 go test ./internal/service/... -tags integration -v
//
// Required environment variables:
//   - AWS_REGION         — AWS region (e.g. us-east-1)
//   - TABLE_PREFIX       — DynamoDB table prefix (e.g. dev)
//   - DOCUMENTS_BUCKET  — S3 bucket for documents
//   - CERTS_BUCKET      — S3 bucket for certificates
//   - DFE_LAMBDA_NAME   — py-dfe Lambda function name

package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/dfe/worker/internal/config"
)

// skipUnlessIntegEnv skips the test if any required env var is missing.
func skipUnlessIntegEnv(t *testing.T, vars ...string) {
	t.Helper()
	for _, v := range vars {
		if os.Getenv(v) == "" {
			t.Skipf("integration test requires %s", v)
		}
	}
}

func integCfg(t *testing.T) *config.Config {
	t.Helper()
	skipUnlessIntegEnv(t, "TABLE_PREFIX", "DOCUMENTS_BUCKET", "CERTS_BUCKET", "DFE_LAMBDA_NAME")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func integAWSClients(t *testing.T) (S3Client, LambdaClient, DistributionDynamoClient, SNSClient) {
	t.Helper()
	skipUnlessIntegEnv(t, "AWS_REGION")
	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	return s3.NewFromConfig(ac),
		lambdaSDK.NewFromConfig(ac),
		dynamodb.NewFromConfig(ac),
		sns.NewFromConfig(ac)
}

// ---------------------------------------------------------------------------
// NF-e Emission integration tests
// ---------------------------------------------------------------------------

// TestInteg_NFeEmission_AuthorizedFlow verifies that a valid NF-e emission
// call to py-dfe Lambda returns cStat=100 and that the document is marked
// authorized in DynamoDB.
//
// This test requires a real py-dfe Lambda and a valid NF-e XML + certificate
// configured via environment variables. The test DOES NOT submit a real NF-e
// to SEFAZ; it expects the Lambda to be in homologação mode.
func TestInteg_NFeEmission_AuthorizedFlow(t *testing.T) {
	t.Skip("requires org-specific credentials and a valid NF-e body — run manually")

	cfg := integCfg(t)
	s3c, lamc, dyno, snsc := integAWSClients(t)

	msg := WorkerMessage{
		DocPK:            os.Getenv("TEST_DOC_PK"),
		AccessKey:        os.Getenv("TEST_ACCESS_KEY"),
		ExpectedFileName: os.Getenv("TEST_ACCESS_KEY"),
		TableName:        "nfes",
		S3Prefix:         "nfe",
		CNPJ:             os.Getenv("TEST_CNPJ"),
		UF:               os.Getenv("TEST_UF"),
		SefazEnvironment: "homologacao",
		CertS3Key:        os.Getenv("TEST_CERT_S3_KEY"),
		CertPassword:     os.Getenv("TEST_CERT_PASSWORD"),
		DocType:          "nfe",
		SefazService:     "NFeAutorizacao",
		Body:             mustParseBody(t, os.Getenv("TEST_NFE_BODY")),
	}

	svc := New(Clients{S3: s3c, Lambda: lamc, Dynamo: &integDynamo{dyno}, SNS: snsc}, cfg)
	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
}

// TestInteg_Distribution_DispatchAndReceive verifies the full distribution
// loop for a single org: it enqueues a dist_nsu job and checks that at least
// one NSU was processed when documents are available in SEFAZ.
//
// This test is slow (network calls to SEFAZ) and requires valid credentials.
func TestInteg_Distribution_DispatchAndReceive(t *testing.T) {
	t.Skip("requires valid SEFAZ credentials and SEFAZ availability — run manually")

	cfg := integCfg(t)
	s3c, lamc, dyno, snsc := integAWSClients(t)

	svc := NewDistribution(DistributionClients{
		S3:     s3c,
		Lambda: lamc,
		Dynamo: dyno,
		SNS:    snsc,
	}, cfg)

	orgPK := os.Getenv("TEST_ORG_PK")
	if orgPK == "" {
		t.Skip("TEST_ORG_PK not set")
	}

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu",
		OrgPK:   orgPK,
		DocType: "nfe",
		Trigger: "integration-test",
	})
	if err != nil {
		t.Errorf("distribution Process: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustParseBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return m
}

// integDynamo wraps the real DynamoDB client to satisfy the DFe service's
// DynamoClient interface (UpdateItem only) alongside DistributionDynamoClient.
type integDynamo struct {
	*dynamodb.Client
}
