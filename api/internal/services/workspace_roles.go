package services

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
)

// ctech-account's ladder roles that manage an organization's DF-e subscription
// (ctech-billing ADR 0025 amendment of 2026-10-07).
const (
	AccountRoleOwner = "owner"
	AccountRoleAdmin = "admin"
)

// workspaceRoleCacheTTL matches reachCacheTTL: a demotion lands within a minute.
const workspaceRoleCacheTTL = 60

// workspaceNameCacheTTL is longer: the name is display-only.
const workspaceNameCacheTTL = 300

// MayManageBilling reports a role that may choose, change, cancel or pay.
func MayManageBilling(role string) bool {
	return role == AccountRoleOwner || role == AccountRoleAdmin
}

type workspaceSource interface {
	Membership(ctx context.Context, organizationID, userID string) (role, kind string, member bool, err error)
	Organizations(ctx context.Context, userID string) ([]accountclient.Workspace, error)
}

// WorkspaceRoleService answers "which role does this person hold in this
// ctech-account organization", cached, failing closed like ReachService: an
// outage is an error and is never cached, and nil is a refusal.
type WorkspaceRoleService struct {
	src   workspaceSource
	cache cache.Backend
}

func NewWorkspaceRoleService(src workspaceSource, c cache.Backend) *WorkspaceRoleService {
	return &WorkspaceRoleService{src: src, cache: c}
}

func workspaceRoleCacheKey(organizationID, userID string) string {
	return fmt.Sprintf("dfe:workspace-role:%s:%s", organizationID, userID)
}

func workspaceNameCacheKey(organizationID string) string {
	return "dfe:workspace-name:" + organizationID
}

type workspaceRoleAnswer struct {
	Role string `json:"role"`
}

// Role returns the person's role, "" for a non-member (or a member of a
// workspace that is not an organization).
func (s *WorkspaceRoleService) Role(ctx context.Context, organizationID, userID string) (string, error) {
	if s == nil || s.src == nil {
		return "", fmt.Errorf("the ctech-account workspace credential is not configured")
	}
	key := workspaceRoleCacheKey(organizationID, userID)
	if v, ok := CacheGet[workspaceRoleAnswer](ctx, s.cache, key); ok {
		return v.Role, nil
	}
	role, kind, member, err := s.src.Membership(ctx, organizationID, userID)
	if err != nil {
		return "", fmt.Errorf("reading the role in %s: %w", organizationID, err)
	}
	if !member || !(accountclient.Workspace{Kind: kind}).IsOrganization() {
		role = ""
	}
	CacheSet(ctx, s.cache, key, workspaceRoleAnswer{Role: role}, workspaceRoleCacheTTL)
	return role, nil
}

// OrganizationName is the organization's display name as the person sees it,
// or "" when it cannot be read. Never an authorization input.
func (s *WorkspaceRoleService) OrganizationName(ctx context.Context, organizationID, userID string) string {
	if s == nil || s.src == nil {
		return ""
	}
	key := workspaceNameCacheKey(organizationID)
	if v, ok := CacheGet[string](ctx, s.cache, key); ok {
		return *v
	}
	orgs, err := s.src.Organizations(ctx, userID)
	if err != nil {
		return ""
	}
	for _, w := range orgs {
		if w.ID == organizationID {
			CacheSet(ctx, s.cache, key, w.DisplayName, workspaceNameCacheTTL)
			return w.DisplayName
		}
	}
	return ""
}
