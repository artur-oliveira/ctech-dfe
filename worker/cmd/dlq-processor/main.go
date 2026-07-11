package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

var (
	snsClient       *sns.Client
	resultsTopicARN string
)

func init() {
	resultsTopicARN = os.Getenv("RESULTS_TOPIC_ARN")
	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("aws config: " + err.Error())
	}
	snsClient = sns.NewFromConfig(ac)
}

type sqsEvent struct {
	Records []sqsRecord `json:"Records"`
}

type sqsRecord struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
}

func handler(ctx context.Context, event sqsEvent) error {
	for _, record := range event.Records {
		var body map[string]any
		_ = json.Unmarshal([]byte(record.Body), &body)

		accessKey, _ := body["access_key"].(string)
		docPK, _ := body["doc_pk"].(string)
		tableName, _ := body["table_name"].(string)

		slog.Warn("DLQ: message exhausted retries",
			"id", record.MessageID,
			"access_key", accessKey,
			"doc_pk", docPK,
		)

		if resultsTopicARN == "" {
			continue
		}

		result := map[string]any{
			"access_key":     accessKey,
			"doc_pk":         docPK,
			"table_name":     tableName,
			"status":         "failed",
			"sefaz_status":   nil,
			"sefaz_motive":   "Falha após todas as tentativas de reprocessamento",
			"sefaz_protocol": nil,
			"xml_s3_key":     nil,
		}
		msgJSON, _ := json.Marshal(result)

		if _, err := snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(resultsTopicARN),
			Message:  aws.String(string(msgJSON)),
		}); err != nil {
			slog.Error("failed to publish DLQ result", "id", record.MessageID, "err", err)
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
