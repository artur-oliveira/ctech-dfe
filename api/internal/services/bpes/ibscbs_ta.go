package bpes

import (
	"regexp"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// IBS/CBS do BP-e TA (NT 2025.002 v2.00). A classificação tributária (cClassTrib) vem da tabela do IT 2025.002 RT, vendorizada em
// go-dfe/validate; o CST sai da própria linha da tabela e os grupos exigidos ou vedados saem dos indicadores do CST e do
// cClassTrib. As alíquotas de 2026 a 2028 são as da lei (LC 214/25) e o servidor as aplica; o corpo só informa alíquota
// onde a lei do ano ainda não a fixa. Base de cálculo = valor pago (vPgto).

const (
	// Ano de partida das alíquotas por lei: 2026 (0,1% UF, 0% município, 0,9% CBS) e 2027 e 2028 (0,05% UF e município; a CBS
	// de 2027 em diante é a de referência, ainda não publicada).
	ibsYearFirst = 2026
	ibsYearLast  = 2028
	// Compra governamental: a partir de 2033 a CBS também zera para estados, DF e municípios.
	ibsGovCBSYear = 2033
	ibsGovYear    = 2027
	ibsRatePlaces = 4
	hundred       = 100
)

// taYearUF dá o fuso (Brasília) do ano de emissão que a validação usa antes de o emitente ser lido; só difere do ano da chave
// por horas na virada de 31/12 para 1/1 em UF de fuso a oeste.
const taYearUF = "SP"

var refDFeAntPattern = regexp.MustCompile(`^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$`)

// BpeTATax é o grupo IBSCBS do TA. ClassTrib (6 dígitos) é escolhido na tabela de classificação tributária do TA. Os demais
// grupos só valem quando a tabela os exige ou admite para a classificação escolhida.
type BpeTATax struct {
	ClassTrib string `json:"class_trib" validate:"omitempty,len=6,numeric"`
	// Donation é indDoacao (valor 1): exigido, vedado ou opcional conforme a classificação.
	Donation bool `json:"donation"`
	// IBSUFRate, IBSMunRate e CBSRate só entram onde a lei do ano não fixa a alíquota (IBS depois de 2028, CBS depois de 2026).
	IBSUFRate  *string `json:"ibs_uf_rate" validate:"omitempty"`
	IBSMunRate *string `json:"ibs_mun_rate" validate:"omitempty"`
	CBSRate    *string `json:"cbs_rate" validate:"omitempty"`
	// Deferral são os percentuais de diferimento (pDif), só com CST que exige diferimento. Devolution são os valores devolvidos.
	Deferral       *BpeTAEntes      `json:"deferral" validate:"omitempty"`
	Devolution     *BpeTAEntes      `json:"devolution" validate:"omitempty"`
	Regular        *BpeTARegular    `json:"regular" validate:"omitempty"`
	Government     *BpeTAGovernment `json:"government" validate:"omitempty"`
	CreditReversal *BpeTAReversal   `json:"credit_reversal" validate:"omitempty"`
	Simples        *BpeTASimples    `json:"simples" validate:"omitempty"`
	TotalTaxes     *string          `json:"total_taxes" validate:"omitempty"`
	FiscalInfo     *string          `json:"fiscal_info" validate:"omitempty,min=1,max=2000"`
}

// BpeTAEntes agrupa um valor por tributo: IBS da UF, IBS do município e CBS.
type BpeTAEntes struct {
	IBSUF  *string `json:"ibs_uf" validate:"omitempty"`
	IBSMun *string `json:"ibs_mun" validate:"omitempty"`
	CBS    *string `json:"cbs" validate:"omitempty"`
}

// BpeTARegular é a tributação regular (gTribRegular): como seria a tributação se a condição resolutória ou suspensiva não fosse
// cumprida. As alíquotas são as efetivas; os valores saem de vBC x alíquota.
type BpeTARegular struct {
	CST        string `json:"cst" validate:"required,len=3,numeric"`
	ClassTrib  string `json:"class_trib" validate:"required,len=6,numeric"`
	IBSUFRate  string `json:"ibs_uf_rate" validate:"required"`
	IBSMunRate string `json:"ibs_mun_rate" validate:"required"`
	CBSRate    string `json:"cbs_rate" validate:"required"`
}

// BpeTAGovernment é a compra governamental (ide/gCompraGov e imp/gTribCompraGov). Entity é tpEnteGov: 1 União, 2 Estado, 3 Distrito
// Federal, 4 Município, 5 e 6 outros entes. Operation é tpOperGov: 1 fornecimento com pagamento posterior, 2 recebimento com
// fornecimento já realizado, 3 fornecimento com pagamento já realizado, 4 recebimento com fornecimento posterior.
type BpeTAGovernment struct {
	Entity     string   `json:"entity" validate:"required,oneof=1 2 3 4 5 6"`
	Reducer    string   `json:"reducer" validate:"required"`
	Operation  string   `json:"operation" validate:"required,oneof=1 2 3 4"`
	References []string `json:"references" validate:"omitempty,max=99"`
}

// BpeTAReversal é o estorno de crédito (gEstornoCred), exigido por classificações como a 410026.
type BpeTAReversal struct {
	IBS string `json:"ibs" validate:"required"`
	CBS string `json:"cbs" validate:"required"`
}

// BpeTASimples é a composição do IBS e da CBS para optante do Simples Nacional (gTribSN): alíquotas efetivas sobre o valor pago.
type BpeTASimples struct {
	IBSRate *string `json:"ibs_rate" validate:"omitempty"`
	CBSRate *string `json:"cbs_rate" validate:"omitempty"`
}

// validateTATax confere o grupo contra a tabela de classificação tributária e as regras de presença do leiaute. year é o ano de emissão.
func validateTATax(t BpeTATax, year int) error {
	class, ok := validate.IBSCBSClassByCode(t.ClassTrib)
	if !ok {
		return problem.BadRequest("classificação tributária inexistente na tabela")
	}
	if !class.BPeTA {
		return problem.BadRequest("classificação tributária não permitida no BP-e TA")
	}
	cst, _ := validate.IBSCBSCSTByCode(class.CST)
	switch {
	case class.Doacao == 0 && t.Donation:
		return problem.BadRequest("a classificação tributária não admite indicador de doação")
	case class.Doacao == 1 && !t.Donation:
		return problem.BadRequest("a classificação tributária exige o indicador de doação")
	}
	for _, v := range []*string{t.TotalTaxes} {
		if v == nil {
			continue
		}
		if _, err := nonNegative(*v); err != nil {
			return problem.BadRequest("valor total de tributos inválido")
		}
	}
	if t.Regular != nil != class.Regular {
		return problem.BadRequest("a tributação regular só vale (e é obrigatória) nas classificações que a exigem")
	}
	if t.CreditReversal != nil != class.Estorno {
		return problem.BadRequest("o estorno de crédito só vale (e é obrigatório) nas classificações que o exigem")
	}
	if t.Deferral != nil && !cst.GDif {
		return problem.BadRequest("o CST da classificação não admite diferimento")
	}
	if t.Deferral == nil && cst.GDif && cst.GIBSCBS {
		return problem.BadRequest("o CST da classificação exige o percentual de diferimento")
	}
	if !cst.GIBSCBS && (t.Deferral != nil || t.Devolution != nil || t.Government != nil || t.Regular != nil) {
		return problem.BadRequest("a classificação tributária não admite os grupos de IBS e CBS")
	}
	if err := validateTARates(t, year, cst.GIBSCBS); err != nil {
		return err
	}
	for _, e := range []*BpeTAEntes{t.Deferral, t.Devolution} {
		if e == nil {
			continue
		}
		for _, v := range []*string{e.IBSUF, e.IBSMun, e.CBS} {
			if v == nil {
				continue
			}
			if _, err := nonNegative(*v); err != nil {
				return problem.BadRequest("valor de diferimento ou devolução inválido")
			}
		}
	}
	if r := t.Regular; r != nil {
		if _, ok := validate.IBSCBSCSTByCode(r.CST); !ok {
			return problem.BadRequest("CST da tributação regular inexistente")
		}
		if _, ok := validate.IBSCBSClassByCode(r.ClassTrib); !ok {
			return problem.BadRequest("classificação da tributação regular inexistente")
		}
		for _, v := range []string{r.IBSUFRate, r.IBSMunRate, r.CBSRate} {
			if _, err := nonNegative(v); err != nil {
				return problem.BadRequest("alíquota da tributação regular inválida")
			}
		}
	}
	if g := t.Government; g != nil {
		if _, err := nonNegative(g.Reducer); err != nil {
			return problem.BadRequest("redutor de compra governamental inválido")
		}
		if err := validateGovReferences(g); err != nil {
			return err
		}
	}
	if c := t.CreditReversal; c != nil {
		for _, v := range []string{c.IBS, c.CBS} {
			if _, err := nonNegative(v); err != nil {
				return problem.BadRequest("valor de estorno de crédito inválido")
			}
		}
	}
	if s := t.Simples; s != nil {
		for _, v := range []*string{s.IBSRate, s.CBSRate} {
			if v == nil {
				continue
			}
			if _, err := nonNegative(*v); err != nil {
				return problem.BadRequest("alíquota do Simples Nacional inválida")
			}
		}
	}
	return nil
}

// validateGovReferences confere as chaves de DFe anterior conforme o tipo de operação (F139a a F139d).
func validateGovReferences(g *BpeTAGovernment) error {
	n := len(g.References)
	for _, k := range g.References {
		if !refDFeAntPattern.MatchString(k) {
			return problem.BadRequest("chave de DFe referenciado inválida")
		}
	}
	switch {
	case (g.Operation == "1" || g.Operation == "4") && n != 0:
		return problem.BadRequest("esta operação governamental não leva chave de DFe referenciado")
	case g.Operation == "2" && n != 1:
		return problem.BadRequest("esta operação governamental exige uma chave de DFe referenciado")
	case g.Operation == "3" && n < 1:
		return problem.BadRequest("esta operação governamental exige ao menos uma chave de DFe referenciado")
	}
	return nil
}

// validateTARates recusa alíquota informada onde a lei do ano já a fixa e exige onde ela ainda não existe.
func validateTARates(t BpeTATax, year int, hasGroup bool) error {
	given := map[string]*string{"gIBSUF": t.IBSUFRate, "gIBSMun": t.IBSMunRate, "gCBS": t.CBSRate}
	for _, e := range []string{"gIBSUF", "gIBSMun", "gCBS"} {
		v := given[e]
		_, fixed := legalRate(e, year)
		switch {
		case v != nil:
			if _, err := nonNegative(*v); err != nil {
				return problem.BadRequest("alíquota de IBS ou CBS inválida")
			}
			if fixed {
				return problem.BadRequest("a alíquota deste tributo é fixada em lei para o ano de emissão; não a informe")
			}
		case !fixed && hasGroup:
			return problem.BadRequest("informe a alíquota: a lei ainda não a fixa para o ano de emissão")
		}
	}
	return nil
}

// legalRate é a alíquota fixada em lei para o ano (F82, F92 e F103).
func legalRate(group string, year int) (decimal.Decimal, bool) {
	r := func(s string) decimal.Decimal { return decimal.RequireFromString(s) }
	switch group {
	case "gIBSUF":
		if year == ibsYearFirst {
			return r("0.1"), true
		}
		if year > ibsYearFirst && year <= ibsYearLast {
			return r("0.05"), true
		}
	case "gIBSMun":
		if year == ibsYearFirst {
			return decimal.Zero, true
		}
		if year > ibsYearFirst && year <= ibsYearLast {
			return r("0.05"), true
		}
	case "gCBS":
		if year == ibsYearFirst {
			return r("0.9"), true
		}
	}
	return decimal.Zero, false
}

// taEnte é o cálculo de um dos três tributos.
type taEnte struct {
	group, rateTag, valueTag string
	base                     decimal.Decimal
}

func (e taEnte) pct(d decimal.Decimal) decimal.Decimal {
	return e.base.Mul(d).Div(decimal.NewFromInt(hundred))
}

// govZeroed diz se a compra governamental zera a alíquota do tributo (F139).
func govZeroed(group, entity string, year int) bool {
	if year < ibsGovYear {
		return false
	}
	switch entity {
	case "1":
		return group == "gIBSUF" || group == "gIBSMun"
	case "2", "3":
		return group == "gIBSMun" || (group == "gCBS" && year >= ibsGovCBSYear)
	case "4":
		return group == "gIBSUF" || (group == "gCBS" && year >= ibsGovCBSYear)
	}
	return false
}

func entePtr(e *BpeTAEntes, group string) *string {
	if e == nil {
		return nil
	}
	switch group {
	case "gIBSUF":
		return e.IBSUF
	case "gIBSMun":
		return e.IBSMun
	}
	return e.CBS
}

// buildTATax monta o imp do TA: IBSCBS (com os grupos que a classificação pede), vTotTrib, infAdFisco e vTotDFe. Devolve também o
// grupo gCompraGov, que vai em ide.
func buildTATax(t BpeTATax, tot totals, year int) (imp map[string]any, gov map[string]any) {
	class, _ := validate.IBSCBSClassByCode(t.ClassTrib)
	cst, _ := validate.IBSCBSCSTByCode(class.CST)
	ibscbs := map[string]any{"CST": class.CST, "cClassTrib": class.Code}
	if t.Donation {
		ibscbs["indDoacao"] = "1"
	}
	vTot := tot.vBP
	if cst.GIBSCBS {
		g, vIBS, vCBS, govGroup := buildTAGroups(t, class, cst, tot, year)
		ibscbs["gIBSCBS"] = g
		gov = govGroup
		if year >= ibsGovYear {
			vTot = vTot.Add(vIBS).Add(vCBS) // F140: de 2027 em diante o total soma IBS e CBS; em 2026 é só vBP
		}
	}
	if c := t.CreditReversal; c != nil {
		ibscbs["gEstornoCred"] = map[string]any{"vIBSEstCred": money(parseDec(c.IBS)), "vCBSEstCred": money(parseDec(c.CBS))}
	}
	if s := t.Simples; s != nil {
		sn := map[string]any{}
		if s.IBSRate != nil {
			p := parseDec(*s.IBSRate)
			sn["gIBSSN"] = map[string]any{"pIBSSN": rateStr(p), "vIBSSN": money(tot.vPgto.Mul(p).Div(decimal.NewFromInt(hundred)).Round(moneyScale))}
		}
		if s.CBSRate != nil {
			p := parseDec(*s.CBSRate)
			sn["gCBSSN"] = map[string]any{"pCBSSN": rateStr(p), "vCBSSN": money(tot.vPgto.Mul(p).Div(decimal.NewFromInt(hundred)).Round(moneyScale))}
		}
		ibscbs["gTribSN"] = sn
	}
	imp = map[string]any{"IBSCBS": ibscbs, "vTotDFe": money(vTot)}
	if t.TotalTaxes != nil {
		imp["vTotTrib"] = money(parseDec(*t.TotalTaxes))
	}
	setIfPtr(imp, "infAdFisco", t.FiscalInfo)
	return imp, gov
}

// buildTAGroups monta gIBSCBS. Devolve também vIBS e vCBS (para o vTotDFe) e o gCompraGov de ide.
func buildTAGroups(t BpeTATax, class validate.IBSCBSClass, cst validate.IBSCBSCST, tot totals, year int) (g map[string]any, vIBS, vCBS decimal.Decimal, gov map[string]any) {
	base := tot.vPgto
	g = map[string]any{"vBC": money(base)}
	var reducer decimal.Decimal
	if t.Government != nil {
		reducer = parseDec(t.Government.Reducer)
		gov = map[string]any{"tpEnteGov": t.Government.Entity, "pRedutor": rateStr(reducer), "tpOperGov": t.Government.Operation}
		if len(t.Government.References) > 0 {
			refs := make([]any, 0, len(t.Government.References))
			for _, k := range t.Government.References {
				refs = append(refs, k)
			}
			gov["refDFeAnt"] = refs
		}
	}
	given := map[string]*string{"gIBSUF": t.IBSUFRate, "gIBSMun": t.IBSMunRate, "gCBS": t.CBSRate}
	entes := []taEnte{
		{"gIBSUF", "pIBSUF", "vIBSUF", base}, {"gIBSMun", "pIBSMun", "vIBSMun", base}, {"gCBS", "pCBS", "vCBS", base},
	}
	values := map[string]decimal.Decimal{}
	var compraGovTrib map[string]any
	if t.Government != nil {
		compraGovTrib = map[string]any{}
	}
	for _, e := range entes {
		nominal, fixed := legalRate(e.group, year)
		if !fixed {
			nominal = parseOpt(given[e.group])
		}
		if class.Regular {
			nominal = decimal.Zero
		}
		applied := nominal
		if t.Government != nil {
			tag := map[string]string{"gIBSUF": "IBSUF", "gIBSMun": "IBSMun", "gCBS": "CBS"}[e.group]
			compraGovTrib["pAliq"+tag] = rateStr(nominal)
			compraGovTrib["vTrib"+tag] = money(e.pct(nominal).Round(moneyScale))
			if govZeroed(e.group, t.Government.Entity, year) {
				applied = decimal.Zero
			}
		}
		grp := map[string]any{e.rateTag: rateStr(applied)}
		rate := applied
		if cst.GRed || t.Government != nil {
			red := class.PRedIBS
			if e.group == "gCBS" {
				red = class.PRedCBS
			}
			pRed := decimal.NewFromFloat(red)
			if !cst.GRed {
				pRed = decimal.Zero // compra governamental sem redução do CST: gRed com zero (F86)
			}
			eff := applied.Mul(decimal.NewFromInt(hundred).Sub(pRed)).Div(decimal.NewFromInt(hundred))
			if t.Government != nil {
				eff = eff.Mul(decimal.NewFromInt(hundred).Sub(reducer)).Div(decimal.NewFromInt(hundred))
			}
			eff = eff.Round(ibsRatePlaces)
			grp["gRed"] = map[string]any{"pRedAliq": rateStr(pRed), "pAliqEfet": rateStr(eff)}
			rate = eff
		}
		gross := e.pct(rate)
		vDif := decimal.Zero
		if p := entePtr(t.Deferral, e.group); cst.GDif {
			pDif := parseOpt(p)
			vDif = gross.Mul(pDif).Div(decimal.NewFromInt(hundred)).Round(moneyScale)
			grp["gDif"] = map[string]any{"pDif": rateStr(pDif), "vDif": money(vDif)}
		}
		vDev := decimal.Zero
		if p := entePtr(t.Devolution, e.group); p != nil {
			vDev = parseDec(*p)
			grp["gDevTrib"] = map[string]any{"vDevTrib": money(vDev)}
		}
		v := gross.Round(moneyScale).Sub(vDif).Sub(vDev)
		grp[e.valueTag] = money(v)
		values[e.group] = v
		g[e.group] = grp
	}
	vIBS = values["gIBSUF"].Add(values["gIBSMun"])
	vCBS = values["gCBS"]
	// A ordem do leiaute coloca vIBS entre gIBSMun e gCBS; o mapa é reordenado pelo xsdorder na assinatura.
	g["vIBS"] = money(vIBS)
	if r := t.Regular; r != nil {
		pUF, pMun, pCBS := parseDec(r.IBSUFRate), parseDec(r.IBSMunRate), parseDec(r.CBSRate)
		pct := func(p decimal.Decimal) string {
			return money(base.Mul(p).Div(decimal.NewFromInt(hundred)).Round(moneyScale))
		}
		g["gTribRegular"] = map[string]any{
			"CSTReg": r.CST, "cClassTribReg": r.ClassTrib,
			"pAliqEfetRegIBSUF": rateStr(pUF), "vTribRegIBSUF": pct(pUF),
			"pAliqEfetRegIBSMun": rateStr(pMun), "vTribRegIBSMun": pct(pMun),
			"pAliqEfetRegCBS": rateStr(pCBS), "vTribRegCBS": pct(pCBS),
		}
	}
	if compraGovTrib != nil {
		g["gTribCompraGov"] = compraGovTrib
	}
	return g, vIBS, vCBS, gov
}
