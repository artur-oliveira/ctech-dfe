package accountclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// A company whose workspace is a personal space is not one this product may
// issue for. ctech-account refuses to put a company in a space; this is the
// check that holds if that refusal ever regresses
// (docs/specs/2026-10-09-personal-workspaces-in-dfe.md).
func TestReachRefusesACompanyInASpace(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":true,"organization_id":"org_1","organization_kind":"personal"}`)
	orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
	if err != nil || ok || orgID != "" {
		t.Fatalf("got %q %v %v, want a plain refusal", orgID, ok, err)
	}
}

// An unknown kind is not consent either.
func TestReachRefusesAnUnknownKind(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":true,"organization_id":"org_1","organization_kind":"household"}`)
	if _, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1"); err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a plain refusal", ok, err)
	}
}

// Before ctech-account sends the field, every workspace is an organization —
// today's behaviour, unchanged.
func TestReachWithNoKindIsAnOrganization(t *testing.T) {
	for _, body := range []string{
		`{"may_act":true,"organization_id":"org_1"}`,
		`{"may_act":true,"organization_id":"org_1","organization_kind":"organization"}`,
	} {
		c := serving(t, http.StatusOK, body)
		orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
		if err != nil || !ok || orgID != "org_1" {
			t.Fatalf("%s: got %q %v %v, want a grant", body, orgID, ok, err)
		}
	}
}

func TestIdentityRefusesACompanyInASpace(t *testing.T) {
	c := serving(t, http.StatusOK, `{"organization_id":"org_1","organization_kind":"personal","tax_id":"11222333000181","tax_id_kind":"cnpj","legal_name":"Acme"}`)
	if _, err := c.companyWithToken(context.Background(), "tok", "org_1", "cmp_1"); !errors.Is(err, ErrNotAnOrganization) {
		t.Fatalf("err = %v, want ErrNotAnOrganization", err)
	}
}

func TestIdentityWithNoKindIsAnOrganization(t *testing.T) {
	c := serving(t, http.StatusOK, `{"organization_id":"org_1","tax_id":"11222333000181","tax_id_kind":"cnpj","legal_name":"Acme"}`)
	ident, err := c.companyWithToken(context.Background(), "tok", "org_1", "cmp_1")
	if err != nil || ident.LegalName != "Acme" {
		t.Fatalf("got %+v %v", ident, err)
	}
}
