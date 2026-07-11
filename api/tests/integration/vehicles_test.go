//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
)

var validOwner = map[string]any{
	"cpf_cnpj": "52998224725",
	"rntrc":    "12345678",
	"name":     "Proprietário",
	"type":     "TAC",
}

func TestVehicle_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	fields := map[string]any{
		"plate":    "ABC1D23",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   12000,
		"owner":    validOwner,
	}
	item, err := vehicleSvc.Create(ctx, orgPK, fields, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	if skAV == nil {
		t.Fatal("created vehicle has no sk")
	}
	sk := skAV.Value

	got, err := vehicleSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing vehicle")
	}
	plateAV, _ := got["plate"].(*types.AttributeValueMemberS)
	if plateAV == nil || plateAV.Value != "ABC1D23" {
		t.Errorf("plate = %v, want ABC1D23", got["plate"])
	}
}

func TestVehicle_InvalidPlateReturnsError(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	fields := map[string]any{
		"plate":    "BADPLATE",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"owner":    validOwner,
	}
	_, err := vehicleSvc.Create(ctx, orgPK, fields, "test-user", "Test User")
	if err == nil {
		t.Error("expected error for invalid plate, got nil")
	}
}

func TestVehicle_InvalidRenavamReturnsError(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	fields := map[string]any{
		"plate":    "ABC1D23",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123",
		"owner":    validOwner,
	}
	_, err := vehicleSvc.Create(ctx, orgPK, fields, "test-user", "Test User")
	if err == nil {
		t.Error("expected error for invalid renavam, got nil")
	}
}

func TestVehicle_List(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	plates := []string{"ABC1234", "DEF5678", "GHI9012"}
	for _, p := range plates {
		_, err := vehicleSvc.Create(ctx, orgPK, map[string]any{
			"plate":    p,
			"plate_uf": "SP",
			"wheelset": "03",
			"bodywork": "02",
			"renavam":  "123456789",
			"weight":   5000,
			"owner":    validOwner,
		}, "test-user", "Test User")
		if err != nil {
			t.Fatalf("Create %s: %v", p, err)
		}
	}

	result, err := vehicleSvc.List(ctx, orgPK, repositories.VehicleListOpts{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) < 3 {
		t.Errorf("List returned %d items, want at least 3", len(result.Items))
	}
}

func TestVehicle_Update(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := vehicleSvc.Create(ctx, orgPK, map[string]any{
		"plate":    "ZZZ9999",
		"plate_uf": "SP",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   8000,
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	updated, err := vehicleSvc.Update(ctx, orgPK, sk, map[string]any{"plate_uf": "RJ"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	ufAV, _ := updated["plate_uf"].(*types.AttributeValueMemberS)
	if ufAV == nil || ufAV.Value != "RJ" {
		t.Errorf("plate_uf after update = %v, want RJ", updated["plate_uf"])
	}
}

func TestVehicle_RequirementsListsMissingFields(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := vehicleSvc.Create(ctx, orgPK, map[string]any{
		"plate":    "REQ0001",
		"plate_uf": "SP",
		"role":     "tractor",
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	got, err := vehicleSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	missing := services.Missing(got, services.DocTypeMdfe, services.VehicleRoleTractor)
	want := []string{"weight", "wheelset", "bodywork"}
	if len(missing) != len(want) {
		t.Fatalf("Missing() = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Errorf("Missing()[%d] = %q, want %q", i, missing[i], want[i])
		}
	}
}

func TestVehicle_RequirementsEmptyForCompleteTractor(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := vehicleSvc.Create(ctx, orgPK, map[string]any{
		"plate":    "REQ0002",
		"plate_uf": "SP",
		"role":     "tractor",
		"weight":   8000,
		"wheelset": "01",
		"bodywork": "00",
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	got, err := vehicleSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if missing := services.Missing(got, services.DocTypeMdfe, services.VehicleRoleTractor); len(missing) != 0 {
		t.Errorf("Missing() = %v, want empty", missing)
	}
}

func TestVehicle_ListByRole(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := vehicleSvc.Create(ctx, orgPK, map[string]any{"plate": "TRC0001", "plate_uf": "SP", "role": "tractor"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create tractor: %v", err)
	}
	if _, err := vehicleSvc.Create(ctx, orgPK, map[string]any{"plate": "TRL0001", "plate_uf": "SP", "role": "trailer"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Create trailer: %v", err)
	}

	result, err := vehicleSvc.ListByRole(ctx, orgPK, "trailer", repositories.VehicleListOpts{Limit: 10})
	if err != nil {
		t.Fatalf("ListByRole: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("ListByRole(trailer) returned %d items, want 1", len(result.Items))
	}
	plateAV, _ := result.Items[0]["plate"].(*types.AttributeValueMemberS)
	if plateAV == nil || plateAV.Value != "TRL0001" {
		t.Errorf("plate = %v, want TRL0001", result.Items[0]["plate"])
	}
}

func TestVehicle_Delete(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := vehicleSvc.Create(ctx, orgPK, map[string]any{
		"plate":    "DEL0001",
		"plate_uf": "MG",
		"wheelset": "03",
		"bodywork": "02",
		"renavam":  "123456789",
		"weight":   3000,
		"owner":    validOwner,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	if err := vehicleSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, getErr := vehicleSvc.Get(ctx, orgPK, sk)
	if problemStatus(getErr) != 404 {
		t.Errorf("expected 404 after delete, got status %d: %v", problemStatus(getErr), getErr)
	}
}
