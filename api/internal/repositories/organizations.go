package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Organization is the DynamoDB item for an organization.
// PK format: CNPJ_{cnpj} or CPF_{cpf} — matches Python OrganizationRepository.parse_pk.
type Organization struct {
	PK        string `dynamodbav:"pk"`
	Name      string `dynamodbav:"name"`
	CreatedAt string `dynamodbav:"created_at"`
	UpdatedAt string `dynamodbav:"updated_at"`
}

// OrganizationRepository manages organization persistence.
type OrganizationRepository struct {
	Base
}

func NewOrganizationRepository(db *dynamodb.Client, cfg *config.Config) *OrganizationRepository {
	return &OrganizationRepository{Base: NewBase(db, cfg, "organizations")}
}

// ParseOrgPK converts a raw CPF/CNPJ (or already-prefixed PK) to the DynamoDB PK format.
// Mirrors OrganizationRepository.parse_pk in Python.
func ParseOrgPK(cpfOrCNPJ string) (string, error) {
	if strings.HasPrefix(cpfOrCNPJ, "CNPJ_") || strings.HasPrefix(cpfOrCNPJ, "CPF_") {
		return cpfOrCNPJ, nil
	}
	digits := stripNonDigits(cpfOrCNPJ)
	switch len(digits) {
	case 11:
		return fmt.Sprintf("CPF_%s", digits), nil
	case 14:
		return fmt.Sprintf("CNPJ_%s", digits), nil
	default:
		return "", problem.BadRequest("invalid CPF or CNPJ")
	}
}

// GetOrganization fetches an organization by its PK (prefixed or raw CNPJ/CPF).
func (r *OrganizationRepository) GetOrganization(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return nil, err
	}
	return r.GetItem(ctx, pk)
}

// CreateOrganization writes a new organization item.
func (r *OrganizationRepository) CreateOrganization(ctx context.Context, cpfOrCNPJ string, item map[string]types.AttributeValue) error {
	pk, err := ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return err
	}
	now := NowStr()
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.PutItem(ctx, item)
}

// BuildCreateTxItem returns a create-only TransactWriteItem (fails if the org
// already exists) for the organization, mirroring CreateOrganization's
// key/timestamp logic without writing. Also returns the finalized item.
func (r *OrganizationRepository) BuildCreateTxItem(cpfOrCNPJ string, item map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	pk, err := ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return types.TransactWriteItem{}, nil, err
	}
	if item == nil {
		item = map[string]types.AttributeValue{}
	}
	now := NowStr()
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItemIfAbsent(item), item, nil
}

// UpdateOrganization applies a partial update.
func (r *OrganizationRepository) UpdateOrganization(ctx context.Context, orgPK string, updates map[string]any) error {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return err
	}
	updates["updated_at"] = NowStr()
	_, err = r.UpdateItem(ctx, pk, nil, updates)
	return err
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// organization, mirroring UpdateOrganization's key/timestamp logic, without writing.
func (r *OrganizationRepository) BuildUpdateTxItem(orgPK string, updates map[string]any) (types.TransactWriteItem, error) {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(pk, nil, updates)
}

func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
