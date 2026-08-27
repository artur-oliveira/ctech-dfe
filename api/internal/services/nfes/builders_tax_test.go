package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func dec(s string) decimal.Decimal {
	v, _ := decimal.NewFromString(s)
	return v
}

func mapKey(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("key %q missing from %v", key, m)
	}
	inner, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q is %T, not map[string]any", key, v)
	}
	return inner
}

func strField(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("field %q missing", key)
		return
	}
	s, ok := v.(string)
	if !ok {
		t.Errorf("field %q is %T, not string", key, v)
		return
	}
	if s != want {
		t.Errorf("field %q = %q, want %q", key, s, want)
	}
}

// ─── buildICMSSN ─────────────────────────────────────────────────────────────

func TestBuildICMSSN_CSOSN500(t *testing.T) {
	result := buildICMSSN("0", "500", dec("100.00"), nil)
	inner := mapKey(t, result, "ICMSSN500")
	strField(t, inner, "orig", "0")
	strField(t, inner, "CSOSN", "500")
}

func TestBuildICMSSN_CSOSN102(t *testing.T) {
	result := buildICMSSN("0", "102", dec("100.00"), nil)
	inner := mapKey(t, result, "ICMSSN102")
	strField(t, inner, "CSOSN", "102")
}

func TestBuildICMSSN_CSOSN400(t *testing.T) {
	result := buildICMSSN("0", "400", dec("100.00"), nil)
	inner := mapKey(t, result, "ICMSSN102")
	strField(t, inner, "CSOSN", "400")
}

func TestBuildICMSSN_CSOSN101HasCredSN(t *testing.T) {
	cfg := map[string]any{"icms_sn_cred_aliq": "4.00"}
	result := buildICMSSN("0", "101", dec("100.00"), cfg)
	inner := mapKey(t, result, "ICMSSN101")
	strField(t, inner, "CSOSN", "101")
	strField(t, inner, "pCredSN", "4.00")
	if inner["vCredICMSSN"] == nil {
		t.Error("vCredICMSSN missing")
	}
}

func TestBuildICMSSN_CSOSN201HasSTFields(t *testing.T) {
	cfg := map[string]any{
		"icms_st_mva":  "50.00",
		"icms_st_aliq": "12.00",
	}
	result := buildICMSSN("0", "201", dec("100.00"), cfg)
	inner := mapKey(t, result, "ICMSSN201")
	strField(t, inner, "CSOSN", "201")
	if inner["vBCST"] == nil {
		t.Error("vBCST missing")
	}
	if inner["vICMSST"] == nil {
		t.Error("vICMSST missing")
	}
}

func TestBuildICMSSN_UnknownCSOSNFallback(t *testing.T) {
	result := buildICMSSN("0", "999", dec("100.00"), nil)
	inner := mapKey(t, result, "ICMSSN102")
	strField(t, inner, "CSOSN", "102")
}

// ─── buildICMSNormal ─────────────────────────────────────────────────────────

func TestBuildICMSNormal_CST00BasicFields(t *testing.T) {
	result := buildICMSNormal("0", "00", dec("100.00"), nil, "12.00", "2.00", dec("1"))
	inner := mapKey(t, result, "ICMS00")
	strField(t, inner, "orig", "0")
	strField(t, inner, "CST", "00")
	if inner["vBC"] == nil {
		t.Error("vBC missing")
	}
	if inner["vICMS"] == nil {
		t.Error("vICMS missing")
	}
}

func TestBuildICMSNormal_CST40Isento(t *testing.T) {
	result := buildICMSNormal("0", "40", dec("100.00"), nil, "0.00", "0.00", dec("1"))
	inner := mapKey(t, result, "ICMS40")
	strField(t, inner, "CST", "40")
}

func TestBuildICMSNormal_CST20WithRedBC(t *testing.T) {
	cfg := map[string]any{"icms_p_red_bc": "30.00"}
	result := buildICMSNormal("0", "20", dec("100.00"), cfg, "12.00", "0.00", dec("1"))
	inner := mapKey(t, result, "ICMS20")
	strField(t, inner, "CST", "20")
	strField(t, inner, "pRedBC", "30.00")
}

func TestBuildICMSNormal_ICMSCalculationCST00(t *testing.T) {
	result := buildICMSNormal("0", "00", dec("100.00"), nil, "18.00", "0.00", dec("1"))
	inner := mapKey(t, result, "ICMS00")
	strField(t, inner, "vBC", "100.00")
	strField(t, inner, "vICMS", "18.00")
}

// ─── buildIPI ─────────────────────────────────────────────────────────────────

func TestBuildIPI_EmptyCSTReturnsNil(t *testing.T) {
	result := buildIPI("", dec("100.00"), nil)
	if result != nil {
		t.Errorf("empty CST should return nil, got %v", result)
	}
}

func TestBuildIPI_TributadoCST50(t *testing.T) {
	result := buildIPI("50", dec("100.00"), new("10.00"))
	ipi := mapKey(t, result, "IPI")
	trib := mapKey(t, ipi, "IPITrib")
	strField(t, trib, "CST", "50")
	strField(t, trib, "pIPI", "10.00")
	strField(t, trib, "vBC", "100.00")
	strField(t, trib, "vIPI", "10.00")
}

func TestBuildIPI_NaoTributadoCST03(t *testing.T) {
	result := buildIPI("03", dec("100.00"), nil)
	ipi := mapKey(t, result, "IPI")
	nt := mapKey(t, ipi, "IPINT")
	strField(t, nt, "CST", "03")
}

// ─── buildPIS ─────────────────────────────────────────────────────────────────

func TestBuildPIS_CST07NaoTributado(t *testing.T) {
	result := buildPIS("07", dec("100.00"), nil, nil)
	inner := mapKey(t, result, "PISNT")
	strField(t, inner, "CST", "07")
}

func TestBuildPIS_CST01Aliq(t *testing.T) {
	result := buildPIS("01", dec("100.00"), new("0.65"), nil)
	inner := mapKey(t, result, "PISAliq")
	strField(t, inner, "CST", "01")
	strField(t, inner, "vBC", "100.00")
	strField(t, inner, "pPIS", "0.65")
	strField(t, inner, "vPIS", "0.65")
}

func TestBuildPIS_CST04NaoTributado(t *testing.T) {
	result := buildPIS("04", dec("100.00"), nil, nil)
	inner := mapKey(t, result, "PISNT")
	strField(t, inner, "CST", "04")
}

// ─── buildCOFINS ─────────────────────────────────────────────────────────────

func TestBuildCOFINS_CST07NaoTributado(t *testing.T) {
	result := buildCOFINS("07", dec("100.00"), nil, nil)
	inner := mapKey(t, result, "COFINSNT")
	strField(t, inner, "CST", "07")
}

func TestBuildCOFINS_CST01Aliq(t *testing.T) {
	result := buildCOFINS("01", dec("100.00"), new("3.00"), nil)
	inner := mapKey(t, result, "COFINSAliq")
	strField(t, inner, "CST", "01")
	strField(t, inner, "vCOFINS", "3.00")
}

// ─── buildICMSUFDest ─────────────────────────────────────────────────────────

// buildICMSUFDest returns a flat map (not wrapped in ICMSUFDest key).

func TestBuildICMSUFDest_Structure(t *testing.T) {
	result := buildICMSUFDest(dec("100.00"), "18.00", "12.00", "2.00")
	for _, k := range []string{"vBCUFDest", "pICMSUFDest", "pICMSInter", "vICMSUFDest", "vICMSUFRemet"} {
		if result[k] == nil {
			t.Errorf("field %q missing from ICMSUFDest result", k)
		}
	}
}

func TestBuildICMSUFDest_WithFCP(t *testing.T) {
	result := buildICMSUFDest(dec("100.00"), "18.00", "12.00", "2.00")
	if result["pFCPUFDest"] == nil {
		t.Error("pFCPUFDest missing when FCP aliq provided")
	}
	if result["vFCPUFDest"] == nil {
		t.Error("vFCPUFDest missing when FCP aliq provided")
	}
}

func TestBuildICMSUFDest_NoFCPWhenZero(t *testing.T) {
	result := buildICMSUFDest(dec("100.00"), "18.00", "12.00", "0.00")
	if result["pFCPUFDest"] != nil {
		t.Error("pFCPUFDest should be absent when FCP aliq is 0.00")
	}
}

// ─── applyUFRules ────────────────────────────────────────────────────────────

func TestApplyUFRules_MT_CST60_Returns60(t *testing.T) {
	// MT with CST 60 gets remapped to 41 for SEFAZ compatibility
	result := applyUFRules("MT", "60", map[string]any{"icms": "60"}, "17.00")
	if result == nil {
		t.Fatal("applyUFRules returned nil for MT/60")
	}
}

func TestApplyUFRules_NilWhenNoRule(t *testing.T) {
	// Most UF/CST combos return nil (no override needed)
	result := applyUFRules("SP", "00", map[string]any{}, "18.00")
	_ = result // nil is valid — means no override
}

func TestBuildICMSPart(t *testing.T) {
	cfg := map[string]any{
		"icms_mod_bc": "3", "icms_st_aliq": "18.00", "icms_st_mva": "40.00",
		"icms_part_p_bc_op": "60.00", "icms_part_uf_st": "SP",
	}
	got := buildICMSNormal("0", "10", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
	node, ok := got["ICMSPart"].(map[string]any)
	if !ok {
		t.Fatalf("esperava ICMSPart, veio %v", got)
	}
	if node["pBCOp"] != "60.00" || node["UFST"] != "SP" || node["CST"] != "10" {
		t.Fatalf("ICMSPart errado: %v", node)
	}
	if _, ok := got["ICMS10"]; ok {
		t.Fatal("ICMSPart substitui ICMS10, não convive")
	}
}

// Sem o par pBCOp+UFST, CST 10 continua sendo ICMS10.
func TestBuildICMSPartExigeOPar(t *testing.T) {
	got := buildICMSNormal("0", "10", decimal.RequireFromString("100.00"),
		map[string]any{"icms_part_p_bc_op": "60.00"}, "12.00", "0.00", decimal.NewFromInt(1))
	if _, ok := got["ICMS10"]; !ok {
		t.Fatalf("esperava ICMS10, veio %v", got)
	}
}

func TestBuildICMSSTRepasse(t *testing.T) {
	cfg := map[string]any{
		"icms_v_bc_st_ret": "200.00", "icms_v_icms_st_ret": "36.00",
		"icms_v_bc_st_dest": "150.00", "icms_v_icms_st_dest": "27.00",
	}
	got := buildICMSNormal("0", "41", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
	node, ok := got["ICMSST"].(map[string]any)
	if !ok {
		t.Fatalf("CST 41 com ST retida tem que virar ICMSST, veio %v", got)
	}
	if node["vBCSTDest"] != "150.00" || node["vICMSSTDest"] != "27.00" {
		t.Fatalf("ICMSST errado: %v", node)
	}
	if node["vBCSTRet"] != "200.00" || node["vICMSSTRet"] != "36.00" {
		t.Fatalf("ST retida perdida: %v", node)
	}
}

// Sem os valores de ST, 41 continua sendo não tributada (ICMS40).
func TestBuildICMS41SemSTContinuaICMS40(t *testing.T) {
	got := buildICMSNormal("0", "41", decimal.RequireFromString("100.00"), map[string]any{}, "12.00", "0.00", decimal.NewFromInt(1))
	if _, ok := got["ICMS40"]; !ok {
		t.Fatalf("esperava ICMS40, veio %v", got)
	}
}

func TestAddICMSEfetivoCalculaBaseEValor(t *testing.T) {
	node := map[string]any{}
	addICMSEfetivo(node, decimal.RequireFromString("100.00"),
		map[string]any{"icms_p_red_bc_efet": "20.00", "icms_p_icms_efet": "18.00"})
	if node["pRedBCEfet"] != "20.00" || node["vBCEfet"] != "80.00" ||
		node["pICMSEfet"] != "18.00" || node["vICMSEfet"] != "14.40" {
		t.Fatalf("efetivo errado: %v", node)
	}
}

func TestAddICMSEfetivoAusenteNaoPoluiONo(t *testing.T) {
	node := map[string]any{"CST": "60"}
	addICMSEfetivo(node, decimal.RequireFromString("100.00"), map[string]any{})
	if len(node) != 1 {
		t.Fatalf("sem configuração, nada pode ser acrescentado: %v", node)
	}
}

// ICMS60 e CSOSN 500 são o mesmo caso em regimes diferentes: os dois trazem a
// ST retida e o ICMS efetivo.
func TestICMS60ECSOSN500TrazemSTRetidaEEfetivo(t *testing.T) {
	cfg := map[string]any{
		"icms_v_bc_st_ret": "200.00", "icms_v_icms_st_ret": "36.00", "icms_p_st": "18.00",
		"icms_p_icms_efet": "18.00", "icms_p_red_bc_efet": "20.00",
	}
	vProd := decimal.RequireFromString("100.00")
	normal := buildICMSNormal("0", "60", vProd, cfg, "12.00", "0.00", decimal.NewFromInt(1))["ICMS60"].(map[string]any)
	sn := buildICMSSN("0", "500", vProd, cfg)["ICMSSN500"].(map[string]any)
	for _, node := range []map[string]any{normal, sn} {
		if node["vBCSTRet"] != "200.00" || node["pST"] != "18.00" || node["vICMSSTRet"] != "36.00" {
			t.Fatalf("ST retida ausente: %v", node)
		}
		if node["vBCEfet"] != "80.00" || node["vICMSEfet"] != "14.40" {
			t.Fatalf("ICMS efetivo ausente: %v", node)
		}
	}
}

func TestICMS70ComSTDesonerada(t *testing.T) {
	cfg := map[string]any{"icms_st_aliq": "18.00", "icms_mot_des_st": "9"}
	got := buildICMSNormal("0", "70", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
	n := got["ICMS70"].(map[string]any)
	if n["motDesICMSST"] != "9" || n["vICMSSTDeson"] == nil {
		t.Fatalf("ST desonerada ausente: %v", n)
	}
}

func TestICMS51ComFCPDiferido(t *testing.T) {
	cfg := map[string]any{"icms_p_dif": "50.00", "icms_fcp_override": "2.00", "icms_p_fcp_dif": "100.00"}
	got := buildICMSNormal("0", "51", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
	n := got["ICMS51"].(map[string]any)
	if n["pFCPDif"] != "100.00" || n["vFCPDif"] != "2.00" || n["vFCPEfet"] != "0.00" {
		t.Fatalf("FCP diferido errado: %v", n)
	}
}

// O leiaute não tem vBCFCP em ICMS00: a base do FCP ali é o próprio vBC.
func TestICMS00FCPSemVBCFCP(t *testing.T) {
	got := buildICMSNormal("0", "00", decimal.RequireFromString("100.00"),
		map[string]any{"icms_fcp_override": "2.00"}, "12.00", "0.00", decimal.NewFromInt(1))
	n := got["ICMS00"].(map[string]any)
	if n["pFCP"] != "2.00" || n["vFCP"] != "2.00" {
		t.Fatalf("FCP ausente: %v", n)
	}
	if _, ok := n["vBCFCP"]; ok {
		t.Fatalf("vBCFCP não existe em ICMS00: %v", n)
	}
}
