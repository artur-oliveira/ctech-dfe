package services

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestMembershipFromItem(t *testing.T) {
	item := map[string]types.AttributeValue{
		"pk":      &types.AttributeValueMemberS{Value: "CNPJ_12345678000195"},
		"user_id": &types.AttributeValueMemberS{Value: "abc-123"},
		"role":    &types.AttributeValueMemberS{Value: "ADMIN"},
		"permissions": &types.AttributeValueMemberL{Value: []types.AttributeValue{
			&types.AttributeValueMemberS{Value: "create.nfes"},
		}},
	}
	m := membershipFromItem(item)
	if m.OrgPK != "CNPJ_12345678000195" || m.UserID != "abc-123" || m.Role != "ADMIN" {
		t.Fatalf("unexpected membership: %+v", m)
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != "create.nfes" {
		t.Errorf("permissions = %v, want [create.nfes]", m.Permissions)
	}
}

func TestEffectivePermissions_OwnerAdminGetAll(t *testing.T) {
	// roleRepo is never consulted for OWNER/ADMIN, so nil is fine here.
	s := &MembershipService{}
	for _, role := range []string{repositories.RoleOwner, repositories.RoleAdmin} {
		perms := s.EffectivePermissions(context.Background(), &Membership{Role: role})
		if len(perms) != len(repositories.AllPermissions) {
			t.Errorf("role %s: got %d perms, want all (%d)", role, len(perms), len(repositories.AllPermissions))
		}
	}
}

func TestEffectivePermissions_NilMembership(t *testing.T) {
	s := &MembershipService{}
	if perms := s.EffectivePermissions(context.Background(), nil); len(perms) != 0 {
		t.Errorf("nil membership should have no permissions, got %v", perms)
	}
}
