package services

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// membershipCacheTTL is short by design: this entry backs every authorization
// decision, so a stale value is the window during which a removed member keeps
// access. 60s (vs the 300s of other caches) bounds that window.
const membershipCacheTTL = 60

// userOrgsCacheTTL backs /auth/me and GET /organizations — not an authorization
// decision, so the longer TTL is fine.
const userOrgsCacheTTL = 300

// Membership is the decoded, cache-friendly shape of an organization_users row.
// Revoked is never persisted — it is a cache tombstone marking "no access"
// (either a removed member or a cached non-membership), so an in-flight request
// that read the DB before a delete cannot repopulate a positive entry after it.
type Membership struct {
	OrgPK       string   `json:"org_pk"`
	UserID      string   `json:"user_id"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	InvitedBy   string   `json:"invited_by"`
	CreatedAt   string   `json:"created_at"`
	Revoked     bool     `json:"revoked,omitempty"`
}

// MembershipService owns membership reads/writes and their cache. It is the
// single source of truth for user↔organization access.
type MembershipService struct {
	repo      *repositories.OrgUserRepository
	roleRepo  *repositories.RoleRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewMembershipService(repo *repositories.OrgUserRepository, auditRepo *repositories.AuditLogRepository, roleRepo *repositories.RoleRepository, c cache.Backend) *MembershipService {
	return &MembershipService{repo: repo, roleRepo: roleRepo, auditRepo: auditRepo, cache: c}
}

// EffectivePermissions returns the union of the role's permissions and the
// membership's extra grants — the full set the member can exercise. OWNER/ADMIN
// resolve to the full permission set.
func (s *MembershipService) EffectivePermissions(ctx context.Context, m *Membership) []string {
	if m == nil {
		return []string{}
	}
	if m.Role == repositories.RoleOwner || m.Role == repositories.RoleAdmin {
		return repositories.AllPermissions
	}
	set := make(map[string]struct{})
	if role, err := s.roleRepo.Get(ctx, m.Role); err == nil && role != nil {
		if permsAV, ok := role["permissions"].(*types.AttributeValueMemberL); ok {
			for _, p := range permsAV.Value {
				if sv, ok := p.(*types.AttributeValueMemberS); ok {
					set[sv.Value] = struct{}{}
				}
			}
		}
	}
	for _, p := range m.Permissions {
		set[p] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

func memberCacheKey(orgPK, userID string) string {
	return fmt.Sprintf("member:%s:%s", orgPK, repositories.RawUserID(userID))
}

func userOrgsCacheKey(userID string) string {
	return fmt.Sprintf("user_orgs:%s", repositories.RawUserID(userID))
}

// Get returns the caller's membership in orgPK, or nil if they are not a member.
// Cache-first; on a miss it reads organization_users, and if still absent falls
// back to the legacy users.organizations list (migration window) and self-heals
// the new table. The result is cached either way (positive or tombstone).
func (s *MembershipService) Get(ctx context.Context, orgPK, userID string) (*Membership, error) {
	key := memberCacheKey(orgPK, userID)
	if v, ok := CacheGet[Membership](ctx, s.cache, key); ok {
		if v.Revoked {
			return nil, nil
		}
		return v, nil
	}

	item, err := s.repo.Get(ctx, orgPK, userID)
	if err != nil {
		return nil, err
	}
	if item != nil {
		m := membershipFromItem(item)
		CacheSet(ctx, s.cache, key, *m, membershipCacheTTL)
		return m, nil
	}

	// Cache a tombstone so repeated unauthorized hits don't all reach DynamoDB.
	CacheSet(ctx, s.cache, key, Membership{OrgPK: orgPK, UserID: repositories.RawUserID(userID), Revoked: true}, membershipCacheTTL)
	return nil, nil
}

// ListByUser returns every organization the user belongs to.
func (s *MembershipService) ListByUser(ctx context.Context, userID string) ([]Membership, error) {
	if v, ok := CacheGet[[]Membership](ctx, s.cache, userOrgsCacheKey(userID)); ok {
		return *v, nil
	}

	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Membership, 0, len(items))
	for _, it := range items {
		out = append(out, *membershipFromItem(it))
	}

	CacheSet(ctx, s.cache, userOrgsCacheKey(userID), out, userOrgsCacheTTL)
	return out, nil
}

// Create writes a membership and invalidates its caches. Idempotent.
//
// It cannot write an OWNER. Ownership is established by creating the
// organization — OrganizationService writes that row in the same transaction as
// the organization itself — and there is no second way in, because a second way
// in is a second OWNER.
func (s *MembershipService) Create(ctx context.Context, orgPK, userID, role, invitedBy, name string, permissions []string) error {
	if err := guardGrantableRole(role); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, orgPK, userID, role, invitedBy, name, permissions); err != nil {
		return err
	}
	s.Invalidate(ctx, orgPK, userID)
	return nil
}

// ListByOrg returns all members of an organization (member-management screen).
func (s *MembershipService) ListByOrg(ctx context.Context, orgPK string) ([]Membership, error) {
	items, err := s.repo.ListByOrg(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	out := make([]Membership, 0, len(items))
	for _, it := range items {
		out = append(out, *membershipFromItem(it))
	}
	return out, nil
}

// ChangeRole updates a member's role, refusing both to create a second OWNER
// and to leave the organization without one. Invalidates the affected caches.
//
// The two guards are the same invariant read from its two ends: exactly one
// OWNER, the one who created the organization. Promoting somebody is the end
// that used to be open — the route's payload happened to reject it, but a
// validation tag on one DTO is not an invariant, and the ownership-transfer
// feature will arrive as a second caller of this method.
func (s *MembershipService) ChangeRole(ctx context.Context, orgPK, userID, role string) error {
	if err := guardGrantableRole(role); err != nil {
		return err
	}
	if err := s.guardLastOwner(ctx, orgPK, userID); err != nil {
		return err
	}
	ok, err := s.repo.UpdateRole(ctx, orgPK, userID, role, nil)
	if err != nil {
		return err
	}
	if !ok {
		return problem.NotFound("membro não encontrado")
	}
	s.Invalidate(ctx, orgPK, userID)
	return nil
}

// Remove deletes a membership, refusing to remove the last OWNER. Writes a
// cache tombstone (rather than a plain delete) so a concurrent in-flight read
// cannot repopulate a positive entry.
func (s *MembershipService) Remove(ctx context.Context, orgPK, userID, deletedById, deletedByName string) error {
	if err := s.guardLastOwner(ctx, orgPK, userID); err != nil {
		return err
	}

	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceMember, userID, repositories.AuditActionDelete,
		deletedById, deletedByName, Diff(map[string]any{"user_id": userID}, nil),
	)
	if err != nil {
		return err
	}

	txItems := []types.TransactWriteItem{
		s.repo.BuildDeleteTxItem(orgPK, repositories.RawUserID(userID)),
		auditTx,
	}
	if err := s.repo.TransactWrite(ctx, txItems); err != nil {
		if repositories.IsConditionFailed(err) {
			return problem.Conflict("membro não encontrado ou já excluído")
		}
		return err
	}
	s.tombstone(ctx, orgPK, userID)
	_ = s.cache.Delete(ctx, userOrgsCacheKey(userID))
	_ = s.cache.Delete(ctx, userMeCacheKey(userID))
	return nil
}

// Invalidate clears the membership and user-orgs cache for one (org, user).
func (s *MembershipService) Invalidate(ctx context.Context, orgPK, userID string) {
	_ = s.cache.Delete(ctx, memberCacheKey(orgPK, userID))
	_ = s.cache.Delete(ctx, userOrgsCacheKey(userID))
	// GET /auth/me caches the orgs list under "me:{userID}".
	_ = s.cache.Delete(ctx, userMeCacheKey(userID))
}

// tombstone caches a "no access" marker for the invalidation window, closing
// the removed-member repopulation race described on the Membership type.
func (s *MembershipService) tombstone(ctx context.Context, orgPK, userID string) {
	CacheSet(ctx, s.cache, memberCacheKey(orgPK, userID),
		Membership{OrgPK: orgPK, UserID: repositories.RawUserID(userID), Revoked: true}, membershipCacheTTL)
}

// guardGrantableRole refuses a role member management may not hand out, which
// today means OWNER — see repositories.GrantableRoles for why there is exactly
// one and where it comes from.
func guardGrantableRole(role string) error {
	if repositories.IsGrantableRole(role) {
		return nil
	}
	if role == repositories.RoleOwner {
		return problem.Conflict(
			"a organização já tem um proprietário e não pode ter outro; use ADMIN, que tem os mesmos acessos")
	}
	return problem.BadRequest("função inválida")
}

// guardLastOwner blocks any change that would drop the last OWNER: removing
// (newRole == "") or demoting the sole owner.
//
// It still counts rather than refusing outright, and that is deliberate: an
// organization that already carries two OWNERs from before this rule existed can
// be repaired by demoting one, and a flat refusal would leave it stuck at two
// forever.
func (s *MembershipService) guardLastOwner(ctx context.Context, orgPK, userID string) error {
	current, err := s.repo.Get(ctx, orgPK, userID)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if roleAV, ok := current["role"].(*types.AttributeValueMemberS); !ok || roleAV.Value != repositories.RoleOwner {
		return nil
	}
	owners, err := s.repo.CountOwners(ctx, orgPK)
	if err != nil {
		return err
	}
	if owners <= 1 {
		return problem.Conflict("A organização precisa de ao menos um proprietário")
	}
	return nil
}

func membershipFromItem(item map[string]types.AttributeValue) *Membership {
	m := &Membership{}
	if v, ok := item["pk"].(*types.AttributeValueMemberS); ok {
		m.OrgPK = v.Value
	}
	if v, ok := item["user_id"].(*types.AttributeValueMemberS); ok {
		m.UserID = v.Value
	}
	if v, ok := item["name"].(*types.AttributeValueMemberS); ok {
		m.Name = v.Value
	}
	if v, ok := item["role"].(*types.AttributeValueMemberS); ok {
		m.Role = v.Value
	}
	if v, ok := item["invited_by"].(*types.AttributeValueMemberS); ok {
		m.InvitedBy = v.Value
	}
	if v, ok := item["created_at"].(*types.AttributeValueMemberS); ok {
		m.CreatedAt = v.Value
	}
	if v, ok := item["permissions"].(*types.AttributeValueMemberL); ok {
		for _, p := range v.Value {
			if s, ok := p.(*types.AttributeValueMemberS); ok {
				m.Permissions = append(m.Permissions, s.Value)
			}
		}
	}
	return m
}
