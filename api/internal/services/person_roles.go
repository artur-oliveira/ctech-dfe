package services

// Person roles. A role is a *registry filter*, never a fiscal rule: issuance
// never validates it (receiver_id / transporta_pk keep doing a plain GetItem on
// the person). Requiring a role at issuance time would break every person
// already stored without one, for no fiscal gain.
//
// A person carries all the roles that apply to it at once — a carrier is
// commonly a customer too — so `roles` is a list on the person item and the
// person appears once in the general listing and in every role listing it
// belongs to. See docs/specs/2026-08-08-cadastros-reutilizaveis-emissao.md §3.6.
const (
	RoleCustomer = "customer"
	RoleSupplier = "supplier"
	RoleCarrier  = "carrier"
	RoleDriver   = "driver"
	RoleProvider = "provider"
	// RoleContractor é o contratante do frete (MDF-e infANTT/infContratante).
	RoleContractor = "freight_contractor"
)

// PersonRolesField is the attribute name holding the role list on the person
// item — the target of the contains() filter in PersonRepository.List.
const PersonRolesField = "roles"

// AllPersonRoles is the single source of truth for the accepted roles: DTO
// validation, the UI options and the docs all derive from it.
var AllPersonRoles = []string{
	RoleCustomer,
	RoleSupplier,
	RoleCarrier,
	RoleDriver,
	RoleProvider,
	RoleContractor,
}

// IsValidPersonRole reports whether role is one of AllPersonRoles.
func IsValidPersonRole(role string) bool {
	for _, r := range AllPersonRoles {
		if r == role {
			return true
		}
	}
	return false
}
