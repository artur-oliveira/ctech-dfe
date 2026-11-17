package documents

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// DANFE-COM: documento auxiliar da NFCom (MOC NFCom, Anexo II). Representação resumida impressa em papel comum para o
// assinante: cabeçalho do emitente, dados do cliente e da fatura, identificação e protocolo, itens, tributos e a
// mensagem fiscal. Nada além do que consta do XML, exceto o protocolo.

var (
	nfcomUnitLabels = map[string]string{"1": "Minuto", "2": "MB", "3": "GB", "4": "UN"}
	nfcomTextLabels = map[string]any{
		"gerado_por": "Gerado por", "danfecom": "DANFE-COM",
		"danfecom_desc":  "Documento Auxiliar da Nota Fiscal de Fatura de Serviços de Comunicação Eletrônica",
		"homologacao":    "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL",
		"contingencia":   "EMITIDO EM CONTINGÊNCIA",
		"contingencia_2": "Pendente de autorização",
		"protocolo":      "Protocolo de autorização:", "cancelada": "CANCELADA",
		"consulta": "Consulte pela Chave de Acesso em",
	}
)

func buildNFComContext(root *xmlNode, canceled bool) (map[string]any, error) {
	inf := root.firstDeep("infNFCom")
	if inf == nil {
		return nil, fmt.Errorf("infNFCom not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelNFCom {
		return nil, fmt.Errorf("DANFE-COM requires model %s, got %q", modelNFCom, ide.value("mod"))
	}
	emit, dest, ender, enderDest := inf.child("emit"), inf.child("dest"), inf.child("emit").child("enderEmit"), inf.child("dest").child("enderDest")
	subscriber, invoice, total := inf.child("assinante"), inf.child("gFat"), inf.child("total")
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "NFCom")
	if prot != nil && prot.value("chNFCom") != "" {
		key = prot.value("chNFCom")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	qrURI := ""
	if qrValue := root.firstDeep("infNFComSupl").value("qrCodNFCom"); qrValue != "" {
		if qrURI, err = qrDataURI(qrValue); err != nil {
			return nil, err
		}
	}
	// Autorizada imprime o protocolo; na contingência o protocolo é suprimido (MOC 2.1.3 e 2.1.6).
	protocol := map[string]any(nil)
	if prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	contingency := ide.value("tpEmis") == "2" && protocol == nil

	items, sumPISCOFINS := buildNFComItems(inf)
	additional := inf.child("infAdic")
	return map[string]any{
		"layout": "retrato",
		"emit": map[string]any{
			"nome": emit.value("xNome"), "fantasia": emit.value("xFant"),
			"doc": maskCPFCNPJ(emit.value("CNPJ")), "ie": emit.value("IE"),
			"endereco": address(ender), "mun": ender.value("xMun"), "uf": ender.value("UF"), "cep": maskCEP(ender.value("CEP")),
		},
		"ide": map[string]any{"serie": ide.value("serie"), "nNF": fiscalNumber(ide.value("nNF")), "dhEmi": dateBR(ide.value("dhEmi"))},
		"cliente": map[string]any{
			"nome": dest.value("xNome"), "doc": maskCPFCNPJ(firstNonempty(dest.value("CNPJ"), dest.value("CPF"), dest.value("idOutros"))),
			"endereco": address(enderDest), "mun": enderDest.value("xMun"), "uf": enderDest.value("UF"), "cep": maskCEP(enderDest.value("CEP")),
			"codigo": subscriber.value("iCodAssinante"), "telefone": subscriber.value("NroTermPrinc"),
			"vencimento": dateBR(invoice.value("dVencFat")), "referencia": competenceBR(invoice.value("CompetFat")),
			"total": moneyBR(total.value("vNF")),
		},
		"itens": items,
		"totais": map[string]any{
			"vProd": moneyBR(total.value("vProd")), "vNF": moneyBR(total.value("vNF")),
			"vDesc": moneyBR(total.value("vDesc")), "vOutro": moneyBR(total.value("vOutro")),
			"bc_icms": moneyBR(total.child("ICMSTot").value("vBC")), "icms": moneyBR(total.child("ICMSTot").value("vICMS")),
			"pis": moneyBR(total.value("vPIS")), "cofins": moneyBR(total.value("vCOFINS")),
			"fust": moneyBR(total.value("vFUST")), "funttel": moneyBR(total.value("vFUNTTEL")),
			"pis_cofins": moneyBR(sumPISCOFINS.String()),
		},
		"chave_fmt": keyBlocks(key), "chave_raw": digits(key), "chave_barcode": barcode, "qr_uri": qrURI,
		"protocolo": protocol, "is_contingencia": contingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": canceled,
		"msg_fiscal": additional.value("infAdFisco"), "msg_contribuinte": additional.value("infCpl"),
		"site": "https://dfe.aoctech.app", "gerado_em": uftz.Now(ender.value("UF")).Format("02/01/2006 15:04:05"),
		"consulta_url": nfcomConsultaURL,
		"brand":        map[string]any{"logo_b64": "", "box_bg": defaultBoxBackground},
		"text":         nfcomTextLabels,
	}, nil
}

// buildNFComItems monta as linhas da divisão IV e devolve a soma de PIS e COFINS dos itens.
func buildNFComItems(inf *xmlNode) ([]map[string]any, decimal.Decimal) {
	sum := decimal.Zero
	items := make([]map[string]any, 0)
	for _, det := range inf.childrenNamed("det") {
		prod, imp := det.child("prod"), det.child("imposto")
		icms := firstTax(imp, "ICMS00", "ICMS20", "ICMS90")
		pis, cofins := parseOrZero(imp.child("PIS").value("vPIS")), parseOrZero(imp.child("COFINS").value("vCOFINS"))
		sum = sum.Add(pis).Add(cofins)
		desc := prod.value("xProd")
		if prod.value("indDevolucao") == "1" {
			desc += " (Devolução)"
		}
		items = append(items, map[string]any{
			"desc": desc, "unid": nfcomUnitLabels[prod.value("uMed")], "qtd": quantityBR(prod.value("qFaturada"), 2),
			"unitario": quantityBR(prod.value("vItem"), 2), "valor": moneyBR(prod.value("vProd")),
			"pis_cofins": moneyBR(pis.Add(cofins).String()),
			"bc_icms":    moneyBR(icms.value("vBC")), "aliq": icms.value("pICMS"),
			"icms": moneyBR(parseOrZero(icms.value("vICMS")).Add(parseOrZero(icms.value("vFCP"))).String()),
		})
	}
	return items, sum
}

// firstTax devolve o primeiro dos grupos de ICMS que existe no imposto; um nó vazio se nenhum.
func firstTax(imp *xmlNode, names ...string) *xmlNode {
	for _, n := range names {
		if g := imp.child(n); g != nil && g.value("CST") != "" {
			return g
		}
	}
	return imp.child("ICMS00")
}

func parseOrZero(s string) decimal.Decimal {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Zero
	}
	return d
}

// competenceBR formata AAAAMM como MM/AAAA (referência da fatura).
func competenceBR(value string) string {
	d := digits(value)
	if len(d) != 6 {
		return ""
	}
	return d[4:] + "/" + d[:4]
}
