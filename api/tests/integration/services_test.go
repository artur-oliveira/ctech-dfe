//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	v1 "gopkg.aoctech.app/dfe/api/internal/api/v1"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func serviceFields(code, desc string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"code":               &types.AttributeValueMemberS{Value: code},
		"description":        &types.AttributeValueMemberS{Value: desc},
		"trib_nacional_code": &types.AttributeValueMemberS{Value: "010101"},
		"value":              &types.AttributeValueMemberS{Value: "1500.00"},
		"unit":               &types.AttributeValueMemberS{Value: "UN"},
	}
}

func TestService_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := serviceSvc.Create(ctx, orgPK, serviceFields("SRV001", "Desenvolvimento de sistemas"), "test-user", "Test User", v1.ServiceSchemaVersion)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	if skAV == nil {
		t.Fatal("serviço criado sem sk")
	}
	if got := skAV.Value; len(got) < 9 || got[:8] != "SERVICE_" {
		t.Errorf("sk = %q, esperado prefixo SERVICE_", got)
	}

	got, err := serviceSvc.Get(ctx, orgPK, skAV.Value)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	descAV, _ := got["description"].(*types.AttributeValueMemberS)
	if descAV == nil || descAV.Value != "Desenvolvimento de sistemas" {
		t.Errorf("description = %v", got["description"])
	}
}

func TestService_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := serviceSvc.Get(ctx, orgPK, "SERVICE_inexistente"); problemStatus(err) != 404 {
		t.Errorf("status = %d, esperado 404", problemStatus(err))
	}
}

func TestService_ListByCodePrefix(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	for _, code := range []string{"CONS001", "CONS002", "DEV001"} {
		if _, err := serviceSvc.Create(ctx, orgPK, serviceFields(code, "Serviço "+code), "test-user", "Test User", v1.ServiceSchemaVersion); err != nil {
			t.Fatalf("Create(%s): %v", code, err)
		}
	}

	res, err := serviceSvc.List(ctx, orgPK, repositories.ServiceListOpts{CodePrefix: "CONS", Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("len(Items) = %d, esperado 2", len(res.Items))
	}
}

func TestService_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := serviceSvc.Create(ctx, orgPK, serviceFields("SRV009", "Antes"), "test-user", "Test User", v1.ServiceSchemaVersion)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	if _, err := serviceSvc.Update(ctx, orgPK, sk, map[string]any{"description": "Depois"}, "test-user", "Test User", v1.ServiceSchemaVersion); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := serviceSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get após update: %v", err)
	}
	if got["description"].(*types.AttributeValueMemberS).Value != "Depois" {
		t.Error("update não persistiu")
	}

	if err := serviceSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := serviceSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("após delete, status = %d, esperado 404", problemStatus(err))
	}
}
