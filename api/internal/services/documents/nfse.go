package documents

import (
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// Textos fixos da NT 008 v1.02 que não vêm do XML.
const (
	nfseTitle                = "DANFSe - Documento Auxiliar da NFS-e"
	nfseHomologacao          = "NFS-e SEM VALIDADE JURÍDICA"
	nfseWatermarkCancelled   = "CANCELADA"
	nfseWatermarkSubstituted = "SUBSTITUÍDA"
	nfseEmptyField           = "-"

	// nfseDescriptionLimit é o único campo com truncamento permitido pela NT:
	// a descrição do serviço, que tem 2000 caracteres no XSD e não cabe no
	// quadro de uma página. Os demais campos nunca são truncados.
	nfseDescriptionLimit = 900

	ambienteHomologacao = "2"
	tribISSQNExportacao = "2"
	tribISSQNNaoIncid   = "3"
	tribISSQNImune      = "4"
)

// buildNFSeContext monta o contexto do DANFSe a partir do XML NFSe autorizado.
// Lê apenas o documento: nada que não exista no XML pode ser impresso.
func buildNFSeContext(root *xmlNode, state DocumentState) (map[string]any, error) {
	inf := root.firstDeep("infNFSe")
	if inf == nil {
		return nil, fmt.Errorf("infNFSe not found")
	}
	accessKey := digits(inf.attr("Id"))
	if len(accessKey) != accessKeyLengthByDocType[DocTypeNFSe] {
		return nil, fmt.Errorf("DANFSe requires a %d-digit access key, got %q",
			accessKeyLengthByDocType[DocTypeNFSe], inf.attr("Id"))
	}
	infDPS := inf.firstDeep("infDPS")
	if infDPS == nil {
		return nil, fmt.Errorf("infDPS not found inside infNFSe")
	}
	qrURI, err := qrDataURI(nfseConsultaURL + accessKey)
	if err != nil {
		return nil, err
	}

	serv := infDPS.child("serv")
	valoresDPS := infDPS.child("valores")
	trib := valoresDPS.child("trib")
	valoresNFSe := inf.child("valores")

	return map[string]any{
		"ident":          nfseIdent(inf, infDPS, accessKey),
		"emitente":       nfsePerson(inf.child("emit"), inf.child("emit").child("enderNac")),
		"prestador":      nfsePerson(infDPS.child("prest"), addressNode(infDPS.child("prest"))),
		"tomador":        nfseOptionalPerson(infDPS.child("toma")),
		"intermediario":  nfseOptionalPerson(infDPS.child("interm")),
		"destinatario":   nfseOptionalPerson(nfseIBSCBSNode(infDPS).child("dest")),
		"servico":        nfseServico(inf, serv),
		"trib_mun":       nfseTribMunicipal(inf, trib.child("tribMun"), valoresNFSe),
		"trib_fed":       nfseTribFederal(trib.child("tribFed")),
		"ibscbs":         nfseIBSCBS(inf.child("IBSCBS")),
		"totais":         nfseTotais(valoresDPS, valoresNFSe, trib.child("totTrib")),
		"complementares": nfseComplementares(inf, serv),
		"chave_fmt":      keyBlocks(accessKey),
		"url_consulta":   nfseConsultaURL + accessKey,
		"qr_uri":         qrURI,
		"is_homologacao": infDPS.value("tpAmb") == ambienteHomologacao,
		"watermark":      nfseWatermark(state),
		"gerado_em":      time.Now().Format("02/01/2006 15:04:05"),
		"text": map[string]any{
			"titulo": nfseTitle, "homologacao": nfseHomologacao, "gerado_por": "Gerado por",
			"vazio": nfseEmptyField,
		},
		"site": "https://dfe.aoctech.app",
	}, nil
}

// nfseWatermark devolve o texto diagonal da NT; documento ativo não tem.
func nfseWatermark(state DocumentState) string {
	switch state {
	case StateCancelled:
		return nfseWatermarkCancelled
	case StateSubstituted:
		return nfseWatermarkSubstituted
	default:
		return ""
	}
}

func nfseIdent(inf, infDPS *xmlNode, accessKey string) map[string]any {
	subst := infDPS.child("subst")
	substituicao := map[string]any(nil)
	if subst != nil {
		substituicao = map[string]any{
			"chave":   keyBlocks(subst.value("chSubstda")),
			"motivo":  enumLabel(tables.EnumCodJustSubst, subst.value("cMotivo")),
			"xmotivo": subst.value("xMotivo"),
		}
	}
	return map[string]any{
		"chave":            accessKey,
		"numero":           inf.value("nNFSe"),
		"numero_dfse":      inf.value("nDFSe"),
		"competencia":      dateBR(infDPS.value("dCompet")),
		"emissao":          dateTimeBR(infDPS.value("dhEmi")),
		"processamento":    dateTimeBR(inf.value("dhProc")),
		"dps_serie":        infDPS.value("serie"),
		"dps_numero":       infDPS.value("nDPS"),
		"dps_id":           infDPS.attr("Id"),
		"emitente_tipo":    enumLabel(tables.EnumEmitenteDPS, infDPS.value("tpEmit")),
		"motivo_emissao":   enumLabel(tables.EnumMotivoEmisTI, infDPS.value("cMotivoEmisTI")),
		"local_emissao":    inf.value("xLocEmi"),
		"local_prestacao":  inf.value("xLocPrestacao"),
		"local_incidencia": inf.value("xLocIncid"),
		"situacao":         inf.value("cStat"),
		"substituicao":     orNil(substituicao),
	}
}

// nfseIBSCBSNode isola o grupo IBSCBS da DPS, que pode não existir.
func nfseIBSCBSNode(infDPS *xmlNode) *xmlNode {
	if group := infDPS.child("IBSCBS"); group != nil {
		return group
	}
	return &xmlNode{}
}

func addressNode(person *xmlNode) *xmlNode {
	if person == nil {
		return nil
	}
	end := person.child("end")
	if end == nil {
		return nil
	}
	// TCEndereco tem escolha endNac|endExt e o logradouro fica no nível de cima;
	// achatamos para o template não conhecer a diferença.
	flat := &xmlNode{name: "end", attrs: map[string]string{}, children: end.children}
	if nac := end.child("endNac"); nac != nil {
		flat.children = append(flat.children, nac.children...)
	}
	if ext := end.child("endExt"); ext != nil {
		flat.children = append(flat.children, ext.children...)
	}
	return flat
}

// nfseOptionalPerson devolve nil quando a parte não existe no XML — o template
// suprime o quadro inteiro em vez de imprimir campos vazios.
func nfseOptionalPerson(person *xmlNode) any {
	if person == nil {
		return nil
	}
	return nfsePerson(person, addressNode(person))
}

func nfsePerson(person, end *xmlNode) map[string]any {
	if person == nil {
		return nil
	}
	document := person.value("CNPJ")
	if document == "" {
		document = person.value("CPF")
	}
	if document == "" {
		document = person.value("NIF")
	}
	return map[string]any{
		"nome":      person.value("xNome"),
		"fantasia":  person.value("xFant"),
		"documento": maskCPFCNPJ(document),
		"nao_nif":   enumLabel(tables.EnumCodNaoNIF, person.value("cNaoNIF")),
		"caepf":     person.value("CAEPF"),
		"im":        person.value("IM"),
		"endereco":  address(end),
		"municipio": nfseMunicipio(end),
		"cep":       maskCEP(end.value("CEP")),
		"fone":      person.value("fone"),
		"email":     person.value("email"),
	}
}

// nfseMunicipio junta município e UF nacionais ou cidade e região no exterior.
func nfseMunicipio(end *xmlNode) string {
	if end == nil {
		return ""
	}
	if city := end.value("xCidade"); city != "" {
		return strings.Join(nonempty(city, end.value("xEstProvReg"), end.value("cPais")), " / ")
	}
	return strings.Join(nonempty(end.value("cMun"), end.value("UF")), " / ")
}

func nfseServico(inf, serv *xmlNode) map[string]any {
	cServ := serv.child("cServ")
	comExt := serv.child("comExt")
	comercioExterior := map[string]any(nil)
	if comExt != nil {
		comercioExterior = map[string]any{
			"modo_prestacao":      enumLabel(tables.EnumModoPrestacao, comExt.value("mdPrestacao")),
			"vinculo":             enumLabel(tables.EnumVincPrest, comExt.value("vincPrest")),
			"moeda":               comExt.value("tpMoeda"),
			"valor_moeda":         moneyBR(comExt.value("vServMoeda")),
			"mecanismo_prestador": enumLabel(tables.EnumMecAFComExPrestador, comExt.value("mecAFComexP")),
			"mecanismo_tomador":   enumLabel(tables.EnumMecAFComExTomador, comExt.value("mecAFComexT")),
			"movimentacao_bens":   enumLabel(tables.EnumMovTempBens, comExt.value("movTempBens")),
			"numero_di":           comExt.value("nDI"),
			"numero_re":           comExt.value("nRE"),
			"mdic":                enumLabel(tables.EnumEnvMDIC, comExt.value("mdic")),
		}
	}
	return map[string]any{
		"cod_trib_nac":      cServ.value("cTribNac"),
		"desc_trib_nac":     inf.value("xTribNac"),
		"cod_trib_mun":      cServ.child("cTribMun").value("cTribMun"),
		"desc_trib_mun":     inf.value("xTribMun"),
		"cod_nbs":           cServ.value("cNBS"),
		"desc_nbs":          inf.value("xNBS"),
		"cod_interno":       cServ.value("cIntContrib"),
		"descricao":         truncate(cServ.value("xDescServ"), nfseDescriptionLimit),
		"comercio_exterior": orNil(comercioExterior),
		"obra":              nfseObra(serv.child("obra")),
		"evento":            nfseEvento(serv.child("atvEvento")),
	}
}

func nfseObra(obra *xmlNode) any {
	if obra == nil {
		return nil
	}
	return map[string]any{
		"inscricao_imobiliaria": obra.value("inscImobFisc"),
		"codigo_obra":           obra.value("cObra"),
		"cib":                   obra.value("cCIB"),
		"endereco":              address(obra.child("end")),
	}
}

func nfseEvento(evento *xmlNode) any {
	if evento == nil {
		return nil
	}
	return map[string]any{
		"nome":     evento.value("xNome"),
		"inicio":   dateBR(evento.value("dtIni")),
		"fim":      dateBR(evento.value("dtFim")),
		"id":       evento.value("idAtvEvt"),
		"endereco": address(evento.child("end")),
	}
}

func nfseTribMunicipal(inf, tribMun, valoresNFSe *xmlNode) any {
	if tribMun == nil {
		return nil
	}
	exigSusp := tribMun.child("exigSusp")
	suspensao := map[string]any(nil)
	if exigSusp != nil {
		suspensao = map[string]any{
			"tipo":     enumLabel(tables.EnumOpExigSuspensa, exigSusp.value("tpSusp")),
			"processo": exigSusp.value("nProcesso"),
		}
	}
	beneficio := map[string]any(nil)
	if bm := tribMun.child("BM"); bm != nil {
		beneficio = map[string]any{
			"numero":          bm.value("nBM"),
			"tipo":            enumLabel(tables.EnumBeneficioMunicipal, valoresNFSe.value("tpBM")),
			"valor_reducao":   moneyBR(bm.value("vRedBCBM")),
			"perc_reducao":    percentBR(bm.value("pRedBCBM")),
			"valor_calculado": moneyBR(valoresNFSe.value("vCalcBM")),
		}
	}
	tribISSQN := tribMun.value("tribISSQN")
	return map[string]any{
		"tributacao":           enumLabel(tables.EnumTribISSQN, tribISSQN),
		"retencao":             enumLabel(tables.EnumTipoRetISSQN, tribMun.value("tpRetISSQN")),
		"imunidade":            enumLabel(tables.EnumTipoImunidadeISSQN, tribMun.value("tpImunidade")),
		"pais_resultado":       tribMun.value("cPaisResult"),
		"suspensao":            orNil(suspensao),
		"beneficio":            orNil(beneficio),
		"aliquota":             percentBR(tribMun.value("pAliq")),
		"aliquota_aplicada":    percentBR(valoresNFSe.value("pAliqAplic")),
		"base_calculo":         moneyBR(valoresNFSe.value("vBC")),
		"valor_issqn":          moneyBR(valoresNFSe.value("vISSQN")),
		"municipio_incidencia": inf.value("xLocIncid"),
		// A NT destaca operações sem ISSQN devido no município: exportação,
		// não incidência e imunidade não imprimem alíquota como se houvesse.
		"sem_issqn_devido": tribISSQN == tribISSQNExportacao ||
			tribISSQN == tribISSQNNaoIncid || tribISSQN == tribISSQNImune,
	}
}

func nfseTribFederal(tribFed *xmlNode) any {
	if tribFed == nil {
		return nil
	}
	pisCofins := map[string]any(nil)
	if pc := tribFed.child("piscofins"); pc != nil {
		pisCofins = map[string]any{
			"cst":          enumLabel(tables.EnumCSTPISCofins, pc.value("CST")),
			"base_calculo": moneyBR(pc.value("vBCPisCofins")),
			"aliq_pis":     percentBR(pc.value("pAliqPis")),
			"aliq_cofins":  percentBR(pc.value("pAliqCofins")),
			"valor_pis":    moneyBR(pc.value("vPis")),
			"valor_cofins": moneyBR(pc.value("vCofins")),
			"retencao":     enumLabel(tables.EnumTipoRetPISCofins, pc.value("tpRetPisCofins")),
		}
	}
	return map[string]any{
		"piscofins": orNil(pisCofins),
		"ret_cp":    moneyBR(tribFed.value("vRetCP")),
		"ret_irrf":  moneyBR(tribFed.value("vRetIRRF")),
		"ret_csll":  moneyBR(tribFed.value("vRetCSLL")),
		"tem_retencao": nonzero(tribFed.value("vRetCP")) || nonzero(tribFed.value("vRetIRRF")) ||
			nonzero(tribFed.value("vRetCSLL")),
	}
}

func nfseIBSCBS(group *xmlNode) any {
	if group == nil {
		return nil
	}
	valores := group.child("valores")
	tot := group.child("totCIBS")
	gIBS := tot.child("gIBS")
	gCBS := tot.child("gCBS")
	return map[string]any{
		"localidade":    group.value("xLocalidadeIncid"),
		"redutor":       percentBR(group.value("pRedutor")),
		"base_calculo":  moneyBR(valores.value("vBC")),
		"ree_rep_res":   moneyBR(valores.value("vCalcReeRepRes")),
		"aliq_uf":       percentBR(valores.child("uf").value("pAliqEfetUF")),
		"aliq_mun":      percentBR(valores.child("mun").value("pAliqEfetMun")),
		"aliq_cbs":      percentBR(valores.child("fed").value("pAliqEfetCBS")),
		"valor_ibs":     moneyBR(gIBS.value("vIBSTot")),
		"valor_ibs_uf":  moneyBR(gIBS.child("gIBSUFTot").value("vIBSUF")),
		"valor_ibs_mun": moneyBR(gIBS.child("gIBSMunTot").value("vIBSMun")),
		"valor_cbs":     moneyBR(gCBS.value("vCBS")),
		"total_nf":      moneyBR(tot.value("vTotNF")),
	}
}

func nfseTotais(valoresDPS, valoresNFSe, totTrib *xmlNode) map[string]any {
	descontos := valoresDPS.child("vDescCondIncond")
	dedRed := valoresDPS.child("vDedRed")
	monetario := totTrib.child("vTotTrib")
	percentual := totTrib.child("pTotTrib")
	return map[string]any{
		"valor_servico":           moneyBR(valoresDPS.child("vServPrest").value("vServ")),
		"valor_recebido":          moneyBR(valoresDPS.child("vServPrest").value("vReceb")),
		"desconto_incondicionado": moneyBR(descontos.value("vDescIncond")),
		"desconto_condicionado":   moneyBR(descontos.value("vDescCond")),
		"deducao_valor":           moneyBR(dedRed.value("vDR")),
		"deducao_perc":            percentBR(dedRed.value("pDR")),
		"deducao_calculada":       moneyBR(valoresNFSe.value("vCalcDR")),
		"total_retencoes":         moneyBR(valoresNFSe.value("vTotalRet")),
		"valor_liquido":           moneyBR(valoresNFSe.value("vLiq")),
		"trib_federal":            moneyBR(monetario.value("vTotTribFed")),
		"trib_estadual":           moneyBR(monetario.value("vTotTribEst")),
		"trib_municipal":          moneyBR(monetario.value("vTotTribMun")),
		"perc_federal":            percentBR(percentual.value("pTotTribFed")),
		"perc_estadual":           percentBR(percentual.value("pTotTribEst")),
		"perc_municipal":          percentBR(percentual.value("pTotTribMun")),
		"perc_simples":            percentBR(totTrib.value("pTotTribSN")),
	}
}

func nfseComplementares(inf, serv *xmlNode) map[string]any {
	infoCompl := serv.child("infoCompl")
	itens := make([]string, 0)
	if pedido := infoCompl.child("gItemPed"); pedido != nil {
		for _, item := range pedido.childrenNamed("xItemPed") {
			itens = append(itens, strings.TrimSpace(item.text))
		}
	}
	return map[string]any{
		"doc_tecnico":        infoCompl.value("idDocTec"),
		"doc_referencia":     infoCompl.value("docRef"),
		"pedido":             infoCompl.value("xPed"),
		"itens_pedido":       itens,
		"informacoes":        infoCompl.value("xInfComp"),
		"outras_informacoes": inf.value("xOutInf"),
	}
}

// enumLabel devolve o rótulo oficial do domínio fechado; valor vazio some do
// DANFSe e valor desconhecido é impresso cru, nunca traduzido por adivinhação.
func enumLabel(typeName, value string) string {
	if value == "" {
		return ""
	}
	if label, ok := tables.EnumLabel(typeName, value); ok {
		return value + " - " + label
	}
	return value
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// orNil evita o clássico "interface não-nil com mapa nil": um grupo ausente
// precisa chegar ao template como nil, ou `{% if %}` o trataria como presente.
func orNil(value map[string]any) any {
	if value == nil {
		return nil
	}
	return value
}
