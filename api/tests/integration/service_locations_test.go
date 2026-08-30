//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func serviceLocationFields(name string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"name": &types.AttributeValueMemberS{Value: name},
		"roles": &types.AttributeValueMemberL{Value: []types.AttributeValue{
			&types.AttributeValueMemberS{Value: "work"},
			&types.AttributeValueMemberS{Value: "property"},
		}},
		"address": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"street":         &types.AttributeValueMemberS{Value: "Rua C"},
			"number":         &types.AttributeValueMemberS{Value: "300"},
			"neighborhood":   &types.AttributeValueMemberS{Value: "Centro"},
			"postal_code":    &types.AttributeValueMemberS{Value: "64000000"},
			"city_ibge_code": &types.AttributeValueMemberS{Value: "2211001"},
		}},
		"c_obra": &types.AttributeValueMemberS{Value: "CNO-12345"},
	}
}

func TestServiceLocation_CRUDAndNameIndex(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := svcLocationSvc.Create(ctx, orgPK, serviceLocationFields("Obra Centro"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")

	if _, err := svcLocationSvc.Create(ctx, orgPK, serviceLocationFields("Obra Jockey"), "test-user", "Test User"); err != nil {
		t.Fatalf("Create segundo local: %v", err)
	}

	// O picker da emissão busca por nome; sem o GSI a tela cairia em Scan.
	byName, err := svcLocationSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{NamePrefix: "Obra J", Limit: 50})
	if err != nil {
		t.Fatalf("List por nome: %v", err)
	}
	if len(byName.Items) != 1 || avString(byName.Items[0], "name") != "Obra Jockey" {
		t.Fatalf("busca por prefixo devolveu %d itens: %v", len(byName.Items), byName.Items)
	}

	// Um local de outro tenant nunca aparece: a partição é a organização.
	other, err := svcLocationSvc.List(ctx, "CNPJ_"+randomCNPJ(), repositories.OrgEntityListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List de outro tenant: %v", err)
	}
	if len(other.Items) != 0 {
		t.Errorf("local vazou entre organizações: %v", other.Items)
	}

	if err := svcLocationSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svcLocationSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("Get após Delete = %v, esperado 404", err)
	}
}

func referenceDocumentFields(name, kind string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"name":      &types.AttributeValueMemberS{Value: name},
		"kind":      &types.AttributeValueMemberS{Value: kind},
		"issued_at": &types.AttributeValueMemberS{Value: "2026-08-01"},
		"dfe": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"tipo_chave_dfe": &types.AttributeValueMemberS{Value: "2"},
			"chave_dfe":      &types.AttributeValueMemberS{Value: "35260812345678000190550010000000011000000010"},
		}},
	}
}

func TestReferenceDocument_CRUDPreservesUnion(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := refDocSvc.Create(ctx, orgPK, referenceDocumentFields("NF-e do fornecedor", "dfe"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")

	fetched, err := refDocSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// O subobjeto da família tem de sobreviver ao round-trip: a emissão decide
	// o ramo do xs:choice a partir dele.
	dfe, ok := fetched["dfe"].(*types.AttributeValueMemberM)
	if !ok {
		t.Fatalf("subobjeto dfe perdido na persistência: %v", fetched)
	}
	if avString(dfe.Value, "tipo_chave_dfe") != "2" {
		t.Errorf("tipo_chave_dfe = %q", avString(dfe.Value, "tipo_chave_dfe"))
	}

	if _, err := refDocSvc.Update(ctx, orgPK, sk, map[string]any{"name": "NF-e 123"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := refDocSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get após Update: %v", err)
	}
	if avString(updated, "name") != "NF-e 123" {
		t.Errorf("name = %q", avString(updated, "name"))
	}

	if err := refDocSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := refDocSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("Get após Delete = %v, esperado 404", err)
	}
}
