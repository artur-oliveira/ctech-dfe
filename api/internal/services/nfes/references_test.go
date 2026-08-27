package nfes

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildNFrefChaveDeNotaDaBase(t *testing.T) {
	got := buildNFref([]map[string]any{
		{"kind": refKindNFe, "access_key": "22260811647612000197550010000000011100000015"},
	})
	want := []map[string]any{
		{"refNFe": "22260811647612000197550010000000011100000015"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestBuildNFrefNotaModelo1ForaDoSistema(t *testing.T) {
	got := buildNFref([]map[string]any{{
		"kind": refKindNF, "c_uf": "22", "aamm": "2608", "cnpj": "11647612000197",
		"mod": "01", "serie": "1", "n_nf": "42",
	}})
	inner, ok := got[0]["refNF"].(map[string]any)
	if !ok {
		t.Fatalf("refNF ausente: %v", got)
	}
	if inner["mod"] != "01" || inner["nNF"] != "42" || inner["AAMM"] != "2608" {
		t.Fatalf("refNF errado: %v", inner)
	}
}

func TestBuildNFrefCupomFiscal(t *testing.T) {
	got := buildNFref([]map[string]any{{
		"kind": refKindECF, "mod": "2D", "n_ecf": "001", "n_coo": "000123",
	}})
	inner := got[0]["refECF"].(map[string]any)
	if inner["mod"] != "2D" || inner["nECF"] != "001" || inner["nCOO"] != "000123" {
		t.Fatalf("refECF errado: %v", inner)
	}
}

func TestBuildNFrefVazioDevolveNil(t *testing.T) {
	if got := buildNFref(nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestBuildEnviNFeIncluiNFrefNoIde(t *testing.T) {
	org, receiver, productItems, payments := minimalEnviNFeArgs()
	result := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011000000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "4", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil, nil, nil,
		NormalEmission(nfModel55),
		docExtras{NFRefs: buildNFref([]map[string]any{
			{"kind": refKindNFe, "access_key": "22260811647612000197550010000000011100000015"},
		})},
	)
	infNFe := result["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
	ide := infNFe["ide"].(map[string]any)
	refs, ok := ide["NFref"].([]map[string]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("NFref ausente no ide: %v", ide["NFref"])
	}
	if refs[0]["refNFe"] != "22260811647612000197550010000000011100000015" {
		t.Fatalf("refNFe errado: %v", refs[0])
	}
}

func TestFinNFeExigeRef(t *testing.T) {
	for _, fin := range []string{"2", "3", "4"} {
		if !finNFeExigeRef[fin] {
			t.Errorf("finNFe %s deveria exigir NFref", fin)
		}
	}
	if finNFeExigeRef["1"] {
		t.Error("finNFe 1 (normal) não exige NFref")
	}
}
