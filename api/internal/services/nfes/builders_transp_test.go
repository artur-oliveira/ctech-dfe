package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

func emitParty() map[string]any {
	return buildPartyTransporta("11222333000181", true, "Emit Ltda", "111222333",
		map[string]any{"street": "Rua Emit", "city": "Sao Paulo", "state_federation": "SP"})
}

func destParty() map[string]any {
	return buildPartyTransporta("99888777000166", true, "Dest Ltda", "999888777",
		map[string]any{"street": "Rua Dest", "city": "Rio de Janeiro", "state_federation": "RJ"})
}

func transpNode(t *testing.T, modFrete string) map[string]any {
	t.Helper()
	transport := map[string]any{
		"mod_frete":       modFrete,
		"transporta_cnpj": "55555555000155",
		"transporta_nome": "Carrier Req",
	}
	return buildTransp(false, false, decimal.Zero, decimal.Zero, transport, emitParty(), destParty())
}

func TestBuildTransp_ModFrete3_UsesEmit(t *testing.T) {
	transp := transpNode(t, modFreteProprioRemetente)
	transporta := mapKey(t, transp, "transporta")
	if transporta["CNPJ"] != "11222333000181" {
		t.Fatalf("modFrete 3 expected emit CNPJ, got %v", transporta["CNPJ"])
	}
	if transporta["xNome"] != "Emit Ltda" || transporta["UF"] != "SP" {
		t.Fatalf("modFrete 3 expected emit data, got %v", transporta)
	}
}

func TestBuildTransp_ModFrete4_UsesDest(t *testing.T) {
	transp := transpNode(t, modFreteProprioDestinatario)
	transporta := mapKey(t, transp, "transporta")
	if transporta["CNPJ"] != "99888777000166" {
		t.Fatalf("modFrete 4 expected dest CNPJ, got %v", transporta["CNPJ"])
	}
	if transporta["xNome"] != "Dest Ltda" || transporta["UF"] != "RJ" {
		t.Fatalf("modFrete 4 expected dest data, got %v", transporta)
	}
}

func TestBuildTransp_OtherModFrete_UsesRequest(t *testing.T) {
	transp := transpNode(t, "1")
	transporta := mapKey(t, transp, "transporta")
	if transporta["CNPJ"] != "55555555000155" || transporta["xNome"] != "Carrier Req" {
		t.Fatalf("modFrete 1 expected request transporta, got %v", transporta)
	}
}
