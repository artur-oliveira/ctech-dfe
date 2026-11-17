package services

import (
	"slices"
	"testing"
)

// A NFCom entra na limpeza da conta: PDFs e XMLs no S3 e a série de modelo 62 reivindicada pela configuração.
func TestErasureCoversNFCom(t *testing.T) {
	if !slices.Contains(erasureDocTypes, "nfcom") {
		t.Error("erasureDocTypes sem nfcom: os PDFs e XMLs no S3 sobrariam")
	}
	if serieConfigModels["organization_nfcom_configs"] != ModelNFCom {
		t.Error("serieConfigModels sem a configuração da NFCom: a série 62 nunca seria liberada")
	}
}
