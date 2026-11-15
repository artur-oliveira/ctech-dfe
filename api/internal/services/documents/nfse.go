package documents

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// nfseLogoPNG é o logotipo da NFS-e que o DANFSe nacional imprime no cabeçalho.
//
//go:embed assets/nfse-logo.png
var nfseLogoPNG []byte

// nfseLogoURI entrega o logotipo como data URI, o mesmo caminho do QR Code.
var nfseLogoURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(nfseLogoPNG)

// Textos fixos da NT 008 v1.02 que não vêm do XML.
const (
	nfseTitle       = "DANFSe v2.0"
	nfseSubtitle    = "Documento Auxiliar da NFS-e"
	nfseWordmark    = "NFS-e"
	nfseWordmarkSub = "Nota Fiscal de Serviço eletrônica"
	nfseHomologacao = "NFS-e SEM VALIDADE JURÍDICA"
	nfseQRLegend    = "A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR " +
		"ou pela consulta da chave de acesso no portal nacional da NFS-e"
	nfseTributosLegend = "Totais Aproximados dos Tributos cfe. Lei nº 12.741/2012"

	nfseWatermarkCancelled   = "CANCELADA"
	nfseWatermarkSubstituted = "SUBSTITUÍDA"
	nfseEmptyField           = "-"

	// nfseDescriptionLimit é o único campo com truncamento permitido pela NT:
	// a descrição do serviço tem 2000 caracteres no XSD e o quadro é fixo.
	nfseDescriptionLimit = 1200

	ambienteHomologacao = "2"

	// Códigos de TSTribISSQN. 1 é a única operação com ISSQN devido no
	// município; as outras três não têm alíquota a imprimir.
	tribISSQNImune      = "2"
	tribISSQNExportacao = "3"
	tribISSQNNaoIncid   = "4"

	// finalidadeNormal é o que se imprime quando a DPS não traz o grupo subst:
	// a finalidade só assume "Substituição" quando existe nota substituída.
	finalidadeNormal       = "NFS-e regular"
	finalidadeSubstituicao = "Substituição"
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
	cities := nfseKnownCities(inf, serv)
	valoresDPS := infDPS.child("valores")
	trib := valoresDPS.child("trib")
	valoresNFSe := inf.child("valores")
	ibscbsDPS := nfseIBSCBSNode(infDPS)

	return map[string]any{
		"cabecalho":      nfseCabecalho(inf, infDPS),
		"ident":          nfseIdent(inf, infDPS, accessKey),
		"prestador":      nfsePrestador(inf.child("emit"), infDPS.child("prest"), cities),
		"tomador":        nfseOptionalPerson(infDPS.child("toma"), cities),
		"destinatario":   nfseOptionalPerson(ibscbsDPS.child("dest"), cities),
		"intermediario":  nfseOptionalPerson(infDPS.child("interm"), cities),
		"servico":        nfseServico(inf, serv),
		"trib_mun":       nfseTribMunicipal(inf, infDPS, trib.child("tribMun"), valoresDPS, valoresNFSe),
		"trib_fed":       nfseTribFederal(trib.child("tribFed")),
		"ibscbs":         nfseIBSCBS(inf.child("IBSCBS"), ibscbsDPS),
		"totais":         nfseTotais(valoresDPS, valoresNFSe, inf.child("IBSCBS"), trib.child("totTrib")),
		"complementares": nfseComplementares(inf, serv, ibscbsDPS),
		"chave_fmt":      accessKey,
		"url_consulta":   nfseConsultaURL + accessKey,
		"qr_uri":         qrURI,
		"logo_uri":       nfseLogoURI,
		"is_homologacao": infDPS.value("tpAmb") == ambienteHomologacao,
		"watermark":      nfseWatermark(state),
		"canhoto": map[string]any{
			"numero": inf.value("nNFSe"),
			"chave":  accessKey,
		},
		"gerado_em": time.Now().Format("02/01/2006 15:04:05"),
		"site":      "https://dfe.aoctech.app",
		"text": map[string]any{
			"titulo": nfseTitle, "subtitulo": nfseSubtitle,
			"wordmark": nfseWordmark, "wordmark_sub": nfseWordmarkSub,
			"homologacao": nfseHomologacao, "qr_legenda": nfseQRLegend,
			"tributos_legenda": nfseTributosLegend,
			"gerado_por":       "Gerado por", "vazio": nfseEmptyField,
		},
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

// nfseCabecalho é a coluna direita do topo: município emissor e ambiente.
func nfseCabecalho(inf, infDPS *xmlNode) map[string]any {
	return map[string]any{
		"municipio":        strings.Join(nonempty(inf.value("xLocEmi"), inf.child("emit").child("enderNac").value("UF")), " - "),
		"ambiente_gerador": enumLabel(tables.EnumAmbGeradorNFSe, inf.value("ambGer")),
		"tipo_ambiente":    enumLabel(tables.EnumTipoAmbiente, infDPS.value("tpAmb")),
	}
}

func nfseIdent(inf, infDPS *xmlNode, accessKey string) map[string]any {
	subst := infDPS.child("subst")
	substituicao := map[string]any(nil)
	// A finalidade vem do finNFSe da DPS (NT 009, em infDPS) ou do grupo IBSCBS
	// (leiaute anterior); sem nenhum dos dois cai em "NFS-e regular".
	finalidade := finalidadeNormal
	if fin := firstNonEmpty(infDPS.value("finNFSe"), infDPS.child("IBSCBS").value("finNFSe")); fin != "" {
		finalidade = enumLabel("TSRTCFinNFSe", fin)
	}
	if subst != nil {
		finalidade = finalidadeSubstituicao
		substituicao = map[string]any{
			"chave":   subst.value("chSubstda"),
			"motivo":  enumLabel(tables.EnumCodJustSubst, subst.value("cMotivo")),
			"xmotivo": subst.value("xMotivo"),
		}
	}
	return map[string]any{
		"chave":            accessKey,
		"numero":           inf.value("nNFSe"),
		"numero_dfse":      inf.value("nDFSe"),
		"competencia":      dateBR(infDPS.value("dCompet")),
		"emissao_nfse":     dateTimeBR(inf.value("dhProc")),
		"emissao_dps":      dateTimeBR(infDPS.value("dhEmi")),
		"dps_serie":        infDPS.value("serie"),
		"dps_numero":       infDPS.value("nDPS"),
		"emitente_tipo":    enumLabel(tables.EnumEmitenteDPS, infDPS.value("tpEmit")),
		"motivo_emissao":   enumLabel(tables.EnumMotivoEmisTI, infDPS.value("cMotivoEmisTI")),
		"situacao":         enumLabel("TStat", inf.value("cStat")),
		"finalidade":       finalidade,
		"local_prestacao":  inf.value("xLocPrestacao"),
		"local_incidencia": inf.value("xLocIncid"),
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

// nfsePrestador funde o emitente da NFS-e (que tem nome e endereço completos,
// resolvidos pelo fisco) com o prestador declarado na DPS, que traz o regime
// tributário. São a mesma pessoa quando tpEmit é o prestador.
func nfsePrestador(emit, prest *xmlNode, cities map[string]string) map[string]any {
	person, _ := nfsePerson(emit, emit.child("enderNac"), cities).(map[string]any)
	if person == nil {
		return nil
	}
	regTrib := prest.child("regTrib")
	if im := prest.value("IM"); im != "" && person["inscricao_municipal"] == "" {
		person["inscricao_municipal"] = im
	}
	person["simples_nacional"] = enumLabel(tables.EnumOpSimpNac, regTrib.value("opSimpNac"))
	person["regime_apuracao_sn"] = enumLabel(tables.EnumRegApuracaoSimpNac, regTrib.value("regApTribSN"))
	return person
}

// nfseOptionalPerson devolve nil quando a parte não existe no XML — o template
// suprime o quadro inteiro em vez de imprimir campos vazios.
func nfseOptionalPerson(person *xmlNode, cities map[string]string) any {
	if person == nil {
		return nil
	}
	return nfsePerson(person, addressNode(person), cities)
}

func nfsePerson(person, end *xmlNode, cities map[string]string) any {
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
		"nome":                person.value("xNome"),
		"fantasia":            person.value("xFant"),
		"documento":           maskCPFCNPJ(document),
		"nao_nif":             enumLabel(tables.EnumCodNaoNIF, person.value("cNaoNIF")),
		"caepf":               person.value("CAEPF"),
		"inscricao_municipal": person.value("IM"),
		"endereco":            address(end),
		"municipio_uf":        nfseMunicipioUF(end, cities),
		"ibge_cep":            nfseIBGECEP(end),
		"telefone":            person.value("fone"),
		"email":               person.value("email"),
	}
}

// nfseKnownCities reúne os nomes de município que o próprio XML traz (emissor,
// incidência e local da prestação), indexados pelo código IBGE. O endereço
// nacional de uma parte só tem o código, e o nome só é impresso quando o XML o
// informa para aquele código.
func nfseKnownCities(inf, serv *xmlNode) map[string]string {
	cities := map[string]string{}
	for code, name := range map[string]string{
		inf.value("cLocEmi"):                          inf.value("xLocEmi"),
		inf.value("cLocIncid"):                        inf.value("xLocIncid"),
		serv.child("locPrest").value("cLocPrestacao"): inf.value("xLocPrestacao"),
	} {
		if code != "" && name != "" {
			cities[code] = name
		}
	}
	return cities
}

// ufByIBGEPrefix traduz os dois primeiros dígitos do código IBGE na sigla da UF.
var ufByIBGEPrefix = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL", "28": "SE", "29": "BA",
	"31": "MG", "32": "ES", "33": "RJ", "35": "SP", "41": "PR", "42": "SC", "43": "RS",
	"50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// nfseMunicipioUF só imprime o que o XML traz ou o que o código IBGE determina
// sem ambiguidade: o nome vem do próprio documento (nfseKnownCities) e a UF, dos
// dois primeiros dígitos do código.
func nfseMunicipioUF(end *xmlNode, cities map[string]string) string {
	if end == nil {
		return ""
	}
	if city := end.value("xCidade"); city != "" {
		return strings.Join(nonempty(city, end.value("xEstProvReg"), end.value("cPais")), " / ")
	}
	code := end.value("cMun")
	uf := end.value("UF")
	if uf == "" && len(code) == 7 {
		uf = ufByIBGEPrefix[code[:2]]
	}
	return strings.Join(nonempty(firstNonEmpty(end.value("xMun"), cities[code]), uf), " / ")
}

func nfseIBGECEP(end *xmlNode) string {
	if end == nil {
		return ""
	}
	if postal := end.value("cEndPost"); postal != "" {
		return postal
	}
	return strings.Join(nonempty(maskIBGE(end.value("cMun")), maskCEPNacional(end.value("CEP"))), " / ")
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
			"valor_moeda":         moneyOpt(comExt.value("vServMoeda")),
			"mecanismo_prestador": enumLabel(tables.EnumMecAFComExPrestador, comExt.value("mecAFComexP")),
			"mecanismo_tomador":   enumLabel(tables.EnumMecAFComExTomador, comExt.value("mecAFComexT")),
			"movimentacao_bens":   enumLabel(tables.EnumMovTempBens, comExt.value("movTempBens")),
			"numero_di":           comExt.value("nDI"),
			"numero_re":           comExt.value("nRE"),
			"mdic":                enumLabel(tables.EnumEnvMDIC, comExt.value("mdic")),
		}
	}
	// A NT manda imprimir a descrição municipal quando existir e cair na
	// nacional apenas quando não existir.
	descricaoCodigo := inf.value("xTribMun")
	if descricaoCodigo == "" {
		descricaoCodigo = inf.value("xTribNac")
	}
	return map[string]any{
		"codigo_tributacao": strings.Join(nonempty(maskTribNac(cServ.value("cTribNac")),
			cServ.child("cTribMun").value("cTribMun")), " / "),
		"descricao_codigo":  descricaoCodigo,
		"cod_nbs":           maskNBS(cServ.value("cNBS")),
		"desc_nbs":          inf.value("xNBS"),
		"cod_interno":       cServ.value("cIntContrib"),
		"local_prestacao":   inf.value("xLocPrestacao"),
		"descricao":         truncate(cServ.value("xDescServ"), nfseDescriptionLimit),
		"comercio_exterior": orNil(comercioExterior),
	}
}

func nfseObra(obra *xmlNode) string {
	if obra == nil {
		return ""
	}
	return strings.Join(nonempty(
		labelled("Inscrição imobiliária", obra.value("inscImobFisc")),
		labelled("Código da obra", obra.value("cObra")),
		labelled("CIB", obra.value("cCIB")),
		labelled("Endereço", address(obra.child("end"))),
	), "; ")
}

func nfseEvento(evento *xmlNode) string {
	if evento == nil {
		return ""
	}
	period := ""
	if start, end := dateBR(evento.value("dtIni")), dateBR(evento.value("dtFim")); start != "" || end != "" {
		period = start + " a " + end
	}
	return strings.Join(nonempty(
		evento.value("xNome"),
		labelled("Período", period),
		labelled("Identificação", evento.value("idAtvEvt")),
		labelled("Endereço", address(evento.child("end"))),
	), "; ")
}

func nfseImovel(imovel *xmlNode) string {
	if imovel == nil {
		return ""
	}
	return strings.Join(nonempty(
		labelled("Inscrição imobiliária", imovel.value("inscImobFisc")),
		labelled("CIB", imovel.value("cCIB")),
		labelled("Endereço", address(imovel.child("end"))),
	), "; ")
}

func nfseTribMunicipal(inf, infDPS, tribMun, valoresDPS, valoresNFSe *xmlNode) map[string]any {
	if tribMun == nil {
		return nil
	}
	exigSusp := tribMun.child("exigSusp")
	beneficio := ""
	if bm := tribMun.child("BM"); bm != nil {
		beneficio = strings.Join(nonempty(bm.value("nBM"),
			enumLabel(tables.EnumBeneficioMunicipal, valoresNFSe.value("tpBM"))), " ")
	}
	tribISSQN := tribMun.value("tribISSQN")
	return map[string]any{
		"tipo_tributacao":      enumLabel(tables.EnumTribISSQN, tribISSQN),
		"municipio_incidencia": inf.value("xLocIncid"),
		"regime_especial": enumLabel(tables.EnumRegEspTrib,
			infDPS.child("prest").child("regTrib").value("regEspTrib")),
		"tipo_imunidade":  enumLabel(tables.EnumTipoImunidadeISSQN, tribMun.value("tpImunidade")),
		"suspensao":       enumLabel(tables.EnumOpExigSuspensa, exigSusp.value("tpSusp")),
		"processo":        exigSusp.value("nProcesso"),
		"beneficio":       beneficio,
		"calculo_bm":      moneyOpt(valoresNFSe.value("vCalcBM")),
		"total_deducoes":  moneyOpt(valoresNFSe.value("vCalcDR")),
		"desconto_incond": moneyOpt(valoresDPS.child("vDescCondIncond").value("vDescIncond")),
		"bc_issqn":        moneyOpt(valoresNFSe.value("vBC")),
		"retencao":        enumLabel(tables.EnumTipoRetISSQN, tribMun.value("tpRetISSQN")),
		"issqn_apurado":   moneyOpt(valoresNFSe.value("vISSQN")),
		"pais_resultado":  tribMun.value("cPaisResult"),
		"aliquota_aplicada": firstNonEmpty(percentOpt(valoresNFSe.value("pAliqAplic")),
			percentOpt(tribMun.value("pAliq"))),
		// A NT destaca operações sem ISSQN devido no município: imunidade,
		// exportação e não incidência não imprimem alíquota como se houvesse.
		"sem_issqn_devido": tribISSQN == tribISSQNImune ||
			tribISSQN == tribISSQNExportacao || tribISSQN == tribISSQNNaoIncid,
	}
}

func nfseTribFederal(tribFed *xmlNode) map[string]any {
	pc := tribFed.child("piscofins")
	return map[string]any{
		"irrf":              moneyOpt(tribFed.value("vRetIRRF")),
		"previdenciaria":    moneyOpt(tribFed.value("vRetCP")),
		"sociais_retidas":   moneyOpt(tribFed.value("vRetCSLL")),
		"pis":               moneyOpt(pc.value("vPis")),
		"cofins":            moneyOpt(pc.value("vCofins")),
		"cst":               enumLabel(tables.EnumCSTPISCofins, pc.value("CST")),
		"retencao":          enumLabel(tables.EnumTipoRetPISCofins, pc.value("tpRetPisCofins")),
		"base_calculo":      moneyOpt(pc.value("vBCPisCofins")),
		"aliq_pis":          percentOpt(pc.value("pAliqPis")),
		"aliq_cofins":       percentOpt(pc.value("pAliqCofins")),
		"descricao_sociais": enumLabel(tables.EnumTipoRetPISCofins, pc.value("tpRetPisCofins")),
	}
}

// nfseIBSCBS junta os valores apurados (bloco IBSCBS da NFS-e) com a
// classificação declarada na DPS: a NT imprime CST/cClassTrib e o indicador de
// operação ao lado dos valores.
func nfseIBSCBS(group, dps *xmlNode) map[string]any {
	if group == nil {
		return nil
	}
	valores := group.child("valores")
	tot := group.child("totCIBS")
	gIBS := tot.child("gIBS")
	classificacao := dps.child("valores").child("trib").child("gIBSCBS")
	return map[string]any{
		"cst_class_trib": strings.Join(nonempty(classificacao.value("CST"),
			classificacao.value("cClassTrib")), " / "),
		"indicador_operacao":   dps.value("cIndOp"),
		"municipio_incidencia": group.value("xLocalidadeIncid"),
		"exclusoes":            moneyOpt(valores.value("vCalcReeRepRes")),
		"bc_apos_exclusoes":    moneyOpt(valores.value("vBC")),
		"redutor":              percentOpt(group.value("pRedutor")),
		"red_aliq_ibs_uf":      percentOpt(valores.child("uf").value("pRedAliqUF")),
		"red_aliq_ibs_mun":     percentOpt(valores.child("mun").value("pRedAliqMun")),
		"red_aliq_cbs":         percentOpt(valores.child("fed").value("pRedAliqCBS")),
		"aliq_ibs_uf":          percentOpt(valores.child("uf").value("pIBSUF")),
		"aliq_ibs_mun":         percentOpt(valores.child("mun").value("pIBSMun")),
		"aliq_efet_ibs_uf":     percentOpt(valores.child("uf").value("pAliqEfetUF")),
		"aliq_efet_ibs_mun":    percentOpt(valores.child("mun").value("pAliqEfetMun")),
		"aliq_cbs":             percentOpt(valores.child("fed").value("pCBS")),
		"aliq_efet_cbs":        percentOpt(valores.child("fed").value("pAliqEfetCBS")),
		"valor_ibs_uf":         moneyOpt(gIBS.child("gIBSUFTot").value("vIBSUF")),
		"valor_ibs_mun":        moneyOpt(gIBS.child("gIBSMunTot").value("vIBSMun")),
		"valor_total_ibs":      moneyOpt(gIBS.value("vIBSTot")),
		"valor_total_cbs":      moneyOpt(tot.child("gCBS").value("vCBS")),
		"total_nf":             moneyOpt(tot.value("vTotNF")),
	}
}

func nfseTotais(valoresDPS, valoresNFSe, ibscbs, totTrib *xmlNode) map[string]any {
	descontos := valoresDPS.child("vDescCondIncond")
	monetario := totTrib.child("vTotTrib")
	percentual := totTrib.child("pTotTrib")
	liquido := valoresNFSe.value("vLiq")
	totalIBSCBS := ibscbsTotal(ibscbs)
	return map[string]any{
		"valor_operacao":  moneyOpt(valoresDPS.child("vServPrest").value("vServ")),
		"valor_recebido":  moneyOpt(valoresDPS.child("vServPrest").value("vReceb")),
		"desconto_incond": moneyOpt(descontos.value("vDescIncond")),
		"desconto_cond":   moneyOpt(descontos.value("vDescCond")),
		"total_retencoes": moneyOpt(valoresNFSe.value("vTotalRet")),
		"valor_liquido":   moneyOpt(liquido),
		"total_ibscbs":    moneyOpt(totalIBSCBS),
		"liquido_ibscbs":  moneyOpt(sumDecimals(liquido, totalIBSCBS)),
		"tem_ibscbs":      ibscbs != nil,
		"trib_federal":    moneyOpt(monetario.value("vTotTribFed")),
		"trib_estadual":   moneyOpt(monetario.value("vTotTribEst")),
		"trib_municipal":  moneyOpt(monetario.value("vTotTribMun")),
		"perc_federal":    percentOpt(percentual.value("pTotTribFed")),
		"perc_estadual":   percentOpt(percentual.value("pTotTribEst")),
		"perc_municipal":  percentOpt(percentual.value("pTotTribMun")),
		"perc_simples":    percentOpt(totTrib.value("pTotTribSN")),
		"deducao_percent": percentOpt(valoresDPS.child("vDedRed").value("pDR")),
		"deducao_valor":   moneyOpt(valoresDPS.child("vDedRed").value("vDR")),
		"deducao_apurada": moneyOpt(valoresNFSe.value("vCalcDR")),
	}
}

// ibscbsTotal soma IBS e CBS apurados. Nenhum nó do XML traz esse total pronto:
// vTotNF já inclui o serviço, então somar aqui é a única forma de imprimir a
// linha "Total do IBS/CBS" sem inventar valor.
func ibscbsTotal(ibscbs *xmlNode) string {
	if ibscbs == nil {
		return ""
	}
	tot := ibscbs.child("totCIBS")
	return sumDecimals(tot.child("gIBS").value("vIBSTot"), tot.child("gCBS").value("vCBS"))
}

func nfseComplementares(inf, serv, ibscbsDPS *xmlNode) map[string]any {
	infoCompl := serv.child("infoCompl")
	itens := make([]string, 0)
	if pedido := infoCompl.child("gItemPed"); pedido != nil {
		for _, item := range pedido.childrenNamed("xItemPed") {
			itens = append(itens, strings.TrimSpace(item.text))
		}
	}
	return map[string]any{
		"imovel":         nfseImovel(ibscbsDPS.child("imovel")),
		"obra":           nfseObra(serv.child("obra")),
		"evento":         nfseEvento(serv.child("atvEvento")),
		"doc_tecnico":    infoCompl.value("idDocTec"),
		"doc_referencia": infoCompl.value("docRef"),
		"pedido":         infoCompl.value("xPed"),
		"itens_pedido":   itens,
		"informacoes":    infoCompl.value("xInfComp"),
		"municipais":     inf.value("xOutInf"),
	}
}

// enumLabel devolve só o texto oficial do domínio fechado, como o DANFSe do
// ambiente nacional (sem o código na frente); valor vazio some do DANFSe e valor
// desconhecido é impresso cru, nunca traduzido por adivinhação.
func enumLabel(typeName, value string) string {
	if value == "" {
		return ""
	}
	if label, ok := tables.EnumLabel(typeName, value); ok {
		return label
	}
	return value
}

func labelled(label, value string) string {
	if value == "" {
		return ""
	}
	return label + ": " + value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
