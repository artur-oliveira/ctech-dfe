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
