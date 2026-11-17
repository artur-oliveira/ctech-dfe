package services

import (
	"slices"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// O BP-e é cobrado pelo medidor próprio, entra na limpeza de conta e na cota de empresas.
func TestBPeWiring(t *testing.T) {
	if MeterBPe != "bpe" || !slices.Contains(DocumentMeters, MeterBPe) || MeterForTable["bpes"] != MeterBPe {
		t.Error("medidor do BP-e não está ligado em DocumentMeters e MeterForTable")
	}
	if got := EmissionOptions(ModelBPe); !slices.Equal(got, []string{repositories.EmissionNormal, repositories.EmissionOffline}) {
		t.Errorf("EmissionOptions(63) = %v", got)
	}
	e := NewFiscalConfigEnablement(nil, nil, nil, nil, nil, nil, nil)
	if _, ok := e.repos[DocTypeBPe]; !ok {
		t.Error("a configuração fiscal do BP-e não conta para a cota de empresas")
	}
	if !slices.Contains(erasureDocTypes, "bpe") {
		t.Error("erasureDocTypes sem bpe: PDFs e XMLs no S3 sobrariam")
	}
	if serieConfigModels["organization_bpe_configs"] != ModelBPe {
		t.Error("serieConfigModels sem a configuração do BP-e: a série 63 nunca seria liberada")
	}
}
