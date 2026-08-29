package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// WorkerMessage is the SNS payload sent to py-dfe workers.
type WorkerMessage struct {
	DocPK                 string `json:"doc_pk"`
	AccessKey             string `json:"access_key"`
	TableName             string `json:"table_name"`
	S3Prefix              string `json:"s3_prefix"`
	ExpectedFileName      string `json:"expected_file_name"`
	CNPJ                  string `json:"cnpj"`
	UF                    string `json:"uf"`
	SefazEnvironment      string `json:"sefaz_environment"`
	CertS3Key             string `json:"cert_s3_key"`
	CertPassword          string `json:"cert_password"`
	DocType               string `json:"doc_type"`
	SefazService          string `json:"sefaz_service"`
	Body                  any    `json:"body"`
	BillingUserID         string `json:"billing_user_id,omitempty"`
	BillingPeriod         string `json:"billing_period,omitempty"`
	BillingSubscriptionID string `json:"billing_subscription_id,omitempty"`
	BillingPriceID        string `json:"billing_price_id,omitempty"`
	BillingMeter          string `json:"billing_meter,omitempty"`
	BillingExempt         bool   `json:"billing_exempt,omitempty"`
	// Optional event fields (cancellation, CC-e, manifestation).
	EventsTableName *string `json:"events_table_name,omitempty"`
	EventType       *string `json:"event_type,omitempty"`
	SequenceNumber  *int    `json:"sequence_number,omitempty"`
	EventSK         *string `json:"event_sk,omitempty"`
}

const (
	workerOutboxTable   = "worker_outbox"
	workerOutboxSK      = "command"
	workerOutboxPending = "pending"
	workerOutboxTTL     = 30 * 24 * time.Hour
)

type WorkerService struct {
	clients     *awsclient.Clients
	topicARN    string
	tablePrefix string
}

func NewWorkerService(clients *awsclient.Clients, topicARN, tablePrefix string) *WorkerService {
	return &WorkerService{
		clients:     clients,
		topicARN:    topicARN,
		tablePrefix: tablePrefix,
	}
}

// BuildOutboxTx serializes a worker command into an immutable DynamoDB
// transaction item. The document/counter transaction includes this item, so
// publication can never be lost after a successful issuance commit.
func (s *WorkerService) BuildOutboxTx(msg WorkerMessage) (types.TransactWriteItem, string, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return types.TransactWriteItem{}, "", problem.InternalServer("failed to marshal worker event")
	}
	operationID := fmt.Sprintf("%s#%s", msg.TableName, msg.AccessKey)
	now := time.Now().UTC()
	return types.TransactWriteItem{Put: &types.Put{
		TableName: aws.String(s.tablePrefix + "_" + workerOutboxTable),
		Item: map[string]types.AttributeValue{
			"pk":         &types.AttributeValueMemberS{Value: operationID},
			"sk":         &types.AttributeValueMemberS{Value: workerOutboxSK},
			"status":     &types.AttributeValueMemberS{Value: workerOutboxPending},
			"payload":    &types.AttributeValueMemberS{Value: string(body)},
			"created_at": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
			"ttl":        &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Add(workerOutboxTTL).Unix())},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk) AND attribute_not_exists(sk)"),
	}}, operationID, nil
}

func (s *WorkerService) PublishWorkerEvent(ctx context.Context, msg WorkerMessage) error {
	if s.topicARN == "" {
		return nil // no-op in local dev
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return problem.InternalServer("failed to marshal worker event")
	}
	_, err = s.clients.SNS.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(string(body)),
	})
	if err != nil {
		return problem.InternalServer("failed to publish worker event: " + err.Error())
	}
	return nil
}
