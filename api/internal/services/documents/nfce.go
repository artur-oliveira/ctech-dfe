package documents

import (
	"fmt"
	"strings"
	"time"
)

var paymentLabels = map[string]string{
	"01": "Dinheiro", "02": "Cheque", "03": "Cartão de Crédito", "04": "Cartão de Débito",
	"05": "Crédito Loja", "10": "Vale Alimentação", "11": "Vale Refeição", "12": "Vale Presente",
	"13": "Vale Combustível", "15": "Boleto Bancário", "16": "Depósito Bancário",
	"17": "Pagamento Instantâneo (PIX) - Dinâmico", "18": "Transferência bancária, Carteira Digital",
	"19": "Programa de fidelidade, Cashback, Crédito Virtual", "20": "Pagamento Instantâneo (PIX) - Estático",
	"21": "Crédito em loja", "22": "Pagamento Eletrônico não Informado", "90": "Sem pagamento", "99": "Outros",
}

func buildNFCeContext(root *xmlNode, canceled bool) (map[string]any, error) {
	inf := root.firstDeep("infNFe")
	if inf == nil {
		return nil, fmt.Errorf("infNFe not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelNFCe {
		return nil, fmt.Errorf("DANFC-e requires model %s, got %q", modelNFCe, ide.value("mod"))
	}
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "NFe")
	if prot != nil && prot.value("chNFe") != "" {
		key = prot.value("chNFe")
	}
	supl := root.firstDeep("infNFeSupl")
	qrURI, err := qrDataURI(supl.value("qrCode"))
	if err != nil {
		return nil, err
	}
	emit := inf.child("emit")
	ender := emit.child("enderEmit")
	items := make([]map[string]any, 0)
	for _, det := range inf.childrenNamed("det") {
		prod := det.child("prod")
		items = append(items, map[string]any{
			"cProd": prod.value("cProd"), "xProd": prod.value("xProd"), "qCom": prod.value("qCom"),
			"uCom": prod.value("uCom"), "vUnCom": moneyBR(prod.value("vUnCom")), "vProd": moneyBR(prod.value("vProd")),
		})
	}
	pag := inf.child("pag")
	payments := make([]map[string]any, 0)
	for _, detPag := range pag.childrenNamed("detPag") {
		code := detPag.value("tPag")
		label := paymentLabels[code]
		if label == "" {
			label = code
		}
		payments = append(payments, map[string]any{"forma": label, "valor": moneyBR(detPag.value("vPag"))})
	}
	total := inf.child("total").child("ICMSTot")
	hasAdditions := false
	for _, name := range []string{"vFrete", "vSeg", "vOutro", "vDesc"} {
		if value := total.value(name); value != "" && value != "0" && value != "0.00" {
			hasAdditions = true
		}
	}
	tpEmis := ide.value("tpEmis")
	isContingency := tpEmis == "9"
	protocol := map[string]any(nil)
	if !isContingency && prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	troco := pag.value("vTroco")
	hasChange := troco != "" && troco != "0" && troco != "0.00"
	dest := inf.child("dest")
	consumer := "CONSUMIDOR NÃO IDENTIFICADO"
	if dest.value("CNPJ") != "" {
		consumer = "CONSUMIDOR CNPJ: " + maskCNPJ(dest.value("CNPJ"))
	} else if dest.value("CPF") != "" {
		consumer = "CONSUMIDOR CPF: " + maskCPF(dest.value("CPF"))
	} else if dest.value("idEstrangeiro") != "" {
		consumer = "CONSUMIDOR Id. Estrangeiro: " + dest.value("idEstrangeiro")
	}
	additional := inf.child("infAdic")
	vias := []map[string]any{{"label": ""}}
	if isContingency {
		vias = []map[string]any{{"label": "Via Consumidor"}, {"label": "Via do Estabelecimento"}}
	}
	return map[string]any{
		"emit": map[string]any{
			"cnpj": maskCNPJ(emit.value("CNPJ")), "nome": emit.value("xNome"),
			"endereco": strings.Join(nonempty(ender.value("xLgr"), ender.value("nro"), ender.value("xBairro"), ender.value("xMun"), ender.value("UF")), ", "),
			"logo_b64": "",
		},
		"show_items": true, "items": items,
		"totals": map[string]any{
			"qtd": len(items), "vProd": moneyBR(total.value("vProd")), "vDesc": moneyBR(total.value("vDesc")),
			"vFrete": moneyBR(total.value("vFrete")), "vSeg": moneyBR(total.value("vSeg")), "vOutro": moneyBR(total.value("vOutro")),
			"vNF": moneyBR(total.value("vNF")), "vTotTrib": moneyBR(total.value("vTotTrib")),
			"has_acrescimo_desconto": hasAdditions, "pagamentos": payments,
			"troco": moneyBR(troco), "has_troco": hasChange,
		},
		"chave_fmt": keyBlocks(key), "url_chave": supl.value("urlChave"), "qr_uri": qrURI, "consumidor": consumer,
		"ident":     map[string]any{"nNF": ide.value("nNF"), "serie": ide.value("serie"), "dhEmi": dateTimeBR(ide.value("dhEmi"))},
		"protocolo": protocol, "is_contingencia": isContingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": canceled,
		"msg_fiscal": additional.value("infAdFisco"), "msg_contribuinte": additional.value("infCpl"),
		"site": "https://dfe.aoctech.app", "gerado_em": time.Now().Format("02/01/2006 15:04:05"), "vias": vias,
		"text": map[string]any{
			"doc_auxiliar": "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica",
			"cont_l1":      "EMITIDA EM CONTINGÊNCIA", "cont_l2": "Pendente de autorização",
			"homologacao": "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL", "cancelada": "CANCELADA", "gerado_por": "Gerado por",
		},
	}, nil
}

func nonempty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
