package documents

import (
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// paymentLabels: tabela de tPag do Portal Nacional (mesma de ui/src/lib/data/payment-tables.ts), todos os
// códigos que a API aceita (dto.go oneof).
var paymentLabels = map[string]string{
	"01": "Dinheiro", "02": "Cheque", "03": "Cartão de Crédito", "04": "Cartão de Débito",
	"05": "Cartão da Loja (Private Label), Crediário Digital, Outros Crediários",
	"10": "Vale Alimentação", "11": "Vale Refeição", "12": "Vale Presente", "13": "Vale Combustível",
	"14": "Duplicata Mercantil", "15": "Boleto Bancário", "16": "Depósito Bancário",
	"17": "Pagamento Instantâneo (PIX) - Dinâmico", "18": "TED (Transferência Eletrônica Disponível)",
	"19": "Programa de fidelidade, Cashback, Crédito Virtual", "20": "Pagamento Instantâneo (PIX) - Estático",
	"21": "Crédito em Loja", "22": "Pagamento Eletrônico não Informado - falha de hardware do sistema emissor",
	"23": "Pagamento Instantâneo (PIX) - Automático", "24": "TEF - \"Book Transfer\"",
	"90": "Sem Pagamento", "91": "Pagamento Posterior", "99": "Outros",
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
	qrURI, err := qrDataURIQuiet(supl.value("qrCode"))
	if err != nil {
		return nil, err
	}
	emit := inf.child("emit")
	ender := emit.child("enderEmit")
	items := make([]map[string]any, 0)
	for _, det := range inf.childrenNamed("det") {
		prod := det.child("prod")
		items = append(items, map[string]any{
			"cProd": prod.value("cProd"), "xProd": prod.value("xProd"), "qCom": quantityBR(prod.value("qCom"), 0),
			"uCom": prod.value("uCom"), "vUnCom": quantityBR(prod.value("vUnCom"), 2), "vProd": moneyBR(prod.value("vProd")),
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
	// Manual 3.1.3: "Acréscimos (frete, seguro e outras despesas)" sums vFrete, vSeg and vOutro; each line and
	// "Valor a Pagar" are printed only when an addition or a discount exists.
	additions := sumDecimals(total.value("vFrete"), total.value("vSeg"), total.value("vOutro"))
	hasAddition, hasDiscount := nonzero(additions), nonzero(total.value("vDesc"))
	uf := ender.value("UF")
	// Manual 3.1.7: the protocol is printed whenever SEFAZ authorized the document; an offline (tpEmis 9) NFC-e
	// is "pendente" only while it has no protocol (contingency spec 9.3: re-rendered with it after authorization).
	protocol := map[string]any(nil)
	if prot != nil && prot.value("nProt") != "" && protocolAuthorized(prot.value("cStat")) {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": localDateTimeBR(prot.value("dhRecbto"), uf)}
	}
	isContingency := ide.value("tpEmis") == "9" && protocol == nil
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
		// Manual DANFE NFC-e 3.1.8: only the 2nd copy is identified, next to the date and time of emission.
		vias = []map[string]any{{"label": ""}, {"label": "Via do Estabelecimento"}}
	}
	return map[string]any{
		"emit": map[string]any{
			"doc_label": emitDocLabel(emit), "cnpj": maskCPFCNPJ(emit.value("CNPJ") + emit.value("CPF")), "nome": emit.value("xNome"),
			"endereco": strings.Join(nonempty(ender.value("xLgr"), ender.value("nro"), ender.value("xCpl"), ender.value("xBairro"), ender.value("xMun"), ender.value("UF"), maskCEP(ender.value("CEP"))), ", "),
			"logo_b64": "",
		},
		"show_items": true, "items": items,
		"totals": map[string]any{
			"qtd": len(items), "vProd": moneyBR(total.value("vProd")), "vDesc": moneyBR(total.value("vDesc")),
			"vAcrescimos": moneyBR(additions), "has_acrescimo": hasAddition, "has_desconto": hasDiscount,
			"vNF": moneyBR(total.value("vNF")), "vTotTrib": moneyBR(total.value("vTotTrib")),
			"has_acrescimo_desconto": hasAddition || hasDiscount, "pagamentos": payments,
			"troco": moneyBR(troco), "has_troco": hasChange,
		},
		"chave_fmt": keyBlocks(key), "url_chave": supl.value("urlChave"), "qr_uri": qrURI, "consumidor": consumer,
		"ident":     map[string]any{"nNF": ide.value("nNF"), "serie": ide.value("serie"), "dhEmi": localDateTimeBR(ide.value("dhEmi"), uf)},
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

// protocolAuthorized: the protocol of a document in use (100 authorized, 150 authorized out of time). Absent
// cStat counts as authorized: only an authorized document carries an infProt in the nfeProc.
func protocolAuthorized(cStat string) bool {
	return cStat == "" || cStat == "100" || cStat == "150"
}

func emitDocLabel(emit *xmlNode) string {
	if emit.value("CNPJ") == "" && emit.value("CPF") != "" {
		return "CPF"
	}
	return "CNPJ"
}

// localDateTimeBR prints an XML instant in the clock of the issuer's UF (Manual 3.1.7 asks for local time).
func localDateTimeBR(value, uf string) string {
	parsed, ok := parseISO(value)
	if !ok {
		return ""
	}
	return parsed.In(uftz.Location(uf)).Format("02/01/2006 15:04:05")
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
