package middleware

import (
	"sort"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/oauthresource"
)

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

func TestScopeManifestMatchesEnforcementFamilies(t *testing.T) {
	m, err := oauthresource.ManifestDocument()
	if err != nil {
		t.Fatal(err)
	}
	if m.ResourceServerID != "dfe" || m.SchemaVersion != 1 {
		t.Fatalf("unexpected manifest identity: %#v", m)
	}
	var got []string
	for _, scope := range m.Scopes {
		if scope.Visibility != "public" || scope.Status != "active" {
			t.Fatalf("DFe scope %q must be public and active", scope.Name)
		}
		if scope.Descriptions["pt-BR"] == "" || scope.Descriptions["en"] == "" {
			t.Fatalf("DFe scope %q must have pt-BR and en descriptions", scope.Name)
		}
		got = append(got, scope.Name)
	}
	var want []string
	for family := range scopeFamilies {
		want = append(want, scopePrefix+family+":"+scopeRead, scopePrefix+family+":"+scopeWrite)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("manifest contains %d scopes, enforcement defines %d: got=%v want=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("manifest/enforcement drift: got=%v want=%v", got, want)
		}
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
		{"nfse read covers ADN distributions", []string{"dfe:nfses:read"}, "list.nfse_distributions", true},
		{"nfse write covers ADN sync", []string{"dfe:nfses:write"}, "create.nfse_distributions", true},
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
