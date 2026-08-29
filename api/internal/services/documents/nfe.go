package documents

import (
	"fmt"
	"strings"
	"time"
)

// operationLabels renders tpNF for the MOC minimum header the DANFE repeats on
// every continuation sheet. It is unaccented because that header lives in a
// @page margin box, and folio writes margin-box content as raw bytes.
var operationLabels = map[string]string{"0": "ENTRADA", "1": "SAIDA"}

var freightLabels = map[string]string{
	"0": "0 - Remetente", "1": "1 - Destinatário", "2": "2 - Terceiros",
	"3": "3 - Próprio (Remetente)", "4": "4 - Próprio (Destinatário)", "9": "9 - Sem Frete",
}

func buildNFeContext(root *xmlNode, canceled bool) (map[string]any, error) {
	inf := root.firstDeep("infNFe")
	if inf == nil {
		return nil, fmt.Errorf("infNFe not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelNFe {
		return nil, fmt.Errorf("DANFE requires model %s, got %q", modelNFe, ide.value("mod"))
	}
	prot := root.firstDeep("infProt")
	key := inf.attr("Id")
	key = strings.TrimPrefix(key, "NFe")
	if prot != nil && prot.value("chNFe") != "" {
		key = prot.value("chNFe")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	emit := inf.child("emit")
	enderEmit := emit.child("enderEmit")
	dest := inf.child("dest")
	enderDest := dest.child("enderDest")

	products := make([]map[string]any, 0)
	for _, det := range inf.childrenNamed("det") {
		prod := det.child("prod")
		icms := det.child("imposto").child("ICMS")
		var icmsGroup *xmlNode
		if icms != nil && len(icms.children) > 0 {
			icmsGroup = icms.children[0]
		}
		ipi := det.child("imposto").child("IPI").child("IPITrib")
		cst := icmsGroup.value("CST")
		if cst == "" {
			cst = icmsGroup.value("CSOSN")
		}
		orig := icmsGroup.value("orig")
		origCST := cst
		if orig != "" && cst != "" {
			origCST = orig + "/" + cst
		}
		products = append(products, map[string]any{
			"cProd": prod.value("cProd"), "xProd": prod.value("xProd"), "NCM": prod.value("NCM"),
			"CFOP": prod.value("CFOP"), "uCom": prod.value("uCom"), "qCom": prod.value("qCom"),
			"vUnCom": moneyBR(prod.value("vUnCom")), "vProd": moneyBR(prod.value("vProd")),
			"vDesc": moneyBR(prod.value("vDesc")), "origCST": origCST,
			"vBC": moneyBR(icmsGroup.value("vBC")), "vICMS": moneyBR(icmsGroup.value("vICMS")),
			"vIPI": moneyBR(ipi.value("vIPI")), "pICMS": moneyBR(icmsGroup.value("pICMS")),
			"pIPI": moneyBR(ipi.value("pIPI")), "infAdProd": det.value("infAdProd"),
		})
	}

	totalNode := inf.child("total").child("ICMSTot")
	totals := map[string]any{}
	for _, key := range []string{"vBC", "vICMS", "vBCST", "vST", "vII", "vICMSUFRemet", "vFCPUFDest", "vProd", "vFrete", "vSeg", "vDesc", "vOutro", "vIPI", "vICMSUFDest", "vNF", "vTotTrib"} {
		totals[key] = moneyBR(totalNode.value(key))
	}

	transp := inf.child("transp")
	carrier := transp.child("transporta")
	vehicle := transp.child("veicTransp")
	volumes := transp.childrenNamed("vol")
	var volume *xmlNode
	if len(volumes) > 0 {
		volume = volumes[0]
	}
	freight := transp.value("modFrete")
	freightLabel := freightLabels[freight]
	if freightLabel == "" {
		freightLabel = freight
	}
	plateUF := vehicle.value("placa")
	if vehicle.value("UF") != "" {
		plateUF += " / " + vehicle.value("UF")
	}

	cobr := inf.child("cobr")
	fat := cobr.child("fat")
	dups := make([]map[string]any, 0)
	for _, dup := range cobr.childrenNamed("dup") {
		dups = append(dups, map[string]any{"nDup": dup.value("nDup"), "dVenc": dup.value("dVenc"), "vDup": moneyBR(dup.value("vDup"))})
	}

	tpEmis := ide.value("tpEmis")
	isFS := tpEmis == "2" || tpEmis == "5"
	isEPEC := tpEmis == "4"
	showProtocol := !isFS
	protocol := map[string]any(nil)
	if showProtocol && prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	dadosCode := ""
	dadosBarcode := ""
	if isFS {
		emissionDay := "01"
		if parsed, ok := parseISO(ide.value("dhEmi")); ok {
			emissionDay = parsed.Format("02")
		}
		dadosCode = dadosNFeCode(
			firstNonempty(keyPrefix(key, 2), ide.value("cUF")), tpEmis,
			firstNonempty(dest.value("CNPJ"), dest.value("CPF")), cents(totalNode.value("vNF")),
			nonzero(totalNode.value("vICMS")), nonzero(totalNode.value("vST")), emissionDay,
		)
		dadosBarcode, err = code128DataURI(dadosCode)
		if err != nil {
			return nil, err
		}
	}

	additional := inf.child("infAdic")
	return map[string]any{
		"layout": "retrato",
		"emit": map[string]any{
			"nome": emit.value("xNome"), "fantasia": emit.value("xFant"), "cnpj": maskCNPJ(emit.value("CNPJ")),
			"ie": emit.value("IE"), "iest": emit.value("IEST"), "im": emit.value("IM"),
			"endereco": address(enderEmit), "cep": maskCEP(enderEmit.value("CEP")), "mun": enderEmit.value("xMun"),
			"uf": enderEmit.value("UF"), "fone": enderEmit.value("fone"), "logo_b64": "",
		},
		"dest": map[string]any{
			"nome": dest.value("xNome"), "doc": maskCPFCNPJ(firstNonempty(dest.value("CNPJ"), dest.value("CPF"))),
			"ie": dest.value("IE"), "endereco": streetAddress(enderDest), "bairro": enderDest.value("xBairro"),
			"cep": maskCEP(enderDest.value("CEP")),
			"mun": enderDest.value("xMun"), "uf": enderDest.value("UF"), "fone": enderDest.value("fone"),
		},
		"ide": map[string]any{
			"natOp": ide.value("natOp"), "tpNF": ide.value("tpNF"), "nNF": fiscalNumber(ide.value("nNF")),
			"tpNF_ascii": operationLabels[ide.value("tpNF")],
			"serie":      ide.value("serie"), "dhEmi": dateTimeBR(ide.value("dhEmi")), "dhEmi_data": dateBR(ide.value("dhEmi")),
			"dhSaiEnt": dateTimeBR(ide.value("dhSaiEnt")), "dhSaiEnt_data": dateBR(ide.value("dhSaiEnt")),
			"dhSaiEnt_hora": timeBR(ide.value("dhSaiEnt")),
		},
		"produtos": products, "totals": totals,
		"transp": map[string]any{
			"modFrete_label": freightLabel, "nome": carrier.value("xNome"),
			"doc": maskCPFCNPJ(firstNonempty(carrier.value("CNPJ"), carrier.value("CPF"))), "ie": carrier.value("IE"),
			"ender": carrier.value("xEnder"), "mun": carrier.value("xMun"), "uf": carrier.value("UF"),
			"placa": vehicle.value("placa"), "placa_uf": vehicle.value("UF"), "qVol": volume.value("qVol"),
			"placa_uf_fmt": plateUF,
			"esp":          volume.value("esp"), "marca": volume.value("marca"), "pesoB": volume.value("pesoB"), "pesoL": volume.value("pesoL"),
		},
		"fatura":     map[string]any{"nFat": fat.value("nFat"), "vOrig": moneyBR(fat.value("vOrig")), "vLiq": moneyBR(fat.value("vLiq"))},
		"duplicatas": dups, "chave_fmt": keyBlocks(key), "chave_raw": digits(key), "chave_barcode": barcode,
		"protocolo": protocol, "protocolo_label": ternary(isEPEC, "PROTOCOLO DE AUTORIZAÇÃO DO EPEC", "PROTOCOLO DE AUTORIZAÇÃO DE USO"),
		"show_protocolo": showProtocol, "is_contingencia": tpEmis != "1" && tpEmis != "3" && tpEmis != "6" && tpEmis != "7",
		"is_fs": isFS, "is_epec": isEPEC, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": canceled,
		"dados_nfe_code": dadosCode, "dados_nfe_barcode": dadosBarcode, "msg_fiscal": splitAdditional(additional.value("infAdFisco")),
		"msg_contribuinte": splitAdditional(additional.value("infCpl")), "site": "https://dfe.aoctech.app", "gerado_em": time.Now().Format("02/01/2006 15:04:05"),
		"text": map[string]any{
			"gerado_por": "Gerado por", "danfe": "DANFE", "danfe_desc": "DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA",
			"homologacao": "SEM VALOR FISCAL", "contingencia": "DANFE EM CONTINGÊNCIA - IMPRESSO EM DECORRÊNCIA DE PROBLEMAS TÉCNICOS",
			"consulta":  "Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da Sefaz Autorizadora",
			"dados_nfe": "DADOS DA NF-E", "cancelada": "CANCELADA",
		},
	}, nil
}

func keyPrefix(value string, length int) string {
	value = digits(value)
	if len(value) < length {
		return ""
	}
	return value[:length]
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func splitAdditional(value string) []string {
	parts := strings.Split(value, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ternary[T any](condition bool, yes, no T) T {
	if condition {
		return yes
	}
	return no
}
