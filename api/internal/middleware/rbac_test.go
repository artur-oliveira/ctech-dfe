package middleware

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ---------------------------------------------------------------------------
// UserOrgRole
// ---------------------------------------------------------------------------

func userItem(orgs []map[string]string) map[string]types.AttributeValue {
	entries := make([]types.AttributeValue, 0, len(orgs))
	for _, o := range orgs {
		m := make(map[string]types.AttributeValue, len(o))
		for k, v := range o {
			m[k] = &types.AttributeValueMemberS{Value: v}
		}
		entries = append(entries, &types.AttributeValueMemberM{Value: m})
	}
	return map[string]types.AttributeValue{
		"organizations": &types.AttributeValueMemberL{Value: entries},
	}
}

func TestUserOrgRole_Found(t *testing.T) {
	user := userItem([]map[string]string{
		{"pk": "CNPJ_11", "role": "OWNER"},
		{"pk": "CNPJ_22", "role": "USER"},
	})
	role, ok := UserOrgRole(user, "CNPJ_22")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if role != "USER" {
		t.Errorf("got role %q, want USER", role)
	}
}

func TestUserOrgRole_NotMember(t *testing.T) {
	user := userItem([]map[string]string{{"pk": "CNPJ_11", "role": "OWNER"}})
	_, ok := UserOrgRole(user, "CNPJ_99")
	if ok {
		t.Error("expected ok=false for non-member org")
	}
}

func TestUserOrgRole_EmptyOrgs(t *testing.T) {
	user := userItem(nil)
	_, ok := UserOrgRole(user, "CNPJ_11")
	if ok {
		t.Error("expected ok=false for empty organizations list")
	}
}

func TestUserOrgRole_MissingOrgsKey(t *testing.T) {
	user := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "USER_x"},
	}
	_, ok := UserOrgRole(user, "CNPJ_11")
	if ok {
		t.Error("expected ok=false when organizations key absent")
	}
}

func TestUserOrgRole_Owner(t *testing.T) {
	user := userItem([]map[string]string{{"pk": "CNPJ_11", "role": "OWNER"}})
	role, ok := UserOrgRole(user, "CNPJ_11")
	if !ok || role != "OWNER" {
		t.Errorf("got (%q, %v), want (OWNER, true)", role, ok)
	}
}

// ---------------------------------------------------------------------------
// RolePermissions
// ---------------------------------------------------------------------------

func roleItem(perms []string) map[string]types.AttributeValue {
	entries := make([]types.AttributeValue, 0, len(perms))
	for _, p := range perms {
		entries = append(entries, &types.AttributeValueMemberS{Value: p})
	}
	return map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: "ROLE_USER"},
		"permissions": &types.AttributeValueMemberL{Value: entries},
	}
}

func TestRolePermissions_Basic(t *testing.T) {
	item := roleItem([]string{"list.nfes", "get.nfes", "create.nfes"})
	perms := RolePermissions(item)
	if len(perms) != 3 {
		t.Fatalf("got %d perms, want 3", len(perms))
	}
	if perms[1] != "get.nfes" {
		t.Errorf("got perms[1]=%q, want get.nfes", perms[1])
	}
}

func TestRolePermissions_Empty(t *testing.T) {
	item := roleItem(nil)
	perms := RolePermissions(item)
	if len(perms) != 0 {
		t.Errorf("expected empty permissions, got %v", perms)
	}
}

func TestRolePermissions_MissingKey(t *testing.T) {
	item := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "ROLE_X"},
	}
	perms := RolePermissions(item)
	if perms != nil {
		t.Errorf("expected nil, got %v", perms)
	}
}

// ---------------------------------------------------------------------------
// containsStr
// ---------------------------------------------------------------------------

func TestContainsStr_Found(t *testing.T) {
	if !containsStr([]string{"a", "b", "c"}, "b") {
		t.Error("expected true")
	}
}

func TestContainsStr_NotFound(t *testing.T) {
	if containsStr([]string{"a", "b"}, "z") {
		t.Error("expected false")
	}
}

func TestContainsStr_Empty(t *testing.T) {
	if containsStr(nil, "x") {
		t.Error("expected false for nil slice")
	}
}
