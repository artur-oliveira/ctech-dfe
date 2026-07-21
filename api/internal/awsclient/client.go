// Package awsclient wraps AWS SDK v2 clients, replacing the hand-rolled SigV4
// aiohttp client in api/app/aws_client.py.
package awsclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"gopkg.aoctech.app/api-commons/awsconfig"
	"gopkg.aoctech.app/dfe/api/internal/certcrypt"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Clients holds all AWS SDK v2 service clients.
type Clients struct {
	DynamoDB      *dynamodb.Client
	S3            *s3.Client
	SQS           *sqs.Client
	SNS           *sns.Client
	Lambda        *lambda.Client
	SecretManager *secretsmanager.Client
	// CertCodec encrypts/decrypts certificate PFX passwords (B4). Encryption
	// is active only when CERT_PASSWORD_KMS_KEY_ID is set; decryption always
	// handles both encrypted and legacy plaintext values.
	CertCodec *certcrypt.Codec
}

// New creates all AWS clients from the loaded configuration.
// Credentials are resolved via the standard SDK chain:
// env vars → ~/.aws/credentials → EC2 IMDS → ECS task role.
func New(ctx context.Context, cfg *config.Config) (*Clients, error) {
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("awsclient: load config: %w", err)
	}

	clients := &Clients{
		DynamoDB:      newDynamoDB(awsCfg, cfg),
		S3:            s3.NewFromConfig(awsCfg),
		SQS:           sqs.NewFromConfig(awsCfg),
		SNS:           sns.NewFromConfig(awsCfg),
		Lambda:        lambda.NewFromConfig(awsCfg),
		SecretManager: secretsmanager.NewFromConfig(awsCfg),
		CertCodec:     certcrypt.New(kms.NewFromConfig(awsCfg), cfg.CertPasswordKMSKeyID),
	}
	return clients, nil
}

func loadAWSConfig(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	return awsconfig.Load(ctx, cfg.AWSRegion)
}

func newDynamoDB(awsCfg aws.Config, cfg *config.Config) *dynamodb.Client {
	return awsconfig.NewDynamoDBClient(awsCfg, cfg.DynamoDBEndpoint)
}
