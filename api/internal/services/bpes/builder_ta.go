package bpes

import (
	"strconv"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// ide/tpCompra do TA: normal, BP-e múltiplo (fretamento) e vinculado a BP-e múltiplo.
var tpCompraCodes = map[string]string{"": "0", PurchaseNormal: "0", PurchaseMultiple: "1", PurchaseLinked: "2"}

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
		"verProc": strOrDefault(p.tech.Version, "ctech-dfe"), "tpBPe": tpBPeOf(r.Substitution), "tpCompra": tpCompraCodes[r.PurchaseType],
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

	var imp map[string]any
	if r.PurchaseType != PurchaseLinked {
		var gov map[string]any
		imp, gov = buildTATax(r.Tax, t, p.now.Year())
		if gov != nil {
			ide["gCompraGov"] = gov
		}
	}
	inf := map[string]any{
		"@versao":     bpeVersao,
		"@Id":         "BPe" + p.accessKey,
		"ide":         ide,
		"emit":        buildEmit(p.org, orgPerson, p.orgPK),
		"infPassagem": passage,
		"infViagem":   trips,
	}
	if r.PurchaseType == PurchaseLinked {
		// Vinculado a BP-e múltiplo: a choice do leiaute troca valores, imposto e pagamento pela chave do múltiplo.
		key := ""
		if r.MultipleKey != nil {
			key = *r.MultipleKey
		}
		inf["infBPeVinculado"] = map[string]any{"chBPeMultiplo": key}
	} else {
		inf["infValorBPe"] = shared.buildValue(t)
		inf["imp"] = imp
		inf["pag"] = shared.buildPayments()
	}
	if r.Buyer != nil {
		inf["comp"] = shared.buildBuyer(*r.Buyer)
	}
	if r.Agency != nil {
		inf["agencia"] = buildAgency(r.Agency)
	}
	if r.Substitution != nil {
		inf["infBPeSub"] = buildSubstitution(r.Substitution)
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
