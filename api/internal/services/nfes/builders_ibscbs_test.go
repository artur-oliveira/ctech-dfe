package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

// baseIBSCBS é o item tributado normal: CST 000, alíquotas cheias e base 1000.
func baseIBSCBS(cfg map[string]any) ibsCBSParams {
	return ibsCBSParams{
		CST: "000", ClassTrib: "000001",
		VBC:       decimal.RequireFromString("1000.00"),
		IBSUFAliq: "10.0000", IBSMunAliq: "2.0000", CBSAliq: "8.0000",
		Cfg: cfg, Quantity: decimal.RequireFromString("100.0000"),
		CompetApur: "2026-09",
	}
}

func gIBSCBSOf(t *testing.T, node map[string]any) map[string]any {
	t.Helper()
	inner, ok := node["gIBSCBS"].(map[string]any)
	if !ok {
		t.Fatalf("gIBSCBS ausente: %v", node)
	}
	return inner
}

func sub(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	inner, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s ausente em %v", key, parent)
	}
	return inner
}

// ── Base: gIBSCBS com os três nós de alíquota ───────────────────────────────

func TestBuildIBSCBSValoresDerivadosDasAliquotas(t *testing.T) {
	got := buildIBSCBS(baseIBSCBS(nil))
	if got["CST"] != "000" || got["cClassTrib"] != "000001" {
		t.Fatalf("cabeçalho errado: %v", got)
	}
	inner := gIBSCBSOf(t, got)
	if inner["vBC"] != "1000.00" {
		t.Fatalf("vBC errado: %v", inner)
	}
	if sub(t, inner, "gIBSUF")["vIBSUF"] != "100.00" {
		t.Fatalf("vIBSUF errado: %v", inner["gIBSUF"])
	}
	if sub(t, inner, "gIBSMun")["vIBSMun"] != "20.00" {
		t.Fatalf("vIBSMun errado: %v", inner["gIBSMun"])
	}
	if inner["vIBS"] != "120.00" {
		t.Fatalf("vIBS não é a soma das duas esferas: %v", inner)
	}
	if sub(t, inner, "gCBS")["vCBS"] != "80.00" {
		t.Fatalf("vCBS errado: %v", inner["gCBS"])
	}
}

// indDoacao só aceita "1" — "S" era o domínio de uma NT anterior.
func TestBuildIBSCBSIndDoacaoSoAceita1(t *testing.T) {
	if got := buildIBSCBS(baseIBSCBS(map[string]any{"ibs_ind_doacao": "1"})); got["indDoacao"] != "1" {
		t.Fatalf("indDoacao=1 tinha que sair: %v", got)
	}
	for _, v := range []string{"S", "N", "0", ""} {
		got := buildIBSCBS(baseIBSCBS(map[string]any{"ibs_ind_doacao": v}))
		if _, ok := got["indDoacao"]; ok {
			t.Fatalf("indDoacao=%q não pode ser emitido: %v", v, got)
		}
	}
}

// CST exento sai só com o cabeçalho: nó de valores vazio é rejeição.
func TestBuildIBSCBSCSTExento(t *testing.T) {
	p := baseIBSCBS(nil)
	p.CST = "400"
	got := buildIBSCBS(p)
	if _, ok := got["gIBSCBS"]; ok {
		t.Fatalf("CST 400 não declara valores: %v", got)
	}
}

// ── Task 48: gDevTrib e pDevTrib nas três esferas ───────────────────────────

func TestBuildIBSCBSDevTribNasTresEsferas(t *testing.T) {
	got := buildIBSCBS(baseIBSCBS(map[string]any{"ibs_cbs_p_dev_trib": "50.0000"}))
	inner := gIBSCBSOf(t, got)
	for _, tc := range []struct {
		group, want string
	}{
		{"gIBSUF", "50.00"},  // 50% de 100.00
		{"gIBSMun", "10.00"}, // 50% de 20.00
		{"gCBS", "40.00"},    // 50% de 80.00
	} {
		dev := sub(t, sub(t, inner, tc.group), "gDevTrib")
		if dev["pDevTrib"] != "50.0000" {
			t.Fatalf("%s: pDevTrib ausente: %v", tc.group, dev)
		}
		if dev["vDevTrib"] != tc.want {
			t.Fatalf("%s: vDevTrib = %v, want %s", tc.group, dev["vDevTrib"], tc.want)
		}
	}
}

// Sem percentual cadastrado, gDevTrib não existe em esfera nenhuma.
func TestBuildIBSCBSSemDevTrib(t *testing.T) {
	inner := gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(nil)))
	for _, group := range []string{"gIBSUF", "gIBSMun", "gCBS"} {
		if _, ok := sub(t, inner, group)["gDevTrib"]; ok {
			t.Fatalf("%s não pode ter gDevTrib sem percentual", group)
		}
	}
}

// gDif e gRed: o diferido é percentual sobre o tributo, e a alíquota efetiva é
// a alíquota menos a redução.
func TestBuildIBSCBSDiferimentoEReducao(t *testing.T) {
	inner := gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(map[string]any{
		"ibs_uf_p_dif": "30.0000", "cbs_p_red": "25.0000",
	})))
	dif := sub(t, sub(t, inner, "gIBSUF"), "gDif")
	if dif["pDif"] != "30.0000" || dif["vDif"] != "30.00" {
		t.Fatalf("gDif errado: %v", dif)
	}
	red := sub(t, sub(t, inner, "gCBS"), "gRed")
	if red["pRedAliq"] != "25.0000" || red["pAliqEfet"] != "6.0000" {
		t.Fatalf("gRed errado: %v", red)
	}
}

// ── Task 44: gTribRegular e gTribCompraGov ─────────────────────────────────

func TestBuildTribRegular(t *testing.T) {
	inner := gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(map[string]any{
		"ibs_reg_cst": "000", "ibs_reg_class_trib": "000001",
		"ibs_reg_uf_aliq": "12.0000", "ibs_reg_mun_aliq": "3.0000", "cbs_reg_aliq": "9.0000",
	})))
	reg := sub(t, inner, "gTribRegular")
	if reg["CSTReg"] != "000" || reg["cClassTribReg"] != "000001" {
		t.Fatalf("classificação de referência errada: %v", reg)
	}
	if reg["pAliqEfetRegIBSUF"] != "12.0000" || reg["vTribRegIBSUF"] != "120.00" {
		t.Fatalf("IBS-UF de referência errado: %v", reg)
	}
	if reg["pAliqEfetRegIBSMun"] != "3.0000" || reg["vTribRegIBSMun"] != "30.00" {
		t.Fatalf("IBS-Mun de referência errado: %v", reg)
	}
	if reg["pAliqEfetRegCBS"] != "9.0000" || reg["vTribRegCBS"] != "90.00" {
		t.Fatalf("CBS de referência errada: %v", reg)
	}
}

func TestBuildTribCompraGov(t *testing.T) {
	inner := gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(map[string]any{
		"ibs_gov_uf_aliq": "10.0000", "ibs_gov_mun_aliq": "2.0000", "cbs_gov_aliq": "8.0000",
	})))
	gov := sub(t, inner, "gTribCompraGov")
	if gov["pAliqIBSUF"] != "10.0000" || gov["vTribIBSUF"] != "100.00" {
		t.Fatalf("IBS-UF de compra gov errado: %v", gov)
	}
	if gov["vTribIBSMun"] != "20.00" || gov["vTribCBS"] != "80.00" {
		t.Fatalf("valores de compra gov errados: %v", gov)
	}
	// Os dois blocos são independentes: sem cadastro, nenhum sai.
	plain := gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(nil)))
	for _, key := range []string{"gTribRegular", "gTribCompraGov"} {
		if _, ok := plain[key]; ok {
			t.Fatalf("%s não pode sair sem cadastro: %v", key, plain)
		}
	}
}

// ── Task 45: gIBSCBSMono ───────────────────────────────────────────────────

func monoCfg() map[string]any {
	return map[string]any{
		"ibs_ad_rem": "0.5000", "cbs_ad_rem": "0.3000",
		"ibs_ad_rem_reten": "0.2000", "cbs_ad_rem_reten": "0.1000",
		"ibs_ad_rem_ret": "0.4000", "cbs_ad_rem_ret": "0.2500",
		"ibs_p_dif_mono": "50.0000", "cbs_p_dif_mono": "10.0000",
	}
}

// A base da monofasia é a quantidade, e cada valor é quantidade × alíquota
// específica — o qBCMono era zero fixo antes desta tarefa.
func TestBuildIBSCBSMonoUsaQuantidadeComoBase(t *testing.T) {
	p := baseIBSCBS(monoCfg())
	p.CST = cstIBSCBSMono
	mono := sub(t, buildIBSCBS(p), "gIBSCBSMono")

	padrao := sub(t, mono, "gMonoPadrao")
	if padrao["qBCMono"] != "100.0000" {
		t.Fatalf("qBCMono não é a quantidade: %v", padrao)
	}
	if padrao["vIBSMono"] != "50.00" || padrao["vCBSMono"] != "30.00" {
		t.Fatalf("monofásico padrão errado: %v", padrao)
	}

	reten := sub(t, mono, "gMonoReten")
	if reten["qBCMonoReten"] != "100.0000" || reten["vIBSMonoReten"] != "20.00" ||
		reten["vCBSMonoReten"] != "10.00" {
		t.Fatalf("retenção errada: %v", reten)
	}

	ret := sub(t, mono, "gMonoRet")
	if ret["vIBSMonoRet"] != "40.00" || ret["vCBSMonoRet"] != "25.00" {
		t.Fatalf("monofásico já retido errado: %v", ret)
	}

	dif := sub(t, mono, "gMonoDif")
	if dif["pDifIBS"] != "50.0000" || dif["vIBSMonoDif"] != "25.00" {
		t.Fatalf("diferimento do IBS errado: %v", dif)
	}
	if dif["pDifCBS"] != "10.0000" || dif["vCBSMonoDif"] != "3.00" {
		t.Fatalf("diferimento da CBS errado: %v", dif)
	}

	// Os totais do item são a soma do padrão e da retenção; o já retido não
	// entra, porque foi recolhido por outro.
	if mono["vTotIBSMonoItem"] != "70.00" || mono["vTotCBSMonoItem"] != "40.00" {
		t.Fatalf("totais do item errados: %v", mono)
	}
}

// Só o padrão cadastrado: os três sub-grupos opcionais não existem.
func TestBuildIBSCBSMonoSoPadrao(t *testing.T) {
	p := baseIBSCBS(map[string]any{"ibs_ad_rem": "0.5000", "cbs_ad_rem": "0.3000"})
	p.CST = cstIBSCBSMono
	mono := sub(t, buildIBSCBS(p), "gIBSCBSMono")
	for _, key := range []string{"gMonoReten", "gMonoRet", "gMonoDif"} {
		if _, ok := mono[key]; ok {
			t.Fatalf("%s não pode sair sem cadastro: %v", key, mono)
		}
	}
	if mono["vTotIBSMonoItem"] != "50.00" {
		t.Fatalf("total do item errado: %v", mono)
	}
	// Monofásico não tem gIBSCBS: os dois são ramos do mesmo choice.
	if _, ok := buildIBSCBS(p)["gIBSCBS"]; ok {
		t.Fatal("gIBSCBS e gIBSCBSMono são alternativos no XSD")
	}
}

// ── Task 46: gTransfCred, gAjusteCompet e gEstornoCred ─────────────────────

func TestBuildIBSCBSTransferenciaDeCredito(t *testing.T) {
	p := baseIBSCBS(nil)
	p.TransfCred = &ibsCBSPair{VIBS: "150.00", VCBS: "90.00"}
	got := buildIBSCBS(p)
	transf := sub(t, got, "gTransfCred")
	if transf["vIBS"] != "150.00" || transf["vCBS"] != "90.00" {
		t.Fatalf("gTransfCred errado: %v", transf)
	}
	if _, ok := got["gIBSCBS"]; ok {
		t.Fatal("transferência de crédito substitui a apuração do item (choice)")
	}
}

func TestBuildIBSCBSAjusteDeCompetencia(t *testing.T) {
	p := baseIBSCBS(nil)
	p.AjusteCompet = &ibsCBSPair{VIBS: "10.00", VCBS: "5.00"}
	got := buildIBSCBS(p)
	aj := sub(t, got, "gAjusteCompet")
	if aj["competApur"] != "2026-09" || aj["vIBS"] != "10.00" || aj["vCBS"] != "5.00" {
		t.Fatalf("gAjusteCompet errado: %v", aj)
	}
	if _, ok := got["gIBSCBS"]; ok {
		t.Fatal("ajuste de competência substitui a apuração do item (choice)")
	}
}

// O estorno convive com a apuração normal: não é ramo do choice.
func TestBuildIBSCBSEstornoConviveComApuracao(t *testing.T) {
	p := baseIBSCBS(nil)
	p.EstornoCred = &ibsCBSPair{VIBS: "7.00", VCBS: "3.00"}
	got := buildIBSCBS(p)
	est := sub(t, got, "gEstornoCred")
	if est["vIBSEstCred"] != "7.00" || est["vCBSEstCred"] != "3.00" {
		t.Fatalf("gEstornoCred errado: %v", est)
	}
	if _, ok := got["gIBSCBS"]; !ok {
		t.Fatal("estorno não pode suprimir a apuração do item")
	}
}

// O par é tudo-ou-nada: um lado sozinho vira o par com o outro em zero.
func TestItemIBSCBSPair(t *testing.T) {
	if itemIBSCBSPair(map[string]any{}, "transf_cred") != nil {
		t.Fatal("item sem valores não produz par")
	}
	got := itemIBSCBSPair(map[string]any{"estorno_cred_v_ibs": "5.00"}, "estorno_cred")
	if got == nil || got.VIBS != "5.00" || got.VCBS != "0.00" {
		t.Fatalf("par incompleto errado: %v", got)
	}
}

// ── Task 47: gCredPresOper, gCredPresIBSZFM e gALCZFMCBS ───────────────────

func TestBuildCredPresOperDerivaOsValores(t *testing.T) {
	got := buildIBSCBS(baseIBSCBS(map[string]any{
		"ibs_cbs_c_cred_pres": "01", "ibs_p_cred_pres": "5.0000", "cbs_p_cred_pres": "3.0000",
	}))
	oper := sub(t, got, "gCredPresOper")
	if oper["vBCCredPres"] != "1000.00" || oper["cCredPres"] != "01" {
		t.Fatalf("cabeçalho do crédito presumido errado: %v", oper)
	}
	ibs := sub(t, oper, "gIBSCredPres")
	if ibs["pCredPres"] != "5.0000" || ibs["vCredPres"] != "50.00" {
		t.Fatalf("crédito de IBS errado: %v", ibs)
	}
	if _, ok := ibs["vCredPresCondSus"]; ok {
		t.Fatalf("sem condição suspensiva não sai a tag: %v", ibs)
	}
	cbs := sub(t, oper, "gCBSCredPres")
	if cbs["vCredPres"] != "30.00" {
		t.Fatalf("crédito de CBS errado: %v", cbs)
	}
}

// Condição suspensiva é um flag: a conta é a mesma, só muda a tag de destino.
func TestBuildCredPresOperCondicaoSuspensiva(t *testing.T) {
	got := buildIBSCBS(baseIBSCBS(map[string]any{
		"ibs_cbs_c_cred_pres": "02", "ibs_p_cred_pres": "5.0000",
		"ibs_cbs_cred_pres_cond_sus": "1",
	}))
	ibs := sub(t, sub(t, got, "gCredPresOper"), "gIBSCredPres")
	if ibs["vCredPresCondSus"] != "50.00" {
		t.Fatalf("condição suspensiva errada: %v", ibs)
	}
	if _, ok := ibs["vCredPres"]; ok {
		t.Fatalf("o choice do XSD proíbe as duas tags: %v", ibs)
	}
}

// gCredPresOper e gCredPresIBSZFM são um choice: o da operação vence.
func TestBuildCredPresIBSZFM(t *testing.T) {
	p := baseIBSCBS(map[string]any{"ibs_zfm_p_cred_pres": "7.0000"})
	p.TpCredPresIBSZFM = "1"
	zfm := sub(t, buildIBSCBS(p), "gCredPresIBSZFM")
	if zfm["competApur"] != "2026-09" || zfm["tpCredPresIBSZFM"] != "1" ||
		zfm["vCredPresIBSZFM"] != "70.00" {
		t.Fatalf("crédito da ZFM errado: %v", zfm)
	}

	p.Cfg["ibs_cbs_c_cred_pres"] = "01"
	p.Cfg["ibs_p_cred_pres"] = "5.0000"
	got := buildIBSCBS(p)
	if _, ok := got["gCredPresOper"]; !ok {
		t.Fatalf("crédito da operação tinha que vencer: %v", got)
	}
	if _, ok := got["gCredPresIBSZFM"]; ok {
		t.Fatalf("os dois créditos presumidos são alternativos: %v", got)
	}
}

// gALCZFMCBS mora dentro de gCBS e mede o benefício: a alíquota de referência é
// a que valeria fora da área.
func TestBuildALCZFMCBS(t *testing.T) {
	p := baseIBSCBS(map[string]any{"alc_zfm_tp_cbs": "1", "cbs_reg_aliq": "9.0000"})
	p.NProcSuframa = "12345678"
	alc := sub(t, sub(t, gIBSCBSOf(t, buildIBSCBS(p)), "gCBS"), "gALCZFMCBS")
	if alc["tpALCZFMCBS"] != "1" || alc["nProcSuframa"] != "12345678" {
		t.Fatalf("cabeçalho do ALC/ZFM errado: %v", alc)
	}
	if alc["pAliqEfetRegCBS"] != "9.0000" || alc["vTribRegCBS"] != "90.00" {
		t.Fatalf("tributo de referência errado: %v", alc)
	}
	// Sem o tipo cadastrado, o grupo não existe.
	if _, ok := sub(t, gIBSCBSOf(t, buildIBSCBS(baseIBSCBS(nil))), "gCBS")["gALCZFMCBS"]; ok {
		t.Fatal("gALCZFMCBS não pode sair sem tpALCZFMCBS")
	}
}

// ── Task 49: conservação — o total é exatamente a soma dos itens ────────────

// reformItemCfgs são os cenários da reforma, um por grupo introduzido no bloco.
func reformItemCfgs() []map[string]any {
	return []map[string]any{
		// Tributado normal.
		nil,
		// Com diferimento e devolução de tributo nas três esferas.
		{"ibs_uf_p_dif": "10.0000", "ibs_mun_p_dif": "10.0000", "cbs_p_dif": "10.0000",
			"ibs_cbs_p_dev_trib": "20.0000"},
		// Com crédito presumido da operação.
		{"ibs_cbs_c_cred_pres": "01", "ibs_p_cred_pres": "5.0000", "cbs_p_cred_pres": "3.0000"},
		// Com crédito presumido em condição suspensiva.
		{"ibs_cbs_c_cred_pres": "02", "ibs_p_cred_pres": "4.0000", "cbs_p_cred_pres": "2.0000",
			"ibs_cbs_cred_pres_cond_sus": "1"},
	}
}

// TestTotaisDaReformaConservamASomaDosItens monta uma nota com um item de cada
// grupo introduzido nas Tasks 42-48 e afirma que cada acumulador do IBSCBSTot é
// exatamente a soma dos itens correspondentes. É o teste que impede o total de
// virar um segundo cálculo em vez da soma do que foi emitido.
func TestTotaisDaReformaConservamASomaDosItens(t *testing.T) {
	tot := newTotals(decimal.Zero, decimal.Zero)

	var wantIBSUF, wantIBSMun, wantCBS decimal.Decimal
	var wantIBSUFDif, wantIBSMunDif, wantCBSDif decimal.Decimal
	var wantIBSUFDev, wantIBSMunDev, wantCBSDev decimal.Decimal
	var wantIBSCred, wantCBSCred decimal.Decimal
	var wantIBSCredSus, wantCBSCredSus decimal.Decimal
	var wantIBSMono, wantCBSMono, wantIBSMonoReten, wantCBSMonoReten decimal.Decimal
	var wantIBSEst, wantCBSEst decimal.Decimal

	// Itens tributados normalmente, cada um com um grupo diferente.
	for _, cfg := range reformItemCfgs() {
		p := baseIBSCBS(cfg)
		node := buildIBSCBS(p)
		accumulateIBSCBS(&tot, node, p.VBC)

		inner := gIBSCBSOf(t, node)
		gUF, gMun, gCBS := sub(t, inner, "gIBSUF"), sub(t, inner, "gIBSMun"), sub(t, inner, "gCBS")
		wantIBSUF = wantIBSUF.Add(d(anyStr(gUF, "vIBSUF", "0")))
		wantIBSMun = wantIBSMun.Add(d(anyStr(gMun, "vIBSMun", "0")))
		wantCBS = wantCBS.Add(d(anyStr(gCBS, "vCBS", "0")))
		wantIBSUFDif = wantIBSUFDif.Add(subgroupValue(gUF, "gDif", "vDif"))
		wantIBSMunDif = wantIBSMunDif.Add(subgroupValue(gMun, "gDif", "vDif"))
		wantCBSDif = wantCBSDif.Add(subgroupValue(gCBS, "gDif", "vDif"))
		wantIBSUFDev = wantIBSUFDev.Add(subgroupValue(gUF, "gDevTrib", "vDevTrib"))
		wantIBSMunDev = wantIBSMunDev.Add(subgroupValue(gMun, "gDevTrib", "vDevTrib"))
		wantCBSDev = wantCBSDev.Add(subgroupValue(gCBS, "gDevTrib", "vDevTrib"))
		if oper, ok := node["gCredPresOper"].(map[string]any); ok {
			ibs := sub(t, oper, "gIBSCredPres")
			cbs := sub(t, oper, "gCBSCredPres")
			if v := anyStr(ibs, "vCredPresCondSus", ""); v != "" {
				wantIBSCredSus = wantIBSCredSus.Add(d(v))
				wantCBSCredSus = wantCBSCredSus.Add(d(anyStr(cbs, "vCredPresCondSus", "0")))
			} else {
				wantIBSCred = wantIBSCred.Add(d(anyStr(ibs, "vCredPres", "0")))
				wantCBSCred = wantCBSCred.Add(d(anyStr(cbs, "vCredPres", "0")))
			}
		}
	}

	// Um item monofásico, com retenção.
	monoP := baseIBSCBS(monoCfg())
	monoP.CST = cstIBSCBSMono
	monoNode := buildIBSCBS(monoP)
	accumulateIBSCBS(&tot, monoNode, monoP.VBC)
	mono := sub(t, monoNode, "gIBSCBSMono")
	padrao := sub(t, mono, "gMonoPadrao")
	reten := sub(t, mono, "gMonoReten")
	wantIBSMono = d(anyStr(padrao, "vIBSMono", "0"))
	wantCBSMono = d(anyStr(padrao, "vCBSMono", "0"))
	wantIBSMonoReten = d(anyStr(reten, "vIBSMonoReten", "0"))
	wantCBSMonoReten = d(anyStr(reten, "vCBSMonoReten", "0"))

	// Um item com estorno de crédito.
	estP := baseIBSCBS(nil)
	estP.EstornoCred = &ibsCBSPair{VIBS: "12.34", VCBS: "5.66"}
	estNode := buildIBSCBS(estP)
	accumulateIBSCBS(&tot, estNode, estP.VBC)
	wantIBSUF = wantIBSUF.Add(d(anyStr(sub(t, gIBSCBSOf(t, estNode), "gIBSUF"), "vIBSUF", "0")))
	wantIBSMun = wantIBSMun.Add(d(anyStr(sub(t, gIBSCBSOf(t, estNode), "gIBSMun"), "vIBSMun", "0")))
	wantCBS = wantCBS.Add(d(anyStr(sub(t, gIBSCBSOf(t, estNode), "gCBS"), "vCBS", "0")))
	wantIBSEst = decimal.RequireFromString("12.34")
	wantCBSEst = decimal.RequireFromString("5.66")

	node := buildIBSCBSTot(tot)
	gIBS := sub(t, node, "gIBS")
	gUF := sub(t, gIBS, "gIBSUF")
	gMun := sub(t, gIBS, "gIBSMun")
	gCBS := sub(t, node, "gCBS")

	for _, tc := range []struct {
		name, got string
		want      decimal.Decimal
	}{
		{"gIBSUF/vIBSUF", anyStr(gUF, "vIBSUF", ""), wantIBSUF},
		{"gIBSUF/vDif", anyStr(gUF, "vDif", ""), wantIBSUFDif},
		{"gIBSUF/vDevTrib", anyStr(gUF, "vDevTrib", ""), wantIBSUFDev},
		{"gIBSMun/vIBSMun", anyStr(gMun, "vIBSMun", ""), wantIBSMun},
		{"gIBSMun/vDif", anyStr(gMun, "vDif", ""), wantIBSMunDif},
		{"gIBSMun/vDevTrib", anyStr(gMun, "vDevTrib", ""), wantIBSMunDev},
		{"gIBS/vIBS", anyStr(gIBS, "vIBS", ""), wantIBSUF.Add(wantIBSMun)},
		{"gIBS/vCredPres", anyStr(gIBS, "vCredPres", ""), wantIBSCred},
		{"gIBS/vCredPresCondSus", anyStr(gIBS, "vCredPresCondSus", ""), wantIBSCredSus},
		{"gCBS/vCBS", anyStr(gCBS, "vCBS", ""), wantCBS},
		{"gCBS/vDif", anyStr(gCBS, "vDif", ""), wantCBSDif},
		{"gCBS/vDevTrib", anyStr(gCBS, "vDevTrib", ""), wantCBSDev},
		{"gCBS/vCredPres", anyStr(gCBS, "vCredPres", ""), wantCBSCred},
		{"gCBS/vCredPresCondSus", anyStr(gCBS, "vCredPresCondSus", ""), wantCBSCredSus},
	} {
		if tc.got != q2(tc.want.RoundBank(2)) {
			t.Errorf("%s = %q, want %q (soma dos itens)", tc.name, tc.got, q2(tc.want.RoundBank(2)))
		}
	}

	gMono := sub(t, node, "gMono")
	for _, tc := range []struct {
		name, got string
		want      decimal.Decimal
	}{
		{"gMono/vIBSMono", anyStr(gMono, "vIBSMono", ""), wantIBSMono},
		{"gMono/vCBSMono", anyStr(gMono, "vCBSMono", ""), wantCBSMono},
		{"gMono/vIBSMonoReten", anyStr(gMono, "vIBSMonoReten", ""), wantIBSMonoReten},
		{"gMono/vCBSMonoReten", anyStr(gMono, "vCBSMonoReten", ""), wantCBSMonoReten},
	} {
		if tc.got != q2(tc.want.RoundBank(2)) {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, q2(tc.want.RoundBank(2)))
		}
	}

	est := sub(t, node, "gEstornoCred")
	if est["vIBSEstCred"] != q2(wantIBSEst) || est["vCBSEstCred"] != q2(wantCBSEst) {
		t.Errorf("gEstornoCred = %v, want %s/%s", est, q2(wantIBSEst), q2(wantCBSEst))
	}
}

// Sem monofasia nem estorno, os dois sub-grupos opcionais do total não existem.
func TestIBSCBSTotSemGruposOpcionais(t *testing.T) {
	tot := newTotals(decimal.Zero, decimal.Zero)
	p := baseIBSCBS(nil)
	accumulateIBSCBS(&tot, buildIBSCBS(p), p.VBC)
	node := buildIBSCBSTot(tot)
	for _, key := range []string{"gMono", "gEstornoCred"} {
		if _, ok := node[key]; ok {
			t.Fatalf("%s não pode sair vazio: %v", key, node)
		}
	}
}

// vNFTot é o total do documento com os tributos por fora. Sem reforma
// incidente, a tag não existe: repetir o vNF não informa nada.
func TestVNFTotSoExisteComTributoPorFora(t *testing.T) {
	vNF := decimal.RequireFromString("1000.00")

	plain := newTotals(decimal.Zero, decimal.Zero)
	if got := reformDocumentTotal(plain, vNF); got != nil {
		t.Fatalf("nota sem reforma não tem vNFTot: %v", *got)
	}

	tot := newTotals(decimal.Zero, decimal.Zero)
	p := baseIBSCBS(nil)
	accumulateIBSCBS(&tot, buildIBSCBS(p), p.VBC)
	tot.VIS = decimal.RequireFromString("15.00")
	got := reformDocumentTotal(tot, vNF)
	if got == nil {
		t.Fatal("com IBS/CBS/IS a tag tinha que existir")
	}
	// 1000 + IBS 120 + CBS 80 + IS 15
	if *got != "1215.00" {
		t.Fatalf("vNFTot = %q, want 1215.00", *got)
	}
}
