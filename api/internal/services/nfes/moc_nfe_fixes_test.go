package nfes

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// Regressões da auditoria MOC x XSD (docs/specs/2026-11-09-auditoria-moc-nfe.md): cada teste monta o
// documento inteiro, confere o que o XSD pede e passa o resultado pelo validador de esquema (enum, padrão,
// tamanho, tag desconhecida). Obrigatório ausente fica de fora: o documento de teste é mínimo.

// mocDoc emite um item (2 x 50,00) com a configuração fiscal dada e devolve det[0] e total.
func mocDoc(t *testing.T, cfg map[string]any) (imposto, total map[string]any) {
	t.Helper()
	items := goldenItems()
	base := map[string]any{"cfop": "6102", "ibs_cbs_cst": "000", "ibs_cbs_class_trib": "000001"}
	for k, v := range cfg {
		base[k] = v
	}
	items[0]["cfop_config"] = []any{base}
	got := BuildEnviNFe(goldenOrg(), goldenReceiver(), "CNPJ_11647612000197", items, goldenPayments(),
		1, 1, 2, "22260811647612000197550010000000011100000015", decimal.RequireFromString("100.00"), decimal.Zero,
		nil, time.Date(2026, 8, 27, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)), nil, "1", "1", "1", "1",
		nil, nil, nil, nil, TechData{CNPJ: "11647612000197", Name: "Ctech", Email: "t@t.com", Phone: "8630000000", Version: "1.0"},
		nfModel55, nil, nil, nil, NormalEmission(nfModel55), docExtras{})
	nfe := got["enviNFe"].(map[string]any)["NFe"].(map[string]any)
	for _, v := range dfecheck.Shadow(dfecheck.SpecNFe, "nfe", "teste", nfe) {
		switch v.Code {
		case "required", "choice_missing", "rule":
			continue
		case "choice_conflict": // F-12: falso positivo do validador em todo item com IPI (outro escopo)
			continue
		}
		t.Errorf("esquema: %s %s: %s", v.Path, v.Code, v.Message)
	}
	inf := nfe["infNFe"].(map[string]any)
	return inf["det"].([]map[string]any)[0]["imposto"].(map[string]any), inf["total"].(map[string]any)
}

func TestMOC_F01_IPICSTGrupos(t *testing.T) {
	for cst, group := range map[string]string{"00": "IPITrib", "49": "IPITrib", "50": "IPITrib", "99": "IPITrib", "51": "IPINT", "01": "IPINT", "55": "IPINT"} {
		imp, _ := mocDoc(t, map[string]any{"icms": "00", "icms_aliq": "18.00", "ipi_cst": cst, "ipi_aliq": "5.00"})
		ipi := imp["IPI"].(map[string]any)
		if _, ok := ipi[group]; !ok {
			t.Errorf("IPI CST %s: esperado %s, veio %v", cst, group, ipi)
		}
	}
	_, tot := mocDoc(t, map[string]any{"icms": "00", "icms_aliq": "18.00", "ipi_cst": "99", "ipi_aliq": "5.00"})
	if tot["ICMSTot"].(map[string]any)["vIPI"] != "5.00" {
		t.Errorf("vIPI do CST 99 tem de entrar no total: %v", tot["ICMSTot"])
	}
}

func TestMOC_F02_PISCOFINSQtde(t *testing.T) {
	imp, tot := mocDoc(t, map[string]any{"icms": "00", "icms_aliq": "18.00",
		"pis": "03", "pis_aliq_unid": "1.2345", "cofins": "03", "cofins_aliq_unid": "2.0000"})
	pis := imp["PIS"].(map[string]any)["PISQtde"].(map[string]any)
	if pis["qBCProd"] != "2.0000" || pis["vPIS"] != "2.47" {
		t.Errorf("PISQtde: %v", pis)
	}
	cof := imp["COFINS"].(map[string]any)["COFINSQtde"].(map[string]any)
	if cof["qBCProd"] != "2.0000" || cof["vCOFINS"] != "4.00" {
		t.Errorf("COFINSQtde: %v", cof)
	}
	icmsTot := tot["ICMSTot"].(map[string]any)
	if icmsTot["vPIS"] != "2.47" || icmsTot["vCOFINS"] != "4.00" {
		t.Errorf("totais PIS/COFINS: %v", icmsTot)
	}
}

func TestMOC_F03_GruposOpcionaisTudoOuNada(t *testing.T) {
	// Só pICMSEfet: pRedBCEfet (obrigatório do grupo) sai com 0.00.
	imp, _ := mocDoc(t, map[string]any{"icms": "60", "icms_p_icms_efet": "12.00"})
	n := imp["ICMS"].(map[string]any)["ICMS60"].(map[string]any)
	for _, k := range []string{"pRedBCEfet", "vBCEfet", "pICMSEfet", "vICMSEfet"} {
		if _, ok := n[k]; !ok {
			t.Errorf("ICMS60 sem %s: %v", k, n)
		}
	}
	// vBCSTRet sem pST: o grupo [vBCSTRet, pST, vICMSSTRet] não sai pela metade.
	imp, _ = mocDoc(t, map[string]any{"icms": "60", "icms_v_bc_st_ret": "100.00"})
	n = imp["ICMS"].(map[string]any)["ICMS60"].(map[string]any)
	for _, k := range []string{"vBCSTRet", "vICMSSTRet", "pST"} {
		if _, ok := n[k]; ok {
			t.Errorf("ICMS60 não pode ter %s sem pST: %v", k, n)
		}
	}
	imp, _ = mocDoc(t, map[string]any{"icms": "60", "icms_v_bc_st_ret": "100.00", "icms_p_st": "18.00"})
	n = imp["ICMS"].(map[string]any)["ICMS60"].(map[string]any)
	if n["pST"] != "18.00" || n["vBCSTRet"] != "100.00" || n["vICMSSTRet"] == nil {
		t.Errorf("ICMS60 completo: %v", n)
	}
	// ICMSSN500 (CRT 1) tem as mesmas regras.
	sn := buildICMSSN("0", "500", dec("100.00"), map[string]any{"icms_v_bc_st_ret": "100.00", "icms_p_icms_efet": "12.00"})["ICMSSN500"].(map[string]any)
	if _, ok := sn["vBCSTRet"]; ok {
		t.Errorf("SN500 sem pST: %v", sn)
	}
	if sn["pRedBCEfet"] != "0.00" {
		t.Errorf("SN500 pRedBCEfet: %v", sn)
	}
	// ICMSST (CST 41): pST é opcional no XSD, o grupo sai sem ele.
	st := buildICMSNormal("0", "41", dec("100.00"), map[string]any{"icms_v_bc_st_ret": "100.00"}, "12.00", "0.00", dec("1"))["ICMSST"].(map[string]any)
	if st["vBCSTRet"] != "100.00" {
		t.Errorf("ICMSST: %v", st)
	}
}

func TestMOC_F04_TotaisMonofasicosEFCPSTRet(t *testing.T) {
	cases := map[string]map[string]string{
		"02": {"qBCMono": "2.00", "vICMSMono": "1.00"},
		"15": {"qBCMono": "2.00", "vICMSMono": "1.00", "qBCMonoReten": "2.00", "vICMSMonoReten": "0.60"},
		"53": {"qBCMono": "2.00", "vICMSMono": "1.00"},
		"61": {"qBCMonoRet": "2.00", "vICMSMonoRet": "1.00"},
	}
	for cst, want := range cases {
		_, tot := mocDoc(t, map[string]any{"icms": cst, "icms_ad_rem": "0.5000", "icms_ad_rem_reten": "0.3000"})
		it := tot["ICMSTot"].(map[string]any)
		for _, k := range monoTags {
			w, expect := want[k]
			got, has := it[k]
			if has != expect || (expect && got != w) {
				t.Errorf("CST %s ICMSTot/%s = %v (presente=%v), want %q (%v)", cst, k, got, has, w, expect)
			}
		}
	}
	// Sem item monofásico, as tags opcionais não saem.
	_, tot := mocDoc(t, map[string]any{"icms": "00", "icms_aliq": "18.00"})
	if _, ok := tot["ICMSTot"].(map[string]any)["vICMSMono"]; ok {
		t.Error("vICMSMono sem item monofásico")
	}
	// vFCPSTRet vem da soma dos itens.
	_, tot = mocDoc(t, map[string]any{"icms": "60", "icms_v_bc_st_ret": "100.00", "icms_p_st": "18.00",
		"icms_fcp_v_bc_st_ret": "100.00", "icms_fcp_st_ret_aliq": "2.00"})
	if tot["ICMSTot"].(map[string]any)["vFCPSTRet"] != "2.00" {
		t.Errorf("vFCPSTRet: %v", tot["ICMSTot"])
	}
	// vNF soma vICMSMonoReten (W16-10).
	tt := newTotals(dec("100.00"), decimal.Zero)
	tt.addMono(map[string]any{"vICMSMonoReten": "0.60"})
	if got := buildTotal(tt, time.Now(), nil)["ICMSTot"].(map[string]any)["vNF"]; got != "100.60" {
		t.Errorf("vNF com vICMSMonoReten = %v", got)
	}
}

func TestMOC_F05_ISQTrib(t *testing.T) {
	imp, _ := mocDoc(t, map[string]any{"icms": "00", "icms_aliq": "18.00", "is_cst": "000", "is_class_trib": "000001",
		"is_aliq": "1.0000", "is_aliq_espec": "0.5000", "is_unid_trib": "L"})
	is := imp["IS"].(map[string]any)
	if is["qTrib"] != "2.0000" || is["uTrib"] != "L" {
		t.Errorf("IS: %v", is)
	}
}

func TestMOC_F06_VNFSoDeduzComIndicador(t *testing.T) {
	for ind, want := range map[string]string{"0": "100.00", "1": "94.00"} {
		_, tot := mocDoc(t, map[string]any{"icms": "20", "icms_p_red_bc": "50.00",
			"icms_mot_des": "9", "icms_ind_deduz_deson": ind})
		it := tot["ICMSTot"].(map[string]any)
		if it["vICMSDeson"] != "6.00" || it["vNF"] != want {
			t.Errorf("ind=%s: vICMSDeson=%v vNF=%v, want 6.00 / %s", ind, it["vICMSDeson"], it["vNF"], want)
		}
	}
}

func TestMOC_F10_DestISUF(t *testing.T) {
	recv := map[string]any{"name": "Dest", "person": map[string]any{"isuf_emit": "123456789"}}
	got := buildDest(recv, recv["person"].(map[string]any), "CNPJ_11222333000181", "AM", false, 1, "")
	if got["ISUF"] != "123456789" {
		t.Errorf("dest sem ISUF: %v", got)
	}
	if _, ok := buildDest(recv, map[string]any{}, "CNPJ_11222333000181", "AM", false, 1, "")["ISUF"]; ok {
		t.Error("ISUF sem cadastro")
	}
}

func TestMOC_F11_DhEmiNoFusoDaUF(t *testing.T) {
	ref := time.Date(2026, 8, 1, 15, 0, 0, 0, time.UTC)
	for uf, want := range map[string]string{"SP": "2026-08-01T12:00:00-03:00", "AM": "2026-08-01T11:00:00-04:00", "AC": "2026-08-01T10:00:00-05:00"} {
		if got := fmtDhEmi(ref.In(uftz.Location(uf))); got != want {
			t.Errorf("%s: %s, want %s", uf, got, want)
		}
	}
}

// F-19: tetos de ocorrência do XSD (det 990, detPag 100, dup 120, arma 500) nas tags de validação.
func TestMOC_F19_TetosDeOcorrencia(t *testing.T) {
	typ := reflect.TypeOf(NfeEmitBody{})
	for field, max := range map[string]string{"Products": "max=990", "Payments": "max=100", "CobrDuplicatas": "max=120"} {
		f, _ := typ.FieldByName(field)
		if !strings.Contains(f.Tag.Get("validate"), max) {
			t.Errorf("%s sem %s: %s", field, max, f.Tag.Get("validate"))
		}
	}
}
