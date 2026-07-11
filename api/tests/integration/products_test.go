//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

func productFields(code, desc string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"code":        &types.AttributeValueMemberS{Value: code},
		"description": &types.AttributeValueMemberS{Value: desc},
		"value":       &types.AttributeValueMemberS{Value: "99.90"},
		"ncm":         &types.AttributeValueMemberS{Value: "12345678"},
	}
}

func TestProduct_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := productSvc.Create(ctx, orgPK, productFields("PROD001", "Produto Teste"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	if skAV == nil {
		t.Fatal("created product has no sk")
	}
	sk := skAV.Value

	got, err := productSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing product")
	}
	descAV, _ := got["description"].(*types.AttributeValueMemberS)
	if descAV == nil || descAV.Value != "Produto Teste" {
		t.Errorf("description = %v, want Produto Teste", got["description"])
	}
}

func TestProduct_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	_, err := productSvc.Get(ctx, orgPK, "PRODUCT_nonexistent")
	if problemStatus(err) != 404 {
		t.Errorf("expected 404 for unknown product, got status %d: %v", problemStatus(err), err)
	}
}

func TestProduct_List(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	for i := 0; i < 3; i++ {
		_, err := productSvc.Create(ctx, orgPK, productFields(
			"PROD"+string(rune('A'+i)),
			"Produto "+string(rune('A'+i)),
		), "test-user", "Test User")
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	result, err := productSvc.List(ctx, orgPK, repositories.ProductListOpts{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) < 3 {
		t.Errorf("List returned %d items, want at least 3", len(result.Items))
	}
}

func TestProduct_Update(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := productSvc.Create(ctx, orgPK, productFields("UPD001", "Before"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	updated, err := productSvc.Update(ctx, orgPK, sk, map[string]any{"description": "After"}, "test-user", "Test User")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	descAV, _ := updated["description"].(*types.AttributeValueMemberS)
	if descAV == nil || descAV.Value != "After" {
		t.Errorf("description after update = %v, want After", updated["description"])
	}
}

func TestProduct_Delete(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := productSvc.Create(ctx, orgPK, productFields("DEL001", "Delete Me"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	if err := productSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = productSvc.Get(ctx, orgPK, sk)
	if problemStatus(err) != 404 {
		t.Errorf("expected 404 after delete, got status %d: %v", problemStatus(err), err)
	}
}

func TestProduct_CacheInvalidatedOnCreate(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	// populate cache by listing
	_, _ = productSvc.List(ctx, orgPK, repositories.ProductListOpts{Limit: 10})

	// create should evict prefix cache
	if _, err := productSvc.Create(ctx, orgPK, productFields("CACHEP", "Cache Product"), "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	result, err := productSvc.List(ctx, orgPK, repositories.ProductListOpts{Limit: 10})
	if err != nil {
		t.Fatalf("List after create: %v", err)
	}
	if len(result.Items) < 1 {
		t.Error("list after create should include new product")
	}
}
