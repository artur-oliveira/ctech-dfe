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

// Upsert não pode zerar o contador — é campo de processo interno.
func TestNfseConfig_UpsertPreservesCounter(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom"); err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}

	fields := nfseConfigFields()
	delete(fields, "hom_current_number")
	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, fields, "test-user", "Test User"); err != nil {
		t.Fatalf("segundo Upsert: %v", err)
	}

	got, err := nfseConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n := got["hom_current_number"].(*types.AttributeValueMemberN).Value; n != "1" {
		t.Errorf("hom_current_number = %s, esperado 1 — o upsert zerou o contador", n)
	}
}
