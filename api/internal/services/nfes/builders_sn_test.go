package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// NT 2026.005, seção 2.3: o que cada emitente do Simples informa por item.
func TestSNRegimeFor_TabelaDaNT(t *testing.T) {
	cases := []struct {
		name    string
		crt, tp int
		want    snRegime
	}{
		{"CRT1 com regime regular", 1, 1, snRegime{}},
		{"CRT2 com regime regular", 2, 1, snRegime{}},
		{"CRT1 sem regime regular", 1, 0, snRegime{omitGIBSCBS: true, gIBSSN: true, gCBSSN: true}},
		{"CRT2 sem regime regular", 2, 0, snRegime{gCBSSN: true, zeroCBS: true}},
		{"CRT3 opcao pendente tipo 2", 3, 2, snRegime{gIBSSN: true, gCBSSN: true}},
		{"CRT3 opcao pendente tipo 3", 3, 3, snRegime{gCBSSN: true}},
		{"CRT3 opcao pendente tipo 4", 3, 4, snRegime{}},
		{"CRT3 comum", 3, 0, snRegime{}},
		{"MEI", 4, 0, snRegime{}},
	}
	for _, c := range cases {
		if got := snRegimeFor(c.crt, c.tp); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestIsSNIssuerEValidTpRegime(t *testing.T) {
	if !isSNIssuer(1, 0) || !isSNIssuer(2, 1) || !isSNIssuer(3, 2) || isSNIssuer(3, 0) || isSNIssuer(4, 0) {
		t.Error("isSNIssuer fora da NT 2026.005 §2.2")
	}
	if !validTpRegimeIBSCBS(1, 1) || validTpRegimeIBSCBS(1, 2) || !validTpRegimeIBSCBS(3, 4) || validTpRegimeIBSCBS(3, 1) || validTpRegimeIBSCBS(4, 1) {
		t.Error("validTpRegimeIBSCBS fora da NT 2026.005 §2.1")
	}
}

func snParams(crt, tp int, cfg map[string]any) ibsCBSParams {
	return ibsCBSParams{
		CST: "000", ClassTrib: "000001", VBC: decimal.RequireFromString("1000.00"),
		IBSUFAliq: "0.1000", IBSMunAliq: "0.0000", CBSAliq: "0.9000",
		Cfg: cfg, Quantity: decimal.NewFromInt(1), SN: snRegimeFor(crt, tp),
	}
}

// CRT 1 sem regime regular: sem gIBSCBS, com gTribSN dos dois tributos sobre a base do item.
func TestBuildIBSCBS_SimplesSemRegimeRegular(t *testing.T) {
	node := buildIBSCBS(snParams(1, 0, map[string]any{cfgPIBSSN: "0.50", cfgPCBSSN: "1.25"}))
	if _, ok := node["gIBSCBS"]; ok {
		t.Fatal("CRT 1 sem regime regular não informa gIBSCBS")
	}
	sn, _ := node["gTribSN"].(map[string]any)
	ibs, _ := sn["gIBSSN"].(map[string]any)
	cbs, _ := sn["gCBSSN"].(map[string]any)
	if ibs["pIBSSN"] != "0.5000" || ibs["vIBSSN"] != "5.00" || cbs["pCBSSN"] != "1.2500" || cbs["vCBSSN"] != "12.50" {
		t.Fatalf("gTribSN = %v", sn)
	}
}

// CRT 2 sem regime regular: gIBSCBS com a CBS zerada e só gCBSSN no Simples.
func TestBuildIBSCBS_SimplesExcessoSublimite(t *testing.T) {
	node := buildIBSCBS(snParams(2, 0, map[string]any{cfgPCBSSN: "1.00"}))
	g, ok := node["gIBSCBS"].(map[string]any)
	if !ok {
		t.Fatal("CRT 2 informa gIBSCBS")
	}
	if gCBS, _ := g["gCBS"].(map[string]any); gCBS["pCBS"] != "0.0000" {
		t.Errorf("CBS devia sair zerada: %v", gCBS)
	}
	sn, _ := node["gTribSN"].(map[string]any)
	if _, has := sn["gIBSSN"]; has || sn["gCBSSN"] == nil {
		t.Errorf("CRT 2 informa só gCBSSN: %v", sn)
	}
}

// Sem a alíquota efetiva configurada, o grupo não sai (a NT ainda não o obriga).
func TestBuildGTribSN_SemAliquotaNaoEmite(t *testing.T) {
	if node := buildGTribSN(snParams(1, 0, map[string]any{cfgPIBSSN: "0.50"})); node != nil {
		t.Fatalf("faltou pCBSSN, o grupo não deveria sair: %v", node)
	}
	if node := buildGTribSN(snParams(3, 4, nil)); node != nil {
		t.Fatalf("CRT 3 com tp 4 não tem gTribSN: %v", node)
	}
}

// Regras W59h-10 e W59h-20 (1275, 1276): o total do Simples só existe quando algum item o tem, e é a soma deles.
func TestIBSCBSTot_TotalDoSimplesSomaOsItens(t *testing.T) {
	tt := newTotals(decimal.RequireFromString("2000.00"), decimal.Zero)
	for _, vbc := range []string{"1000.00", "1000.00"} {
		p := snParams(1, 0, map[string]any{cfgPIBSSN: "0.50", cfgPCBSSN: "1.25"})
		p.VBC = decimal.RequireFromString(vbc)
		accumulateIBSCBS(&tt, buildIBSCBS(p), p.VBC)
	}
	got, _ := buildTotal(tt, time.Now(), nil)["IBSCBSTot"].(map[string]any)
	sn, _ := got["gTribSN"].(map[string]any)
	if sn["vIBSSN"] != "10.00" || sn["vCBSSN"] != "25.00" {
		t.Fatalf("gTribSN total = %v", sn)
	}

	semSN := newTotals(decimal.RequireFromString("1000.00"), decimal.Zero)
	accumulateIBSCBS(&semSN, buildIBSCBS(snParams(3, 0, nil)), decimal.RequireFromString("1000.00"))
	got, _ = buildTotal(semSN, time.Now(), nil)["IBSCBSTot"].(map[string]any)
	if _, has := got["gTribSN"]; has {
		t.Fatal("nenhum item tem gTribSN: o total não pode tê-lo (1276)")
	}
}

func TestBuildEmit_TpRegimeIBSCBS(t *testing.T) {
	org := map[string]any{"name": "Empresa", "tax_id": "12345678000190", "tax_id_kind": "cnpj"}
	person := map[string]any{"tp_regime_ibs_cbs": 1}
	if got := buildEmit(org, person, "org#1", "SP", "SP", 1)["tpRegimeIBSCBS"]; got != "1" {
		t.Errorf("CRT 1 com opção pelo regime regular: %v", got)
	}
	if _, has := buildEmit(org, person, "org#1", "SP", "SP", 4)["tpRegimeIBSCBS"]; has {
		t.Error("MEI não informa tpRegimeIBSCBS")
	}
	if _, has := buildEmit(org, map[string]any{}, "org#1", "SP", "SP", 3)["tpRegimeIBSCBS"]; has {
		t.Error("Regime Normal comum não informa tpRegimeIBSCBS")
	}
}
