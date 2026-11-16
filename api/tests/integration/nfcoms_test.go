//go:build integration

package integration_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func nfcomConfigFields() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"environment":         &types.AttributeValueMemberN{Value: "2"},
		"prod_current_serie":  &types.AttributeValueMemberN{Value: "1"},
		"hom_current_serie":   &types.AttributeValueMemberN{Value: "1"},
		"prod_current_number": &types.AttributeValueMemberN{Value: "0"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "0"},
	}
}

func nfcomRow(pk, key string, number int) map[string]any {
	return map[string]any{"pk": pk, "sk": key, "number": number, "status": "pending", "incoming": 0, "year": 2026, "month": 11, "day": 5}
}

// Duas emissões concorrentes que leram o mesmo contador: só uma grava a nota e avança o número; a outra é cancelada
// pela condição do contador (a API devolve 409 e o cliente tenta de novo).
func TestNfcom_NumberReservationIsAtomic(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if _, err := nfcomConfigSvc.Upsert(ctx, orgPK, nfcomConfigFields(), testUserID, testUserName); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	pk := "hom#" + orgPK
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, _ := repositories.EncodeItem(nfcomRow(pk, strings.Repeat("0", 43)+string(rune('1'+i)), 1))
			errs[i] = nfcomRepo.TransactReserveAndCreate(ctx, nfcomConfigRepo.TableName, orgPK, "hom", 0, item)
		}()
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		} else if !strings.Contains(e.Error(), "TransactionCanceledException") {
			t.Errorf("erro inesperado: %v", e)
		}
	}
	if ok != 1 {
		t.Fatalf("emissões que gravaram = %d, queria 1 (%v)", ok, errs)
	}
	cfg, err := nfcomConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg["hom_current_number"].(*types.AttributeValueMemberN).Value; got != "1" {
		t.Errorf("hom_current_number = %s, queria 1", got)
	}
}

// A configuração da NFCom guarda a forma de emissão e a justificativa; a troca automática é recusada.
func TestNfcomConfig_OfflineEmissionAndAutoRefused(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if _, err := nfcomConfigSvc.Upsert(ctx, orgPK, nfcomConfigFields(), testUserID, testUserName); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	reason := "Falha de comunicação com a SEFAZ autorizadora"
	if _, err := nfcomConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: repositories.EmissionOffline, Reason: reason}); err != nil {
		t.Fatalf("SetEmission offline: %v", err)
	}
	cfg, err := nfcomConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	if repositories.EmissionTypeOf(cfg) != repositories.EmissionOffline {
		t.Errorf("emission_type = %q", repositories.EmissionTypeOf(cfg))
	}
	if got := cfg[repositories.AttrContingencyReason].(*types.AttributeValueMemberS).Value; got != reason {
		t.Errorf("contingency_reason = %q", got)
	}
	_, err = nfcomConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: repositories.EmissionOffline, Reason: reason, Auto: true, AutoType: repositories.EmissionOffline})
	if problemStatus(err) != 422 {
		t.Errorf("auto_contingency: status %d, queria 422 (%v)", problemStatus(err), err)
	}
	if _, err := nfcomConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: "svc", Reason: reason}); problemStatus(err) != 422 {
		t.Errorf("SVC não existe para a NFCom: status %d", problemStatus(err))
	}
}
