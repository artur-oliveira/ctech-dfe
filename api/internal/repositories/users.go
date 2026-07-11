package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

type UserRepository struct {
	Base
}

func NewUserRepository(db *dynamodb.Client, cfg *config.Config) *UserRepository {
	return &UserRepository{Base: NewBase(db, cfg, "users")}
}

func BuildUserPK(userID string) string {
	if strings.HasPrefix(userID, "USER_") {
		return userID
	}
	return fmt.Sprintf("USER_%s", userID)
}

func (r *UserRepository) GetByID(ctx context.Context, userID string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, BuildUserPK(userID))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (map[string]types.AttributeValue, error) {
	res, err := r.QueryGSI(ctx, "email-index", "email", strings.ToLower(email), 1, nil)
	if err != nil || len(res.Items) == 0 {
		return nil, err
	}
	return res.Items[0], nil
}

func (r *UserRepository) GetByCTechID(ctx context.Context, userID string) (map[string]types.AttributeValue, error) {
	res, err := r.QueryGSI(ctx, "ctech-user-id-index", "ctech_user_id", userID, 1, nil)
	if err != nil || len(res.Items) == 0 {
		return nil, err
	}
	return res.Items[0], nil
}

// CreateMinimal creates a user record from a ctech-account JWT sub (no local password).
// Profile fields (email, name, verification) are intentionally NOT stored here — they
// are owned by ctech-account and fetched live (see UserService.GetMeData) so there is
// never a second, driftable copy of them to keep in sync.
func (r *UserRepository) CreateMinimal(ctx context.Context, userID string) (map[string]types.AttributeValue, error) {
	now := NowStr()
	item := map[string]types.AttributeValue{
		"pk":            &types.AttributeValueMemberS{Value: BuildUserPK(userID)},
		"ctech_user_id": &types.AttributeValueMemberS{Value: userID},
		"organizations": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		"created_at":    &types.AttributeValueMemberS{Value: now},
		"updated_at":    &types.AttributeValueMemberS{Value: now},
	}
	return item, r.PutItem(ctx, item)
}

func (r *UserRepository) Update(ctx context.Context, userID string, updates map[string]any) (bool, error) {
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, BuildUserPK(userID), nil, updates)
}

// AddOrgMembership atomically appends an organization entry to the user's organizations list.
// Uses list_append so no read is required. Idempotency is not enforced at the DB level —
// callers must ensure they don't add duplicates (org creation is the only expected call site).
func (r *UserRepository) AddOrgMembership(ctx context.Context, userID, orgPK, role string, permissions []string) error {
	if permissions == nil {
		permissions = []string{}
	}
	entry, err := attributevalue.MarshalMap(map[string]any{"pk": orgPK, "role": role, "permissions": permissions})
	if err != nil {
		return fmt.Errorf("marshal org entry: %w", err)
	}
	_, err = r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: BuildUserPK(userID)},
		},
		UpdateExpression: aws.String(
			"SET organizations = list_append(if_not_exists(organizations, :empty), :new_org), updated_at = :now",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":empty": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
			":new_org": &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberM{Value: entry},
			}},
			":now": &types.AttributeValueMemberS{Value: NowStr()},
		},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	})
	return err
}

// GenerateID returns a new UUID v7 string.
func GenerateID() string {
	id, _ := uuid.NewV7()
	return id.String()
}
