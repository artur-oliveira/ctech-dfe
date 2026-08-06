package nfses

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestEmit_BuildsWorkerBodyForDispatch(t *testing.T) {
	// buildWorkerBody é a fronteira testável sem AWS: o que vai no comando
	// do outbox tem que casar com as chaves que nfse.Dispatch lê.
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	body, err := buildWorkerBody(nfse.ProviderNacional, doc)
	if err != nil {
		t.Fatalf("buildWorkerBody: %v", err)
	}
	if body[nfse.BodyKeyProvider] != nfse.ProviderNacional {
		t.Errorf("provider = %v", body[nfse.BodyKeyProvider])
	}
	sub, ok := body[nfse.BodyKeyDocument].(map[string]any)
	if !ok {
		t.Fatalf("document não é um objeto: %T", body[nfse.BodyKeyDocument])
	}
	if sub["c_loc_emi"] != "2211001" {
		t.Errorf("c_loc_emi perdido na serialização: %v", sub["c_loc_emi"])
	}
	// A chave "prestador" tem que existir com o reg_trib dentro — se o
	// achatamento do embedded Pessoa quebrar, isso pega.
	prest, ok := sub["prestador"].(map[string]any)
	if !ok {
		t.Fatalf("prestador ausente: %T", sub["prestador"])
	}
	if _, ok := prest["reg_trib"]; !ok {
		t.Error("reg_trib ausente no prestador serializado")
	}
}

// O documento que vai no outbox tem que sobreviver ao DecodeDocument do
// go-dfe, que rejeita campo desconhecido: se buildDocument produzir uma chave
// que o modelo neutro não tem, a emissão só falharia dentro do worker.
func TestEmit_WorkerBodyDecodesBackInGoDfe(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	body, err := buildWorkerBody(nfse.ProviderNacional, doc)
	if err != nil {
		t.Fatalf("buildWorkerBody: %v", err)
	}
	got, err := nfse.DecodeDocument(body[nfse.BodyKeyDocument].(map[string]any))
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if got.Prestador.CNPJ != doc.Prestador.CNPJ {
		t.Errorf("CNPJ = %q, esperado %q", got.Prestador.CNPJ, doc.Prestador.CNPJ)
	}
	if got.Valores.Trib.TribMun.PAliq != doc.Valores.Trib.TribMun.PAliq {
		t.Errorf("pAliq = %q, esperado %q", got.Valores.Trib.TribMun.PAliq, doc.Valores.Trib.TribMun.PAliq)
	}
}

func TestEmit_RejectsUnconfiguredOrg(t *testing.T) {
	svc := &NfseService{}
	if _, err := svc.emitPreflight(nil, nil); err == nil ||
		!strings.Contains(err.Error(), "organização não encontrada") {
		t.Fatalf("esperado erro de organização ausente, veio: %v", err)
	}

	org := attrs(map[string]string{"name": "Prestador LTDA"})
	_, err := svc.emitPreflight(org, nil)
	if err == nil || !strings.Contains(err.Error(), "Configuração Fiscal") {
		t.Fatalf("esperado erro de config ausente, veio: %v", err)
	}
}

// provider abrasf204 sem o bloco abrasf não tem endpoint de município: falha
// antes de reservar número, não depois.
func TestEmit_RejectsAbrasfWithoutEndpoint(t *testing.T) {
	svc := &NfseService{}
	org := attrs(map[string]string{"name": "Prestador LTDA"})
	cfg := attrs(map[string]string{"provider": nfse.ProviderAbrasf204})
	_, err := svc.emitPreflight(org, cfg)
	if err == nil || !strings.Contains(err.Error(), "ABRASF") {
		t.Fatalf("esperado erro de config ABRASF incompleta, veio: %v", err)
	}
}
