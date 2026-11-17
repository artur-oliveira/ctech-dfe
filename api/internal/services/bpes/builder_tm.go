package bpes

import (
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const tpBPeTM = "4" // ide/tpBPe do transporte metropolitano

// BuildBPeTM monta o corpo da autorização do BP-e TM (raiz BPeTM, sem infBPeSupl: o TM não tem QR Code).
func BuildBPeTM(p buildParams) map[string]any {
	orgPerson := personMap(p.org)
	r := p.tm
	groups := make([]any, 0, len(r.Groups))
	var qPass int
	vBP, vBC, vICMS := decimal.Zero, decimal.Zero, decimal.Zero
	for gi, g := range r.Groups {
		items := make([]any, 0, len(g.Items))
		for ii, it := range g.Items {
			value := parseDec(it.Value)
			q, _ := strconv.Atoi(it.Passengers)
			qPass += q
			vBP = vBP.Add(value)
			imp, base, tax := tmTax(it.ICMS, value)
			vBC, vICMS = vBC.Add(base), vICMS.Add(tax)
			comps := make([]any, 0, len(it.Components))
			for _, c := range it.Components {
				comps = append(comps, map[string]any{"xNome": c.Name, "qComp": fmt.Sprintf("%05d", c.Quantity)})
			}
			item := map[string]any{"@nViagem": strconv.Itoa(orPos(it.TripNumber, ii+1)), "cMunIni": it.OriginIBGE, "qPass": strconv.Itoa(q), "vBP": money(value), "imp": imp, "Comp": comps}
			setIfPtr(item, "cMunFim", it.DestIBGE)
			setIfPtr(item, "nContInicio", it.StartCounter)
			setIfPtr(item, "nContFim", it.EndCounter)
			items = append(items, item)
		}
		grp := map[string]any{"@idEqpCont": strconv.Itoa(orPos(g.CounterID, gi+1)), "UFIniViagem": g.OriginUF, "det": items}
		setIfPtr(grp, "UFFimViagem", g.DestUF)
		setIfPtr(grp, "placa", g.Plate)
		setIfPtr(grp, "prefixo", g.Prefix)
		groups = append(groups, grp)
	}
	ide := map[string]any{
		"cUF": p.accessKey[0:2], "tpAmb": strconv.Itoa(p.environment), "mod": services.ModelBPe,
		"serie": strconv.Itoa(p.serie), "nBP": strconv.Itoa(p.number), "cBP": p.accessKey[35:43], "cDV": p.accessKey[43:],
		"modal": modalCodes[r.Modal], "dhEmi": p.now.Format(layoutDateTimeTZ), "dCompet": r.Competence, "tpEmis": p.tpEmis,
		"verProc": strOrDefault(p.tech.Version, "ctech-dfe"), "tpBPe": tpBPeTM, "CFOP": r.CFOP,
	}
	if r.SupplyIndicator {
		ide["indFornecimento"] = "1"
	}
	if p.tpEmis == tpEmisOffline {
		ide["dhCont"] = p.now.Format(layoutDateTimeTZ)
		ide["xJust"] = p.contingencyReason
	}
	inf := map[string]any{
		"@versao":  bpeVersao,
		"@Id":      "BPe" + p.accessKey,
		"ide":      ide,
		"emit":     buildEmit(p.org, orgPerson, p.orgPK),
		"detBPeTM": groups,
		"total": map[string]any{
			"qPass": strconv.Itoa(qPass), "vBP": money(vBP),
			"ICMSTot": map[string]any{"vBC": money(vBC), "vICMS": money(vICMS)},
		},
	}
	if aut := services.AutXMLNodes(r.AuthorizedXML); aut != nil {
		inf["autXML"] = aut
	}
	if r.AdditionalInfo != nil && *r.AdditionalInfo != "" {
		inf["infAdic"] = map[string]any{"infCpl": *r.AdditionalInfo}
	}
	if rt := p.respTec(); rt != nil {
		inf["infRespTec"] = rt
	}
	return services.PruneEmpty(map[string]any{
		"BPeTM": map[string]any{"@xmlns": bpeXMLNS, "infBPe": inf},
	}).(map[string]any)
}

// tmTax monta o imp de um item do TM e devolve a base e o valor do ICMS que entram no total. Base = valor do item.
func tmTax(icms BpeICMS, value decimal.Decimal) (map[string]any, decimal.Decimal, decimal.Decimal) {
	group := map[string]any{}
	base, tax := decimal.Zero, decimal.Zero
	switch icms.CST {
	case "00", "90":
		rate := parseOpt(icms.Rate)
		base = value
		tax = value.Mul(rate).Div(decimal.NewFromInt(100)).Round(moneyScale)
		group["ICMS"+icms.CST] = map[string]any{"CST": icms.CST, "vBC": money(base), "pICMS": rateStr(rate), "vICMS": money(tax)}
	case "40", "41", "51":
		group["ICMS45"] = map[string]any{"CST": icms.CST}
	case icmsSN:
		group["ICMSSN"] = map[string]any{"CST": icmsCSTSimples, "indSN": indSN}
	}
	return map[string]any{"ICMS": group}, base, tax
}

// orPos devolve o valor informado ou, na falta dele, a posição (base 1).
func orPos(v *int, pos int) int {
	if v != nil {
		return *v
	}
	return pos
}
