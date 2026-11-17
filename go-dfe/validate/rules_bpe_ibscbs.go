package validate

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Regras de IBS/CBS do BP-e TA (NT 2025.002 v2.00, F76 a F140), provadas só com o documento e as tabelas de CST e cClassTrib
// vendorizadas (ibscbs.go). Só a família "bpeta" (o comum e o TM não têm o grupo no leiaute lido).
//
// Aritmética e estrutura certas são Block; o que depende da tabela (que o fisco atualiza) é Warn. Fora desta lista, de
// propósito: F138 (a regra do MOC não diz se a soma das tags de compra governamental deve igualar ou diferir dos valores
// do grupo), F139e a F139k (consultam o banco do autorizador), as regras do Simples Nacional da NT 2026.005 (só a emissão
// do grupo gTribSN é feita) e as de BP-e múltiplo e substituição.

const (
	ibsTol           = 0.01
	ibsRateTol       = 0.0001
	ibsCRTSimples    = "1"
	ibsCRTExcesso    = "2"
	ibsCRTMEI        = "4"
	ibsRefDFeAntMax  = 99
	ibsEnteOutros    = "5"
	ibsEnteOutros2   = "6"
	ibsOperPagPost   = "1"
	ibsOperRecebido  = "2"
	ibsOperPagoJa    = "3"
	ibsOperRecebPost = "4"
)

// ibsEnte descreve um dos três grupos de tributo (IBS da UF, IBS do município e CBS).
type ibsEnte struct {
	group, rate, value string // gIBSUF, pIBSUF, vIBSUF
	label              string
	cstatReqRed        int
	cstatNoRed         int
	cstatReqDif        int
	cstatNoDif         int
	cstatDif           int
	cstatValue         int
	cstatNeg           int
	cstatEfet          int
	cstatPRed          int
	cstatRate          int
}

var ibsEntes = []ibsEnte{
	{group: "gIBSUF", rate: "pIBSUF", value: "vIBSUF", label: "IBS da UF", cstatRate: 316, cstatReqRed: 366, cstatPRed: 389, cstatEfet: 390, cstatNoRed: 367, cstatReqDif: 319, cstatNoDif: 368, cstatDif: 320, cstatNeg: 395, cstatValue: 318},
	{group: "gIBSMun", rate: "pIBSMun", value: "vIBSMun", label: "IBS do municipio", cstatRate: 321, cstatReqRed: 369, cstatPRed: 391, cstatEfet: 392, cstatNoRed: 381, cstatReqDif: 324, cstatNoDif: 382, cstatDif: 325, cstatNeg: 396, cstatValue: 323},
	{group: "gCBS", rate: "pCBS", value: "vCBS", label: "CBS", cstatRate: 326, cstatReqRed: 383, cstatPRed: 393, cstatEfet: 394, cstatNoRed: 384, cstatReqDif: 328, cstatNoDif: 385, cstatDif: 329, cstatNeg: 397, cstatValue: 327},
}

// ibsDoc reúne o que as regras leem do imp do TA.
type ibsDoc struct {
	inf      map[string]any
	ibscbs   map[string]any
	g        map[string]any // gIBSCBS
	class    IBSCBSClass
	classOK  bool
	cst      IBSCBSCST
	cstOK    bool
	compraGv map[string]any // ide/gCompraGov
	crt      string
	year     int
}

func ibsLoad(inf map[string]any) (ibsDoc, bool) {
	d := ibsDoc{inf: inf, ibscbs: sub(inf, "imp", "IBSCBS")}
	if d.ibscbs == nil {
		return d, false
	}
	d.g = sub(d.ibscbs, "gIBSCBS")
	d.class, d.classOK = IBSCBSClassByCode(mdfeStr(d.ibscbs, "cClassTrib"))
	d.cst, d.cstOK = IBSCBSCSTByCode(mdfeStr(d.ibscbs, "CST"))
	d.compraGv = sub(inf, "ide", "gCompraGov")
	d.crt = mdfeStr(sub(inf, "emit"), "CRT")
	if t, err := time.Parse(time.RFC3339, mdfeStr(sub(inf, "ide"), "dhEmi")); err == nil {
		d.year = t.Year()
	}
	return d, true
}

func (d ibsDoc) simples() bool {
	return d.crt == ibsCRTSimples || d.crt == ibsCRTExcesso || d.crt == ibsCRTMEI
}

// effRate é a alíquota que entra no cálculo: a efetiva quando há redução, senão a nominal.
func ibsEffRate(e ibsEnte, g map[string]any) (float64, bool) {
	grp := sub(g, e.group)
	if grp == nil {
		return 0, false
	}
	if red := sub(grp, "gRed"); red != nil {
		if f, ok := num(red, "pAliqEfet"); ok {
			return f, true
		}
	}
	return num(grp, e.rate)
}

func ibsClose(a, b, tol float64) bool { return math.Abs(a-b) <= tol+1e-9 }

func ibsRule(id string, cstat int, mode Mode, check func(d ibsDoc) []string) Rule {
	return bpeRule("bpeta", id, cstat, mode, func(inf map[string]any) []string {
		d, ok := ibsLoad(inf)
		if !ok {
			return nil
		}
		return check(d)
	})
}

// ibsPerEnte repete a mesma verificação para os três grupos, com o ID e o cStat de cada um.
func ibsPerEnte(ids [3]string, mode Mode, cstat func(e ibsEnte) int, check func(d ibsDoc, e ibsEnte) []string) []Rule {
	out := make([]Rule, 0, len(ibsEntes))
	for i, e := range ibsEntes {
		e := e
		out = append(out, ibsRule(ids[i], cstat(e), mode, func(d ibsDoc) []string { return check(d, e) }))
	}
	return out
}

// ibsExpectedRate é a alíquota legal por ano de emissão (F82, F92, F103). ok=false quando o ano depende de publicação futura.
func ibsExpectedRate(e ibsEnte, year int) (float64, bool) {
	switch e.group {
	case "gIBSUF":
		switch {
		case year == 2026:
			return 0.1, true
		case year == 2027 || year == 2028:
			return 0.05, true
		}
	case "gIBSMun":
		switch {
		case year == 2026:
			return 0, true
		case year == 2027 || year == 2028:
			return 0.05, true
		}
	case "gCBS":
		if year == 2026 {
			return 0.9, true
		}
	}
	return 0, false
}

func ibsRules() []Rule {
	var rules []Rule
	rules = append(rules,
		ibsRule("F76", 311, ModeWarn, func(d ibsDoc) []string {
			if c := mdfeStr(d.ibscbs, "CST"); c != "" && !d.cstOK {
				return []string{"CST do IBS/CBS inexistente na tabela"}
			}
			return nil
		}),
		ibsRule("F77", 312, ModeWarn, func(d ibsDoc) []string {
			if c := mdfeStr(d.ibscbs, "cClassTrib"); c != "" && !d.classOK {
				return []string{"classificacao tributaria inexistente na tabela"}
			}
			return nil
		}),
		ibsRule("F78", 313, ModeWarn, func(d ibsDoc) []string {
			if d.classOK && d.class.CST != mdfeStr(d.ibscbs, "CST") {
				return []string{"classificacao tributaria incompativel com o CST"}
			}
			return nil
		}),
		ibsRule("F79", 388, ModeWarn, func(d ibsDoc) []string {
			if d.classOK && !d.class.BPeTA {
				return []string{"classificacao tributaria nao permitida no BP-e TA"}
			}
			return nil
		}),
		ibsRule("F80", 314, ModeWarn, func(d ibsDoc) []string {
			if d.cstOK && !d.cst.GIBSCBS && d.g != nil {
				return []string{"grupo gIBSCBS nao deve ser preenchido para o CST"}
			}
			return nil
		}),
		ibsRule("F81", 315, ModeWarn, func(d ibsDoc) []string {
			if d.cstOK && d.cst.GIBSCBS && d.g == nil && !d.simples() {
				return []string{"grupo gIBSCBS deve ser preenchido para o CST"}
			}
			return nil
		}),
	)
	// F82, F92, F103: alíquota legal do ano (exceto tributação regular e compra governamental, que têm regra própria).
	rules = append(rules, ibsPerEnte([3]string{"F82", "F92", "F103"}, ModeWarn, func(e ibsEnte) int { return e.cstatRate }, func(d ibsDoc, e ibsEnte) []string {
		grp := sub(d.g, e.group)
		want, ok := ibsExpectedRate(e, d.year)
		got, has := num(grp, e.rate)
		if grp == nil || !ok || !has || d.compraGv != nil {
			return nil
		}
		if d.classOK && d.class.Regular {
			want = 0
		}
		if !ibsClose(got, want, ibsRateTol) {
			return []string{fmt.Sprintf("aliquota do %s difere da vigente em %d", e.label, d.year)}
		}
		return nil
	})...)
	// F83/F93/F104 e F86/F96/F107: gRed exigido ou vedado pelo CST (e pela compra governamental).
	rules = append(rules, ibsPerEnte([3]string{"F83", "F93", "F104"}, ModeWarn, func(e ibsEnte) int { return e.cstatReqRed }, func(d ibsDoc, e ibsEnte) []string {
		if d.g == nil || !d.cstOK || !d.cst.GIBSCBS {
			return nil
		}
		if (d.cst.GRed || d.compraGv != nil) && sub(d.g, e.group, "gRed") == nil {
			return []string{"grupo gRed deve ser informado no " + e.label}
		}
		return nil
	})...)
	rules = append(rules, ibsPerEnte([3]string{"F86", "F96", "F107"}, ModeWarn, func(e ibsEnte) int { return e.cstatNoRed }, func(d ibsDoc, e ibsEnte) []string {
		if d.g == nil || !d.cstOK || d.cst.GRed {
			return nil
		}
		red := sub(d.g, e.group, "gRed")
		if red == nil {
			return nil
		}
		if d.compraGv != nil {
			if f, ok := num(red, "pRedAliq"); ok && f != 0 {
				return []string{"pRedAliq do " + e.label + " deve ser zero na compra governamental sem reducao do CST"}
			}
			return nil
		}
		return []string{"CST nao permite gRed no " + e.label}
	})...)
	// F84/F94/F105: percentual de redução igual ao da classificação.
	rules = append(rules, ibsPerEnte([3]string{"F84", "F94", "F105"}, ModeWarn, func(e ibsEnte) int { return e.cstatPRed }, func(d ibsDoc, e ibsEnte) []string {
		red := sub(d.g, e.group, "gRed")
		got, ok := num(red, "pRedAliq")
		if red == nil || !ok || !d.classOK {
			return nil
		}
		want := d.class.PRedIBS
		if e.group == "gCBS" {
			want = d.class.PRedCBS
		}
		if d.compraGv != nil && got == 0 {
			return nil // F86: com compra governamental o gRed pode vir com zero
		}
		if !ibsClose(got, want, ibsRateTol) {
			return []string{"percentual de reducao do " + e.label + " invalido para a classificacao"}
		}
		return nil
	})...)
	// F85/F95/F106: alíquota efetiva.
	rules = append(rules, ibsPerEnte([3]string{"F85", "F95", "F106"}, ModeBlock, func(e ibsEnte) int { return e.cstatEfet }, func(d ibsDoc, e ibsEnte) []string {
		grp := sub(d.g, e.group)
		red := sub(grp, "gRed")
		p, ok1 := num(grp, e.rate)
		r, ok2 := num(red, "pRedAliq")
		got, ok3 := num(red, "pAliqEfet")
		if red == nil || !ok1 || !ok2 || !ok3 {
			return nil
		}
		want := p * (1 - r/100)
		if d.compraGv != nil {
			redutor, _ := num(d.compraGv, "pRedutor")
			want *= 1 - redutor/100
		}
		if !ibsClose(got, want, ibsRateTol) {
			return []string{"aliquota efetiva do " + e.label + " calculada incorretamente"}
		}
		return nil
	})...)
	// F87/F97/F108 e F88/F98/F109: diferimento exigido ou vedado pelo CST.
	rules = append(rules, ibsPerEnte([3]string{"F87", "F97", "F108"}, ModeWarn, func(e ibsEnte) int { return e.cstatReqDif }, func(d ibsDoc, e ibsEnte) []string {
		if d.g != nil && d.cstOK && d.cst.GIBSCBS && d.cst.GDif && sub(d.g, e.group, "gDif") == nil {
			return []string{"CST obriga o diferimento do " + e.label}
		}
		return nil
	})...)
	rules = append(rules, ibsPerEnte([3]string{"F88", "F98", "F109"}, ModeWarn, func(e ibsEnte) int { return e.cstatNoDif }, func(d ibsDoc, e ibsEnte) []string {
		if d.g != nil && d.cstOK && !d.cst.GDif && sub(d.g, e.group, "gDif") != nil {
			return []string{"CST nao permite diferimento do " + e.label}
		}
		return nil
	})...)
	// F89/F99/F110: vDif = vBC x alíquota x pDif.
	rules = append(rules, ibsPerEnte([3]string{"F89", "F99", "F110"}, ModeBlock, func(e ibsEnte) int { return e.cstatDif }, func(d ibsDoc, e ibsEnte) []string {
		dif := sub(d.g, e.group, "gDif")
		bc, ok1 := num(d.g, "vBC")
		rate, ok2 := ibsEffRate(e, d.g)
		pDif, ok3 := num(dif, "pDif")
		vDif, ok4 := num(dif, "vDif")
		if dif == nil || !ok1 || !ok2 || !ok3 || !ok4 {
			return nil
		}
		if !ibsClose(vDif, bc*rate/100*pDif/100, ibsTol) {
			return []string{"valor do diferimento do " + e.label + " difere do calculado"}
		}
		return nil
	})...)
	// F90/F101/F111: diferimento mais devolução não pode passar do tributo bruto.
	rules = append(rules, ibsPerEnte([3]string{"F90", "F101", "F111"}, ModeBlock, func(e ibsEnte) int { return e.cstatNeg }, func(d ibsDoc, e ibsEnte) []string {
		grp := sub(d.g, e.group)
		bc, ok1 := num(d.g, "vBC")
		rate, ok2 := ibsEffRate(e, d.g)
		if grp == nil || !ok1 || !ok2 {
			return nil
		}
		vDif, _ := num(sub(grp, "gDif"), "vDif")
		vDev, _ := num(sub(grp, "gDevTrib"), "vDevTrib")
		if vDif+vDev > bc*rate/100+ibsTol {
			return []string{"valor do " + e.label + " nao pode resultar negativo"}
		}
		return nil
	})...)
	// F91/F100/F112: valor do tributo.
	rules = append(rules, ibsPerEnte([3]string{"F91", "F100", "F112"}, ModeBlock, func(e ibsEnte) int { return e.cstatValue }, func(d ibsDoc, e ibsEnte) []string {
		grp := sub(d.g, e.group)
		bc, ok1 := num(d.g, "vBC")
		rate, ok2 := ibsEffRate(e, d.g)
		got, ok3 := num(grp, e.value)
		if grp == nil || !ok1 || !ok2 || !ok3 {
			return nil
		}
		vDif, _ := num(sub(grp, "gDif"), "vDif")
		vDev, _ := num(sub(grp, "gDevTrib"), "vDevTrib")
		if !ibsClose(got, bc*rate/100-vDif-vDev, ibsTol) {
			return []string{"valor do " + e.label + " difere do calculado"}
		}
		return nil
	})...)
	rules = append(rules,
		ibsRule("F102", 317, ModeBlock, func(d ibsDoc) []string {
			uf, ok1 := num(sub(d.g, "gIBSUF"), "vIBSUF")
			mun, ok2 := num(sub(d.g, "gIBSMun"), "vIBSMun")
			tot, ok3 := num(d.g, "vIBS")
			if d.g == nil || !ok1 || !ok2 || !ok3 {
				return nil
			}
			if !ibsClose(tot, uf+mun, ibsTol) {
				return []string{"total de IBS difere da soma de UF e municipio"}
			}
			return nil
		}),
		ibsRule("F113", 330, ModeWarn, func(d ibsDoc) []string {
			if d.g != nil && d.classOK && d.class.Regular && sub(d.g, "gTribRegular") == nil {
				return []string{"classificacao tributaria obriga a tributacao regular"}
			}
			return nil
		}),
		ibsRule("F114", 322, ModeWarn, func(d ibsDoc) []string {
			if d.g != nil && d.classOK && !d.class.Regular && sub(d.g, "gTribRegular") != nil {
				return []string{"classificacao tributaria nao permite a tributacao regular"}
			}
			return nil
		}),
		ibsRule("F115", 331, ModeWarn, func(d ibsDoc) []string {
			if r := sub(d.g, "gTribRegular"); r != nil {
				if _, ok := IBSCBSCSTByCode(mdfeStr(r, "CSTReg")); !ok {
					return []string{"CST da tributacao regular inexistente"}
				}
			}
			return nil
		}),
		ibsRule("F116", 332, ModeWarn, func(d ibsDoc) []string {
			if r := sub(d.g, "gTribRegular"); r != nil {
				if _, ok := IBSCBSClassByCode(mdfeStr(r, "cClassTribReg")); !ok {
					return []string{"classificacao da tributacao regular inexistente"}
				}
			}
			return nil
		}),
	)
	// F117 a F119: valor da tributação regular = vBC x alíquota efetiva regular.
	regular := [3][3]string{{"F117", "pAliqEfetRegIBSUF", "vTribRegIBSUF"}, {"F118", "pAliqEfetRegIBSMun", "vTribRegIBSMun"}, {"F119", "pAliqEfetRegCBS", "vTribRegCBS"}}
	cstatReg := [3]int{333, 334, 335}
	for i, r := range regular {
		r, cs := r, cstatReg[i]
		rules = append(rules, ibsRule(r[0], cs, ModeBlock, func(d ibsDoc) []string {
			reg := sub(d.g, "gTribRegular")
			bc, ok1 := num(d.g, "vBC")
			p, ok2 := num(reg, r[1])
			v, ok3 := num(reg, r[2])
			if reg == nil || !ok1 || !ok2 || !ok3 {
				return nil
			}
			if !ibsClose(v, bc*p/100, ibsTol) {
				return []string{"valor da tributacao regular (" + r[2] + ") difere do calculado"}
			}
			return nil
		}))
	}
	rules = append(rules,
		ibsRule("F136", 398, ModeWarn, func(d ibsDoc) []string {
			if d.compraGv != nil && d.g != nil && sub(d.g, "gTribCompraGov") == nil && (!d.cstOK || d.cst.GIBSCBS) {
				return []string{"compra governamental exige o grupo gTribCompraGov"}
			}
			return nil
		}),
		ibsRule("F137", 399, ModeWarn, func(d ibsDoc) []string {
			if d.compraGv == nil && sub(d.g, "gTribCompraGov") != nil {
				return []string{"gTribCompraGov so vale com compra governamental"}
			}
			return nil
		}),
		ibsRule("F138a", 303, ModeWarn, func(d ibsDoc) []string {
			if d.classOK && !d.class.Estorno && sub(d.ibscbs, "gEstornoCred") != nil {
				return []string{"classificacao tributaria veda o grupo de estorno de credito"}
			}
			return nil
		}),
		ibsRule("F138b", 304, ModeWarn, func(d ibsDoc) []string {
			if d.classOK && d.class.Estorno && sub(d.ibscbs, "gEstornoCred") == nil {
				return []string{"classificacao tributaria exige o grupo de estorno de credito"}
			}
			return nil
		}),
		ibsRule("F139", 347, ModeWarn, func(d ibsDoc) []string { return ibsCompraGovRates(d) }),
		ibsRule("F139a-d", 1004, ModeWarn, func(d ibsDoc) []string {
			if d.compraGv == nil {
				return nil
			}
			n := len(list(d.compraGv, "refDFeAnt"))
			switch mdfeStr(d.compraGv, "tpOperGov") {
			case ibsOperPagPost, ibsOperRecebPost:
				if n != 0 {
					return []string{"chave de DFe referenciado nao deve ser informada nesta operacao governamental"}
				}
			case ibsOperRecebido:
				if n != 1 {
					return []string{"deve ser informada uma chave de DFe referenciado nesta operacao governamental"}
				}
			case ibsOperPagoJa:
				if n < 1 || n > ibsRefDFeAntMax {
					return []string{"deve ser informada ao menos uma chave de DFe referenciado nesta operacao governamental"}
				}
			}
			return nil
		}),
		ibsRule("F140", 365, ModeBlock, func(d ibsDoc) []string {
			tot, ok := num(sub(d.inf, "imp"), "vTotDFe")
			vbp, ok2 := num(sub(d.inf, "infValorBPe"), "vBP")
			if !ok || !ok2 {
				return nil
			}
			want := vbp
			if d.year >= 2027 {
				ibs, _ := num(d.g, "vIBS")
				cbs, _ := num(sub(d.g, "gCBS"), "vCBS")
				want += ibs + cbs
			}
			if !ibsClose(tot, want, ibsTol) {
				return []string{"total do DFe difere do valor do bilhete mais IBS e CBS (2026: so o valor do bilhete)"}
			}
			return nil
		}),
	)
	return rules
}

// ibsCompraGovRates é a F139: na compra governamental, as alíquotas dos outros entes são zero conforme o ente comprador.
func ibsCompraGovRates(d ibsDoc) []string {
	if d.compraGv == nil || d.g == nil {
		return nil
	}
	ente := mdfeStr(d.compraGv, "tpEnteGov")
	if ente == ibsEnteOutros || ente == ibsEnteOutros2 || d.year < 2027 {
		return nil
	}
	zero := map[string][]string{
		"1": {"pIBSUF", "pIBSMun"}, "2": {"pIBSMun"}, "3": {"pIBSMun"}, "4": {"pIBSUF"},
	}[ente]
	if d.year >= 2033 && ente != "1" {
		zero = append(zero, "pCBS")
	}
	var out []string
	for _, tag := range zero {
		for _, e := range ibsEntes {
			if e.rate != tag {
				continue
			}
			if f, ok := num(sub(d.g, e.group), tag); ok && f != 0 {
				out = append(out, "aliquota "+strings.TrimPrefix(tag, "p")+" deve ser zero para este ente governamental")
			}
		}
	}
	return out
}

func init() { catalog = append(catalog, ibsRules()...) }
