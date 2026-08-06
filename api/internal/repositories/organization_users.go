package repositories

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// orgUserGSI is the inverted index (PK sk, SK pk) — lists every org a user
// belongs to without duplicating any attribute.
const orgUserGSI = "user-index"

// OrgUserRepository is the source of truth for user↔organization membership.
// Table structure (organization_users):
//
//	pk = {org_pk}          e.g. CNPJ_12345678000195
//	sk = USER_{sub}
type OrgUserRepository struct {
	Base
}

func NewOrgUserRepository(db *dynamodb.Client, cfg *config.Config) *OrgUserRepository {
	return &OrgUserRepository{Base: NewBase(db, cfg, "organization_users")}
}

// BuildMemberSK returns the membership sort key "USER_{sub}", idempotent
// against a sub that already carries the prefix.
func BuildMemberSK(userID string) string {
	if strings.HasPrefix(userID, "USER_") {
		return userID
	}
	return "USER_" + userID
}

// RawUserID strips the "USER_" prefix if present, yielding the bare ctech sub.
func RawUserID(userID string) string {
	return strings.TrimPrefix(userID, "USER_")
}

func (r *OrgUserRepository) membershipItem(orgPK, userID, role, invitedBy, name string, permissions []string) map[string]types.AttributeValue {
	if permissions == nil {
		permissions = []string{}
	}
	permAV, _ := attributevalue.MarshalList(permissions)
	now := NowStr()
	item := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: orgPK},
		"sk":          &types.AttributeValueMemberS{Value: BuildMemberSK(userID)},
		"user_id":     &types.AttributeValueMemberS{Value: RawUserID(userID)},
		"role":        &types.AttributeValueMemberS{Value: role},
		"permissions": &types.AttributeValueMemberL{Value: permAV},
		"invited_by":  &types.AttributeValueMemberS{Value: invitedBy},
		// name is a display-only snapshot of the member's name at grant time. It
		// is never kept in sync with ctech-account — it only spares the members
		// screen from showing a bare UUID.
		"name":       &types.AttributeValueMemberS{Value: name},
		"created_at": &types.AttributeValueMemberS{Value: now},
		"updated_at": &types.AttributeValueMemberS{Value: now},
	}
	return item
}

// Get fetches a single membership by (org_pk, user_id). Strongly consistent —
// this is the RBAC hot path.
func (r *OrgUserRepository) Get(ctx context.Context, orgPK, userID string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, BuildMemberSK(userID))
}

// ListByOrg returns all members of an organization.
func (r *OrgUserRepository) ListByOrg(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{PK: orgPK, SKPrefix: "USER_", Limit: 500})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// ListByUser returns every membership for a user, via the inverted GSI
// (eventually consistent — never used for an authorization decision).
func (r *OrgUserRepository) ListByUser(ctx context.Context, userID string) ([]map[string]types.AttributeValue, error) {
	res, err := r.QueryGSI(ctx, orgUserGSI, "sk", BuildMemberSK(userID), 500, nil)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// Create writes a membership immediately (used by the migration backfill and
// the read-fallback self-heal). Idempotent: no-op if it already exists.
func (r *OrgUserRepository) Create(ctx context.Context, orgPK, userID, role, invitedBy, name string, permissions []string) error {
	return r.TransactWrite(ctx, []types.TransactWriteItem{
		r.BuildCreateTxItem(orgPK, userID, role, invitedBy, name, permissions),
	})
}

// BuildCreateTxItem returns a create-only TransactWriteItem (fails if the
// membership already exists) plus the item, for composing atomic writes with
// the organization/invitation rows.
func (r *OrgUserRepository) BuildCreateTxItem(orgPK, userID, role, invitedBy, name string, permissions []string) types.TransactWriteItem {
	return r.BuildPutTxItemIfAbsent(r.membershipItem(orgPK, userID, role, invitedBy, name, permissions))
}

// BuildDeleteTxItem returns a delete TransactWriteItem for a membership.
func (r *OrgUserRepository) BuildDeleteTxItem(orgPK, userID string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, BuildMemberSK(userID))
}

// Delete removes a membership. Returns false if it did not exist.
func (r *OrgUserRepository) Delete(ctx context.Context, orgPK, userID string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, BuildMemberSK(userID))
}

// UpdateRole changes a member's role and its extra permissions. Returns false
// if the membership does not exist.
func (r *OrgUserRepository) UpdateRole(ctx context.Context, orgPK, userID, role string, permissions []string) (bool, error) {
	if permissions == nil {
		permissions = []string{}
	}
	return r.UpdateItem(ctx, orgPK, new(BuildMemberSK(userID)), map[string]any{
		"role":        role,
		"permissions": permissions,
	})
}

// CountOwners returns how many OWNER members an organization has — used to
// block removing or demoting the last owner.
func (r *OrgUserRepository) CountOwners(ctx context.Context, orgPK string) (int, error) {
	items, err := r.ListByOrg(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, it := range items {
		if roleAV, ok := it["role"].(*types.AttributeValueMemberS); ok && roleAV.Value == RoleOwner {
			n++
		}
	}
	return n, nil
}
