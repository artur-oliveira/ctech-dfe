package documents

import (
	"fmt"
	"strings"
	"time"
)

var (
	modalLabels   = map[string]string{"1": "Rodoviário", "2": "Aéreo", "3": "Aquaviário", "4": "Ferroviário"}
	emitterLabels = map[string]string{
		"1": "Prestador de Serviço de Transporte", "2": "Transportador de Carga Própria",
		"3": "Prestador de Serviço de Transporte (CT-e Globalizado)",
	}
	carrierTypeLabels = map[string]string{"1": "ETC", "2": "TAC", "3": "CTC"}
	cargoLabels       = map[string]string{
		"01": "Granel sólido", "02": "Granel líquido", "03": "Frigorificada", "04": "Conteinerizada",
		"05": "Carga Geral", "06": "Neogranel", "07": "Perigosa (granel sólido)",
		"08": "Perigosa (granel líquido)", "09": "Perigosa (carga frigorificada)",
		"10": "Perigosa (conteinerizada)", "11": "Perigosa (carga geral)",
	}
	weightLabels = map[string]string{"01": "KG", "02": "TON"}
)

func buildMDFeContext(root *xmlNode, canceled bool) (map[string]any, error) {
	inf := root.firstDeep("infMDFe")
	if inf == nil {
		return nil, fmt.Errorf("infMDFe not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelMDFe {
		return nil, fmt.Errorf("DAMDFE requires model %s, got %q", modelMDFe, ide.value("mod"))
	}
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "MDFe")
	if prot != nil && prot.value("chMDFe") != "" {
		key = prot.value("chMDFe")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	supl := root.firstDeep("infMDFeSupl")
	qrURI := ""
	if qrValue := supl.value("qrCodMDFe"); qrValue != "" {
		qrURI, err = qrDataURI(qrValue)
		if err != nil {
			return nil, err
		}
	}
	emit := inf.child("emit")
	ender := emit.child("enderEmit")
	modal := ide.value("modal")
	modalInfo := buildModalInfo(inf.child("infModal"), modal)
	loading := make([]string, 0)
	for _, municipality := range ide.childrenNamed("infMunCarrega") {
		loading = append(loading, municipality.value("xMunCarrega"))
	}
	route := make([]string, 0)
	for _, state := range ide.childrenNamed("infPercurso") {
		route = append(route, state.value("UFPer"))
	}
	municipalities := buildMunicipalities(inf.child("infDoc"))
	insurance := buildInsurance(inf)
	seals := make([]string, 0)
	for _, seal := range inf.childrenNamed("lacres") {
		if value := seal.value("nLacre"); value != "" {
			seals = append(seals, value)
		}
	}
	contingency := ide.value("tpEmis") == "2"
	protocol := map[string]any(nil)
	if !contingency && prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	predominant := inf.child("prodPred")
	total := inf.child("tot")
	additional := inf.child("infAdic")
	return map[string]any{
		"layout": "retrato", "modal": modal, "modal_label": modalLabels[modal],
		"emit": map[string]any{
			"nome": emit.value("xNome"), "fantasia": emit.value("xFant"),
			"doc": maskCPFCNPJ(firstNonempty(emit.value("CNPJ"), emit.value("CPF"))), "ie": emit.value("IE"),
			"endereco": address(ender), "mun": ender.value("xMun"), "uf": ender.value("UF"),
			"cep": maskCEP(ender.value("CEP")), "fone": ender.value("fone"),
		},
		"ide": map[string]any{
			"serie": ide.value("serie"), "nMDF": fiscalNumber(ide.value("nMDF")), "dhEmi": dateTimeBR(ide.value("dhEmi")),
			"ufIni": ide.value("UFIni"), "ufFim": ide.value("UFFim"), "tpEmit_label": emitterLabels[ide.value("tpEmit")],
			"tpTransp_label": carrierTypeLabels[ide.value("tpTransp")],
		},
		"carrega": loading, "percurso": strings.Join(nonempty(route...), " "), "modal_info": modalInfo,
		"prodPred": map[string]any{
			"tpCarga_label": cargoLabels[predominant.value("tpCarga")], "xProd": predominant.value("xProd"), "ncm": predominant.value("NCM"),
		},
		"rntrc": rntrcOf(modalInfo),
		"tot": map[string]any{
			"qCTe": defaultValue(total.value("qCTe"), "0"), "qNFe": defaultValue(total.value("qNFe"), "0"),
			"qMDFe": defaultValue(total.value("qMDFe"), "0"), "vCarga": moneyBR(total.value("vCarga")),
			"qCarga": total.value("qCarga"), "cUnid_label": weightLabels[total.value("cUnid")],
		},
		"municipios": municipalities, "seguros": insurance, "lacres": seals,
		"chave_fmt": keyBlocks(key), "chave_raw": digits(key), "chave_barcode": barcode, "qr_uri": qrURI,
		"protocolo": protocol, "is_contingencia": contingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": canceled,
		"msg_fiscal": additional.value("infAdFisco"), "msg_contribuinte": additional.value("infCpl"),
		"site": "https://dfe.aoctech.app", "gerado_em": time.Now().Format("02/01/2006 15:04:05"),
		"consulta_url": mdfeConsultaURL,
		// Presentation knobs the emitente will be able to set; the zero values
		// keep the stock look until the fiscal_config fields are wired in.
		"brand": map[string]any{"logo_b64": "", "box_bg": defaultBoxBackground},
		"text": map[string]any{
			"gerado_por": "Gerado por", "damdfe": "DAMDFE", "damdfe_desc": "DOCUMENTO AUXILIAR DO MANIFESTO ELETRÔNICO DE DOCUMENTOS FISCAIS",
			"homologacao": "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL", "contingencia": "EMISSÃO EM CONTINGÊNCIA",
			"protocolo": "PROTOCOLO DE AUTORIZAÇÃO DE USO", "cancelada": "CANCELADA",
		},
	}, nil
}

func buildModalInfo(info *xmlNode, modal string) map[string]any {
	ctx := map[string]any{"is_rodo": modal == "1", "is_aereo": modal == "2", "is_aqua": modal == "3", "is_ferrov": modal == "4"}
	switch modal {
	case "1":
		road := info.child("rodo")
		antt := road.child("infANTT")
		vehicle := road.child("veicTracao")
		ciots := make([]string, 0)
		for _, ciot := range antt.childrenNamed("infCIOT") {
			ciots = append(ciots, ciot.value("CIOT"))
		}
		drivers := make([]map[string]any, 0)
		for _, driver := range vehicle.childrenNamed("condutor") {
			cpf := maskCPFCNPJ(driver.value("CPF"))
			drivers = append(drivers, map[string]any{"nome": driver.value("xNome"), "cpf": cpf, "nome_cpf": joinOptional(driver.value("xNome"), cpf, " - ")})
		}
		trailers := make([]map[string]any, 0)
		for _, trailer := range road.childrenNamed("veicReboque") {
			trailers = append(trailers, map[string]any{"placa": trailer.value("placa"), "uf": trailer.value("UF"), "placa_uf_fmt": joinOptional(trailer.value("placa"), trailer.value("UF"), " / "), "renavam": trailer.value("RENAVAM"), "tara": trailer.value("tara")})
		}
		ctx["rntrc"], ctx["ciot"] = antt.value("RNTRC"), strings.Join(nonempty(ciots...), ", ")
		ctx["veic"] = map[string]any{"placa": vehicle.value("placa"), "uf": vehicle.value("UF"), "placa_uf_fmt": joinOptional(vehicle.value("placa"), vehicle.value("UF"), " / "), "renavam": vehicle.value("RENAVAM"), "tara": vehicle.value("tara"), "capkg": vehicle.value("capKG")}
		ctx["condutores"], ctx["reboques"] = drivers, trailers
	case "2":
		a := info.child("aereo")
		ctx["aereo"] = map[string]any{"nac": a.value("nac"), "matr": a.value("matr"), "nvoo": a.value("nVoo"), "caeremb": a.value("cAerEmb"), "caerdes": a.value("cAerDes"), "dvoo": a.value("dVoo")}
	case "3":
		a := info.child("aquav")
		barges := make([]map[string]any, 0)
		for _, barge := range a.childrenNamed("infEmbComb") {
			barges = append(barges, map[string]any{"cod": barge.value("cEmbComb"), "nome": barge.value("xBalsa"), "nome_cod": joinOptional(barge.value("xBalsa"), barge.value("cEmbComb"), " - ")})
		}
		ctx["aqua"] = map[string]any{"irin": a.value("irin"), "tpemb": a.value("tpEmb"), "cembar": a.value("cEmbar"), "xembar": a.value("xEmbar"), "nviag": a.value("nViag"), "cprtemb": a.value("cPrtEmb"), "cprtdest": a.value("cPrtDest"), "mmsi": a.value("MMSI"), "balsas": barges}
	case "4":
		f := info.child("ferrov")
		train := f.child("trem")
		wagons := make([]map[string]any, 0)
		for _, wagon := range f.childrenNamed("vag") {
			wagons = append(wagons, map[string]any{"serie": wagon.value("serie"), "nvag": wagon.value("nVag")})
		}
		ctx["ferrov"] = map[string]any{"xpref": train.value("xPref"), "dhtrem": dateTimeBR(train.value("dhTrem")), "xori": train.value("xOri"), "xdest": train.value("xDest"), "qvag": train.value("qVag"), "vagoes": wagons}
	}
	return ctx
}

func buildMunicipalities(info *xmlNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, municipality := range info.childrenNamed("infMunDescarga") {
		docs := make([]map[string]any, 0)
		for _, spec := range []struct{ tag, key, label string }{{"infNFe", "chNFe", "NF-e"}, {"infCTe", "chCTe", "CT-e"}, {"infMDFeTransp", "chMDFe", "MDF-e"}} {
			for _, doc := range municipality.childrenNamed(spec.tag) {
				key := doc.value(spec.key)
				emitDoc, series, number := keyParts(key)
				docs = append(docs, map[string]any{
					"tipo": spec.label, "chave": key, "chave_fmt": keyBlocks(key),
					"emit_doc": emitDoc, "serie": series, "numero": number,
				})
			}
		}
		out = append(out, map[string]any{"mun": municipality.value("xMunDescarga"), "docs": docs, "count": len(docs)})
	}
	return out
}

func buildInsurance(inf *xmlNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, insurance := range inf.childrenNamed("seg") {
		info := insurance.child("infSeg")
		endorsements := make([]string, 0)
		for _, endorsement := range insurance.childrenNamed("nAver") {
			if value := strings.TrimSpace(endorsement.text); value != "" {
				endorsements = append(endorsements, value)
			}
		}
		out = append(out, map[string]any{"nome": info.value("xSeg"), "cnpj": maskCNPJ(info.value("CNPJ")), "apol": insurance.value("nApol"), "averbacoes": strings.Join(endorsements, ", ")})
	}
	return out
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func joinOptional(first, second, separator string) string {
	if second == "" {
		return first
	}
	return first + separator + second
}

// rntrcOf surfaces the road RNTRC for the DAMDFE header, where it sits beside
// the emitente's CNPJ and IE regardless of modal.
func rntrcOf(modalInfo map[string]any) string {
	value, _ := modalInfo["rntrc"].(string)
	return value
}
