package repositories

import "testing"

func has(perms []string, p string) bool {
	for _, v := range perms {
		if v == p {
			return true
		}
	}
	return false
}

func TestViewerPermissionsAreReadOnly(t *testing.T) {
	for _, p := range ViewerPermissions {
		if !hasPrefix(p, "list.") && !hasPrefix(p, "get.") {
			t.Errorf("VIEWER has non-read permission: %s", p)
		}
	}
	if !has(ViewerPermissions, "list.nfes") || !has(ViewerPermissions, "get.organizations") {
		t.Error("VIEWER missing expected read permissions")
	}
}

func TestUserPermissionsExcludeDangerous(t *testing.T) {
	for _, p := range UserPermissions {
		if hasPrefix(p, "delete.") {
			t.Errorf("USER must not have delete permission: %s", p)
		}
	}
	if has(UserPermissions, "update.organizations") {
		t.Error("USER must not update organizations")
	}
	for _, p := range []string{
		"list.organization_certificates", "create.organization_certificates",
	} {
		if has(UserPermissions, p) {
			t.Errorf("USER must not access certificates: %s", p)
		}
	}
	// but it can do day-to-day work
	for _, p := range []string{"create.nfes", "update.organization_products", "get.organizations"} {
		if !has(UserPermissions, p) {
			t.Errorf("USER missing expected permission: %s", p)
		}
	}
}

func TestSystemRolesCoverFour(t *testing.T) {
	roles := SystemRoles()
	if len(roles) != 4 {
		t.Fatalf("expected 4 system roles, got %d", len(roles))
	}
	names := map[string]bool{}
	for _, r := range roles {
		names[r.Name] = true
		if len(r.Permissions) == 0 {
			t.Errorf("role %s has no permissions", r.Name)
		}
	}
	for _, n := range []string{RoleOwner, RoleAdmin, RoleUser, RoleViewer} {
		if !names[n] {
			t.Errorf("missing system role %s", n)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
