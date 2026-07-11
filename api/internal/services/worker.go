package services

import (
	"context"
	"encoding/json"

	"github.com/artur-oliveira/ctech-dfe/api/internal/awsclient"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// WorkerMessage is the SNS payload sent to py-dfe workers.
type WorkerMessage struct {
	DocPK            string `json:"doc_pk"`
	AccessKey        string `json:"access_key"`
	TableName        string `json:"table_name"`
	S3Prefix         string `json:"s3_prefix"`
	ExpectedFileName string `json:"expected_file_name"`
	CNPJ             string `json:"cnpj"`
	UF               string `json:"uf"`
	SefazEnvironment string `json:"sefaz_environment"`
	CertS3Key        string `json:"cert_s3_key"`
	CertPassword     string `json:"cert_password"`
	DocType          string `json:"doc_type"`
	SefazService     string `json:"sefaz_service"`
	Body             any    `json:"body"`
	// Optional event fields (cancellation, CC-e, manifestation).
	EventsTableName *string `json:"events_table_name,omitempty"`
	EventType       *string `json:"event_type,omitempty"`
	SequenceNumber  *int    `json:"sequence_number,omitempty"`
	EventSK         *string `json:"event_sk,omitempty"`
}

type WorkerService struct {
	clients  *awsclient.Clients
	topicARN string
}

func NewWorkerService(clients *awsclient.Clients, topicARN string) *WorkerService {
	return &WorkerService{
		clients:  clients,
		topicARN: topicARN,
	}
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
