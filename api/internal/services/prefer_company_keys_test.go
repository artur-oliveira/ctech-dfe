package services

import "testing"

func ms(pks ...string) []Membership {
	out := make([]Membership, 0, len(pks))
	for _, pk := range pks {
		out = append(out, Membership{OrgPK: pk, Role: "OWNER"})
	}
	return out
}

const (
	cmpA = "01a04fc3-baa2-7cae-ac62-0ca3260a5888"
	cmpB = "01a04fc3-b6f7-7bb9-8cfe-6e19b66019f6"
)

// The state during the re-key: the GSI returns both eras, so two organizations
// come back as four. The count matters — ownedOrganizations counts this list and
// the count IS the company quota, so a plan for two would refuse the third at
// "4 of 2".
func TestTheLegacyHalfIsDroppedOnceRekeyed(t *testing.T) {
	got := preferCompanyKeys(ms(cmpA, cmpB, "CNPJ_11647612000197", "CNPJ_62787449000107"))
	if len(got) != 2 {
		t.Fatalf("got %d memberships, want 2: %+v", len(got), got)
	}
	for _, m := range got {
		if !isCompanyKeyForTest(m.OrgPK) {
			t.Errorf("a legacy key survived: %q", m.OrgPK)
		}
	}
}

// Before the re-key nothing is company-keyed, and every membership must survive.
// Dropping here would lock every existing customer out of their own account.
func TestNothingIsDroppedBeforeTheRekey(t *testing.T) {
	in := ms("CNPJ_11647612000197", "CNPJ_62787449000107")
	if got := preferCompanyKeys(in); len(got) != 2 {
		t.Fatalf("got %d, want 2 — a legacy-only account lost access", len(got))
	}
}

// After the deletion this returns its input unchanged, which is what makes the
// function safe to remove with the old rows.
func TestAfterTheDeletionItIsANoOp(t *testing.T) {
	in := ms(cmpA, cmpB)
	got := preferCompanyKeys(in)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

func TestAnEmptyListStaysEmpty(t *testing.T) {
	if got := preferCompanyKeys(nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

// A half-migrated account keeps only what was migrated. All-or-nothing is
// deliberate: pairing per row would need to know which legacy key each company
// id came from, which is the lookup the re-key removed. The cost is visible —
// an organization not yet copied disappears from the list — and that is the
// safer failure: an account showing an organization it can no longer read would
// 403 on every click instead.
func TestAHalfMigratedAccountKeepsOnlyWhatMoved(t *testing.T) {
	got := preferCompanyKeys(ms(cmpA, "CNPJ_11647612000197", "CNPJ_62787449000107"))
	if len(got) != 1 || got[0].OrgPK != cmpA {
		t.Fatalf("got %+v, want only the migrated one", got)
	}
}

func isCompanyKeyForTest(pk string) bool { return len(pk) == 36 }
