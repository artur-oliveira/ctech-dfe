package service

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Mirrors api/internal/repositories/audit_logs.go's schema and constants —
// worker has no shared repository layer with api, so this is a standalone
// equivalent for the one write path worker needs (auto-created suppliers).
const (
	auditActionCreate   = "CREATE"
	auditResourcePerson = "PERSON"
	systemActorID       = "SYSTEM"
	systemActorName     = "Sistema (Distribuição DFe)"
)

type auditModification struct {
	Name   string `dynamodbav:"name"`
	Before any    `dynamodbav:"before"`
	After  any    `dynamodbav:"after"`
}

// buildAuditLogTxItem returns a TransactWriteItem writing one audit_logs row,
// for composing into the same transaction as the resource it documents.
func buildAuditLogTxItem(tablePrefix, orgPK, resourceType, resourceID, action string, modifications []auditModification) types.TransactWriteItem {
	modsAV, _ := attributevalue.MarshalList(modifications) // modifications is always a small, well-typed local slice — marshal cannot fail here
	item := map[string]types.AttributeValue{
		"pk":            &types.AttributeValueMemberS{Value: orgPK},
		"sk":            &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%s", resourceType, resourceID, genULID())},
		"resource_type": &types.AttributeValueMemberS{Value: resourceType},
		"resource_id":   &types.AttributeValueMemberS{Value: resourceID},
		"action":        &types.AttributeValueMemberS{Value: action},
		"modifications": &types.AttributeValueMemberL{Value: modsAV},
		"user_id":       &types.AttributeValueMemberS{Value: systemActorID},
		"user_name":     &types.AttributeValueMemberS{Value: systemActorName},
		"created_at":    &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
	}
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(tablePrefix + "_audit_logs"),
			Item:      item,
		},
	}
}
