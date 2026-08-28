package nfes

import (
	"strings"
	"testing"
	"time"
)

// ── Task 41: dhSaiEnt, dPrevEntrega e indIntermed em ide ─────────────────────

func baseIdeParams() ideParams {
	return ideParams{
		CUF: "22", CNF: "12345678", NatOp: "VENDA", Model: "55", AccessKey: "0",
		Serie: 1, Number: 1, Environment: 2, DhEmi: "2026-09-10T10:00:00-03:00",
		TpNF: "1", IdDest: "1", CMunFG: "2211001",
		FinNFe: "1", IndFinal: "1", IndPres: "1", VerProc: "test",
	}
}

// A data de saída é derivada do offset cadastrado na operação, contado da
// emissão — ninguém digita "amanhã" a cada nota.
func TestBuildIdeDhSaiEntDerivadaDoOffset(t *testing.T) {
	p := baseIdeParams()
	p.DhSaiEnt = "2026-09-11T10:00:00-03:00"
	p.DPrevEntrega = "2026-09-15"
	ide := buildIde(p)
	if ide["dhSaiEnt"] != "2026-09-11T10:00:00-03:00" {
		t.Fatalf("dhSaiEnt ausente: %v", ide)
	}
	if ide["dPrevEntrega"] != "2026-09-15" {
		t.Fatalf("dPrevEntrega ausente: %v", ide)
	}
}

// Sem os campos, as tags não existem — nó opcional vazio é rejeição.
func TestBuildIdeSemDatasOpcionais(t *testing.T) {
	ide := buildIde(baseIdeParams())
	for _, tag := range []string{"dhSaiEnt", "dPrevEntrega", "indIntermed"} {
		if _, ok := ide[tag]; ok {
			t.Fatalf("%s não pode aparecer sem valor: %v", tag, ide)
		}
	}
}

func TestBuildIdeIndIntermed(t *testing.T) {
	p := baseIdeParams()
	p.IndIntermed = "1"
	if got := buildIde(p)["indIntermed"]; got != "1" {
		t.Fatalf("indIntermed errado: %v", got)
	}
}

// ── dhSaiEnt derivada: offset em dias sobre a emissão ────────────────────────

func TestResolveDhSaiEntOffsetDaOperacao(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	op := map[string]any{"dh_sai_ent_offset_days": 1.0}
	got := resolveDhSaiEnt(op, nil, now)
	if !strings.HasPrefix(got, "2026-09-11T10:00:00") {
		t.Fatalf("offset de 1 dia não aplicado: %q", got)
	}
}

// O valor explícito da nota vence o offset da operação.
func TestResolveDhSaiEntRequestVence(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	op := map[string]any{"dh_sai_ent_offset_days": 5.0}
	explicit := "2026-09-20T08:00:00-03:00"
	if got := resolveDhSaiEnt(op, &explicit, now); got != explicit {
		t.Fatalf("request tinha que vencer a operação: %q", got)
	}
}

// Sem offset e sem valor explícito, a tag não existe: a maioria das notas não
// declara data de saída.
func TestResolveDhSaiEntAusente(t *testing.T) {
	if got := resolveDhSaiEnt(map[string]any{}, nil, time.Now()); got != "" {
		t.Fatalf("want vazio, got %q", got)
	}
	if got := resolveDhSaiEnt(nil, nil, time.Now()); got != "" {
		t.Fatalf("operação nula não pode derivar data: %q", got)
	}
}

// ── infIntermed ──────────────────────────────────────────────────────────────

func TestBuildInfIntermed(t *testing.T) {
	person := map[string]any{"sk": "CNPJ_11647612000197", "intermediary_id": "LOJA-42"}
	got := buildInfIntermed(person)
	if got["CNPJ"] != "11647612000197" || got["idCadIntTran"] != "LOJA-42" {
		t.Fatalf("infIntermed errado: %v", got)
	}
}

// Os dois campos são obrigatórios no XSD: faltando um, o grupo não sai.
func TestBuildInfIntermedIncompletoDevolveNil(t *testing.T) {
	if buildInfIntermed(map[string]any{"sk": "CNPJ_11647612000197"}) != nil {
		t.Fatal("sem idCadIntTran o grupo não pode existir")
	}
	if buildInfIntermed(map[string]any{"intermediary_id": "LOJA-42"}) != nil {
		t.Fatal("sem CNPJ o grupo não pode existir")
	}
	if buildInfIntermed(nil) != nil {
		t.Fatal("sem intermediador o grupo não pode existir")
	}
}

// Intermediador é pessoa jurídica: CPF não serve (TCnpj no XSD).
func TestBuildInfIntermedRecusaCPF(t *testing.T) {
	person := map[string]any{"sk": "CPF_11144477735", "intermediary_id": "LOJA-42"}
	if buildInfIntermed(person) != nil {
		t.Fatal("intermediador com CPF não pode virar nó (o XSD exige CNPJ)")
	}
}

// ── Task 42: ide da reforma tributária ───────────────────────────────────────

func TestBuildIdeCamposDaReforma(t *testing.T) {
	p := baseIdeParams()
	p.CIndOp = "220110"
	p.CMunFGIBS = "2211001"
	p.TpNFDebito = "1"
	p.TpNFCredito = "0"
	ide := buildIde(p)
	if ide["cIndOp"] != "220110" || ide["cMunFGIBS"] != "2211001" {
		t.Fatalf("local da operação errado: %v", ide)
	}
	if ide["tpNFDebito"] != "1" || ide["tpNFCredito"] != "0" {
		t.Fatalf("notas de débito/crédito erradas: %v", ide)
	}
}

// Nenhum campo da reforma aparece numa nota comum.
func TestBuildIdeSemCamposDaReforma(t *testing.T) {
	ide := buildIde(baseIdeParams())
	for _, tag := range []string{"cIndOp", "cMunFGIBS", "tpNFDebito", "tpNFCredito", "gCompraGov", "gPagAntecipado"} {
		if _, ok := ide[tag]; ok {
			t.Fatalf("%s não pode aparecer sem valor: %v", tag, ide)
		}
	}
}

func TestBuildIdeCompraGovEAntecipacao(t *testing.T) {
	p := baseIdeParams()
	p.CompraGov = map[string]any{
		"tpEnteGov": "1", "pRedutor": "20.0000", "tpOperGov": "3",
		"refDFeAnt": []string{"22260111647612000197550010000000011000000010"},
	}
	p.PagAntecipado = []string{"22260111647612000197550010000000021000000029"}
	ide := buildIde(p)
	gov := ide["gCompraGov"].(map[string]any)
	if gov["tpEnteGov"] != "1" || gov["tpOperGov"] != "3" {
		t.Fatalf("gCompraGov errado: %v", gov)
	}
	ant := ide["gPagAntecipado"].(map[string]any)
	if len(ant["refNFe"].([]string)) != 1 {
		t.Fatalf("gPagAntecipado errado: %v", ant)
	}
}

// ── buildCompraGov: refDFeAnt só existe em tpOperGov 2 e 3 ──────────────────

func TestBuildCompraGovRefDFeAnt(t *testing.T) {
	op := map[string]any{
		"compra_gov_tp_ente": "2", "compra_gov_p_redutor": "20.0000", "compra_gov_tp_oper": "3",
	}
	refs := []string{
		"22260111647612000197550010000000011000000010",
		"22260111647612000197550010000000021000000029",
	}
	got, err := buildCompraGov(op, refs)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["tpEnteGov"] != "2" || got["pRedutor"] != "20.0000" || got["tpOperGov"] != "3" {
		t.Fatalf("grupo errado: %v", got)
	}
	if len(got["refDFeAnt"].([]string)) != 2 {
		t.Fatalf("tpOperGov 3 aceita várias chaves: %v", got)
	}
}

// tpOperGov 2 aceita uma chave só; duas é erro do XSD adiado para a SEFAZ.
func TestBuildCompraGovTipo2AceitaUmaChave(t *testing.T) {
	op := map[string]any{"compra_gov_tp_ente": "1", "compra_gov_p_redutor": "0.0000", "compra_gov_tp_oper": "2"}
	if _, err := buildCompraGov(op, []string{
		"22260111647612000197550010000000011000000010",
		"22260111647612000197550010000000021000000029",
	}); err == nil {
		t.Fatal("tpOperGov 2 com duas chaves tinha que falhar")
	}
	if _, err := buildCompraGov(op, nil); err == nil {
		t.Fatal("tpOperGov 2 sem chave tinha que falhar")
	}
}

// tpOperGov 1 e 4 proíbem refDFeAnt.
func TestBuildCompraGovTipos1e4ProibemRef(t *testing.T) {
	for _, tp := range []string{"1", "4"} {
		op := map[string]any{"compra_gov_tp_ente": "1", "compra_gov_p_redutor": "0.0000", "compra_gov_tp_oper": tp}
		if _, err := buildCompraGov(op, []string{"22260111647612000197550010000000011000000010"}); err == nil {
			t.Fatalf("tpOperGov %s com chave referenciada tinha que falhar", tp)
		}
		got, err := buildCompraGov(op, nil)
		if err != nil {
			t.Fatalf("tpOperGov %s sem chave é válido: %v", tp, err)
		}
		if _, ok := got["refDFeAnt"]; ok {
			t.Fatalf("tpOperGov %s não leva refDFeAnt: %v", tp, got)
		}
	}
}

// Operação que não é compra governamental não produz o grupo.
func TestBuildCompraGovAusente(t *testing.T) {
	got, err := buildCompraGov(map[string]any{}, nil)
	if got != nil || err != nil {
		t.Fatalf("sem ente governamental não há grupo: %v %v", got, err)
	}
	if _, err := buildCompraGov(map[string]any{"compra_gov_tp_ente": "1"}, nil); err == nil {
		t.Fatal("ente sem tipo de operação tinha que falhar")
	}
}
