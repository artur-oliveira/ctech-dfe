package nfes

import (
	"reflect"
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
	return buildTransp(false, false, decimal.Zero, decimal.Zero, transport, emitParty(), destParty(), nil, nil)
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

func TestBuildTranspVolumesELacres(t *testing.T) {
	transport := map[string]any{"mod_frete": "1", "veiculo_placa": "ABC1D23", "veiculo_uf": "PI", "veiculo_rntrc": "12345678"}
	vols := []map[string]any{{
		"qVol": "2", "esp": "CAIXA", "marca": "ACME", "nVol": "001/002",
		"pesoL": "10.000", "pesoB": "12.000",
		"lacres": []map[string]any{{"nLacre": "L1"}, {"nLacre": "L2"}},
	}}
	reboques := []map[string]any{{"placa": "XYZ9Z99", "UF": "PI", "RNTC": "87654321"}}

	got := buildTransp(false, false, decimal.Zero, decimal.Zero, transport, nil, nil, vols, reboques)

	if got["veicTransp"].(map[string]any)["RNTC"] != "12345678" {
		t.Fatalf("RNTC ausente: %v", got["veicTransp"])
	}
	if _, ok := got["veicTransp"].(map[string]any)["RNTRC"]; ok {
		t.Fatal("RNTRC não existe no leiaute da NF-e — a tag é RNTC")
	}
	if !reflect.DeepEqual(got["reboque"], reboques) {
		t.Fatalf("reboque errado: %v", got["reboque"])
	}
	v := got["vol"].([]map[string]any)[0]
	if v["esp"] != "CAIXA" || v["nVol"] != "001/002" || len(v["lacres"].([]map[string]any)) != 2 {
		t.Fatalf("vol incompleto: %v", v)
	}
}

// Sem volume explícito, o comportamento antigo continua: um volume só, com peso.
func TestBuildTranspVolumeDerivadoDosPesos(t *testing.T) {
	got := buildTransp(true, true, decimal.RequireFromString("3.5"), decimal.RequireFromString("4.0"),
		map[string]any{"mod_frete": "0"}, nil, nil, nil, nil)
	vols := got["vol"].([]map[string]any)
	if len(vols) != 1 || vols[0]["qVol"] != qVolPadrao || vols[0]["pesoL"] != "3.500" {
		t.Fatalf("volume derivado errado: %v", vols)
	}
}
