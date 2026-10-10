//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func nfseConfigFields() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"provider":            &types.AttributeValueMemberS{Value: "nacional"},
		"environment":         &types.AttributeValueMemberN{Value: "2"},
		"timezone":            &types.AttributeValueMemberS{Value: "America/Fortaleza"},
		"c_loc_emi":           &types.AttributeValueMemberS{Value: "2211001"},
		"serie":               &types.AttributeValueMemberS{Value: "00001"},
		"prod_current_number": &types.AttributeValueMemberN{Value: "0"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "0"},
	}
}

func TestNfseConfig_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := nfseConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["provider"].(*types.AttributeValueMemberS).Value != "nacional" {
		t.Errorf("provider = %v", got["provider"])
	}
	if got["c_loc_emi"].(*types.AttributeValueMemberS).Value != "2211001" {
		t.Errorf("c_loc_emi = %v", got["c_loc_emi"])
	}
	if got["timezone"].(*types.AttributeValueMemberS).Value != "America/Fortaleza" {
		t.Errorf("timezone = %v", got["timezone"])
	}
}

func TestNfseConfig_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Get(ctx, orgPK); problemStatus(err) != 404 {
		t.Errorf("status = %d, esperado 404", problemStatus(err))
	}
}

// O contador de numeração é o mesmo mecanismo dos demais documentos fiscais:
// {envPrefix}_current_number, incrementado atomicamente.
func TestNfseConfig_IncrementNumber(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	first, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom")
	if err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	second, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom")
	if err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	if second != first+1 {
		t.Errorf("segundo incremento = %d, esperado %d", second, first+1)
	}
}

// Os contadores de numeração da NFS-e são campos editáveis do PUT
// (NfseConfigBody sempre os envia), não campos "preserve": o valor informado
// pelo usuário vence o gravado, e a emissão continua a partir dele via
// IncrementNumber. O que o Upsert preserva é o processo interno, o cursor NSU
// da distribuição ADN. Ver 7311c08 e o comentário de NfseConfigRepository.
func TestNfseConfig_UpsertWritesCounterAndPreservesNSU(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom"); err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	if err := nfseConfigRepo.UpdateNSU(ctx, orgPK, "hom", 77); err != nil {
		t.Fatalf("UpdateNSU: %v", err)
	}

	// O usuário ajusta a numeração da DPS (ex.: migrando de outro emissor).
	fields := nfseConfigFields()
	fields["hom_current_number"] = &types.AttributeValueMemberN{Value: "41"}
	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, fields, "test-user", "Test User"); err != nil {
		t.Fatalf("segundo Upsert: %v", err)
	}

	got, err := nfseConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n, _ := got["hom_current_number"].(*types.AttributeValueMemberN); n == nil || n.Value != "41" {
		t.Errorf("hom_current_number = %v, esperado 41: o PUT do usuário foi ignorado", got["hom_current_number"])
	}
	if n, _ := got["hom_nsu"].(*types.AttributeValueMemberN); n == nil || n.Value != "77" {
		t.Errorf("hom_nsu = %v, esperado 77: o upsert zerou o cursor NSU", got["hom_nsu"])
	}

	next, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom")
	if err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	if next != 42 {
		t.Errorf("próximo número = %d, esperado 42", next)
	}
}
