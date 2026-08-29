package services

import "testing"

// Two organizations may hold the same CNPJ root under ctech-billing ADR 0022 —
// an accountant and their client. A sibling search that matched on the root
// alone would offer one customer another customer's certificate.
func TestASiblingMustShareTheOrganization(t *testing.T) {
	mine := branchCandidate{OrganizationID: "org_1", Root: "11222333"}
	cases := []struct {
		name  string
		other branchCandidate
		want  bool
	}{
		{"same organization, same root", branchCandidate{OrganizationID: "org_1", Root: "11222333"}, true},
		{"same root, another organization", branchCandidate{OrganizationID: "org_2", Root: "11222333"}, false},
		{"same organization, another root", branchCandidate{OrganizationID: "org_1", Root: "99888777"}, false},
		{"no root at all (a CPF)", branchCandidate{OrganizationID: "org_1", Root: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBranchSibling(mine, tc.other); got != tc.want {
				t.Errorf("isBranchSibling = %v, want %v", got, tc.want)
			}
		})
	}
}

// A company with no root has no siblings, whichever side it is on.
func TestACompanyWithoutARootHasNoSiblings(t *testing.T) {
	cpf := branchCandidate{OrganizationID: "org_1", Root: ""}
	if isBranchSibling(cpf, branchCandidate{OrganizationID: "org_1", Root: "11222333"}) {
		t.Error("a CPF company matched a CNPJ sibling")
	}
}

// Before the migration nothing carries an organization id, and the old key was
// globally unique per CNPJ — so one root meant one company and the scope check
// has nothing to add. Requiring it would break matriz/filial reuse for every
// existing customer on the day this ships.
func TestTheLegacyEraMatchesOnTheRootAlone(t *testing.T) {
	mine := branchCandidate{Root: "11222333"}
	if !isBranchSibling(mine, branchCandidate{Root: "11222333"}) {
		t.Error("two legacy siblings did not match")
	}
	if isBranchSibling(mine, branchCandidate{Root: "99888777"}) {
		t.Error("two legacy companies with different roots matched")
	}
}

// A half-migrated pair — one side re-keyed, the other not — must not match.
// Under a global key that pair cannot exist; if it appears, the migration is
// mid-flight and inheriting a certificate across that boundary is a guess.
func TestAHalfMigratedPairDoesNotMatch(t *testing.T) {
	migrated := branchCandidate{OrganizationID: "org_1", Root: "11222333"}
	legacy := branchCandidate{Root: "11222333"}
	if isBranchSibling(migrated, legacy) {
		t.Error("a migrated company matched an unmigrated one")
	}
	if isBranchSibling(legacy, migrated) {
		t.Error("an unmigrated company matched a migrated one")
	}
}

// The raiz of a typed document, which is what creation has — there is no record
// yet, and no key either.
func TestTypedRootReadsTheDocument(t *testing.T) {
	if got := typedRoot("11.222.333/0001-81"); got != "11222333" {
		t.Errorf("typedRoot = %q, want 11222333", got)
	}
	// A CNPJ is alphanumeric in its first twelve positions since 2026.
	if got := typedRoot("12.ABC.345/01DE-35"); got != "12ABC345" {
		t.Errorf("typedRoot = %q, want 12ABC345", got)
	}
	// Lowercase is what a person types; the root must not depend on the case.
	if got := typedRoot("12abc34501de35"); got != "12ABC345" {
		t.Errorf("typedRoot = %q, want 12ABC345", got)
	}
	// A CPF has no branch concept.
	if got := typedRoot("529.982.247-25"); got != "" {
		t.Errorf("typedRoot = %q, want empty for a CPF", got)
	}
	if got := typedRoot(""); got != "" {
		t.Errorf("typedRoot = %q, want empty", got)
	}
}
