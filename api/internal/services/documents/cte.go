package documents

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	// cteConsultaURL is the national CT-e lookup page printed in the variable-content field 1 (MOC DACTE 2.20.1).
	cteConsultaURL = "https://dfe-portal.svrs.rs.gov.br/cte/consulta"
	// dacteContingencyLegend is the wording the MOC CT-e Contingencia 3.1 prescribes for a DACTE printed in
	// contingency. It is stated for FS-DA (revoked); it is applied here to offline (tpEmis 2) by analogy.
	dacteContingencyLegend = "DACTE em Contingência - impresso em decorrência de problemas técnicos"
	dacteHomologLegend     = "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL"
	tpEmisCTeOffline       = "2"
	cteTomaOutros          = "4"
	modelCTeOS             = "67"
	cteUnitTon             = "02"
	cteUnitKg              = "01"
	cteUnitM3              = "00"
	cteUnitUnits           = "03"
	tonToKg                = 1000
)

var (
	cteModalLabels = map[string]string{"01": "Rodoviário", "02": "Aéreo", "03": "Aquaviário", "04": "Ferroviário", "05": "Dutoviário", "06": "Multimodal"}
	cteTypeLabels  = map[string]string{"0": "Normal", "1": "Complemento de Valores", "2": "Anulação", "3": "Substituto"}
	cteServLabels  = map[string]string{"0": "Normal", "1": "Subcontratação", "2": "Redespacho", "3": "Redespacho Intermediário", "4": "Serviço Vinculado a Multimodal"}
	// tpServ of the CT-e OS (cteOS_v4.00.xsd): 6 transporte de pessoas, 7 valores, 8 excesso de bagagem.
	cteOSServLabels = map[string]string{"6": "Transporte de Pessoas", "7": "Transporte de Valores", "8": "Excesso de Bagagem"}
	// respSeg of seg (MOC Anexo II, section 4): the DACTE OS prints the insured only for 4 and 5.
	cteOSInsurer = map[string]string{"4": "Emitente do CT-e", "5": "Tomador de Serviço"}
	// toma3 codes 0..3 point at rem, exped, receb, dest.
	cteTomaActor = map[string][2]string{"0": {"rem", "enderReme"}, "1": {"exped", "enderExped"}, "2": {"receb", "enderReceb"}, "3": {"dest", "enderDest"}}
)

func cteParty(n *xmlNode, enderTag string) map[string]any {
	if n == nil {
		return map[string]any{"has": false}
	}
	e := n.child(enderTag)
	return map[string]any{
		"has": true, "nome": firstNonempty(n.value("xNome"), n.value("xFant")),
		"endereco": address(e), "mun": e.value("xMun"), "uf": e.value("UF"), "cep": maskCEP(e.value("CEP")),
		"doc": maskCPFCNPJ(firstNonempty(n.value("CNPJ"), n.value("CPF"))), "ie": n.value("IE"),
		"pais": e.value("xPais"), "fone": firstNonempty(n.value("fone"), e.value("fone")),
	}
}

// cteTaker resolves the Tomador do Serviço: toma4 carries its own data, toma3 points at rem/exped/receb/dest.
func cteTaker(inf, ide *xmlNode) map[string]any {
	if t4 := ide.child("toma4"); t4 != nil {
		return cteParty(t4, "enderToma")
	}
	if actor, ok := cteTomaActor[ide.child("toma3").value("toma")]; ok {
		return cteParty(inf.child(actor[0]), actor[1])
	}
	return map[string]any{"has": false}
}

// cteLoad converts every infQ to the DACTE fields: kg for 01/02 (ton x 1000), m3 for 00, volumes for 03.
func cteLoad(infCarga *xmlNode) map[string]any {
	kg, vol, m3 := decimal.Zero, decimal.Zero, decimal.Zero
	for _, q := range infCarga.childrenNamed("infQ") {
		v, err := decimal.NewFromString(q.value("qCarga"))
		if err != nil {
			continue
		}
		switch q.value("cUnid") {
		case cteUnitKg:
			kg = kg.Add(v)
		case cteUnitTon:
			kg = kg.Add(v.Mul(decimal.NewFromInt(tonToKg)))
		case cteUnitM3:
			m3 = m3.Add(v)
		case cteUnitUnits:
			vol = vol.Add(v)
		}
	}
	return map[string]any{
		"pred": infCarga.value("proPred"), "outras": infCarga.value("xOutCat"), "valor": moneyBR(infCarga.value("vCarga")),
		"kg": kg.StringFixed(3), "m3": m3.StringFixed(3), "vol": vol.StringFixed(0),
	}
}

func cteTax(imp *xmlNode) map[string]any {
	var group *xmlNode
	if icms := imp.child("ICMS"); icms != nil && len(icms.children) > 0 {
		group = icms.children[0]
	}
	if group == nil {
		return map[string]any{}
	}
	pick := func(names ...string) string {
		for _, n := range names {
			if v := group.value(n); v != "" {
				return v
			}
		}
		return ""
	}
	return map[string]any{
		"cst": firstNonempty(group.value("CST"), group.name), "bc": moneyBR(pick("vBC", "vBCSTRet", "vBCOutraUF")),
		"aliq": percentBR(pick("pICMS", "pICMSSTRet", "pICMSOutraUF")), "valor": moneyBR(pick("vICMS", "vICMSSTRet", "vICMSOutraUF")),
		"red": percentBR(pick("pRedBC", "pRedBCOutraUF")),
	}
}

func cteOriginDocs(infDoc *xmlNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, d := range infDoc.childrenNamed("infNFe") {
		doc, series, number := keyParts(d.value("chave"))
		out = append(out, map[string]any{"tipo": "NF-e", "emit": doc, "num": series + " / " + number, "chave": keyBlocks(d.value("chave"))})
	}
	for _, d := range infDoc.childrenNamed("infNF") {
		out = append(out, map[string]any{"tipo": "NF " + d.value("mod"), "emit": "", "num": d.value("serie") + " / " + d.value("nDoc"), "chave": ""})
	}
	for _, d := range infDoc.childrenNamed("infOutros") {
		out = append(out, map[string]any{"tipo": firstNonempty(d.value("descOutros"), "Outros"), "emit": "", "num": d.value("nDoc"), "chave": ""})
	}
	return out
}

func buildCTeContext(root *xmlNode, cancelled bool) (map[string]any, error) {
	inf := root.firstDeep("infCte")
	if inf == nil {
		return nil, fmt.Errorf("infCte not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelCTe {
		return nil, fmt.Errorf("DACTE requires model %s, got %q", modelCTe, ide.value("mod"))
	}
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "CTe")
	if prot != nil && prot.value("chCTe") != "" {
		key = prot.value("chCTe")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	qrURI := ""
	if qr := root.firstDeep("infCTeSupl").value("qrCodCTe"); qr != "" {
		if qrURI, err = qrDataURI(qr); err != nil {
			return nil, err
		}
	}
	emit := inf.child("emit")
	ender := emit.child("enderEmit")
	norm := inf.child("infCTeNorm")
	contingency := ide.value("tpEmis") == tpEmisCTeOffline
	protocol := map[string]any(nil)
	if !contingency && prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	comps := make([]map[string]any, 0)
	for _, c := range inf.child("vPrest").childrenNamed("Comp") {
		comps = append(comps, map[string]any{"nome": c.value("xNome"), "valor": moneyBR(c.value("vComp"))})
	}
	rodo := norm.child("infModal").child("rodo")
	return map[string]any{
		"emit": map[string]any{
			"nome": firstNonempty(emit.value("xNome"), emit.value("xFant")), "endereco": address(ender), "mun": ender.value("xMun"),
			"uf": ender.value("UF"), "cep": maskCEP(ender.value("CEP")), "fone": ender.value("fone"),
			"doc": maskCPFCNPJ(emit.value("CNPJ")), "ie": emit.value("IE"),
		},
		"ide": map[string]any{
			"modal": cteModalLabels[ide.value("modal")], "mod": ide.value("mod"), "serie": ide.value("serie"), "nCT": ide.value("nCT"),
			"dhEmi": dateTimeBR(ide.value("dhEmi")), "tpCTe": cteTypeLabels[ide.value("tpCTe")], "tpServ": cteServLabels[ide.value("tpServ")],
			"cfop": ide.value("CFOP"), "natOp": ide.value("natOp"),
			"ini": joinOptional(ide.value("xMunIni"), ide.value("UFIni"), " - "), "fim": joinOptional(ide.value("xMunFim"), ide.value("UFFim"), " - "),
		},
		"rem": cteParty(inf.child("rem"), "enderReme"), "dest": cteParty(inf.child("dest"), "enderDest"),
		"exped": cteParty(inf.child("exped"), "enderExped"), "receb": cteParty(inf.child("receb"), "enderReceb"),
		"toma": cteTaker(inf, ide), "carga": cteLoad(norm.child("infCarga")),
		"comps": comps, "vTPrest": moneyBR(inf.child("vPrest").value("vTPrest")), "vRec": moneyBR(inf.child("vPrest").value("vRec")),
		"imp": cteTax(inf.child("imp")), "docs": cteOriginDocs(norm.child("infDoc")),
		"rntrc": rodo.child("RNTRC").textOrEmpty(), "obs": inf.child("compl").value("xObs"),
		"chave_fmt": keyBlocks(key), "chave_barcode": barcode, "qr_uri": qrURI, "consulta_url": cteConsultaURL,
		"protocolo": protocol, "is_contingencia": contingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": cancelled,
		"legend_contingencia": dacteContingencyLegend, "legend_homologacao": dacteHomologLegend,
		"site": "https://dfe.aoctech.app", "gerado_em": time.Now().Format("02/01/2006 15:04:05"),
	}, nil
}

// cteOSPlace joins a place as the DACTE OS prints it: "UF - município" (Início/Término da Prestação).
func cteOSPlace(ide *xmlNode, ufTag, munTag string) string {
	return joinOptional(ide.value(munTag), ide.value(ufTag), " - ")
}

// buildCTeOSContext reads a CT-e OS (model 67, root CTeOS) into the DACTE OS fields of the MOC Anexo II
// (section 4: correlation XML x DACTE OS; section 6.1/6.2: layout). Contingency prints the legend, like the
// DACTE: FS-DA (the only contingency with a second bar code) is revoked.
func buildCTeOSContext(root *xmlNode, cancelled bool) (map[string]any, error) {
	inf := root.firstDeep("infCte")
	if inf == nil {
		return nil, fmt.Errorf("infCte not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelCTeOS {
		return nil, fmt.Errorf("DACTE OS requires model %s, got %q", modelCTeOS, ide.value("mod"))
	}
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "CTe")
	if prot != nil && prot.value("chCTe") != "" {
		key = prot.value("chCTe")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	qrURI := ""
	if qr := root.firstDeep("infCTeSupl").value("qrCodCTe"); qr != "" {
		if qrURI, err = qrDataURI(qr); err != nil {
			return nil, err
		}
	}
	emit := inf.child("emit")
	ender := emit.child("enderEmit")
	norm := inf.child("infCTeNorm")
	contingency := ide.value("tpEmis") == tpEmisCTeOffline
	protocol := map[string]any(nil)
	if !contingency && prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	comps := make([]map[string]any, 0)
	for _, c := range inf.child("vPrest").childrenNamed("Comp") {
		comps = append(comps, map[string]any{"nome": c.value("xNome"), "valor": moneyBR(c.value("vComp"))})
	}
	route := make([]string, 0)
	for _, p := range ide.childrenNamed("infPercurso") {
		route = append(route, p.value("UFPer"))
	}
	rodo := norm.child("infModal").child("rodoOS")
	veic := rodo.child("veic")
	prop := veic.child("prop")
	obs := func(group string) string {
		texts := make([]string, 0)
		for _, o := range inf.child("compl").childrenNamed(group) {
			texts = append(texts, o.value("xTexto"))
		}
		return strings.Join(texts, " | ")
	}
	seg := inf.child("infCTeNorm").child("seg")
	if seg == nil {
		seg = inf.child("seg")
	}
	return map[string]any{
		"emit": map[string]any{
			"nome": firstNonempty(emit.value("xNome"), emit.value("xFant")), "endereco": address(ender), "mun": ender.value("xMun"),
			"uf": ender.value("UF"), "cep": maskCEP(ender.value("CEP")), "fone": ender.value("fone"),
			"doc": maskCPFCNPJ(emit.value("CNPJ")), "ie": emit.value("IE"),
		},
		"ide": map[string]any{
			"modal": cteModalLabels[ide.value("modal")], "mod": ide.value("mod"), "serie": ide.value("serie"), "nCT": ide.value("nCT"),
			"dhEmi": dateTimeBR(ide.value("dhEmi")), "tpCTe": cteTypeLabels[ide.value("tpCTe")], "tpServ": cteOSServLabels[ide.value("tpServ")],
			"cfop": ide.value("CFOP"), "natOp": ide.value("natOp"),
			"ini": cteOSPlace(ide, "UFIni", "xMunIni"), "fim": cteOSPlace(ide, "UFFim", "xMunFim"), "percurso": strings.Join(route, ", "),
		},
		"toma":    cteParty(inf.child("toma"), "enderToma"),
		"servico": map[string]any{"qtd": norm.child("infServico").child("infQ").value("qCarga"), "desc": norm.child("infServico").value("xDescServ")},
		"comps":   comps, "vTPrest": moneyBR(inf.child("vPrest").value("vTPrest")), "vRec": moneyBR(inf.child("vPrest").value("vRec")),
		"imp": cteTax(inf.child("imp")), "obs": inf.child("compl").value("xObs"),
		"seg": map[string]any{
			"resp": cteOSInsurer[seg.value("respSeg")], "nome": seg.value("xSeg"), "apolice": seg.value("nApol"),
		},
		"rodo": map[string]any{
			"taf": rodo.value("TAF"), "reg": rodo.value("NroRegEstadual"), "placa": veic.value("placa"), "renavam": veic.value("RENAVAM"),
			"prop_doc": maskCPFCNPJ(firstNonempty(prop.value("CNPJ"), prop.value("CPF"))), "prop_taf": prop.value("TAF"), "prop_reg": prop.value("NroRegEstadual"),
		},
		"uso_emissor": obs("ObsCont"), "reservado_fisco": obs("ObsFisco"),
		"chave_fmt": keyBlocks(key), "chave_barcode": barcode, "qr_uri": qrURI, "consulta_url": cteConsultaURL,
		"protocolo": protocol, "is_contingencia": contingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": cancelled,
		"legend_contingencia": dacteContingencyLegend, "legend_homologacao": dacteHomologLegend,
		"site": "https://dfe.aoctech.app", "gerado_em": time.Now().Format("02/01/2006 15:04:05"),
	}, nil
}
