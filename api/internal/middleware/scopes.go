package middleware

import "strings"

// OAuth scope enforcement for machine-to-machine API keys.
//
// ctech-account mints short-lived JWTs for API keys with a space-delimited
// `scope` claim of `dfe:{resource}:{read|write}` entries. A token that carries
// any `dfe:*` scope is "scoped": its effective permission is the INTERSECTION
// of the org RBAC role and the granted scopes (defense-in-depth — a scope never
// widens what the underlying member could do, only narrows it). First-party ui
// sessions carry only identity scopes (openid/profile), so they are treated as
// unrestricted and keep the pure-RBAC behavior.
//
// The scope→RBAC-resource mapping below MUST stay in sync with the scope
// catalog in ctech-account (internal/scopes/catalog.go, served by GET
// /v1.0/scopes). See INTEGRATION.md §"Scope claim enforcement".

const (
	scopePrefix = "dfe:"
	scopeRead   = "read"
	scopeWrite  = "write"
)

// scopeFamilies maps a scope resource to the RBAC resources it grants. A
// document family's scope covers its events, distributions, and fiscal config
// too: `read` grants list/get across the family, `write` grants
// create/update/delete across it (including the config).
var scopeFamilies = map[string][]string{
	"nfes":                           {"nfes", "nfe_events", "nfe_distributions", "organization_nfe_configs"},
	"nfces":                          {"nfces", "nfce_events", "organization_nfce_configs"},
	"ctes":                           {"ctes", "cte_events", "cte_distributions", "organization_cte_configs"},
	"mdfes":                          {"mdfes", "mdfe_events", "mdfe_distributions", "organization_mdfe_configs"},
	"nfses":                          {"nfses", "nfse_events", "nfse_distributions", "organization_nfse_configs"},
	"organization_services":          {"organization_services"},
	"organization_products":          {"organization_products"},
	"organization_vehicles":          {"organization_vehicles"},
	"organization_vehicle_sets":      {"organization_vehicle_sets"},
	"organization_payment_terms":     {"organization_payment_terms"},
	"organization_payment_terminals": {"organization_payment_terminals"},
	"organization_toll_providers":    {"organization_toll_providers"},
	"organization_tax_profiles":      {"organization_tax_profiles"},
	"organization_operations":        {"organization_operations"},
	"organization_persons":           {"organization_persons"},
	"organizations":                  {"organizations"},
	"organization_certificates":      {"organization_certificates"},
}

// readActions / writeActions map a scope access level to RBAC actions.
var readActions = map[string]bool{"get": true, "list": true}
var writeActions = map[string]bool{"create": true, "update": true, "delete": true}

// tokenIsScoped reports whether the token carries any dfe:* service scope. Only
// scoped tokens are subject to scope enforcement; identity-only sessions are not.
func tokenIsScoped(scopes []string) bool {
	for _, s := range scopes {
		if strings.HasPrefix(s, scopePrefix) {
			return true
		}
	}
	return false
}

// scopesGrant reports whether the token scopes cover the RBAC permission
// (format "action.resource", e.g. "create.nfes").
func scopesGrant(scopes []string, permission string) bool {
	action, resource, ok := splitPermission(permission)
	if !ok {
		return false
	}
	for _, s := range scopes {
		if scopeGrantsPermission(s, action, resource) {
			return true
		}
	}
	return false
}

// scopeGrantsPermission reports whether one scope ("dfe:res:access") grants the
// RBAC (action, resource) pair.
func scopeGrantsPermission(scope, action, resource string) bool {
	rest, ok := strings.CutPrefix(scope, scopePrefix)
	if !ok {
		return false
	}
	sres, access, ok := strings.Cut(rest, ":")
	if !ok {
		return false
	}
	switch access {
	case scopeRead:
		if !readActions[action] {
			return false
		}
	case scopeWrite:
		if !writeActions[action] {
			return false
		}
	default:
		return false
	}
	for _, covered := range scopeFamilies[sres] {
		if covered == resource {
			return true
		}
	}
	return false
}

// splitPermission splits "action.resource" into its parts.
func splitPermission(permission string) (action, resource string, ok bool) {
	action, resource, ok = strings.Cut(permission, ".")
	return action, resource, ok
}

// ScopeFamilies devolve o mapa de famílias de escopo. Cópia rasa pela mesma
// razão de AllResources.
func ScopeFamilies() map[string][]string {
	out := make(map[string][]string, len(scopeFamilies))
	for k, v := range scopeFamilies {
		out[k] = append([]string(nil), v...)
	}
	return out
}
