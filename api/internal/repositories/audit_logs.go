package repositories

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Audit action constants — the `action` attribute on an audit_logs row.
const (
	AuditActionCreate = "CREATE"
	AuditActionUpdate = "UPDATE"
	AuditActionDelete = "DELETE"
)

// Audit resource-type constants — the `resource_type` attribute, and the first
// segment of the sort key (`{resource_type}#{resource_id}#{uuidv7}`).
const (
	AuditResourceOrganization    = "ORGANIZATION"
	AuditResourceCertificate     = "CERTIFICATE"
	AuditResourceProduct         = "PRODUCT"
	AuditResourceService         = "SERVICE"
	AuditResourceVehicle         = "VEHICLE"
	AuditResourcePerson          = "PERSON"
	AuditResourceNfeConfig       = "NFE_CONFIG"
	AuditResourceNfceConfig      = "NFCE_CONFIG"
	AuditResourceCteConfig       = "CTE_CONFIG"
	AuditResourceMdfeConfig      = "MDFE_CONFIG"
	AuditResourceNfseConfig      = "NFSE_CONFIG"
	AuditResourceTaxProfile      = "TAX_PROFILE"
	AuditResourceOperation       = "OPERATION"
	AuditResourcePaymentTerm     = "PAYMENT_TERM"
	AuditResourcePaymentTerminal = "PAYMENT_TERMINAL"
	AuditResourceTollProvider    = "TOLL_PROVIDER"
	AuditResourceVehicleSet      = "VEHICLE_SET"
	AuditResourceMember          = "MEMBER"
	AuditResourceInvitation      = "INVITATION"
)

// Modification is one changed field within an audit_logs row.
type Modification struct {
	Name   string `dynamodbav:"name"`
	Before any    `dynamodbav:"before"`
	After  any    `dynamodbav:"after"`
}

// AuditLogRepository stores per-field change records for org-owned mutating
// resources. Table structure (audit_logs):
//
//	pk = {org_pk}
//	sk = {resource_type}#{resource_id}#{uuidv7}
//
// GSIs: org-time-index (pk, created_at), user-id-index (user_id, created_at).
type AuditLogRepository struct {
	Base
}

func NewAuditLogRepository(db *dynamodb.Client, cfg *config.Config) *AuditLogRepository {
	return &AuditLogRepository{Base: NewBase(db, cfg, "audit_logs")}
}

// BuildLogTxItem returns a TransactWriteItem that writes one audit_logs row.
// Callers combine this with the primary resource's own Build*TxItem and execute
// both via Base.TransactWrite, so the mutation and its audit row commit atomically.
func (r *AuditLogRepository) BuildLogTxItem(
	orgPK, resourceType, resourceID, action, userID, userName string,
	modifications []Modification,
) (types.TransactWriteItem, error) {
	modsAV, err := attributevalue.MarshalList(modifications)
	if err != nil {
		return types.TransactWriteItem{}, fmt.Errorf("marshal modifications: %w", err)
	}

	item := map[string]types.AttributeValue{
		"pk":            &types.AttributeValueMemberS{Value: orgPK},
		"sk":            &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%s", resourceType, resourceID, GenerateID())},
		"resource_type": &types.AttributeValueMemberS{Value: resourceType},
		"resource_id":   &types.AttributeValueMemberS{Value: resourceID},
		"action":        &types.AttributeValueMemberS{Value: action},
		"modifications": &types.AttributeValueMemberL{Value: modsAV},
		"user_id":       &types.AttributeValueMemberS{Value: userID},
		"user_name":     &types.AttributeValueMemberS{Value: userName},
		"created_at":    &types.AttributeValueMemberS{Value: NowStr()},
	}
	return r.BuildPutTxItem(item), nil
}
