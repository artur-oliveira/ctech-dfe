//go:build integration

package integration_test

// Service business-logic integration tests.
//
// Each test creates a fresh MemoryBackend so cache state from other tests
// never leaks in. Direct DynamoDB mutations (via `db`) are used to verify
// that the cache is actually serving reads and is properly invalidated.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func freshOrgSvc() *services.OrganizationService {
	c := cache.NewMemoryBackend(100)
	ms := services.NewMembershipService(orgUserRepo, auditRepo, roleRepo, c)
	return services.NewOrganizationService(orgRepo, auditRepo, certRepo, orgUserRepo, certSvc, ms, c)
}

func freshProductSvc() *services.ProductService {
	return services.NewProductService(productRepo, auditRepo, cache.NewMemoryBackend(100))
}

func freshPersonSvc() *services.PersonService {
	return services.NewPersonService(personRepo, auditRepo, cache.NewMemoryBackend(100))
}

func freshVehicleSvc() *services.VehicleService {
	return services.NewVehicleService(vehicleRepo, auditRepo, cache.NewMemoryBackend(100))
}

// deleteOrgDirect bypasses the service to remove an org from DynamoDB,
// allowing cache-hit tests without cache eviction.
func deleteOrgDirect(t *testing.T, orgPK string) {
	t.Helper()
	_, err := db.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tablePrefix + "_organizations"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
	})
	if err != nil {
		t.Fatalf("deleteOrgDirect: %v", err)
	}
}

func deleteProductDirect(t *testing.T, orgPK, sk string) {
	t.Helper()
	_, err := db.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tablePrefix + "_organization_products"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		t.Fatalf("deleteProductDirect: %v", err)
	}
}

func deleteVehicleDirect(t *testing.T, orgPK, sk string) {
	t.Helper()
	_, err := db.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tablePrefix + "_organization_vehicles"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		t.Fatalf("deleteVehicleDirect: %v", err)
	}
}

func problemStatus(err error) int {
	var pe *problem.Problem
	if errors.As(err, &pe) {
		return pe.Status
	}
	return 0
}

// ─── OrganizationService business logic ──────────────────────────────────────

func TestOrgSvc_GetPopulatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshOrgSvc()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj

	fields, _ := attributevalue.MarshalMap(map[string]any{"name": "Cache Test"})
	if _, err := svc.Create(ctx, cnpj, fields); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// First Get — DB hit, cache populated.
	first, err := svc.Get(ctx, orgPK)
	if err != nil || first == nil {
		t.Fatalf("first Get: err=%v item=%v", err, first)
	}

	// Delete directly — bypasses service, no cache eviction.
	deleteOrgDirect(t, orgPK)

	// Second Get — must return cached value, not nil.
	second, err := svc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("second Get (should hit cache): %v", err)
	}
	if second == nil {
		t.Error("second Get returned nil — cache was not used")
	}
}

func TestOrgSvc_CreateIdempotent_SecondCallHitsCache(t *testing.T) {
	ctx := context.Background()
	svc := freshOrgSvc()
	cnpj := randomCNPJ()

	fields, _ := attributevalue.MarshalMap(map[string]any{"name": "Idempotent"})
	first, err := svc.Create(ctx, cnpj, fields)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Second Create must return the same item (no conflict error).
	second, err := svc.Create(ctx, cnpj, fields)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	firstPK, _ := first["pk"].(*types.AttributeValueMemberS)
	secondPK, _ := second["pk"].(*types.AttributeValueMemberS)
	if firstPK.Value != secondPK.Value {
		t.Errorf("idempotent Create returned different pk: %q vs %q", firstPK.Value, secondPK.Value)
	}
}

func TestOrgSvc_UpdateInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshOrgSvc()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj

	fields, _ := attributevalue.MarshalMap(map[string]any{"name": "Before"})
	if _, err := svc.Create(ctx, cnpj, fields); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK); err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Update via service — must invalidate "org:{orgPK}".
	if _, err := svc.Update(ctx, orgPK, map[string]any{"name": "After"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	nameAV, _ := got["name"].(*types.AttributeValueMemberS)
	if nameAV == nil || nameAV.Value != "After" {
		t.Errorf("Get after update returned stale value: name=%v", got["name"])
	}
}

func TestOrgSvc_GetReturnsNilForMissing(t *testing.T) {
	ctx := context.Background()
	svc := freshOrgSvc()

	got, err := svc.Get(ctx, "CNPJ_99999999999999")
	if err != nil {
		t.Fatalf("Get missing: %v", err)
	}
	if got != nil {
		t.Error("Get missing org: expected nil, got item")
	}
}

func TestOrgSvc_UpdateEmptyChangesNoError(t *testing.T) {
	ctx := context.Background()
	svc := freshOrgSvc()
	cnpj := randomCNPJ()

	fields, _ := attributevalue.MarshalMap(map[string]any{"name": "Empty Update"})
	if _, err := svc.Create(ctx, cnpj, fields); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := svc.Update(ctx, "CNPJ_"+cnpj, map[string]any{}, "test-user", "Test User")
	if err != nil {
		t.Errorf("Update with empty changes returned error: %v", err)
	}
}

// ─── ProductService business logic ───────────────────────────────────────────

func TestProductSvc_GetReturns404ForMissing(t *testing.T) {
	ctx := context.Background()
	svc := freshProductSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := svc.Get(ctx, orgPK, "PRODUCT_nonexistent")
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected status 404, got %d: %v", problemStatus(err), err)
	}
}

func TestProductSvc_GetPopulatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshProductSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, productFields("CACHEP2", "Cache Product 2"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	// First Get populates cache.
	if _, err := svc.Get(ctx, orgPK, sk); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Delete directly — no cache eviction.
	deleteProductDirect(t, orgPK, sk)

	// Second Get must serve cached value.
	second, err := svc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("second Get (should hit cache): %v", err)
	}
	if second == nil {
		t.Error("cache was not used — second Get returned nil after direct delete")
	}
}

func TestProductSvc_UpdateInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshProductSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, productFields("UPD002", "Before"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK, sk); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, err := svc.Update(ctx, orgPK, sk, map[string]any{"description": "After"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	descAV, _ := got["description"].(*types.AttributeValueMemberS)
	if descAV == nil || descAV.Value != "After" {
		t.Errorf("Get after update returned stale: description=%v", got["description"])
	}
}

func TestProductSvc_DeleteInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshProductSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, productFields("DEL002", "Delete Cache"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK, sk); err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Delete via service — must evict cache.
	if err := svc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get must hit DB (cache evicted) → 404.
	_, err = svc.Get(ctx, orgPK, sk)
	if err == nil {
		t.Fatal("expected 404 after delete, got nil error")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(err), err)
	}
}

// ─── VehicleService business logic ───────────────────────────────────────────

func TestVehicleSvc_GetReturns404ForMissing(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := svc.Get(ctx, orgPK, "VEHICLE_nonexistent")
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(err), err)
	}
}

func TestVehicleSvc_GetPopulatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, map[string]any{
		"plate":    "TST1234",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   5000,
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	// First Get populates cache.
	if _, err := svc.Get(ctx, orgPK, sk); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Delete directly — no cache eviction.
	deleteVehicleDirect(t, orgPK, sk)

	// Second Get must hit cache.
	second, err := svc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("second Get (should hit cache): %v", err)
	}
	if second == nil {
		t.Error("cache not used — second Get returned nil")
	}
}

func TestVehicleSvc_DeleteInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, map[string]any{
		"plate":    "DEL1234",
		"plate_uf": "RJ",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   4000,
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK, sk); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if err := svc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = svc.Get(ctx, orgPK, sk)
	if err == nil {
		t.Fatal("expected 404 after delete, got nil error")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(err), err)
	}
}

func TestVehicleSvc_InvalidOwnerTypeReturnsError(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := svc.Create(ctx, orgPK, map[string]any{
		"plate":      "ABC1234",
		"plate_uf":   "SP",
		"wheelset":   "03",
		"bodywork":   "02",
		"owner_type": "INVALID",
		"owner":      validOwner,
	}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error for invalid owner_type, got nil")
	}
	if problemStatus(err) != 400 {
		t.Errorf("expected 400, got %d: %v", problemStatus(err), err)
	}
}

func TestVehicleSvc_UpdateValidatesPlate(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, map[string]any{
		"plate":    "ABC1234",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   5000,
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	_, err = svc.Update(ctx, orgPK, sk, map[string]any{"plate": "BADPLATE"}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error updating with invalid plate, got nil")
	}
	if problemStatus(err) != 400 {
		t.Errorf("expected 400, got %d: %v", problemStatus(err), err)
	}
}

func TestVehicleSvc_UpdateValidatesRenavam(t *testing.T) {
	ctx := context.Background()
	svc := freshVehicleSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := svc.Create(ctx, orgPK, map[string]any{
		"plate":    "ABC5678",
		"plate_uf": "MG",
		"wheelset": "03",
		"bodywork": "02",
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	_, err = svc.Update(ctx, orgPK, sk, map[string]any{"renavam": "123"}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error updating with invalid renavam, got nil")
	}
	if problemStatus(err) != 400 {
		t.Errorf("expected 400, got %d: %v", problemStatus(err), err)
	}
}

// ─── PersonService business logic ────────────────────────────────────────────

func TestPersonSvc_GetPopulatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshPersonSvc()
	orgPK := "CNPJ_" + randomCNPJ()
	cnpj := randomCNPJ()

	if _, err := svc.Create(ctx, orgPK, cnpj, map[string]any{"name": "Pessoa Cache"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// First Get populates cache.
	first, err := svc.Get(ctx, orgPK, cnpj)
	if err != nil || first == nil {
		t.Fatalf("first Get: err=%v item=%v", err, first)
	}

	// Delete directly.
	sk := "CNPJ_" + cnpj
	_, err = db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tablePrefix + "_organization_persons"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		t.Fatalf("deletePersonDirect: %v", err)
	}

	// Second Get must hit cache.
	second, err := svc.Get(ctx, orgPK, cnpj)
	if err != nil {
		t.Fatalf("second Get (should hit cache): %v", err)
	}
	if second == nil {
		t.Error("cache not used — second Get returned nil")
	}
}

func TestPersonSvc_UpdateInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshPersonSvc()
	orgPK := "CNPJ_" + randomCNPJ()
	cpf := "52998224725"

	if _, err := svc.Create(ctx, orgPK, cpf, map[string]any{"name": "Before"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK, cpf); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, err := svc.Update(ctx, orgPK, cpf, map[string]any{"name": "After"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.Get(ctx, orgPK, cpf)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	nameAV, _ := got["name"].(*types.AttributeValueMemberS)
	if nameAV == nil || nameAV.Value != "After" {
		t.Errorf("stale cache after update: name=%v", got["name"])
	}
}

func TestPersonSvc_DeleteInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	svc := freshPersonSvc()
	orgPK := "CNPJ_" + randomCNPJ()
	cpf := "04998224726"

	if _, err := svc.Create(ctx, orgPK, cpf, map[string]any{"name": "Delete Cache"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Populate cache.
	if _, err := svc.Get(ctx, orgPK, cpf); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if err := svc.Delete(ctx, orgPK, cpf, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get hits DB (cache evicted) → 404, not cached stale value.
	_, getErr := svc.Get(ctx, orgPK, cpf)
	if getErr == nil {
		t.Fatal("expected 404 after delete, got nil error")
	}
	if problemStatus(getErr) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(getErr), getErr)
	}
}

func TestPersonSvc_GetReturns404ForMissing(t *testing.T) {
	ctx := context.Background()
	svc := freshPersonSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := svc.Get(ctx, orgPK, "52998224725")
	if err == nil {
		t.Fatal("expected NotFound error for unknown person, got nil")
	}
	if problemStatus(err) != 404 {
		t.Errorf("expected 404, got %d: %v", problemStatus(err), err)
	}
}

func TestPersonSvc_InvalidCPFReturnsError(t *testing.T) {
	ctx := context.Background()
	svc := freshPersonSvc()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := svc.Create(ctx, orgPK, "123", map[string]any{"name": "Bad"}, "test-user", "Test User")
	if err == nil {
		t.Fatal("expected error for invalid CPF, got nil")
	}
	if problemStatus(err) != 400 {
		t.Errorf("expected 400, got %d: %v", problemStatus(err), err)
	}
}
