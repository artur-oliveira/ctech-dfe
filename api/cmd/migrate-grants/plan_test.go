package main

import (
	"strings"
	"testing"
)

const (
	cmpA = "01a04fc3-baa2-7cae-ac62-0ca3260a5888"
	usrA = "USER_78fd4d57-88a7-4eec-a997-7d7c09f58a1b"
)

func orgOf(companyID string) map[string]string { return map[string]string{companyID: "org_1"} }

// The ordinary case, and the one the migration is for: a role-only row whose
// person has no edge yet. That is every member before the unification, and the
// edge is the whole point.
func TestARoleOnlyRowGetsItsEdge(t *testing.T) {
	got := plan(
		[]overlay{{CompanyID: cmpA, UserID: usrA, Role: "ADMIN"}},
		map[string]reach{},
		orgOf(cmpA),
	)
	if got[0].NeedsHuman() {
		t.Fatalf("refused: %s", got[0].Review)
	}
	if !got[0].GrantEdge {
		t.Fatal("a role-only row did not get its edge")
	}
}

// The refusal that matters. Granting reach BECAUSE somebody holds permissions is
// circular: the permissions were granted on the assumption they could reach it,
// and that assumption is what this migration is supposed to verify.
func TestARowWithGrantsAndNoEdgeNeedsAHuman(t *testing.T) {
	got := plan(
		[]overlay{{CompanyID: cmpA, UserID: usrA, Role: "USER", Permissions: []string{"emit.nfe", "cancel.nfe"}}},
		map[string]reach{},
		orgOf(cmpA),
	)
	if !got[0].NeedsHuman() {
		t.Fatal("a row with explicit grants and no edge was migrated")
	}
	// The report has to name the permissions, or the person deciding cannot.
	for _, p := range []string{"emit.nfe", "cancel.nfe"} {
		if !strings.Contains(got[0].Review, p) {
			t.Errorf("the review does not name %q: %q", p, got[0].Review)
		}
	}
	if got[0].GrantEdge {
		t.Error("a refusal still proposed writing the edge")
	}
}

// Grants WITH an edge are fine and untouched: the platform already records the
// reach, and the grants are already where ADR 0023 says they belong.
func TestGrantsWithAnEdgeAreLeftAlone(t *testing.T) {
	got := plan(
		[]overlay{{CompanyID: cmpA, UserID: usrA, Role: "USER", Permissions: []string{"emit.nfe"}}},
		map[string]reach{edgeKey(cmpA, usrA): {HasEdge: true, OrganizationID: "org_1"}},
		orgOf(cmpA),
	)
	if got[0].NeedsHuman() {
		t.Fatalf("refused a row whose reach is already recorded: %s", got[0].Review)
	}
	if got[0].GrantEdge {
		t.Error("proposed re-writing an edge that already exists")
	}
}

// An unknown organization refuses rather than guessing: the edge is keyed by
// (organization, company, user), and a guessed organization writes a row nothing
// can read.
func TestAnUnknownOrganizationNeedsAHuman(t *testing.T) {
	got := plan(
		[]overlay{{CompanyID: cmpA, UserID: usrA, Role: "ADMIN"}},
		map[string]reach{},
		map[string]string{},
	)
	if !got[0].NeedsHuman() {
		t.Fatal("an unkeyable edge was migrated")
	}
}

// This tool writes NOTHING to ctech-dfe. The grants are already where they
// belong, so there is nothing to move — only edges to add. A decision proposing
// a product write would mean the tool grew a second job.
func TestTheDecisionOnlyEverProposesAnEdge(t *testing.T) {
	got := plan(
		[]overlay{
			{CompanyID: cmpA, UserID: usrA, Role: "ADMIN"},
			{CompanyID: cmpA, UserID: "USER_other", Role: "USER", Permissions: []string{"emit.nfe"}},
		},
		map[string]reach{},
		orgOf(cmpA),
	)
	for _, d := range got {
		// The struct carries exactly one write, and it is the edge. If a field
		// named after a product table appears here, this test is where the
		// justification has to be written.
		if d.GrantEdge && d.Overlay.HasGrants() {
			t.Errorf("proposed granting an edge for a row with explicit grants: %+v", d)
		}
	}
}

// The report is what somebody reads before allowing writes, so a refusal must be
// impossible to skim past.
func TestTheReportCountsEveryOutcome(t *testing.T) {
	got := report(plan(
		[]overlay{
			{CompanyID: cmpA, UserID: usrA, Role: "ADMIN"},
			{CompanyID: cmpA, UserID: "USER_b", Role: "USER", Permissions: []string{"emit.nfe"}},
			{CompanyID: cmpA, UserID: "USER_c", Role: "OWNER"},
		},
		map[string]reach{edgeKey(cmpA, "USER_c"): {HasEdge: true, OrganizationID: "org_1"}},
		orgOf(cmpA),
	))
	if !strings.Contains(got, "1 edge(s) to grant, 1 already recorded, 1 need a human") {
		t.Fatalf("report = %q", got)
	}
	if !strings.Contains(got, "REVIEW") {
		t.Error("the refusal is not marked")
	}
}

// The key layout is a COPY of ctech-account's, because this tool must not depend
// on that repository's Go module. A copy that drifts writes edges the platform
// cannot find, and the drift would be silent — so it is pinned against a real
// row read from production.
func TestTheEdgeKeyMatchesWhatCtechAccountWrites(t *testing.T) {
	// Verified against prod_account_companies:
	//   pk = ORG#01a04ed6-1d25-7d0d-8985-664f3c0f7811
	//   sk = ACTOR#01a04fc3-baa2-7cae-ac62-0ca3260a5888#78fd4d57-88a7-4eec-a997-7d7c09f58a1b
	const want = "ACTOR#01a04fc3-baa2-7cae-ac62-0ca3260a5888#78fd4d57-88a7-4eec-a997-7d7c09f58a1b"
	got := actorSK("01a04fc3-baa2-7cae-ac62-0ca3260a5888", "USER_78fd4d57-88a7-4eec-a997-7d7c09f58a1b")
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// ctech-dfe keys members USER_{sub}; ctech-account keys edges by the bare
// subject. An edge written with the prefix is a row the platform cannot find,
// and nothing would report it — the write succeeds.
func TestTheUserPrefixIsStrippedForTheEdge(t *testing.T) {
	if got := bareUserID("USER_78fd4d57"); got != "78fd4d57" {
		t.Errorf("got %q, want the bare subject", got)
	}
	// Already bare stays bare: the migration may be re-run against rows some
	// other path wrote.
	if got := bareUserID("78fd4d57"); got != "78fd4d57" {
		t.Errorf("got %q", got)
	}
}
