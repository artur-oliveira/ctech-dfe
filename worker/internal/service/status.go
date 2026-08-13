package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// statusDynamoClient is the shared DynamoDB subset required to transition a
// main fiscal document's status.
type statusDynamoClient interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// updateDocumentStatus persists a main document status transition for both
// issuance/event processing and distribution processing. owner is optional;
// when present, only the worker holding the processing lease may finalize it.
func updateDocumentStatus(
	ctx context.Context,
	dynamo statusDynamoClient,
	tablePrefix, docPK, accessKey, tableName, status string,
	attrs updateAttrs,
	owner string,
) error {
	table := tablePrefix + "_" + tableName
	parts := buildUpdateExpression(status, attrs)

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: docPK},
			"sk": &types.AttributeValueMemberS{Value: accessKey},
		},
		UpdateExpression:          aws.String(parts.expression),
		ExpressionAttributeNames:  parts.attrNames,
		ExpressionAttributeValues: parts.attrValues,
	}
	if owner != "" {
		input.UpdateExpression = aws.String(parts.expression + " REMOVE processing_owner, processing_lease_until")
		input.ConditionExpression = aws.String("processing_owner = :owner")
		input.ExpressionAttributeValues[":owner"] = &types.AttributeValueMemberS{Value: owner}
	}
	if _, err := dynamo.UpdateItem(ctx, input); err != nil {
		return fmt.Errorf("updateStatus %s %s: %w", table, accessKey, err)
	}
	slog.Info("updated document status", "table", table, "access_key", accessKey, "status", status)
	return nil
}
