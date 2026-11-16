package services

import (
	"slices"
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
	e := NewFiscalConfigEnablement(nil, nil, nil, nil, nil, nil)
	if _, ok := e.repos[DocTypeNFCom]; !ok {
		t.Error("a configuração fiscal da NFCom não conta para a cota de empresas")
	}
}
