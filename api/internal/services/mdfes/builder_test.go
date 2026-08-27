package mdfes

import (
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// baseParams returns a minimal valid buildParams for rodoviário emission.
func baseParams(owner *resolvedOwner) buildParams {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	return buildParams{
		org:         testOrg(),
		orgPK:       "CNPJ_12345678000190",
		accessKey:   services.GenerateAccessKey("SP", "12345678000190", services.ModelMDFe, 1, 1, now, services.TpEmisNormal),
		serie:       1,
		number:      1,
		environment: 2,
		now:         now,
		modal:       ModalRodoviario,
		cargo:       buildTestCargo(),
		vehicle:     resolvedVehicle{Placa: "ABC1D23", Tara: "10000", UF: "SP", TpRod: "01", TpCar: "00", RNTRC: "12345678"},
		owner:       owner,
		drivers:     []MdfeDriver{{Name: "MOTORISTA", CPF: "111.222.333-44"}},
	}
}

func ide(t *testing.T, p buildParams) map[string]any {
	t.Helper()
	out := BuildMDFe(p)
	return out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["ide"].(map[string]any)
}

func veicTracao(t *testing.T, p buildParams) map[string]any {
	t.Helper()
	out := BuildMDFe(p)
	infModal := out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["infModal"].(map[string]any)
	return infModal["rodo"].(map[string]any)["veicTracao"].(map[string]any)
}

// TestBuildRodo_IncludesVeicReboque verifies trailers passed via
// buildParams.trailers are emitted as a veicReboque list alongside veicTracao.
func TestBuildRodo_IncludesVeicReboque(t *testing.T) {
	p := baseParams(nil)
	p.trailers = []resolvedVehicle{
		{Placa: "XYZ1A23", Tara: "5000", TpCar: "01", CapKG: "9000"},
	}
	rodo := p.buildRodo()
	reboques, ok := rodo["veicReboque"].([]map[string]any)
	if !ok || len(reboques) != 1 {
		t.Fatalf("veicReboque = %v, want 1-item list", rodo["veicReboque"])
	}
	if reboques[0]["placa"] != "XYZ1A23" || reboques[0]["capKG"] != "9000" {
		t.Errorf("veicReboque[0] = %v, want placa=XYZ1A23 capKG=9000", reboques[0])
	}
}

// TestBuildRodo_NoTrailersOmitsVeicReboque confirms the XSD's minOccurs="0" on
// veicReboque is respected when no trailers are supplied.
func TestBuildRodo_NoTrailersOmitsVeicReboque(t *testing.T) {
	p := baseParams(nil)
	rodo := p.buildRodo()
	if _, ok := rodo["veicReboque"]; ok {
		t.Error("veicReboque must be absent when no trailers are supplied")
	}
}

// TestOwnVehicle_NoTpTranspNoProp is the regression test for the SEFAZ rejection
// "Não é permitido informar o campo tpTransp se o proprietário do veículo não
// for informado." (rule F25/cStat 745). With no owner, neither tpTransp nor the
// prop group may be present.
func TestOwnVehicle_NoTpTranspNoProp(t *testing.T) {
	p := baseParams(nil)
	if v, ok := ide(t, p)["tpTransp"]; ok {
		t.Errorf("tpTransp must be absent for own vehicle (carga própria), got %v", v)
	}
	if _, ok := veicTracao(t, p)["prop"]; ok {
		t.Error("prop group must be absent when the vehicle belongs to the emitter")
	}
}

// TestCPFOwner_TAC: a CPF owner ⇒ tpTransp=TAC(2) and a prop group with CPF.
func TestCPFOwner_TAC(t *testing.T) {
	p := baseParams(&resolvedOwner{CPF: "98765432100", Name: "JOAO TAC", RNTRC: "87654321"})
	if got := ide(t, p)["tpTransp"]; got != tpTranspTAC {
		t.Errorf("tpTransp = %v, want TAC %q", got, tpTranspTAC)
	}
	prop := veicTracao(t, p)["prop"].(map[string]any)
	if prop["CPF"] != "98765432100" {
		t.Errorf("prop.CPF = %v, want 98765432100", prop["CPF"])
	}
	if _, hasCNPJ := prop["CNPJ"]; hasCNPJ {
		t.Error("prop must not carry CNPJ for a CPF owner")
	}
	if prop["RNTRC"] != "87654321" || prop["tpProp"] != tpPropOutros {
		t.Errorf("prop RNTRC/tpProp = %v/%v", prop["RNTRC"], prop["tpProp"])
	}
}

// TestCNPJOwner_ETC: a CNPJ owner defaults to tpTransp=ETC(1).
func TestCNPJOwner_ETC(t *testing.T) {
	p := baseParams(&resolvedOwner{CNPJ: "99888777000166", Name: "TRANSP LTDA", RNTRC: "11112222"})
	if got := ide(t, p)["tpTransp"]; got != tpTranspETC {
		t.Errorf("tpTransp = %v, want ETC %q", got, tpTranspETC)
	}
	prop := veicTracao(t, p)["prop"].(map[string]any)
	if prop["CNPJ"] != "99888777000166" {
		t.Errorf("prop.CNPJ = %v, want 99888777000166", prop["CNPJ"])
	}
}

// TestCNPJOwner_CTCOverride: a CNPJ owner may opt into CTC(3).
func TestCNPJOwner_CTCOverride(t *testing.T) {
	p := baseParams(&resolvedOwner{CNPJ: "99888777000166", Name: "COOP", RNTRC: "11112222", TpTransp: tpTranspCTC})
	if got := ide(t, p)["tpTransp"]; got != tpTranspCTC {
		t.Errorf("tpTransp = %v, want CTC %q", got, tpTranspCTC)
	}
}

func TestModalCode(t *testing.T) {
	cases := map[string]string{
		ModalRodoviario:  "1",
		ModalAereo:       "2",
		ModalAquaviario:  "3",
		ModalFerroviario: "4",
		"":               "1", // unknown/empty defaults to rodoviário
	}
	for modal, want := range cases {
		if got := modalCode(modal); got != want {
			t.Errorf("modalCode(%q) = %q, want %q", modal, got, want)
		}
	}
}

func TestModalDispatch_Aereo(t *testing.T) {
	p := baseParams(nil)
	p.modal = ModalAereo
	p.air = &MdfeAirModal{Nac: "PRABC", Matr: "1234", NVoo: "AB123", CAerEmb: "GRU", CAerDes: "GIG", DVoo: "2026-06-16"}
	out := BuildMDFe(p)
	infModal := out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["infModal"].(map[string]any)
	if _, ok := infModal["rodo"]; ok {
		t.Error("aéreo modal must not emit rodo node")
	}
	aereo, ok := infModal["aereo"].(map[string]any)
	if !ok {
		t.Fatal("aereo node missing")
	}
	if aereo["cAerEmb"] != "GRU" || aereo["cAerDes"] != "GIG" {
		t.Errorf("aereo origin/dest = %v/%v, want GRU/GIG", aereo["cAerEmb"], aereo["cAerDes"])
	}
}

func TestModalDispatch_Ferroviario(t *testing.T) {
	p := baseParams(nil)
	p.modal = ModalFerroviario
	p.rail = &MdfeRailModal{
		XPref: "PREF1", XOri: "ORI", XDest: "DEST",
		Wagons: []MdfeRailWagon{
			{PesoBC: "10.000", PesoR: "9.500", Serie: "S1", NVag: "1001", TU: "9.500"},
			{PesoBC: "11.000", PesoR: "10.500", Serie: "S1", NVag: "1002", TU: "10.500"},
		},
	}
	out := BuildMDFe(p)
	infModal := out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["infModal"].(map[string]any)
	ferrov := infModal["ferrov"].(map[string]any)
	trem := ferrov["trem"].(map[string]any)
	if trem["qVag"] != "2" {
		t.Errorf("trem.qVag = %v, want 2", trem["qVag"])
	}
	if vags := ferrov["vag"].([]map[string]any); len(vags) != 2 {
		t.Errorf("vag count = %d, want 2", len(vags))
	}
}

// TestResolveOwner enforces the validation rules (exactly one doc, required
// fields, and owner ≠ emitter — SEFAZ F21).
func TestResolveOwner(t *testing.T) {
	const orgPK = "CNPJ_12345678000190"

	if o, err := resolveOwner(nil, orgPK); err != nil || o != nil {
		t.Errorf("nil owner: got (%v,%v), want (nil,nil)", o, err)
	}
	if _, err := resolveOwner(&MdfeOwner{CPF: "1", CNPJ: "2", Name: "X", RNTRC: "1"}, orgPK); err == nil {
		t.Error("expected error when both CPF and CNPJ are set")
	}
	if _, err := resolveOwner(&MdfeOwner{CNPJ: "99888777000166", Name: "X"}, orgPK); err == nil {
		t.Error("expected error when RNTRC missing")
	}
	if _, err := resolveOwner(&MdfeOwner{CNPJ: "12345678000190", Name: "SELF", RNTRC: "1"}, orgPK); err == nil {
		t.Error("expected error when owner equals emitter (F21)")
	}
	o, err := resolveOwner(&MdfeOwner{CPF: "987.654.321-00", Name: "OK", RNTRC: "87654321"}, orgPK)
	if err != nil {
		t.Fatalf("valid owner: %v", err)
	}
	if o.CPF != "98765432100" {
		t.Errorf("CPF not normalised: %q", o.CPF)
	}
}

func TestBuildMDFeLacres(t *testing.T) {
	p := baseParams(nil)
	p.seals = []string{"L1", "L2"}
	p.rodoSeals = []string{"R1"}
	p.portAgentCode = "AG-9"
	inf := BuildMDFe(p)["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
	if len(inf["lacres"].([]map[string]any)) != 2 {
		t.Fatalf("lacres da carga ausentes: %v", inf["lacres"])
	}
	rodo := inf["infModal"].(map[string]any)["rodo"].(map[string]any)
	if len(rodo["lacRodo"].([]map[string]any)) != 1 {
		t.Fatalf("lacRodo ausente: %v", rodo)
	}
	if rodo["codAgPorto"] != "AG-9" {
		t.Fatalf("codAgPorto ausente: %v", rodo)
	}
}

// Sem lacre nenhum os nós não existem — lista vazia o XSD recusa.
func TestBuildMDFeSemLacres(t *testing.T) {
	inf := BuildMDFe(baseParams(nil))["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
	if _, ok := inf["lacres"]; ok {
		t.Fatalf("lacres não devia existir: %v", inf["lacres"])
	}
	rodo := inf["infModal"].(map[string]any)["rodo"].(map[string]any)
	if _, ok := rodo["lacRodo"]; ok {
		t.Fatalf("lacRodo não devia existir: %v", rodo)
	}
}

// TestBuildIde_CanalVerdeECargaPosterior verifica os dois indicadores da
// configuração: presentes só quando ligados, e sempre com o único valor que o
// XSD aceita ("1").
func TestBuildIde_CanalVerdeECargaPosterior(t *testing.T) {
	p := baseParams(nil)
	if got := ide(t, p); got["indCanalVerde"] != nil || got["indCarregaPosterior"] != nil {
		t.Fatalf("indicadores desligados não devem sair: %v", got)
	}

	p.indCanalVerde = true
	p.indCarregaPosterior = true
	got := ide(t, p)
	if got["indCanalVerde"] != "1" || got["indCarregaPosterior"] != "1" {
		t.Fatalf("indicadores = %v/%v, want 1/1", got["indCanalVerde"], got["indCarregaPosterior"])
	}
}

// TestBuildInfAdic_FiscoECpl confirma que a mensagem ao fisco (da configuração)
// e a mensagem complementar (da viagem) convivem no mesmo infAdic.
func TestBuildInfAdic_FiscoECpl(t *testing.T) {
	p := baseParams(nil)
	cpl := "CARGA CONSOLIDADA"
	p.addInfo = &cpl
	p.infAdFisco = "REGIME ESPECIAL 123"

	out := BuildMDFe(p)
	infAdic, ok := out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["infAdic"].(map[string]any)
	if !ok {
		t.Fatalf("infAdic ausente: %v", out)
	}
	if infAdic["infAdFisco"] != "REGIME ESPECIAL 123" || infAdic["infCpl"] != cpl {
		t.Fatalf("infAdic = %v", infAdic)
	}
}

// TestBuildInfAdic_SoFisco cobre a configuração com mensagem ao fisco e a
// viagem sem observação: o grupo existe mesmo assim.
func TestBuildInfAdic_SoFisco(t *testing.T) {
	p := baseParams(nil)
	p.infAdFisco = "REGIME ESPECIAL 123"
	out := BuildMDFe(p)
	infAdic, ok := out["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["infAdic"].(map[string]any)
	if !ok || infAdic["infAdFisco"] != "REGIME ESPECIAL 123" {
		t.Fatalf("infAdic = %v", out["MDFe"])
	}
	if _, has := infAdic["infCpl"]; has {
		t.Fatalf("infCpl não deveria existir: %v", infAdic)
	}
}

// TestBuildProdPred_CEAN confirma que o GTIN do produto predominante é
// derivado do documento referenciado, nunca perguntado por viagem.
func TestBuildProdPred_CEAN(t *testing.T) {
	p := baseParams(nil)
	p.cargo.prodPred.CEAN = "7891234567895"
	if got := p.buildProdPred()["cEAN"]; got != "7891234567895" {
		t.Fatalf("cEAN = %v", got)
	}

	p.cargo.prodPred.CEAN = ""
	if _, has := p.buildProdPred()["cEAN"]; has {
		t.Fatal("sem GTIN no documento, cEAN não deve sair")
	}
}
