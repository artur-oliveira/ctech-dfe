package middleware

import "testing"

func TestTokenIsScoped(t *testing.T) {
	if tokenIsScoped([]string{"openid", "profile"}) {
		t.Error("identity-only session must not be treated as scoped")
	}
	if !tokenIsScoped([]string{"openid", "dfe:nfes:read"}) {
		t.Error("a dfe:* scope makes the token scoped")
	}
	if tokenIsScoped(nil) {
		t.Error("no scopes → not scoped")
	}
}

func TestScopesGrant(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []string
		permission string
		want       bool
	}{
		{"read grants get", []string{"dfe:mdfes:read"}, "get.mdfes", true},
		{"read grants list", []string{"dfe:mdfes:read"}, "list.mdfes", true},
		{"read does not grant create", []string{"dfe:mdfes:read"}, "create.mdfes", false},
		{"write grants create", []string{"dfe:nfes:write"}, "create.nfes", true},
		{"write grants delete", []string{"dfe:nfes:write"}, "delete.nfes", true},
		{"read covers events", []string{"dfe:nfes:read"}, "list.nfe_events", true},
		{"read covers distributions", []string{"dfe:nfes:read"}, "get.nfe_distributions", true},
		{"write covers fiscal config", []string{"dfe:nfes:write"}, "update.organization_nfe_configs", true},
		{"read covers config get", []string{"dfe:mdfes:read"}, "get.organization_mdfe_configs", true},
		{"wrong family not granted", []string{"dfe:mdfes:read"}, "get.nfes", false},
		{"products scope", []string{"dfe:organization_products:write"}, "create.organization_products", true},
		{"certificates isolated", []string{"dfe:nfes:write"}, "create.organization_certificates", false},
		{"certificates own scope", []string{"dfe:organization_certificates:write"}, "create.organization_certificates", true},
		{"nfce has no distributions family entry", []string{"dfe:nfces:read"}, "get.nfce_events", true},
		{"multiple scopes union", []string{"dfe:mdfes:read", "dfe:nfes:write"}, "create.nfes", true},
		{"unknown scope resource", []string{"dfe:bogus:read"}, "get.bogus", false},
		{"malformed scope", []string{"dfe:nfes"}, "get.nfes", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopesGrant(tc.scopes, tc.permission); got != tc.want {
				t.Errorf("scopesGrant(%v, %q) = %v, want %v", tc.scopes, tc.permission, got, tc.want)
			}
		})
	}
}
