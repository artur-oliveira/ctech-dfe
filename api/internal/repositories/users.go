package repositories

import (
	"context"
	"fmt"
	"strings"

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

// GenerateID returns a new UUID v7 string.
func GenerateID() string {
	id, _ := uuid.NewV7()
	return id.String()
}
