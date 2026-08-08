//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func operationFields(name string, isDefault bool) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"name":       &types.AttributeValueMemberS{Value: name},
		"nat_op":     &types.AttributeValueMemberS{Value: "Venda de mercadoria"},
		"is_default": &types.AttributeValueMemberBOOL{Value: isDefault},
	}
}

func avBool(item map[string]types.AttributeValue, key string) bool {
	if v, ok := item[key].(*types.AttributeValueMemberBOOL); ok {
		return v.Value
	}
	return false
}

// A regra própria das operações: no máximo uma padrão por organização. Duas
// deixariam a UI escolhendo por sorte de ordenação.
func TestOperation_MarkingDefaultUnmarksThePrevious(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	first, err := operationSvc.Create(ctx, orgPK, operationFields("Venda", true), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create primeira: %v", err)
	}
	second, err := operationSvc.Create(ctx, orgPK, operationFields("Devolução", true), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create segunda: %v", err)
	}

	defaults, err := operationRepo.ListDefaults(ctx, orgPK)
	if err != nil {
		t.Fatalf("ListDefaults: %v", err)
	}
	if len(defaults) != 1 {
		t.Fatalf("%d operações padrão, esperada exatamente 1", len(defaults))
	}
	if avString(defaults[0], "sk") != avString(second, "sk") {
		t.Errorf("a padrão é %q, esperada a última marcada", avString(defaults[0], "name"))
	}

	after, err := operationSvc.Get(ctx, orgPK, avString(first, "sk"))
	if err != nil {
		t.Fatalf("Get primeira: %v", err)
	}
	if avBool(after, "is_default") {
		t.Error("a primeira operação continuou marcada como padrão")
	}
}

// Marcar via update segue a mesma regra do create.
func TestOperation_UpdateToDefaultUnmarksThePrevious(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	first, err := operationSvc.Create(ctx, orgPK, operationFields("Venda", true), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create primeira: %v", err)
	}
	second, err := operationSvc.Create(ctx, orgPK, operationFields("Remessa", false), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create segunda: %v", err)
	}

	if _, err := operationSvc.Update(ctx, orgPK, avString(second, "sk"),
		map[string]any{"is_default": true}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	defaults, err := operationRepo.ListDefaults(ctx, orgPK)
	if err != nil {
		t.Fatalf("ListDefaults: %v", err)
	}
	if len(defaults) != 1 || avString(defaults[0], "sk") != avString(second, "sk") {
		t.Fatalf("%d padrões; esperada só a segunda", len(defaults))
	}
	after, _ := operationSvc.Get(ctx, orgPK, avString(first, "sk"))
	if avBool(after, "is_default") {
		t.Error("a primeira continuou padrão")
	}
}

// Re-salvar a própria operação padrão não pode desmarcá-la.
func TestOperation_ResavingTheDefaultKeepsIt(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	op, err := operationSvc.Create(ctx, orgPK, operationFields("Venda", true), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := operationSvc.Update(ctx, orgPK, avString(op, "sk"),
		map[string]any{"is_default": true, "name": "Venda v2"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	after, _ := operationSvc.Get(ctx, orgPK, avString(op, "sk"))
	if !avBool(after, "is_default") {
		t.Error("re-salvar a operação padrão a desmarcou")
	}
}

// Nenhuma operação marcada: a organização simplesmente não tem padrão.
func TestOperation_NoDefaultIsValid(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := operationSvc.Create(ctx, orgPK, operationFields("Remessa", false), "test-user", "Test User"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	defaults, err := operationRepo.ListDefaults(ctx, orgPK)
	if err != nil {
		t.Fatalf("ListDefaults: %v", err)
	}
	if len(defaults) != 0 {
		t.Errorf("%d padrões, esperado 0", len(defaults))
	}
}

func TestOperation_CRUD(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := operationSvc.Create(ctx, orgPK, operationFields("Venda", false), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")

	list, err := operationSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("List devolveu %d, esperado 1", len(list.Items))
	}

	if err := operationSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := operationSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("esperado 404 após delete, obtido %d", problemStatus(err))
	}
}
