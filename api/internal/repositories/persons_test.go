package repositories

import (
	"testing"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

func TestBuildCreateTxItem_UsesConditionalPut(t *testing.T) {
	repo := &PersonRepository{CRUDRepository: NewCRUDRepository[map[string]any](nil, &config.Config{TablePrefix: "test"}, "organization_persons")}
	txItem, _ := repo.BuildCreateTxItem("ORG_1", "CPF_11122233344", map[string]any{"name": "Test"})
	if txItem.Put == nil {
		t.Fatal("expected Put transact item")
	}
	if txItem.Put.ConditionExpression == nil || *txItem.Put.ConditionExpression != "attribute_not_exists(pk)" {
		t.Fatalf("expected attribute_not_exists(pk) condition, got %v", txItem.Put.ConditionExpression)
	}
}
