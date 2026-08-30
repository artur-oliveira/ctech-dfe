package middleware

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// A real company id. The legacy branch keys off the KEY'S SHAPE, so an
// invented id like "cmp_1" silently takes the pre-migration path — every
// reach test would pass for the wrong reason.
const companyKey = "01a04fc3-baa2-7cae-ac62-0ca3260a5888"

type fakeReach struct {
	orgID string
	ok    bool
	err   error
	calls int
}

func (f *fakeReach) MayAct(context.Context, string, string) (string, bool, error) {
	f.calls++
	return f.orgID, f.ok, f.err
}

// The invariant the whole unification is for. A product row that survived a
// revoked edge must grant nothing, or the unification bought nothing.
func TestARowWithNoEdgeGrantsNothing(t *testing.T) {
	row := &services.Membership{OrgPK: companyKey, UserID: "usr_1", Role: "OWNER"}
	if _, err := authorize(context.Background(), &fakeReach{ok: false}, companyKey, "usr_1", row); err == nil {
		t.Fatal("an OWNER row with no edge was allowed")
	}
}

// An edge with no row grants reach and no verbs: somebody invited to a company
// nobody has given a role in yet. Not an error — a state the invitation flow
// produces on purpose — so the membership comes back nil and the permission
// check refuses on its own.
func TestAnEdgeWithNoRowGrantsNoVerbs(t *testing.T) {
	m, err := authorize(context.Background(), &fakeReach{orgID: "org_1", ok: true}, companyKey, "usr_1", nil)
	if err != nil {
		t.Fatalf("an edge with no row errored: %v", err)
	}
	if m != nil {
		t.Fatalf("got a membership from nothing: %+v", m)
	}
}

// An outage refuses. Reach is an authorization check: a dependency we cannot
// reach must never read as permission.
func TestAnOutageRefuses(t *testing.T) {
	row := &services.Membership{OrgPK: companyKey, Role: "OWNER"}
	if _, err := authorize(context.Background(), &fakeReach{err: errors.New("timeout")}, companyKey, "usr_1", row); err == nil {
		t.Fatal("an outage was allowed")
	}
}

// Both refusals must look identical from outside, or the API becomes a probe:
// "no edge" and "no row" would tell a stranger which company ids are real and
// which people are in them.
func TestEveryRefusalLooksTheSame(t *testing.T) {
	noEdge, _ := authorize(context.Background(), &fakeReach{ok: false}, companyKey, "usr_1",
		&services.Membership{OrgPK: companyKey, Role: "OWNER"})
	_ = noEdge

	a := refusalOf(t, &fakeReach{ok: false}, &services.Membership{OrgPK: companyKey, Role: "OWNER"})
	b := refusalOf(t, &fakeReach{err: errors.New("timeout")}, &services.Membership{OrgPK: companyKey, Role: "OWNER"})
	if a != b {
		t.Fatalf("distinguishable refusals:\n  no edge: %s\n  outage:  %s", a, b)
	}
}

func refusalOf(t *testing.T, r reachChecker, row *services.Membership) string {
	t.Helper()
	_, err := authorize(context.Background(), r, companyKey, "usr_1", row)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	return err.Detail
}

// The happy path, so the refusals above are refusals and not a broken function.
func TestAnEdgeAndARowGrantTheRow(t *testing.T) {
	row := &services.Membership{OrgPK: companyKey, UserID: "usr_1", Role: "ADMIN"}
	got, err := authorize(context.Background(), &fakeReach{orgID: "org_1", ok: true}, companyKey, "usr_1", row)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if got == nil || got.Role != "ADMIN" {
		t.Fatalf("got %+v, want the product's own row", got)
	}
}

// A legacy CNPJ_ key skips the edge entirely: nothing has been migrated for it,
// there is no company id to ask about, and asking would refuse every existing
// customer on the day this ships.
func TestALegacyKeySkipsTheEdge(t *testing.T) {
	r := &fakeReach{ok: false}
	row := &services.Membership{OrgPK: "CNPJ_11222333000181", Role: "OWNER"}
	got, err := authorize(context.Background(), r, "CNPJ_11222333000181", "usr_1", row)
	if err != nil {
		t.Fatalf("a legacy key was refused: %v", err)
	}
	if got == nil {
		t.Fatal("a legacy key lost its membership")
	}
	if r.calls != 0 {
		t.Errorf("asked about reach for a legacy key %d time(s)", r.calls)
	}
}
