//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func createVehicle(t *testing.T, orgPK, plate, role string) string {
	t.Helper()
	item, err := vehicleSvc.Create(ctx0(), orgPK, map[string]any{
		"plate": plate, "plate_uf": "SP", "role": role,
	}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create vehicle %s: %v", plate, err)
	}
	return avString(item, "sk")
}

func ctx0() context.Context { return context.Background() }

func vehicleSetFields(name, tractorSK string, trailerSKs ...string) map[string]types.AttributeValue {
	trailers := make([]types.AttributeValue, 0, len(trailerSKs))
	for _, sk := range trailerSKs {
		trailers = append(trailers, &types.AttributeValueMemberS{Value: sk})
	}
	return map[string]types.AttributeValue{
		"name":        &types.AttributeValueMemberS{Value: name},
		"tractor_sk":  &types.AttributeValueMemberS{Value: tractorSK},
		"trailer_sks": &types.AttributeValueMemberL{Value: trailers},
	}
}

func TestVehicleSet_CRUD(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	tractor := createVehicle(t, orgPK, "ABC1D23", "tractor")
	trailer := createVehicle(t, orgPK, "XYZ9W88", "trailer")

	created, err := vehicleSetSvc.Create(ctx, orgPK,
		vehicleSetFields("Carreta 1", tractor, trailer), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")

	list, err := vehicleSetSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("List devolveu %d, esperado 1", len(list.Items))
	}

	if err := vehicleSetSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := vehicleSetSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("esperado 404 após delete, obtido %d", problemStatus(err))
	}
}

// Papel errado é erro de cadastro, não rejeição da SEFAZ semanas depois.
func TestVehicleSet_RejectsWrongRoles(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	tractor := createVehicle(t, orgPK, "ABC1D23", "tractor")
	trailer := createVehicle(t, orgPK, "XYZ9W88", "trailer")

	// Reboque no lugar do trator.
	if _, err := vehicleSetSvc.Create(ctx, orgPK,
		vehicleSetFields("Errada", trailer), "test-user", "Test User"); err == nil {
		t.Error("esperada recusa: reboque como veículo de tração")
	}

	// Trator na lista de reboques.
	if _, err := vehicleSetSvc.Create(ctx, orgPK,
		vehicleSetFields("Errada 2", tractor, tractor), "test-user", "Test User"); err == nil {
		t.Error("esperada recusa: trator na lista de reboques")
	}

	// Veículo inexistente.
	if _, err := vehicleSetSvc.Create(ctx, orgPK,
		vehicleSetFields("Errada 3", "VEHICLE_nao-existe"), "test-user", "Test User"); err == nil {
		t.Error("esperada recusa: veículo inexistente")
	}
}

func TestVehicleSet_UpdateAlsoValidatesRoles(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	tractor := createVehicle(t, orgPK, "ABC1D23", "tractor")
	trailer := createVehicle(t, orgPK, "XYZ9W88", "trailer")

	created, err := vehicleSetSvc.Create(ctx, orgPK,
		vehicleSetFields("Carreta 1", tractor, trailer), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := vehicleSetSvc.Update(ctx, orgPK, avString(created, "sk"),
		map[string]any{"tractor_sk": trailer}, "test-user", "Test User"); err == nil {
		t.Error("esperada recusa no update: reboque como veículo de tração")
	}
}
