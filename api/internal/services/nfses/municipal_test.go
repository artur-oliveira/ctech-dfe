package nfses

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

func TestCacheKeyMunicipalParams_ExcludesTenant(t *testing.T) {
	args := []string{"2211001", "10101", "2026-08"}
	k1 := cacheKeyMunicipalParams(nacional.ParamAliquota, args)
	k2 := cacheKeyMunicipalParams(nacional.ParamAliquota, args)
	if k1 != k2 {
		t.Fatalf("chave não determinística: %q != %q", k1, k2)
	}
	if k1 == cacheKeyMunicipalParams(nacional.ParamAliquota, []string{"3550308", "10101", "2026-08"}) {
		t.Error("municípios diferentes geraram a mesma chave")
	}
	// Os parâmetros são públicos por município/competência: incluir o tenant
	// faria cada organização pagar a mesma consulta ao ADN.
	for _, tenant := range []string{"CNPJ_", "CPF_", "prod#", "hom#"} {
		if strings.Contains(k1, tenant) {
			t.Errorf("chave %q carrega contexto de tenant (%q)", k1, tenant)
		}
	}
	if municipalParamsTTL != 6*60*60 {
		t.Errorf("TTL = %d, esperado 21600 (6h)", municipalParamsTTL)
	}
}

func TestMunicipalParamKind_Validation(t *testing.T) {
	if err := validateParamKind(nacional.ParamAliquota, []string{"2211001", "10101", "2026-08"}); err != nil {
		t.Errorf("aliquota válida rejeitada: %v", err)
	}
	if err := validateParamKind(nacional.ParamAliquota, []string{"2211001"}); err == nil {
		t.Error("aridade errada aceita")
	}
	if err := validateParamKind("inexistente", nil); err == nil {
		t.Error("tipo desconhecido aceito")
	}
	// A aridade é a tabela do go-dfe, não uma cópia: todo tipo aceito lá tem
	// que ser aceito aqui com a mesma contagem de argumentos.
	for kind, arity := range nacional.ParamArity {
		if err := validateParamKind(kind, make([]string, arity)); err != nil {
			t.Errorf("tipo %q com %d argumentos rejeitado: %v", kind, arity, err)
		}
	}
}

func TestGetDANFSE_AbrasfNotImplemented(t *testing.T) {
	// Não existe PDF padrão no leiaute ABRASF 2.04 (spec §11): o erro tem que
	// ser 501, não um 500 genérico nem um PDF vazio.
	err := danfseSupported("abrasf204")
	if err == nil {
		t.Fatal("abrasf204 aceito para DANFSE")
	}
	if err.Status != 501 {
		t.Errorf("status = %d, esperado 501", err.Status)
	}
	if danfseSupported("nacional") != nil {
		t.Error("provider nacional rejeitado para DANFSE")
	}
}
