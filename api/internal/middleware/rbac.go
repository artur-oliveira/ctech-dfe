package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

const (
	roleCacheTTL = 600
	roleOwner    = "OWNER"
	roleAdmin    = "ADMIN"

	// OrgHeader is the tenant identification header.
	// MUST match the constant in ui/src/lib/api/client.ts — never rename.
	OrgHeader = "Dfe-Organization-Pk"
	OrgPKKey  = "org_pk"
)

// PermChecker validates org membership and role-based permissions.
// Mirrors require_permission from api/app/dependencies/organization.py.
type PermChecker struct {
	userSvc  *services.UserService
	roleRepo *repositories.RoleRepository
	c        cache.Backend
}

// NewPermChecker constructs a PermChecker with the required dependencies.
func NewPermChecker(userSvc *services.UserService, roleRepo *repositories.RoleRepository, c cache.Backend) *PermChecker {
	return &PermChecker{userSvc: userSvc, roleRepo: roleRepo, c: c}
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

func (p *PermChecker) parseUserOrganizationRole(c fiber.Ctx) (string, string, error) {
	userID := GetUserID(c)
	if userID == "" {
		return "", "", c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("missing user identity"))
	}
	foundOrgPK := c.Get(OrgHeader)
	if foundOrgPK == "" {
		foundOrgPK = c.Params(OrgPKKey)
	}
	if foundOrgPK == "" {
		return "", "", c.Status(fiber.StatusBadRequest).JSON(problem.BadRequest("missing organization: " + OrgHeader))
	}
	orgPK, err := repositories.ParseOrgPK(foundOrgPK)
	if err != nil {
		return "", "", c.Status(fiber.StatusBadRequest).JSON(problem.BadRequest("invalid organization: " + foundOrgPK))
	}
	user, err := p.userSvc.GetMe(c.Context(), userID)
	if err != nil {
		return "", "", c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Acesso negado"))
	}
	roleName, ok := UserOrgRole(user, orgPK)
	if !ok {
		return "", "", c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Acesso negado a esta organização"))
	}
	return orgPK, roleName, nil
}

// RequireOwnerOrAdmin returns a Fiber handler that allows only OWNER/ADMIN org
// members, bypassing the granular permission-string check entirely — for
// endpoints like the audit trail where visibility itself is the sensitive
// thing, not a specific action.
func (p *PermChecker) RequireOwnerOrAdmin() fiber.Handler {
	return func(c fiber.Ctx) error {
		orgPK, roleName, err := p.parseUserOrganizationRole(c)
		if err != nil {
			return err
		}
		if roleName != roleOwner && roleName != roleAdmin {
			return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Apenas proprietários e administradores podem ver o log de auditoria"))
		}
		c.Locals(OrgPKKey, orgPK)
		return c.Next()
	}
}

func (p *PermChecker) check(c fiber.Ctx, permission string) error {
	orgPK, roleName, err := p.parseUserOrganizationRole(c)
	if err != nil {
		return err
	}
	if roleName == roleOwner || roleName == roleAdmin {
		c.Locals(OrgPKKey, orgPK)
		return c.Next()
	}

	if !p.hasPermission(c.Context(), roleName, permission) {
		return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Permissão insuficiente"))
	}

	c.Locals(OrgPKKey, orgPK)
	return c.Next()
}

func (p *PermChecker) hasPermission(ctx context.Context, roleName, permission string) bool {
	cacheKey := fmt.Sprintf("role:%s", roleName)

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
		_ = p.c.Set(ctx, cacheKey, data, roleCacheTTL)
	}

	return containsStr(perms, permission)
}

// UserOrgRole finds the user's role string for the given orgPK in the organizations list.
// The organizations attribute is a DynamoDB List of Maps, each with "pk" and "role" string fields.
func UserOrgRole(user map[string]types.AttributeValue, orgPK string) (string, bool) {
	orgsAV, ok := user["organizations"]
	if !ok {
		return "", false
	}
	list, ok := orgsAV.(*types.AttributeValueMemberL)
	if !ok {
		return "", false
	}
	for _, item := range list.Value {
		m, ok := item.(*types.AttributeValueMemberM)
		if !ok {
			continue
		}
		pkAV, ok := m.Value["pk"]
		if !ok {
			continue
		}
		pkS, ok := pkAV.(*types.AttributeValueMemberS)
		if !ok || pkS.Value != orgPK {
			continue
		}
		if roleAV, ok := m.Value["role"]; ok {
			if roleS, ok := roleAV.(*types.AttributeValueMemberS); ok {
				return roleS.Value, true
			}
		}
		return "", true
	}
	return "", false
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
