package documents

import (
	"fmt"
	"strings"

	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// DABPE: documento auxiliar do BP-e (MOC BP-e, Anexo II). Representação resumida do bilhete comum e do TA para acompanhar o
// passageiro: emitente, viagem (uma divisão por trecho), valores, consulta pela chave, passageiro, identificação e protocolo,
// QR Code, mensagem fiscal e do contribuinte. O TM é apuração por viagem e não tem bilhete individual: não gera DABPE.

const (
	modelBPe       = "63"
	bpeConsultaURL = "https://dfe-portal.svrs.rs.gov.br/BPE/Consulta"
	// tpBPeTM é o tpBPe do transporte metropolitano.
	tpBPeTM = "4"
)

var (
	bpeServiceLabels = map[string]string{
		"1": "Convencional com sanitário", "2": "Convencional sem sanitário", "3": "Semileito", "4": "Leito com ar condicionado",
		"5": "Leito sem ar condicionado", "6": "Executivo", "7": "Semiurbano", "8": "Longitudinal", "9": "Travessia",
	}
	bpeDocLabels       = map[string]string{"1": "RG", "2": "TÍTULO DE ELEITOR", "3": "PASSAPORTE", "4": "CNH", "5": "OUTROS"}
	bpePaymentLabels   = map[string]string{"01": "Dinheiro", "02": "Cheque", "03": "Cartão de crédito", "04": "Cartão de débito", "05": "Vale transporte", "99": "Outros"}
	bpeComponentLabels = map[string]string{
		"01": "Tarifa", "02": "Pedágio", "03": "Taxa de embarque", "04": "Seguro", "05": "TMR", "06": "SCT", "07": "Taxa de serviço",
		"08": "Taxa de conveniência", "99": "Outros",
	}
	bpeTextLabels = map[string]any{
		"gerado_por": "Gerado por", "dabpe": "DABPE", "dabpe_desc": "Documento Auxiliar do Bilhete de Passagem Eletrônico",
		"homologacao":  "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL",
		"contingencia": "EMITIDO EM CONTINGÊNCIA", "contingencia_2": "Pendente de autorização",
		"protocolo": "Protocolo de autorização:", "cancelada": "CANCELADA", "consulta": "Consulte pela Chave de Acesso em",
		"nao_identificado": "PASSAGEIRO NÃO IDENTIFICADO", "conexao": "- CONEXÃO -",
	}
)

func buildBPeContext(root *xmlNode, canceled bool) (map[string]any, error) {
	inf := root.firstDeep("infBPe")
	if inf == nil {
		return nil, fmt.Errorf("infBPe not found")
	}
	ide := inf.child("ide")
	if ide.value("mod") != modelBPe {
		return nil, fmt.Errorf("DABPE requires model %s, got %q", modelBPe, ide.value("mod"))
	}
	if ide.value("tpBPe") == tpBPeTM {
		return nil, fmt.Errorf("DABPE não se aplica ao BP-e TM")
	}
	emit, ender := inf.child("emit"), inf.child("emit").child("enderEmit")
	value, passage := inf.child("infValorBPe"), inf.child("infPassagem")
	prot := root.firstDeep("infProt")
	key := strings.TrimPrefix(inf.attr("Id"), "BPe")
	if prot != nil && prot.value("chBPe") != "" {
		key = prot.value("chBPe")
	}
	barcode, err := code128DataURI(key)
	if err != nil {
		return nil, err
	}
	qrURI := ""
	if qrValue := root.firstDeep("infBPeSupl").value("qrCodBPe"); qrValue != "" {
		if qrURI, err = qrDataURI(qrValue); err != nil {
			return nil, err
		}
	}
	// Autorizado imprime o protocolo; na contingência o protocolo é suprimido (MOC 2.2.7).
	protocol := map[string]any(nil)
	if prot != nil {
		protocol = map[string]any{"nProt": prot.value("nProt"), "dhRecbto": dateTimeBR(prot.value("dhRecbto"))}
	}
	contingency := ide.value("tpEmis") == "2" && protocol == nil

	components := make([]map[string]any, 0)
	for _, c := range value.childrenNamed("Comp") {
		label := bpeComponentLabels[c.value("tpComp")]
		if label == "" {
			label = c.value("tpComp")
		}
		components = append(components, map[string]any{"nome": label, "valor": moneyBR(c.value("vComp"))})
	}
	payments := make([]map[string]any, 0)
	for _, p := range inf.childrenNamed("pag") {
		label := bpePaymentLabels[p.value("tPag")]
		if label == "" {
			label = "Outros (" + p.value("tPag") + ")"
		}
		if x := p.value("xPag"); x != "" {
			label = x
		}
		payments = append(payments, map[string]any{"forma": label, "valor": moneyBR(p.value("vPag"))})
	}
	discount := nonzero(value.value("vDesconto"))
	return map[string]any{
		"layout": "retrato",
		"emit": map[string]any{
			"nome": emit.value("xNome"), "fantasia": emit.value("xFant"), "doc": maskCPFCNPJ(emit.value("CNPJ")), "ie": emit.value("IE"),
			"endereco": address(ender), "mun": ender.value("xMun"), "uf": ender.value("UF"), "cep": maskCEP(ender.value("CEP")),
		},
		"ide":        map[string]any{"serie": ide.value("serie"), "nBP": fiscalNumber(ide.value("nBP")), "dhEmi": dateTimeBR(ide.value("dhEmi"))},
		"origem":     map[string]any{"nome": passage.value("xLocOrig"), "uf": ide.value("UFIni")},
		"destino":    map[string]any{"nome": passage.value("xLocDest"), "uf": ide.value("UFFim")},
		"viagens":    buildBPeTrips(inf, passage),
		"passageiro": buildBPePassenger(passage.child("infPassageiro"), value),
		"valores": map[string]any{
			"componentes": components, "total": moneyBR(value.value("vBP")), "desconto": moneyBR(value.value("vDesconto")),
			"tem_desconto": discount, "a_pagar": moneyBR(value.value("vPgto")), "pagamentos": payments,
			"pago": moneyBR(sumDecimals(paymentValues(inf)...)), "troco": moneyBR(value.value("vTroco")),
		},
		"chave_fmt": keyBlocks(key), "chave_raw": digits(key), "chave_barcode": barcode, "qr_uri": qrURI,
		"protocolo": protocol, "is_contingencia": contingency, "is_homologacao": ide.value("tpAmb") == "2", "is_cancelada": canceled,
		"msg_fiscal": inf.child("infAdic").value("infAdFisco"), "msg_contribuinte": inf.child("infAdic").value("infCpl"),
		"site": "https://dfe.aoctech.app", "gerado_em": uftz.Now(ender.value("UF")).Format("02/01/2006 15:04:05"),
		"consulta_url": bpeConsultaURL,
		"brand":        map[string]any{"logo_b64": "", "box_bg": defaultBoxBackground},
		"text":         bpeTextLabels,
	}, nil
}

// buildBPeTrips monta uma divisão III por trecho (MOC 2.2.3). O TA identifica o voo e os aeroportos no lugar de linha e poltrona.
func buildBPeTrips(inf, passage *xmlNode) []map[string]any {
	out := make([]map[string]any, 0)
	for i, v := range inf.childrenNamed("infViagem") {
		service := bpeServiceLabels[v.value("tpServ")]
		if service == "" {
			service = v.value("tpServ")
		}
		out = append(out, map[string]any{
			"conexao": i > 0 || v.value("tpTrecho") == "3", "data": dateBR(v.value("dhViagem")), "hora": timeBR(v.value("dhViagem")),
			"poltrona": v.value("poltrona"), "plataforma": v.value("plataforma"), "prefixo": v.value("prefixo"),
			"linha": v.value("xPercurso"), "tipo": service,
			"voo": v.value("nroVoo"), "companhia": v.value("SiglaCiaOperVoo"),
			"aeroporto_origem": v.value("cAeroOrig"), "aeroporto_destino": v.value("cAeroDest"),
		})
	}
	return out
}

// buildBPePassenger devolve a linha de identificação do passageiro e a de desconto (MOC 2.2.6), em caixa alta.
func buildBPePassenger(px, value *xmlNode) map[string]any {
	out := map[string]any{"identificado": false, "linha": "", "desconto": ""}
	if px != nil && px.value("xNome") != "" {
		doc := bpeDocLabels[px.value("tpDoc")]
		out["identificado"] = true
		out["linha"] = strings.ToUpper(fmt.Sprintf("PASSAGEIRO: %s: %s %s", doc, px.value("nDoc"), px.value("xNome")))
	}
	if t := value.value("tpDesconto"); t != "" {
		out["desconto"] = strings.ToUpper("TIPO DE DESCONTO: " + firstNonempty(value.value("xDesconto"), t))
	}
	return out
}

func paymentValues(inf *xmlNode) []string {
	var out []string
	for _, p := range inf.childrenNamed("pag") {
		out = append(out, p.value("vPag"))
	}
	return out
}
