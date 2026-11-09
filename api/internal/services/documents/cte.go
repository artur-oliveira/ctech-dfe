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
	// cteMultiple joins repeated XML values (balsas, lacres, manuseio) in one DACTE box.
	cteMultiple = ", "
)

var (
	cteModalLabels = map[string]string{"01": "Rodoviário", "02": "Aéreo", "03": "Aquaviário", "04": "Ferroviário", "05": "Dutoviário", "06": "Multimodal"}
	// "2" (Anulação) does not exist in the v4.00 XSD (tpCTe 0, 1, 3); kept only to label received legacy XML.
	cteTypeLabels = map[string]string{"0": "Normal", "1": "Complemento de Valores", "2": "Anulação", "3": "Substituto"}
	cteServLabels = map[string]string{"0": "Normal", "1": "Subcontratação", "2": "Redespacho", "3": "Redespacho Intermediário", "4": "Serviço Vinculado a Multimodal"}
	// tpServ of the CT-e OS (cteOS_v4.00.xsd): 6 transporte de pessoas, 7 valores, 8 excesso de bagagem.
	cteOSServLabels = map[string]string{"6": "Transporte de Pessoas", "7": "Transporte de Valores", "8": "Excesso de Bagagem"}
	// respSeg of seg (MOC Anexo II, section 4): the DACTE OS prints the insured only for 4 and 5.
	cteOSInsurer = map[string]string{"4": "Emitente do CT-e", "5": "Tomador de Serviço"}
	// Value domains printed by the modal boxes (cteModalFerroviario/cteMultiModal/cteTiposBasico XSD documentation).
	cteTrafficLabels  = map[string]string{"0": "Próprio", "1": "Mútuo", "2": "Rodoferroviário", "3": "Rodoviário"}
	cteRailRespLabels = map[string]string{"1": "Ferrovia de origem", "2": "Ferrovia de destino"}
	cteNegotiable     = map[string]string{"0": "Não negociável", "1": "Negociável"}
	cteRetiraLabels   = map[string]string{"0": "SIM", "1": "NÃO"}
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

// cteWeightKind maps a tpMed to the Annex II weight box it feeds ("kg", "kg_base", "kg_afer"); "" when the Annex has
// no box for it (PESO DECLARADO, CUBADO, AFORADO, LITRAGEM, CAIXAS): those are never summed into another box.
func cteWeightKind(tpMed string) string {
	t := strings.ToUpper(strings.TrimSpace(tpMed))
	switch {
	case strings.Contains(t, "BRUTO"):
		return "kg"
	case strings.Contains(t, "BASE"):
		return "kg_base"
	case strings.Contains(t, "AFERIDO"), strings.Contains(t, "AUFERIDO"):
		return "kg_afer"
	}
	return ""
}

// cteLoad converts every infQ to the DACTE fields (MOC Anexo II, section 3): one weight box per tpMed (Peso Bruto,
// Peso Base de Calculo, Peso Aferido; ton x 1000), m3 for cUnid 00, volumes for 03.
func cteLoad(infCarga *xmlNode) map[string]any {
	kg := map[string]decimal.Decimal{}
	vol, m3 := decimal.Zero, decimal.Zero
	for _, q := range infCarga.childrenNamed("infQ") {
		v, err := decimal.NewFromString(q.value("qCarga"))
		if err != nil {
			continue
		}
		switch q.value("cUnid") {
		case cteUnitKg, cteUnitTon:
			if k := cteWeightKind(q.value("tpMed")); k != "" {
				if q.value("cUnid") == cteUnitTon {
					v = v.Mul(decimal.NewFromInt(tonToKg))
				}
				kg[k] = kg[k].Add(v)
			}
		case cteUnitM3:
			m3 = m3.Add(v)
		case cteUnitUnits:
			vol = vol.Add(v)
		}
	}
	return map[string]any{
		"pred": infCarga.value("proPred"), "outras": infCarga.value("xOutCat"), "valor": moneyBR(infCarga.value("vCarga")),
		"kg": kg["kg"].StringFixed(3), "kg_base": kg["kg_base"].StringFixed(3), "kg_afer": kg["kg_afer"].StringFixed(3),
		"m3": m3.StringFixed(3), "vol": vol.StringFixed(0),
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
	// Anexo II 2.1: the Protocolo de Autorizacao de Uso received from SEFAZ is always printed; the contingency
	// legend only stands for an offline CT-e that has no protocol yet.
	protocol := map[string]any(nil)
	if prot != nil && prot.value("nProt") != "" {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	contingency := ide.value("tpEmis") == tpEmisCTeOffline && protocol == nil
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
		"modal_info": cteModalInfo(inf, norm), "rntrc": rodo.child("RNTRC").textOrEmpty(), "obs": inf.child("compl").value("xObs"),
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
	// Anexo II 2.1: the Protocolo de Autorizacao de Uso received from SEFAZ is always printed; the contingency
	// legend only stands for an offline CT-e that has no protocol yet.
	protocol := map[string]any(nil)
	if prot != nil && prot.value("nProt") != "" {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	contingency := ide.value("tpEmis") == tpEmisCTeOffline && protocol == nil
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

// cteJoin joins the values of every child named name under n.
func cteJoin(n *xmlNode, name string) string {
	out := make([]string, 0)
	for _, c := range n.childrenNamed(name) {
		if v := strings.TrimSpace(c.textOrEmpty()); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, cteMultiple)
}

// cteValues reads the child value of each group node.
func cteValues(groups []*xmlNode, name string) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.value(name))
	}
	return out
}

type cteBox = map[string]any

func cteField(label, value string) cteBox { return cteBox{"lbl": label, "val": value} }

// cteModalInfo builds the modal specific quadro of the DACTE (MOC Anexo II, section 3: "INFORMACOES ESPECIFICAS
// DO MODAL ..."). Returns title "" for the rodoviario (its RNTRC line is printed apart) or an unknown modal.
// Dutoviario: the Annex maps every field of the box to "(A ser utilizado no futuro)", so only the heading prints.
func cteModalInfo(inf, norm *xmlNode) map[string]any {
	ide := inf.child("ide")
	modal := norm.child("infModal")
	fields := make([]cteBox, 0)
	tables := make([]cteBox, 0)
	title := ""
	switch ide.value("modal") {
	case "02":
		a := modal.child("aereo")
		title = "Informações específicas do modal aéreo"
		fields = append(fields,
			cteField("Número operacional do conhecimento aéreo", a.value("nOCA")), cteField("Classe", a.child("tarifa").value("CL")),
			cteField("Código da tarifa", a.child("tarifa").value("cTar")), cteField("Valor da tarifa", moneyBR(a.child("tarifa").value("vTar"))),
			cteField("Número da minuta", a.value("nMinu")), cteField("Retira", cteRetiraLabels[ide.value("retira")]),
			cteField("Dados relativos à retirada da carga", ide.value("xDetRetira")),
			cteField("Característica adicional do serviço", inf.child("compl").value("xCaracSer")),
			cteField("Data prevista de entrega", dateBR(a.value("dPrevAereo"))),
			cteField("Natureza da carga: dimensão", a.child("natCarga").value("xDime")),
			cteField("Natureza da carga: informações do manuseio", cteJoin(a.child("natCarga"), "cInfManu")))
	case "03":
		a := modal.child("aquav")
		title = "Informações específicas do modal aquaviário"
		fields = append(fields,
			cteField("Identificação do navio/rebocador", a.value("xNavio")), cteField("Valor do AFRMM", moneyBR(a.value("vAFRMM"))),
			cteField("Identificação da balsa", strings.Join(cteValues(a.childrenNamed("balsa"), "xBalsa"), cteMultiple)))
		rows := make([][]string, 0)
		for _, d := range a.childrenNamed("detCont") {
			rows = append(rows, []string{d.value("nCont"), strings.Join(cteValues(d.childrenNamed("lacre"), "nLacre"), cteMultiple)})
		}
		if len(rows) > 0 {
			tables = append(tables, cteBox{"title": "Detalhamento do contêiner", "head": []string{"Identificação do contêiner", "Lacre"}, "rows": rows})
		}
	case "04":
		f := modal.child("ferrov")
		tm := f.child("trafMut")
		title = "Informações específicas do modal ferroviário"
		fields = append(fields,
			cteField("Tipo de tráfego", cteTrafficLabels[f.value("tpTraf")]), cteField("Fluxo ferroviário", f.value("fluxo")),
			cteField("Ferrovia responsável pelo faturamento", cteRailRespLabels[tm.value("respFat")]),
			cteField("Ferrovia emitente do CT-e", cteRailRespLabels[tm.value("ferrEmi")]), cteField("Valor do frete", moneyBR(tm.value("vFrete"))))
		rows := make([][]string, 0)
		for _, e := range tm.childrenNamed("ferroEnv") {
			rows = append(rows, []string{maskCPFCNPJ(e.value("CNPJ")), e.value("IE"), e.value("cInt"), e.value("xNome")})
		}
		if len(rows) > 0 {
			tables = append(tables, cteBox{"title": "Informações das ferrovias envolvidas", "head": []string{"CNPJ", "Inscrição estadual", "Cód. interno", "Razão social"}, "rows": rows})
		}
	case "05":
		title = "Informações específicas do modal dutoviário"
	case "06":
		m := modal.child("multimodal")
		seg := m.child("seg")
		title = "Informações específicas do transporte multimodal de cargas"
		fields = append(fields,
			cteField("Número do certificado do operador de transporte multimodal", m.value("COTM")),
			cteField("Indicador negociável", cteNegotiable[m.value("indNegociavel")]),
			cteField("Nome da seguradora", seg.child("infSeg").value("xSeg")), cteField("CNPJ da seguradora", maskCPFCNPJ(seg.child("infSeg").value("CNPJ"))),
			cteField("Número da apólice", seg.value("nApol")), cteField("Número da averbação", seg.value("nAver")))
	}
	flow := inf.child("compl").child("fluxo")
	return map[string]any{
		"title": title, "fields": fields, "tables": tables,
		"fluxo": map[string]any{"has": flow != nil, "orig": flow.value("xOrig"), "pass": strings.Join(cteValues(flow.childrenNamed("pass"), "xPass"), cteMultiple), "dest": flow.value("xDest")},
	}
}
