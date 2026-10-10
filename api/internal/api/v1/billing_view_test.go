package v1

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestOrganizationViewNamesTheOrganizationAndWhetherTheCallerManagesIt(t *testing.T) {
	view := organizationSubscriptionView(&repositories.AccountSnapshot{
		OrganizationID: "org_1", SubscriptionID: "sub_1", Status: "ACTIVE", Plan: "pro",
	}, "Escritório Silva", true)

	org, ok := view["organization"].(map[string]string)
	if !ok || org["id"] != "org_1" || org["name"] != "Escritório Silva" {
		t.Fatalf("organization = %#v", view["organization"])
	}
	if view["manageable"] != true || view["has_subscription"] != true {
		t.Fatalf("view = %#v", view)
	}
	if view := organizationSubscriptionView(&repositories.AccountSnapshot{OrganizationID: "org_1"}, "", false); view["manageable"] != false {
		t.Fatalf("a member's view must not be manageable: %#v", view)
	}
}

// Review round 2, minor 8: paying is managing, so the open invoice and its
// checkout link reach only the organization's owners and admins.
func TestTheOpenInvoiceIsShownOnlyToManagers(t *testing.T) {
	snap := &repositories.AccountSnapshot{
		OrganizationID: "org_1", SubscriptionID: "sub_1", Status: "PAST_DUE",
		OpenInvoice: &repositories.OpenInvoice{ID: "in_1", TotalCents: 100, CheckoutURL: "https://pay.example/x"},
	}
	if _, has := organizationSubscriptionView(snap, "", false)["open_invoice"]; has {
		t.Fatal("a member must not see the open invoice")
	}
	if _, has := organizationSubscriptionView(snap, "", true)["open_invoice"]; !has {
		t.Fatal("a manager sees the open invoice")
	}
}
