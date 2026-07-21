package services

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/contract"
)

// WorkerMessage is the SNS payload sent to the SQS workers — defined once in
// the shared contract module (B17).
type WorkerMessage = contract.WorkerMessage

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
