package validate

import (
	"strings"
	"testing"
)

func TestLoadAllSpecs(t *testing.T) {
	for _, n := range []string{"nfe", "mdfe", "cte", "cteos", "ctesimp", "mdfe_rodo", "mdfe_aereo", "mdfe_aquav", "mdfe_ferrov",
		"cte_rodo", "cte_rodoos", "cte_aereo", "cte_aquav", "cte_ferrov", "cte_duto", "cte_multimodal"} {
		if _, err := Load(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if _, err := Load("nope"); err == nil {
		t.Error("esperava erro para esquema inexistente")
	}
}

func codes(vs []Violation) map[string]bool {
	m := map[string]bool{}
	for _, v := range vs {
		m[v.Path+"|"+v.Code] = true
	}
	return m
}

func TestNFeSchemaFindings(t *testing.T) {
	s, err := Load("nfe")
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"infNFe": map[string]any{
			"@versao": "4.00",
			"@Id":     "NFe123",
			"ide":     map[string]any{"cUF": "35", "mod": "99", "bogus": "x"},
		},
	}
	got := codes(s.Validate(doc, Options{Skip: []string{"Signature"}}))
	for _, want := range []string{
		"NFe/infNFe/@Id|pattern",
		"NFe/infNFe/ide/mod|enum",
		"NFe/infNFe/ide/bogus|unknown_tag",
		"NFe/infNFe/ide/natOp|required",
		"NFe/infNFe/emit|required",
	} {
		if !got[want] {
			t.Errorf("faltou %s; got %v", want, got)
		}
	}
	if got["NFe/Signature|required"] {
		t.Error("Signature deveria ser ignorada")
	}
}

func TestCountDigits(t *testing.T) {
	cases := []struct {
		s       string
		tot, fr int
	}{{"10.50", 4, 2}, {"0.0001", 4, 4}, {"123", 3, 0}, {"0", 1, 0}}
	for _, c := range cases {
		if tt, f := countDigits(c.s); tt != c.tot || f != c.fr {
			t.Errorf("%s: %d,%d", c.s, tt, f)
		}
	}
}

func hasRule(vs []Violation, id string) bool {
	for _, v := range vs {
		if v.Rule == id {
			return true
		}
	}
	return false
}

func TestRulesWarnDefaultAndBlockOverride(t *testing.T) {
	doc := map[string]any{"infNFe": map[string]any{
		"ide": map[string]any{"mod": "55", "finNFe": "1"},
		"det": []any{map[string]any{"prod": map[string]any{"qCom": "2", "vUnCom": "10", "vProd": "25.00"}}},
	}}
	f, _ := Apply("nfe", doc, nil)
	if !hasRule(f, "I11-10") {
		t.Fatalf("I11-10 deveria disparar: %+v", f)
	}
	for _, x := range f {
		if x.Rule == "I11-10" && x.Severity != SevWarn {
			t.Errorf("padrão deve ser warn: %+v", x)
		}
	}
	if _, blocking := Apply("nfe", doc, Modes{"I11-10": ModeBlock}); !blocking {
		t.Error("override block deveria bloquear")
	}
	if f, _ := Apply("nfe", doc, Modes{"I11-10": ModeOff}); hasRule(f, "I11-10") {
		t.Error("off deveria silenciar")
	}
}

// Choice obrigatória com braços só de elementos opcionais (infNFe do CT-e) é satisfeita vazia; CNPJ|CPF não.
func TestOptionalArm(t *testing.T) {
	opt := &node{Children: []*node{{Name: "infUnidCarga", Choice: 1, Arm: 0}, {Name: "infUnidTransp", Choice: 1, Arm: 1}}}
	if !optionalArm(opt, 1) {
		t.Fatal("braços opcionais deveriam satisfazer a choice vazia")
	}
	req := &node{Children: []*node{{Name: "CNPJ", Min: 1, Choice: 1, Arm: 0}, {Name: "CPF", Min: 1, Choice: 1, Arm: 1}}}
	if optionalArm(req, 1) {
		t.Fatal("CNPJ|CPF exige uma das alternativas")
	}
}

func TestLengthsAndSizedViolation(t *testing.T) {
	s, err := Load("nfe")
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][2]int{
		"NFe/infNFe/dest/xNome":          {2, 60},
		"NFe/infNFe/dest/enderDest/xLgr": {2, 60},
		"NFe/infNFe/det/prod/xProd":      {1, 120},
		"NFe/infNFe/retirada/xBairro":    {2, 60},
		"NFe/infNFe/emit/enderEmit/xMun": {2, 60},
	} {
		lo, hi, ok := s.Lengths(path)
		if !ok || lo != want[0] || hi != want[1] {
			t.Errorf("%s = %d..%d ok=%v, want %v", path, lo, hi, ok, want)
		}
	}
	if _, _, ok := s.Lengths("NFe/infNFe/nada"); ok {
		t.Error("caminho inexistente deveria devolver ok=false")
	}
	found := s.Validate(map[string]any{"infNFe": map[string]any{"dest": map[string]any{"xNome": strings.Repeat("a", 61)}}}, Options{})
	for _, v := range found {
		if v.Path == "NFe/infNFe/dest/xNome" && v.Code == CodeMaxLength {
			if v.Limit != 60 || v.Got != 61 {
				t.Errorf("limit/got = %d/%d, want 60/61", v.Limit, v.Got)
			}
			return
		}
	}
	t.Fatal("max_length de dest/xNome não encontrado")
}

func impostoDoc(imp map[string]any) map[string]any {
	return map[string]any{"infNFe": map[string]any{"det": []any{map[string]any{"imposto": imp}}}}
}

func findingsAt(t *testing.T, imp map[string]any, _ string) map[string]bool {
	t.Helper()
	s, err := Load("nfe")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, v := range s.Validate(impostoDoc(imp), Options{Skip: []string{"Signature"}}) {
		if strings.Contains(v.Path, "/imposto") {
			got[strings.TrimPrefix(v.Path, "NFe/infNFe/det/imposto")+"|"+v.Code] = true
		}
	}
	return got
}

// F-12: IPI existe nos dois braços da choice de imposto (ICMS,IPI?,II? | IPI?,ISSQN); sozinho ele não escolhe braço.
func TestChoiceSharedElementIPIIsNotAConflict(t *testing.T) {
	for _, ipi := range []map[string]any{
		{"cEnq": "999", "IPITrib": map[string]any{"CST": "50", "vBC": "100.00", "pIPI": "5.0000", "vIPI": "5.00"}},
		{"cEnq": "999", "IPINT": map[string]any{"CST": "53"}},
	} {
		imp := map[string]any{"ICMS": map[string]any{"ICMSSN102": map[string]any{"orig": "0", "CSOSN": "102"}}, "IPI": ipi}
		if got := findingsAt(t, imp, ""); got["|choice_conflict"] || got["|choice_missing"] {
			t.Errorf("ICMS + IPI é válido: %v", got)
		}
		// ISSQN + IPI (braço 2) também.
		imp = map[string]any{"IPI": ipi, "ISSQN": map[string]any{"vBC": "1.00"}}
		if got := findingsAt(t, imp, ""); got["|choice_conflict"] {
			t.Errorf("IPI + ISSQN é válido: %v", got)
		}
	}
	// Conflito real continua: ICMS (braço 1) com ISSQN (braço 2).
	imp := map[string]any{"ICMS": map[string]any{"ICMSSN102": map[string]any{"orig": "0", "CSOSN": "102"}}, "ISSQN": map[string]any{"vBC": "1.00"}}
	if got := findingsAt(t, imp, ""); !got["|choice_conflict"] {
		t.Errorf("ICMS com ISSQN deve conflitar: %v", got)
	}
}

// F-13 (e F-03 da NF-e): sequência opcional do XSD é tudo-ou-nada. Os dois grupos incompletos do ICMS60 saem
// do construtor antigo: pICMSEfet sem pRedBCEfet e vBCSTRet sem pST.
func TestOptionalSequenceAllOrNothing(t *testing.T) {
	icms := func(f map[string]any) map[string]any {
		f["orig"], f["CST"] = "0", "60"
		return map[string]any{"ICMS": map[string]any{"ICMS60": f}}
	}
	got := findingsAt(t, icms(map[string]any{"pICMSEfet": "12.00", "vBCEfet": "100.00", "vICMSEfet": "12.00"}), "")
	if !got["/ICMS/ICMS60/pRedBCEfet|required"] {
		t.Errorf("grupo Efet sem pRedBCEfet: %v", got)
	}
	got = findingsAt(t, icms(map[string]any{"vBCSTRet": "100.00", "vICMSSTRet": "18.00"}), "")
	if !got["/ICMS/ICMS60/pST|required"] {
		t.Errorf("grupo STRet sem pST: %v", got)
	}
	// Grupos inteiros e grupo ausente: nenhum achado.
	ok := icms(map[string]any{"vBCSTRet": "100.00", "pST": "18.00", "vICMSSTRet": "18.00",
		"pRedBCEfet": "0.00", "vBCEfet": "100.00", "pICMSEfet": "12.00", "vICMSEfet": "12.00"})
	for k := range findingsAt(t, ok, "") {
		if strings.HasSuffix(k, "|required") {
			t.Errorf("grupo completo não pode acusar: %s", k)
		}
	}
	for k := range findingsAt(t, icms(map[string]any{}), "") {
		if strings.HasSuffix(k, "|required") {
			t.Errorf("grupo ausente não pode acusar: %s", k)
		}
	}
	// Grupo aninhado (ICMS90: vICMSOp, pDif, vICMSDif dentro do grupo de modBC) tem o seu próprio tudo-ou-nada.
	got = findingsAt(t, map[string]any{"ICMS": map[string]any{"ICMS90": map[string]any{"orig": "0", "CST": "90",
		"modBC": "3", "vBC": "1.00", "pICMS": "1.00", "vICMS": "0.01", "vICMSOp": "1.00"}}}, "")
	if !got["/ICMS/ICMS90/pDif|required"] || !got["/ICMS/ICMS90/vICMSDif|required"] {
		t.Errorf("grupo aninhado de diferimento: %v", got)
	}
}
