//go:build integration

package integration_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services/nfes"
)

func paymentTermFields(name string, installments int) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"name":           &types.AttributeValueMemberS{Value: name},
		"payment_type":   &types.AttributeValueMemberS{Value: "15"},
		"installments":   &types.AttributeValueMemberN{Value: strconv.Itoa(installments)},
		"interval_days":  &types.AttributeValueMemberN{Value: "30"},
		"first_due_days": &types.AttributeValueMemberN{Value: "30"},
	}
}

func TestPaymentTerm_CRUD(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := paymentTermSvc.Create(ctx, orgPK, paymentTermFields("30/60/90", 3), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")

	list, err := paymentTermSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("List devolveu %d, esperado 1", len(list.Items))
	}

	if err := paymentTermSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := paymentTermSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("esperado 404 após delete, obtido %d", problemStatus(err))
	}
}

// A condição só serve se, lida de volta do DynamoDB, ainda produzir duplicatas
// que fecham com o total — os números voltam como float64, não como int.
func TestPaymentTerm_ExpandsFromStoredItem(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := paymentTermSvc.Create(ctx, orgPK, paymentTermFields("30/60/90", 3), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	stored, err := paymentTermSvc.Get(ctx, orgPK, avString(created, "sk"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var term map[string]any
	if err := attributevalue.UnmarshalMap(stored, &term); err != nil {
		t.Fatalf("UnmarshalMap: %v", err)
	}

	total := decimal.RequireFromString("100.00")
	issue := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	payments, fat, dups, err := nfes.ExpandPaymentTerm(term, total, issue)
	if err != nil {
		t.Fatalf("ExpandPaymentTerm: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("%d pagamentos, esperado 1", len(payments))
	}
	if fat == nil {
		t.Fatal("fatura ausente para condição a prazo")
	}
	if len(dups) != 3 {
		t.Fatalf("%d duplicatas, esperadas 3", len(dups))
	}

	sum := decimal.Zero
	for _, d := range dups {
		sum = sum.Add(decimal.RequireFromString(d.VDup))
	}
	if !sum.Equal(total) {
		t.Errorf("soma das duplicatas %s, esperada %s", sum, total)
	}
}
