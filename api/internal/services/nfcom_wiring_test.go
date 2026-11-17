package services

import (
	"slices"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// A NFCom é cobrada pelo medidor próprio: sem ele a cota ficaria invisível e a emissão seria recusada ou grátis.
func TestNFComMeter(t *testing.T) {
	if MeterNFCom != "nfcom" {
		t.Fatalf("MeterNFCom = %q", MeterNFCom)
	}
	if !slices.Contains(DocumentMeters, MeterNFCom) {
		t.Error("DocumentMeters sem nfcom")
	}
	if MeterForTable["nfcoms"] != MeterNFCom {
		t.Errorf("MeterForTable[nfcoms] = %q", MeterForTable["nfcoms"])
	}
}

func TestNFComEmissionOptions(t *testing.T) {
	got := EmissionOptions(ModelNFCom)
	want := []string{repositories.EmissionNormal, repositories.EmissionOffline}
	if !slices.Equal(got, want) {
		t.Errorf("EmissionOptions(62) = %v, want %v (sem SVC, sem EPEC)", got, want)
	}
}

func TestNFComEnablementKnowsDocType(t *testing.T) {
	e := NewFiscalConfigEnablement(nil, nil, nil, nil, nil, nil, nil)
	if _, ok := e.repos[DocTypeNFCom]; !ok {
		t.Error("a configuração fiscal da NFCom não conta para a cota de empresas")
	}
}

// A troca automática de contingência não existe para a NFCom na v1: ligar a flag não faria nada e enganaria o dono.
func TestNFComConfigRefusesAutoContingency(t *testing.T) {
	s := &NfcomConfigService{}
	_, err := s.SetEmission(t.Context(), "ORG1", EmissionUpdate{Type: repositories.EmissionOffline, Reason: "Falha de comunicação com a SEFAZ", Auto: true, AutoType: repositories.EmissionOffline})
	if err == nil || !strings.Contains(err.Error(), "automática") {
		t.Fatalf("erro = %v", err)
	}
}
