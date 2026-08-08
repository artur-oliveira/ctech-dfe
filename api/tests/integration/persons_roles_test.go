//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func personNames(res *repositories.QueryResult) []string {
	names := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		if av, ok := it["name"].(*types.AttributeValueMemberS); ok {
			names = append(names, av.Value)
		}
	}
	return names
}

func containsName(res *repositories.QueryResult, want string) bool {
	for _, n := range personNames(res) {
		if n == want {
			return true
		}
	}
	return false
}

func countName(res *repositories.QueryResult, want string) int {
	n := 0
	for _, got := range personNames(res) {
		if got == want {
			n++
		}
	}
	return n
}

// A person holding several roles at once is the normal case, not the
// exception: it must show up in every one of its role listings and exactly
// once in the unfiltered listing.
func TestPersonList_MultiRolePersonAppearsInEveryRoleListing(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	acme := randomCNPJ()
	if _, err := personSvc.Create(ctx, orgPK, acme, map[string]any{
		"name":  "Transportes Acme",
		"roles": []string{services.RoleCustomer, services.RoleCarrier},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create multi-role person: %v", err)
	}
	if _, err := personSvc.Create(ctx, orgPK, randomCNPJ(), map[string]any{
		"name":  "Cliente Simples",
		"roles": []string{services.RoleCustomer},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create single-role person: %v", err)
	}
	// Pre-roles person: valid, and absent from every role listing.
	if _, err := personSvc.Create(ctx, orgPK, randomCNPJ(), map[string]any{
		"name": "Legado Sem Papel",
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create legacy person: %v", err)
	}

	for _, role := range []string{services.RoleCustomer, services.RoleCarrier} {
		res, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{Role: role, Limit: 50})
		if err != nil {
			t.Fatalf("List role=%s: %v", role, err)
		}
		if !containsName(res, "Transportes Acme") {
			t.Errorf("role=%s: multi-role person missing, got %v", role, personNames(res))
		}
		if containsName(res, "Legado Sem Papel") {
			t.Errorf("role=%s: person without roles must not appear", role)
		}
	}

	// Only the carrier role — the customer-only person must be filtered out.
	carriers, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{Role: services.RoleCarrier, Limit: 50})
	if err != nil {
		t.Fatalf("List role=carrier: %v", err)
	}
	if containsName(carriers, "Cliente Simples") {
		t.Errorf("role=carrier returned a customer-only person: %v", personNames(carriers))
	}

	all, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List unfiltered: %v", err)
	}
	if n := countName(all, "Transportes Acme"); n != 1 {
		t.Errorf("multi-role person appears %d times in the unfiltered listing, want 1", n)
	}
	if len(all.Items) != 3 {
		t.Errorf("unfiltered listing = %d items, want 3", len(all.Items))
	}
}

// A filtered query's Limit counts items *read*, so the page can come back
// short. The service pages until the requested page is full, and end-of-list is
// an absent cursor — never a short page.
func TestPersonList_FilteredPaginationFillsThePage(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	// 4 drivers scattered among 16 non-drivers: any single DynamoDB page of
	// size 5 reads mostly non-matching rows.
	const drivers = 4
	for i := 0; i < 20; i++ {
		roles := []string{services.RoleCustomer}
		if i%5 == 0 {
			roles = []string{services.RoleCustomer, services.RoleDriver}
		}
		if _, err := personSvc.Create(ctx, orgPK, randomCNPJ(), map[string]any{
			"name":  fmt.Sprintf("Pessoa %02d", i),
			"roles": roles,
		}, "test-user", "Test User"); err != nil {
			t.Fatalf("Create person %d: %v", i, err)
		}
	}

	seen := 0
	var start map[string]types.AttributeValue
	for page := 0; page < services.MaxFilteredPageRoundTrips*4; page++ {
		res, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{
			Role: services.RoleDriver, Limit: 3, StartKey: start,
		})
		if err != nil {
			t.Fatalf("List page %d: %v", page, err)
		}
		seen += len(res.Items)
		if res.LastEvaluatedKey == nil {
			break // the only legitimate end-of-list signal
		}
		start = res.LastEvaluatedKey
	}
	if seen != drivers {
		t.Errorf("paginated role=driver returned %d persons, want %d", seen, drivers)
	}
}

// Digits in ?q= search the document, which is the SK itself — and the search
// must span both the CNPJ_ and CPF_ prefixes.
func TestPersonList_DocumentSearchSpansBothDocumentPrefixes(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := personSvc.Create(ctx, orgPK, "11222333000181", map[string]any{
		"name": "PJ Um", "roles": []string{services.RoleCarrier},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create PJ: %v", err)
	}
	if _, err := personSvc.Create(ctx, orgPK, "11222333044", map[string]any{
		"name": "PF Um", "roles": []string{services.RoleCarrier},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create PF: %v", err)
	}

	res, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{
		Role: services.RoleCarrier, Q: "112223330", Limit: 50,
	})
	if err != nil {
		t.Fatalf("List q=digits: %v", err)
	}
	if !containsName(res, "PJ Um") || !containsName(res, "PF Um") {
		t.Errorf("document search must reach CNPJ_ and CPF_ rows, got %v", personNames(res))
	}
}

// Name search keeps working, with and without a role filter.
func TestPersonList_NameSearchWithRole(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := personSvc.Create(ctx, orgPK, randomCNPJ(), map[string]any{
		"name": "João Motorista", "roles": []string{services.RoleDriver},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create driver: %v", err)
	}
	if _, err := personSvc.Create(ctx, orgPK, randomCNPJ(), map[string]any{
		"name": "João Cliente", "roles": []string{services.RoleCustomer},
	}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create customer: %v", err)
	}

	res, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{
		Role: services.RoleDriver, Q: "Jo", Limit: 50,
	})
	if err != nil {
		t.Fatalf("List q=Jo role=driver: %v", err)
	}
	if !containsName(res, "João Motorista") || containsName(res, "João Cliente") {
		t.Errorf("q=Jo&role=driver returned %v", personNames(res))
	}

	// Legacy ?name= path, no role: unchanged.
	legacy, err := personSvc.List(ctx, orgPK, repositories.PersonListOpts{NamePrefix: "Jo", Limit: 50})
	if err != nil {
		t.Fatalf("List name=Jo: %v", err)
	}
	if len(legacy.Items) != 2 {
		t.Errorf("name=Jo returned %d items, want 2 — legacy path must not regress", len(legacy.Items))
	}
}
