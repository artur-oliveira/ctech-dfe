//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPerson_BuildSKCPF(t *testing.T) {
	// pure function — here as a smoke test alongside integration setup
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := personSvc.Create(ctx, orgPK, "52998224725", map[string]any{"name": "João"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create CPF: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	if skAV == nil || skAV.Value != "CPF_52998224725" {
		t.Errorf("sk = %v, want CPF_52998224725", item["sk"])
	}
}

func TestPerson_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	cnpj := randomCNPJ()

	created, err := personSvc.Create(ctx, orgPK, cnpj, map[string]any{"name": "Empresa Parceira"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil {
		t.Fatal("Create returned nil")
	}

	got, err := personSvc.Get(ctx, orgPK, cnpj)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing person")
	}
	nameAV, _ := got["name"].(*types.AttributeValueMemberS)
	if nameAV == nil || nameAV.Value != "Empresa Parceira" {
		t.Errorf("name = %v, want Empresa Parceira", got["name"])
	}
}

func TestPerson_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := personSvc.Get(ctx, orgPK, "52998224725")
	if err == nil {
		t.Fatal("expected NotFound error for unknown person, got nil")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(err), err)
	}
}

func TestPerson_Update(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	cpf := "52998224725"

	if _, err := personSvc.Create(ctx, orgPK, cpf, map[string]any{"name": "Before"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := personSvc.Update(ctx, orgPK, cpf, map[string]any{"name": "After"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	nameAV, _ := updated["name"].(*types.AttributeValueMemberS)
	if nameAV == nil || nameAV.Value != "After" {
		t.Errorf("name after update = %v, want After", updated["name"])
	}
}

func TestPerson_Delete(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	cpf := "04998224725" // different from other tests

	if _, err := personSvc.Create(ctx, orgPK, cpf, map[string]any{"name": "Delete Me"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := personSvc.Delete(ctx, orgPK, cpf, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, getErr := personSvc.Get(ctx, orgPK, cpf)
	if problemStatus(getErr) != 404 {
		t.Errorf("expected 404 after delete, got status %d: %v", problemStatus(getErr), getErr)
	}
}

func TestPerson_CreateDuplicateCpfCnpj_Returns409(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	cpf := "52998224725"

	if _, err := personSvc.Create(ctx, orgPK, cpf, map[string]any{"name": "First"}, "test-user", "Test User"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := personSvc.Create(ctx, orgPK, cpf, map[string]any{"name": "Duplicate"}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error for duplicate CPF/CNPJ, got nil")
	}
	if problemStatus(err) != 409 {
		t.Errorf("expected 409, got %d: %v", problemStatus(err), err)
	}
}

func TestPerson_CreateDuplicateCpfCnpj_DifferentOrgsAllowed(t *testing.T) {
	ctx := context.Background()
	cpf := "04998224725" // distinct from other tests to avoid cross-test collisions
	orgA := "CNPJ_" + randomCNPJ()
	orgB := "CNPJ_" + randomCNPJ()

	if _, err := personSvc.Create(ctx, orgA, cpf, map[string]any{"name": "Org A"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create in org A: %v", err)
	}
	if _, err := personSvc.Create(ctx, orgB, cpf, map[string]any{"name": "Org B"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create in org B (same CPF, different org): unexpected error: %v", err)
	}
}

func TestPerson_InvalidCPFReturnsError(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := personSvc.Create(ctx, orgPK, "123", map[string]any{"name": "Bad CPF"}, "test-user", "Test User")
	if err == nil {
		t.Error("expected error for invalid CPF/CNPJ, got nil")
	}
}
