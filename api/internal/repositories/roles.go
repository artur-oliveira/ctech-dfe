package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

var actions = []string{"list", "get", "create", "update", "delete"}
var resources = []string{
	"ctes", "cte_distributions", "cte_events",
	"mdfes", "mdfe_distributions", "mdfe_events",
	"nfces", "nfce_events",
	"nfes", "nfe_distributions", "nfe_events",
	"organizations",
	"organization_products", "organization_vehicles", "organization_persons",
	"organization_nfe_configs", "organization_nfce_configs",
	"organization_cte_configs", "organization_mdfe_configs",
	"organization_certificates",
}

// AllPermissions is the full permission set (action.resource pairs).
var AllPermissions []string

func init() {
	for _, a := range actions {
		for _, r := range resources {
			AllPermissions = append(AllPermissions, fmt.Sprintf("%s.%s", a, r))
		}
	}
}

type RoleRepository struct {
	Base
}

func NewRoleRepository(db *dynamodb.Client, cfg *config.Config) *RoleRepository {
	return &RoleRepository{Base: NewBase(db, cfg, "roles")}
}

func (r *RoleRepository) Get(ctx context.Context, name string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, fmt.Sprintf("ROLE_%s", strings.ToUpper(name)))
}

// ListAll scans the roles table.
// Justified: table contains at most 4 rows (fixed system roles); no access pattern warrants a GSI.
func (r *RoleRepository) ListAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
	out, err := r.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.TableName),
	})
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	return out.Items, nil
}

func (r *RoleRepository) Upsert(ctx context.Context, name, description string, permissions []string) (map[string]types.AttributeValue, error) {
	now := NowStr()
	pk := fmt.Sprintf("ROLE_%s", strings.ToUpper(name))

	existing, _ := r.GetItem(ctx, pk)

	permAV, _ := attributevalue.MarshalList(permissions)
	item := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: pk},
		"name":        &types.AttributeValueMemberS{Value: strings.ToUpper(name)},
		"description": &types.AttributeValueMemberS{Value: description},
		"permissions": &types.AttributeValueMemberL{Value: permAV},
		"updated_at":  &types.AttributeValueMemberS{Value: now},
	}
	if existing == nil {
		item["created_at"] = &types.AttributeValueMemberS{Value: now}
	} else if ca, ok := existing["created_at"]; ok {
		item["created_at"] = ca
	}

	return item, r.PutItem(ctx, item)
}
