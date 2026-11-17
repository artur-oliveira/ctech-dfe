package bpes

import (
	"strconv"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const tpCompraNormal = "0" // ide/tpCompra: compra normal; múltiplo (1) e vinculado (2) ficam fora desta versão

// BuildBPeTA monta o corpo da autorização do BP-e TA (raiz BPeTA). Os grupos de comprador, valor, imposto e pagamento são os
// do bilhete comum (mesma estrutura no XSD); mudam a identificação, a passagem e a viagem.
func BuildBPeTA(p buildParams) map[string]any {
	r := p.ta
	shared := p
	shared.req = r.common()
	orgPerson := personMap(p.org)
	t := computeTotals(shared.req)

	ide := map[string]any{
		"cUF": p.accessKey[0:2], "tpAmb": strconv.Itoa(p.environment), "mod": services.ModelBPe,
		"serie": strconv.Itoa(p.serie), "nBP": strconv.Itoa(p.number), "cBP": p.accessKey[35:43], "cDV": p.accessKey[43:],
		"modal": "2", "dhEmi": p.now.Format(layoutDateTimeTZ), "tpEmis": p.tpEmis,
		"verProc": strOrDefault(p.tech.Version, "ctech-dfe"), "tpBPe": tpBPeNormal, "tpCompra": tpCompraNormal,
		"indPres": r.Presence, "UFIni": r.Origin.UF, "cMunIni": r.Origin.IBGECode,
		"UFFim": r.Destination.UF, "cMunFim": r.Destination.IBGECode,
	}
	setIfPtr(ide, "nroBPIATA", r.IATANumber)
	if p.tpEmis == tpEmisOffline {
		ide["dhCont"] = p.now.Format(layoutDateTimeTZ)
		ide["xJust"] = p.contingencyReason
	}

	passage := map[string]any{"dhEmb": r.Passage.Boarding, "dhValidade": r.Passage.Validity}
	if px := r.Passage.Passenger; px != nil {
		passage["infPassageiro"] = shared.buildPassage()["infPassageiro"]
	}
	trips := make([]any, 0, len(r.Trips))
	for _, tr := range r.Trips {
		v := map[string]any{
			"nroVoo": tr.FlightNumber, "SiglaCiaOperVoo": tr.Carrier, "tpViagem": tr.TripType,
			"cAeroOrig": tr.FromAirport, "cAeroDest": tr.ToAirport, "tpServ": tr.ServiceType,
			"tpAcomodacao": tr.Accommodation, "tpTrecho": tr.LegType, "dhViagem": tr.Departure,
		}
		setIfPtr(v, "dhConexao", tr.Connection)
		trips = append(trips, v)
	}

	inf := map[string]any{
		"@versao":     bpeVersao,
		"@Id":         "BPe" + p.accessKey,
		"ide":         ide,
		"emit":        buildEmit(p.org, orgPerson, p.orgPK),
		"infPassagem": passage,
		"infViagem":   trips,
		"infValorBPe": shared.buildValue(t),
		"imp":         buildTATax(r.Tax, t),
		"pag":         shared.buildPayments(),
	}
	if r.Buyer != nil {
		inf["comp"] = shared.buildBuyer(*r.Buyer)
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
		"BPeTA": map[string]any{"@xmlns": bpeXMLNS, "infBPe": inf, "infBPeSupl": map[string]any{"qrCodBPe": p.qrCode()}},
	}).(map[string]any)
}

// buildTATax monta o imp do TA: IBSCBS e vTotDFe (obrigatórios). Base = valor pago. vTotDFe = valor pago (o IBS e a CBS
// calculados aqui são informativos sobre a base; ajustar quando a regra de consolidação de 2027 for lida).
func buildTATax(tax BpeTATax, t totals) map[string]any {
	ibscbs := map[string]any{"CST": tax.CST, "cClassTrib": tax.ClassTrib}
	if tax.Donation {
		ibscbs["indDoacao"] = "1"
	}
	if tax.IBSUFRate != nil || tax.IBSMunRate != nil || tax.CBSRate != nil {
		pct := func(rate *string) (decimal.Decimal, decimal.Decimal) {
			r := parseOpt(rate)
			return r, t.vPgto.Mul(r).Div(decimal.NewFromInt(100)).Round(moneyScale)
		}
		pUF, vUF := pct(tax.IBSUFRate)
		pMun, vMun := pct(tax.IBSMunRate)
		pCBS, vCBS := pct(tax.CBSRate)
		ibscbs["gIBSCBS"] = map[string]any{
			"vBC":     money(t.vPgto),
			"gIBSUF":  map[string]any{"pIBSUF": rateStr(pUF), "vIBSUF": money(vUF)},
			"gIBSMun": map[string]any{"pIBSMun": rateStr(pMun), "vIBSMun": money(vMun)},
			"vIBS":    money(vUF.Add(vMun)),
			"gCBS":    map[string]any{"pCBS": rateStr(pCBS), "vCBS": money(vCBS)},
		}
	}
	return map[string]any{"IBSCBS": ibscbs, "vTotDFe": money(t.vPgto)}
}
