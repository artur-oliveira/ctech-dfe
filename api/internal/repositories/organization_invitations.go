package repositories

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

// Invitation status values.
const (
	InvitationPending  = "PENDING"
	InvitationAccepted = "ACCEPTED"
	InvitationRevoked  = "REVOKED"
)

// InvitationTTLDays is how long an invitation link stays valid.
const InvitationTTLDays = 3

// invitationTTLSlack is extra time before DynamoDB's TTL sweep deletes an
// expired row, so the row lingers for auditing after it stops being usable.
const invitationTTLSlack = 48 * time.Hour

const invitationOrgGSI = "org-invite-index"

// OrgInvitationRepository stores single-use invitation links.
// Table structure (organization_invitations):
//
//	pk = INVITE_{sha256hex(token)}   (no sort key)
//	ttl = epoch seconds              (DynamoDB TTL, housekeeping only)
type OrgInvitationRepository struct {
	Base
}

func NewOrgInvitationRepository(db *dynamodb.Client, cfg *config.Config) *OrgInvitationRepository {
	return &OrgInvitationRepository{Base: NewBase(db, cfg, "organization_invitations")}
}

// GenerateInvitationToken returns a new opaque token (raw, for the link) and its
// SHA-256 hex (stored). The raw token is never persisted.
func GenerateInvitationToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = HashInvitationToken(raw)
	return raw, hash, nil
}

// HashInvitationToken returns the SHA-256 hex of a token.
func HashInvitationToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// InvitationPK builds the partition key from a token hash.
func InvitationPK(tokenHash string) string {
	return "INVITE_" + tokenHash
}

// Create writes a new PENDING invitation and returns its item.
func (r *OrgInvitationRepository) Create(ctx context.Context, tokenHash, orgPK, role, invitedBy, invitedByName string, permissions []string) (map[string]types.AttributeValue, error) {
	if permissions == nil {
		permissions = []string{}
	}
	permAV, _ := attributevalue.MarshalList(permissions)
	now := time.Now().UTC()
	expiresAt := now.Add(InvitationTTLDays * 24 * time.Hour)
	ttl := expiresAt.Add(invitationTTLSlack).Unix()
	item := map[string]types.AttributeValue{
		"pk":              &types.AttributeValueMemberS{Value: InvitationPK(tokenHash)},
		"org_pk":          &types.AttributeValueMemberS{Value: orgPK},
		"role":            &types.AttributeValueMemberS{Value: role},
		"permissions":     &types.AttributeValueMemberL{Value: permAV},
		"status":          &types.AttributeValueMemberS{Value: InvitationPending},
		"invited_by":      &types.AttributeValueMemberS{Value: invitedBy},
		"invited_by_name": &types.AttributeValueMemberS{Value: invitedByName},
		"expires_at":      &types.AttributeValueMemberS{Value: expiresAt.Format(time.RFC3339)},
		"ttl":             &types.AttributeValueMemberN{Value: strconv.FormatInt(ttl, 10)},
		"created_at":      &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		"updated_at":      &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
	}
	return item, r.PutItem(ctx, item)
}

// GetByToken fetches an invitation by its raw token (hashing it first).
func (r *OrgInvitationRepository) GetByToken(ctx context.Context, rawToken string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, InvitationPK(HashInvitationToken(rawToken)))
}

// Get fetches an invitation by its PK ("INVITE_{hash}").
func (r *OrgInvitationRepository) Get(ctx context.Context, pk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, pk)
}

// ListPendingByOrg returns an org's PENDING invitations, newest first.
func (r *OrgInvitationRepository) ListPendingByOrg(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{
		IndexName:        invitationOrgGSI,
		PKField:          "org_pk",
		SKField:          "created_at",
		PK:               orgPK,
		FilterField:      "status",
		FilterValue:      InvitationPending,
		ScanIndexForward: false,
		Limit:            200,
	})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// Revoke marks a PENDING invitation REVOKED, scoped to org (so an admin of one
// org can't revoke another org's invite by guessing the id). Returns false if
// no matching PENDING invitation exists.
func (r *OrgInvitationRepository) Revoke(ctx context.Context, pk, orgPK string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		Key:              map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: pk}},
		UpdateExpression: aws.String("SET #status = :revoked, updated_at = :now"),
		ConditionExpression: aws.String(
			"attribute_exists(pk) AND org_pk = :org AND #status = :pending"),
		ExpressionAttributeNames: map[string]string{"#status": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":revoked": &types.AttributeValueMemberS{Value: InvitationRevoked},
			":pending": &types.AttributeValueMemberS{Value: InvitationPending},
			":org":     &types.AttributeValueMemberS{Value: orgPK},
			":now":     &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		if isConditionFailed(err) {
			return false, nil
		}
		return false, wrapDynamoErr(err)
	}
	return true, nil
}

// BuildAcceptTxItem returns the conditional Update that consumes an invitation:
// it flips PENDING→ACCEPTED only if still pending and not past its ttl, giving
// single-use semantics even under concurrent accepts. Composed with the
// membership Put in one TransactWrite by the service.
func (r *OrgInvitationRepository) BuildAcceptTxItem(pk, acceptedBy string) types.TransactWriteItem {
	now := time.Now().UTC()
	return types.TransactWriteItem{
		Update: &types.Update{
			TableName:        aws.String(r.TableName),
			Key:              map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: pk}},
			UpdateExpression: aws.String("SET #status = :accepted, accepted_by = :uid, accepted_at = :now, updated_at = :now"),
			ConditionExpression: aws.String(
				"attribute_exists(pk) AND #status = :pending AND #ttl > :nowEpoch"),
			ExpressionAttributeNames: map[string]string{"#status": "status", "#ttl": "ttl"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":accepted": &types.AttributeValueMemberS{Value: InvitationAccepted},
				":pending":  &types.AttributeValueMemberS{Value: InvitationPending},
				":uid":      &types.AttributeValueMemberS{Value: acceptedBy},
				":now":      &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				":nowEpoch": &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)},
			},
		},
	}
}

// CountPendingByOrg returns how many PENDING invitations an org has (spam cap).
func (r *OrgInvitationRepository) CountPendingByOrg(ctx context.Context, orgPK string) (int, error) {
	items, err := r.ListPendingByOrg(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}
