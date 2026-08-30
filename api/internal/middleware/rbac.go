package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

const (
	roleCacheTTL = 600

	// OrgHeader is the tenant identification header.
	// MUST match the constant in ui/src/lib/api/client.ts — never rename.
	OrgHeader = "Dfe-Organization-Pk"
	OrgPKKey  = "org_pk"
	// OrgRoleKey stores the resolved member role in Fiber locals so downstream
	// handlers can read it via GetOrgRole(c).
	OrgRoleKey = "org_role"
)

// Role name constants re-exported from the repositories package so middleware
// callers don't hardcode the strings.
const (
	roleOwner = repositories.RoleOwner
	roleAdmin = repositories.RoleAdmin
)

// PermChecker validates org membership and role-based permissions.
type PermChecker struct {
	memberSvc *services.MembershipService
	roleRepo  *repositories.RoleRepository
	c         cache.Backend
	// reach asks ctech-account whether the caller may act for this company.
	// Optional: nil keeps the pre-unification behaviour, where the product's own
	// row is the access record. Wiring it is the flip (ctech-billing ADR 0023).
	reach reachChecker
}

// WithReach turns on the edge check. Off by default and on by wiring, because
// this is a live authorization change with no dry run: the only rehearsal
// available is enabling it and watching one account.
func (p *PermChecker) WithReach(r reachChecker) *PermChecker {
	p.reach = r
	return p
}

// NewPermChecker constructs a PermChecker with the required dependencies.
func NewPermChecker(memberSvc *services.MembershipService, roleRepo *repositories.RoleRepository, c cache.Backend) *PermChecker {
	return &PermChecker{memberSvc: memberSvc, roleRepo: roleRepo, c: c}
}

// Require returns a Fiber handler that enforces the given permission string (e.g. "list.nfes").
// Sets org_pk in locals on success so downstream handlers can call middleware.GetOrgPK(c).
func (p *PermChecker) Require(permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return p.check(c, permission)
	}
}

// RequireDynamic builds the permission string at request time by formatting permFmt with the
// value of the named path parameter. Used for generic routes like /distributions/:doc_type.
// Example: RequireDynamic("list.%s_distributions", "doc_type")
func (p *PermChecker) RequireDynamic(permFmt, paramName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return p.check(c, fmt.Sprintf(permFmt, c.Params(paramName)))
	}
}

func (p *PermChecker) parseUserOrganizationRole(c fiber.Ctx) (string, *services.Membership, *problem.Problem) {
	userID := GetUserID(c)
	if userID == "" {
		return "", nil, problem.Unauthorized("missing user identity")
	}
	foundOrgPK := c.Get(OrgHeader)
	if foundOrgPK == "" {
		foundOrgPK = c.Params(OrgPKKey)
	}
	if foundOrgPK == "" {
		return "", nil, problem.BadRequest("missing organization: " + OrgHeader)
	}
	orgPK, err := repositories.ParseOrgPK(foundOrgPK)
	if err != nil {
		return "", nil, problem.BadRequest("invalid organization: " + foundOrgPK)
	}
	m, err := p.memberSvc.Get(c.Context(), orgPK, userID)
	if err != nil {
		return "", nil, problem.Forbidden(accessDenied)
	}

	// Reach from the edge, verbs from the row. Before the flip p.reach is nil
	// and the row is still the access record.
	if p.reach != nil {
		m, prob := authorize(c.Context(), p.reach, orgPK, userID, m)
		if prob != nil {
			return "", nil, prob
		}
		if m == nil {
			// Reach without a role: nothing to check verbs against.
			return "", nil, problem.Forbidden(accessDenied)
		}
		return orgPK, m, nil
	}

	if m == nil {
		// One message with the branch above, not two: "not a member" and
		// "unknown organization" were distinguishable here, which made the API
		// a probe for which organizations exist.
		return "", nil, problem.Forbidden(accessDenied)
	}
	return orgPK, m, nil
}

// requireRoles returns a handler that allows only members whose role is in the
// allowed set, bypassing the granular permission-string check — for endpoints
// where visibility itself is the sensitive thing (audit trail, member
// management), not a specific action.
func (p *PermChecker) requireRoles(msg string, allowed ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		orgPK, m, err := p.parseUserOrganizationRole(c)
		if err != nil {
			return c.Status(err.Status).JSON(err)
		}
		if !containsStr(allowed, m.Role) {
			return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden(msg))
		}
		// These role-gated actions (member/invitation management, audit trail)
		// are not grantable via any OAuth scope, so a scoped API-key token can
		// never perform them — only a full first-party session.
		if tokenIsScoped(GetScopes(c)) && !IsFirstPartyDfeSession(c) {
			return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("esta ação não é permitida para chaves de API"))
		}
		c.Locals(OrgPKKey, orgPK)
		c.Locals(OrgRoleKey, m.Role)
		return c.Next()
	}
}

// RequireOwner allows only the OWNER — for ownership-level actions (removing a
// member, changing a member's role).
func (p *PermChecker) RequireOwner() fiber.Handler {
	return p.requireRoles("Apenas o proprietário pode executar esta ação", roleOwner)
}

// RequireOwnerOrAdmin allows OWNER or ADMIN org members.
func (p *PermChecker) RequireOwnerOrAdmin() fiber.Handler {
	return p.requireRoles("Apenas proprietários e administradores podem executar esta ação", roleOwner, roleAdmin)
}

func (p *PermChecker) check(c fiber.Ctx, permission string) error {
	orgPK, m, err := p.parseUserOrganizationRole(c)
	if err != nil {
		return c.Status(err.Status).JSON(err)
	}
	c.Locals(OrgPKKey, orgPK)
	c.Locals(OrgRoleKey, m.Role)

	// RBAC decision: role bypass (OWNER/ADMIN) or effective permission
	// (role.permissions ∪ membership extras).
	rbacOK := m.Role == roleOwner || m.Role == roleAdmin ||
		containsStr(m.Permissions, permission) || p.hasPermission(c.Context(), m.Role, permission)
	if !rbacOK {
		return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Permissão insuficiente"))
	}

	// Scope decision (defense-in-depth): a scoped API-key token additionally
	// needs the matching OAuth scope. Identity-only sessions (no dfe:* scope)
	// are unrestricted, preserving first-party ui behavior.
	if scopes := GetScopes(c); tokenIsScoped(scopes) && !scopesGrant(scopes, permission) {
		return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("escopo do token insuficiente para esta ação"))
	}
	return c.Next()
}

func (p *PermChecker) hasPermission(ctx context.Context, roleName, permission string) bool {
	cacheKey := fmt.Sprintf("dfe:role:%s", roleName)

	if data, ok, err := p.c.Get(ctx, cacheKey); err == nil && ok {
		var perms []string
		if json.Unmarshal(data, &perms) == nil {
			return containsStr(perms, permission)
		}
	}

	role, err := p.roleRepo.Get(ctx, roleName)
	if err != nil || role == nil {
		return false
	}

	perms := RolePermissions(role)
	if data, err := json.Marshal(perms); err == nil {
		if err := p.c.Set(ctx, cacheKey, data, roleCacheTTL); err != nil {
			observability.Warn(ctx, "role permission cache write failed", err, "role", roleName)
		}
	} else {
		observability.Warn(ctx, "role permission serialization failed", err, "role", roleName)
	}

	return containsStr(perms, permission)
}

// GetOrgRole returns the resolved member role stored in locals by the perm
// guard (empty if not set).
func GetOrgRole(c fiber.Ctx) string {
	if v, ok := c.Locals(OrgRoleKey).(string); ok {
		return v
	}
	return ""
}

// RolePermissions extracts the permissions string slice from a role DynamoDB item.
func RolePermissions(role map[string]types.AttributeValue) []string {
	permsAV, ok := role["permissions"]
	if !ok {
		return nil
	}
	list, ok := permsAV.(*types.AttributeValueMemberL)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list.Value))
	for _, p := range list.Value {
		if sv, ok := p.(*types.AttributeValueMemberS); ok {
			out = append(out, sv.Value)
		}
	}
	return out
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
