//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
)

func orgFields(cnpj, name string) map[string]types.AttributeValue {
	item := map[string]any{
		"name":        name,
		"description": "org de teste",
	}
	av, _ := attributevalue.MarshalMap(item)
	return av
}

func unmarshalOrgItem(item map[string]types.AttributeValue) (map[string]any, error) {
	var out map[string]any
	return out, attributevalue.UnmarshalMap(item, &out)
}

func TestOrganization_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()

	created, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Empresa Criada"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pk, _ := created["pk"].(*types.AttributeValueMemberS)
	if pk == nil || pk.Value != "CNPJ_"+cnpj {
		t.Errorf("pk = %v, want CNPJ_%s", created["pk"], cnpj)
	}

	got, err := orgSvc.Get(ctx, "CNPJ_"+cnpj)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing org")
	}
	namAV, _ := got["name"].(*types.AttributeValueMemberS)
	if namAV == nil || namAV.Value != "Empresa Criada" {
		t.Errorf("name = %v, want Empresa Criada", got["name"])
	}
}

func TestOrganization_CreateIdempotent(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	fields := orgFields(cnpj, "Org Idem")

	_, err := orgSvc.Create(ctx, cnpj, fields)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// second call with same cnpj must return existing without error
	_, err = orgSvc.Create(ctx, cnpj, fields)
	if err != nil {
		t.Errorf("second Create (idempotent): unexpected error: %v", err)
	}
}

func TestOrganization_GetNotFound(t *testing.T) {
	ctx := context.Background()
	got, err := orgSvc.Get(ctx, "CNPJ_00000000000000")
	if err != nil {
		t.Fatalf("Get unknown: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown org, got %v", got)
	}
}

func TestOrganization_CacheHit(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()

	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Cache Test")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// first Get populates cache
	_, _ = orgSvc.Get(ctx, "CNPJ_"+cnpj)
	// second Get should hit cache (no error expected)
	got, err := orgSvc.Get(ctx, "CNPJ_"+cnpj)
	if err != nil {
		t.Fatalf("Get (cached): %v", err)
	}
	if got == nil {
		t.Error("cached Get returned nil")
	}
}

func TestOrganization_Update(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj

	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Before Update")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := orgSvc.Update(ctx, orgPK, map[string]any{"name": "After Update"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	namAV, _ := updated["name"].(*types.AttributeValueMemberS)
	if namAV == nil || namAV.Value != "After Update" {
		t.Errorf("name after update = %v, want After Update", updated["name"])
	}

	// cache invalidated — direct Get should return updated value
	got, err := orgSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	namAV2, _ := got["name"].(*types.AttributeValueMemberS)
	if namAV2 == nil || namAV2.Value != "After Update" {
		t.Errorf("Get after update name = %v, want After Update", got["name"])
	}
}

func TestOrganization_UpdateEmptyStillSucceeds(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj

	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "No Change")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// An empty updates map still runs the transaction (updated_at write +
	// audit row with empty modifications) — it does not skip the DB write.
	_, err := orgSvc.Update(ctx, orgPK, map[string]any{}, "test-user", "Test User")
	if err != nil {
		t.Errorf("Update empty changes: unexpected error: %v", err)
	}
}

func TestOrganization_AddAuthorizedViewer(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj
	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Autorizados")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := orgSvc.AddAuthorizedViewer(ctx, orgPK,
		services.AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "Contador"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("AddAuthorizedViewer: %v", err)
	}
	plain, err := unmarshalOrgItem(updated)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	viewers, _ := plain["authorized_xml_viewers"].([]any)
	if len(viewers) != 1 {
		t.Fatalf("expected 1 authorized viewer, got %+v", viewers)
	}
}

func TestOrganization_AddAuthorizedViewer_DuplicateReturns409(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj
	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Dup Test")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	entry := services.AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "Contador"}
	if _, err := orgSvc.AddAuthorizedViewer(ctx, orgPK, entry, "test-user", "Test User"); err != nil {
		t.Fatalf("first AddAuthorizedViewer: %v", err)
	}
	_, err := orgSvc.AddAuthorizedViewer(ctx, orgPK, entry, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error for duplicate cpf_cnpj")
	}
	if problemStatus(err) != 409 {
		t.Errorf("expected 409, got %d: %v", problemStatus(err), err)
	}
}

func TestOrganization_AddAuthorizedViewer_EleventhReturns400(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj
	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Limit Test")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for i := 0; i < 10; i++ {
		entry := services.AuthorizedViewerEntry{CpfOrCnpj: fmt.Sprintf("%011d", i), Name: "V"}
		if _, err := orgSvc.AddAuthorizedViewer(ctx, orgPK, entry, "test-user", "Test User"); err != nil {
			t.Fatalf("AddAuthorizedViewer #%d: %v", i, err)
		}
	}
	_, err := orgSvc.AddAuthorizedViewer(ctx, orgPK,
		services.AuthorizedViewerEntry{CpfOrCnpj: "99999999999", Name: "Overflow"}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error at 11th entry")
	}
	if problemStatus(err) != 400 {
		t.Errorf("expected 400, got %d: %v", problemStatus(err), err)
	}
}

func TestOrganization_RemoveAuthorizedViewer(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj
	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Remove Test")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	entry := services.AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "Contador"}
	if _, err := orgSvc.AddAuthorizedViewer(ctx, orgPK, entry, "test-user", "Test User"); err != nil {
		t.Fatalf("AddAuthorizedViewer: %v", err)
	}

	updated, err := orgSvc.RemoveAuthorizedViewer(ctx, orgPK, "11122233344", "test-user", "Test User")
	if err != nil {
		t.Fatalf("RemoveAuthorizedViewer: %v", err)
	}
	plain, err := unmarshalOrgItem(updated)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	viewers, _ := plain["authorized_xml_viewers"].([]any)
	if len(viewers) != 0 {
		t.Fatalf("expected 0 authorized viewers after removal, got %+v", viewers)
	}
}
