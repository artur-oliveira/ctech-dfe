package services

import (
	"errors"
	"testing"
)

const (
	linkOrg     = "01a04ed6-1d25-7d0d-8985-664f3c0f7811"
	linkCompany = "01a04fc3-baa2-7cae-ac62-0ca3260a5888"
)

// A service with no ctech-account credential cannot verify reach OR read
// identity, and linking on trust would let anybody adopt any company id they can
// type. It refuses rather than degrading.
func TestLinkIsDisabledWithoutTheAccountCredential(t *testing.T) {
	if (&LinkService{}).Enabled() {
		t.Fatal("a service with no reach and no identity reported itself enabled")
	}
	if (&LinkService{reach: &ReachService{}}).Enabled() {
		t.Fatal("a service with reach and no identity reported itself enabled")
	}
}

// The ids arrive on a URL the person controls. Without verifying reach against
// ctech-account, anybody could adopt any company id they can guess — the
// handoff's own validation protects the redirect, not this call.
func TestLinkVerifiesReachAndNotTheURL(t *testing.T) {
	if _, prob := checkReach(linkOrg, linkOrg, true, nil); prob != nil {
		t.Fatalf("a matching organization was refused: %v", prob)
	}
	if _, prob := checkReach(linkOrg, linkOrg, false, nil); prob == nil {
		t.Fatal("a refused reach was accepted")
	}
}

// The URL named one organization and the edge says another. Trusting the URL
// would let somebody attach a company they reach to an organization they do not
// — which is how a company ends up in a workspace that never agreed to it.
func TestLinkRefusesACrossOrganizationPair(t *testing.T) {
	other := "01a04ed6-1af9-745e-bcf7-d3b66fe52321"
	if _, prob := checkReach(other, linkOrg, true, nil); prob == nil {
		t.Fatal("a company was linked to an organization its edge does not name")
	}
}

// An unreachable ctech-account refuses. Reach is an authorization check, and a
// dependency we cannot reach must never read as consent.
func TestLinkFailsClosedOnAnOutage(t *testing.T) {
	if _, prob := checkReach(linkOrg, "", false, errors.New("timeout")); prob == nil {
		t.Fatal("an outage was read as permission")
	}
}

// Every refusal reads the same from outside: "no access". A caller must not be
// able to tell a wrong organization from a missing edge, or the endpoint becomes
// a probe for which companies belong where.
func TestEveryLinkRefusalLooksTheSame(t *testing.T) {
	other := "01a04ed6-1af9-745e-bcf7-d3b66fe52321"
	_, noEdge := checkReach(linkOrg, "", false, nil)
	_, crossed := checkReach(other, linkOrg, true, nil)
	if noEdge == nil || crossed == nil {
		t.Fatal("expected two refusals")
	}
	if noEdge.Detail != crossed.Detail {
		t.Fatalf("distinguishable:\n  no edge: %s\n  crossed: %s", noEdge.Detail, crossed.Detail)
	}
}
