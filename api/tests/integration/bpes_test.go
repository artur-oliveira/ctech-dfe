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
	"gopkg.aoctech.app/dfe/api/internal/services/bpes"
)

func bpeConfigFields() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"environment":         &types.AttributeValueMemberN{Value: "2"},
		"prod_current_serie":  &types.AttributeValueMemberN{Value: "1"},
		"hom_current_serie":   &types.AttributeValueMemberN{Value: "1"},
		"prod_current_number": &types.AttributeValueMemberN{Value: "0"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "0"},
	}
}

func bpeRow(pk, key string, number int) map[string]any {
	return map[string]any{"pk": pk, "sk": key, "number": number, "status": "pending", "incoming": 0, "year": 2026, "month": 11, "day": 5}
}

// Duas emissões concorrentes que leram o mesmo contador: só uma grava o bilhete e avança o número.
func TestBpe_NumberReservationIsAtomic(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if _, err := bpeConfigSvc.Upsert(ctx, orgPK, bpeConfigFields(), testUserID, testUserName); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	pk := "hom#" + orgPK
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, _ := repositories.EncodeItem(bpeRow(pk, strings.Repeat("0", 43)+string(rune('1'+i)), 1))
			errs[i] = bpeRepo.TransactReserveAndCreate(ctx, bpeConfigRepo.TableName, orgPK, "hom", 0, item)
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
	cfg, err := bpeConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg["hom_current_number"].(*types.AttributeValueMemberN).Value; got != "1" {
		t.Errorf("hom_current_number = %s, queria 1", got)
	}
}

// A configuração guarda a forma de emissão e a justificativa; a troca automática e o SVC são recusados.
func TestBpeConfig_OfflineEmissionAndAutoRefused(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if _, err := bpeConfigSvc.Upsert(ctx, orgPK, bpeConfigFields(), testUserID, testUserName); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	reason := "Falha de comunicação com a SEFAZ autorizadora"
	if _, err := bpeConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: repositories.EmissionOffline, Reason: reason}); err != nil {
		t.Fatalf("SetEmission offline: %v", err)
	}
	cfg, err := bpeConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	if repositories.EmissionTypeOf(cfg) != repositories.EmissionOffline {
		t.Errorf("emission_type = %q", repositories.EmissionTypeOf(cfg))
	}
	if _, err := bpeConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: repositories.EmissionOffline, Reason: reason, Auto: true, AutoType: repositories.EmissionOffline}); problemStatus(err) != 422 {
		t.Errorf("auto_contingency: status %d, queria 422", problemStatus(err))
	}
	if _, err := bpeConfigSvc.SetEmission(ctx, orgPK, services.EmissionUpdate{Type: "svc", Reason: reason}); problemStatus(err) != 422 {
		t.Errorf("SVC não existe para o BP-e: status %d", problemStatus(err))
	}
}

// Os eventos de um BP-e só saem para a organização dona dele: a chave de acesso sozinha não abre a lista (IDOR).
func TestBpe_ListEventsChecksTenant(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	key := strings.Repeat("0", 43) + "7"
	item, _ := repositories.EncodeItem(bpeRow("hom#"+orgA, key, 1))
	if err := bpeRepo.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	events := repositories.NewDocumentEventRepository(db, cfg, "bpe")
	if _, err := events.CreateEvent(ctx, key, "110111", 1, "pending", nil, nil, nil, testUserID, testUserName); err != nil {
		t.Fatal(err)
	}
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, events, nil, nil, nil, nil, "", bpes.TechData{}, nil)
	res, err := svc.ListBPeEvents(ctx, orgA, key, 50, nil)
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("dono: %v %v", res, err)
	}
	if _, err := svc.ListBPeEvents(ctx, orgB, key, 50, nil); problemStatus(err) != 404 {
		t.Fatalf("outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
}

func bpeVariantRow(pk, key, variant string) map[string]any {
	row := bpeRow(pk, key, 1)
	row["variant"] = variant
	row["status"] = "authorized"
	return row
}

// O DABPE só sai para a organização dona do bilhete (404 para outra), recusa formato desconhecido (400) e não existe para o TM.
func TestBpe_DABPEChecksTenantFormatAndVariant(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	keyTA, keyTM := strings.Repeat("0", 43)+"8", strings.Repeat("0", 43)+"9"
	for key, variant := range map[string]string{keyTA: "ta", keyTM: "tm"} {
		item, _ := repositories.EncodeItem(bpeVariantRow("hom#"+orgA, key, variant))
		if err := bpeRepo.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	events := repositories.NewDocumentEventRepository(db, cfg, "bpe")
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, events, nil, nil, nil, nil, "", bpes.TechData{}, nil)
	if _, err := svc.GetDABPEURL(ctx, orgB, keyTA, ""); problemStatus(err) != 404 {
		t.Errorf("outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
	if _, err := svc.GetDABPEURL(ctx, orgA, keyTA, "a3"); problemStatus(err) != 400 {
		t.Errorf("formato desconhecido: status %d, queria 400", problemStatus(err))
	}
	if _, err := svc.GetDABPEURL(ctx, orgA, keyTM, ""); problemStatus(err) != 400 {
		t.Errorf("TM não tem DABPE: status %d, queria 400 (%v)", problemStatus(err), err)
	}
	// TA autorizado sem XML guardado: 404 "ainda não disponível", depois das checagens de tenant, formato e variante.
	if _, err := svc.GetDABPEURL(ctx, orgA, keyTA, "reduzido"); problemStatus(err) != 404 {
		t.Errorf("TA sem XML: status %d, queria 404 (%v)", problemStatus(err), err)
	}
}

// O evento 112140 (fornecimento não realizado) de um TA fica na lista de eventos do dono e some para outra organização.
func TestBpe_TASupplyNotPerformedEventIsTenantScoped(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	key := strings.Repeat("0", 42) + "10"
	item, _ := repositories.EncodeItem(bpeVariantRow("hom#"+orgA, key, "ta"))
	if err := bpeRepo.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	events := repositories.NewDocumentEventRepository(db, cfg, "bpe")
	if _, err := events.CreateEvent(ctx, key, bpes.TpEventoFornecimentoNaoRealizado, 1, "pending", nil, nil, nil, testUserID, testUserName); err != nil {
		t.Fatal(err)
	}
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, events, nil, nil, nil, nil, "", bpes.TechData{}, nil)
	res, err := svc.ListBPeEvents(ctx, orgA, key, 50, nil)
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("dono: %v %v", res, err)
	}
	if _, err := svc.ListBPeEvents(ctx, orgB, key, 50, nil); problemStatus(err) != 404 {
		t.Fatalf("outra organização: status %d, queria 404", problemStatus(err))
	}
}
