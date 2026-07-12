package middleware

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
