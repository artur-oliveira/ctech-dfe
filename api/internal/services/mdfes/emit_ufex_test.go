package mdfes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/validation"
)

// M-05: EX é UF válida (TUf do XSD) em uf_start, uf_end e route.
func TestMdfeEmitBody_AceitaExterior(t *testing.T) {
	b := MdfeEmitBody{Modal: "rodoviario", UFStart: "SP", UFEnd: "EX", Route: []string{"EX"}}
	if p := validation.Struct(&b); p != nil {
		t.Fatalf("EX recusado: %v", p)
	}
	b.UFEnd = "XX"
	if validation.Struct(&b) == nil {
		t.Fatal("UF inexistente deveria ser recusada")
	}
}

// M-07: documento cancelado não entra no manifesto (F33/F40/F48).
func TestCheckDocUsable(t *testing.T) {
	if checkDocUsable("k", "cancelled") == nil {
		t.Fatal("cancelado deveria ser recusado")
	}
	if err := checkDocUsable("k", "authorized"); err != nil {
		t.Fatal(err)
	}
}

// m-01: infLotacao é choice CEP | latitude+longitude.
func TestInfLotacao_ChoiceCEPOuCoordenadas(t *testing.T) {
	lat, lon := "-23.5", "-46.6"
	if validateBulkCargo(&MdfeBulkCargo{CEPLoading: "01001000", LatLoading: &lat, LonLoading: &lon}) == nil {
		t.Fatal("CEP junto de coordenadas deveria ser recusado")
	}
	if validateBulkCargo(&MdfeBulkCargo{LatLoading: &lat}) == nil {
		t.Fatal("latitude sem longitude deveria ser recusada")
	}
	if err := validateBulkCargo(&MdfeBulkCargo{LatLoading: &lat, LonLoading: &lon, CEPUnloading: "01001000"}); err != nil {
		t.Fatal(err)
	}
	if got := lotacaoLocal("", &lat, &lon); got["CEP"] != nil || got["latitude"] != lat {
		t.Fatalf("coordenadas: %v", got)
	}
}

// m-07/m-08: limites de infPag do XSD.
func TestMdfePayment_LimitesXSD(t *testing.T) {
	base := func() MdfePayment {
		return MdfePayment{Components: []MdfePaymentComponent{{Type: "01", Value: "1.00"}}, ContractValue: "1.00", PaymentType: "0"}
	}
	long := strings.Repeat("a", 61)
	bad := []MdfePayment{base(), base(), base()}
	bad[0].AdvanceRequest = sp("0")
	bad[1].Bank.PIX = &long
	bad[2].ContractValue = "10.555"
	for i := range bad {
		if validation.Struct(&bad[i]) == nil {
			t.Errorf("caso %d deveria ser recusado", i)
		}
	}
	if p := base(); validation.Struct(&p) != nil {
		t.Error("base válida recusada")
	}
}
