// Package xsdorder is a 1:1 port of py-dfe's xmlops/xsd_order.py: XSD-defined
// child element order for fiscal document XML nodes.
//
// Keys follow the convention:
//
//	"tag"            — generic (used when no parent context is available)
//	"parentTag:tag"  — specific (takes precedence over generic)
//
// The builder resolves "<parent>:<tag>" first, then falls back to "<tag>".
// This resolves collisions where the same local name (e.g. ide, emit) has
// different child sequences across NF-e, CT-e and MDF-e.
//
// Sources:
//
//	NF-e 4.00      — leiauteNFe_v4.00.xsd + NT 2024.001 (IBS/CBS/IS)
//	NFC-e 4.00     — leiauteNFCe_v4.00.xsd
//	CT-e 4.00      — leiauteCTe_v4.00.xsd
//	MDF-e 3.00     — leiauteMDFe_v3.00.xsd
//	Eventos NF-e   — leiauteEvento_v1.00.xsd + e110110/111/112/140/111500-503/210200-240
//	Eventos CT-e   — eventoCTe_v4.00.xsd + evCancCTe_v4.00.xsd etc.
//	Eventos MDF-e  — eventoMDFe_v3.00.xsd + ev*.xsd (NT012025)
//	inutNFe        — leiauteInutNFe_v4.00.xsd
//	ConsCad        — leiauteConsCad_v2.00.xsd
//	consStatServ   — leiauteConsStatServ_v4.00.xsd
//	consSitNFe     — leiauteConsSitNFe_v4.00.xsd
//	distDFeInt     — leiauteDistDFeInt_v1.01.xsd
package xsdorder

// =============================================================================
// Private per-root/operation tables — merged into Table at init time.
// =============================================================================

// ─────────────────────────────────────────────────────────────────────────────
// NF-e / NFC-e — Autorização  (mod 55 / mod 65)
// ─────────────────────────────────────────────────────────────────────────────
var envInfe = map[string][]string{

	"enviNFe": {"idLote", "indSinc", "NFe"},
	"NFe":     {"infNFe", "infNFeSupl"},

	"infNFe": {
		"ide", "emit", "avulsa", "dest",
		"retirada", "entrega", "autXML",
		"det",
		"total", "transp", "cobr", "pag",
		"infIntermed", "infAdic", "exporta", "compra", "cana",
		"infRespTec", "infSolicNFF",
	},

	// ide — NF-e
	"infNFe:ide": {
		"cUF", "cNF", "natOp", "mod", "serie", "nNF",
		"dhEmi", "dhSaiEnt", "dPrevEntrega", "tpNF", "idDest", "cMunFG", "cMunFGIBS",
		"tpImp", "tpEmis", "cDV", "tpAmb", "finNFe",
		"tpNFDebito", "tpNFCredito",
		"indFinal", "indPres", "indIntermed", "procEmi", "verProc",
		"dhCont", "xJust",
		"NFref", "gCompraGov", "gPagAntecipado",
	},

	// emit — NF-e: CNPJ|CPF vem antes de xNome
	"infNFe:emit": {
		"CNPJ", "CPF",
		"xNome", "xFant", "enderEmit",
		"IE", "IEST", "IM", "CNAE", "CRT",
	},

	// dest — NF-e
	"infNFe:dest": {
		"CNPJ", "CPF", "idEstrangeiro",
		"xNome", "enderDest", "indIEDest",
		"IE", "ISUF", "IM", "email",
	},

	// retirada / entrega
	"retirada": {
		"CNPJ", "CPF", "xNome",
		"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "UF",
		"cPais", "xPais", "fone", "email",
	},
	"entrega": {
		"CNPJ", "CPF", "xNome",
		"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "UF",
		"cPais", "xPais", "fone", "email",
	},

	// autXML
	"autXML": {"CNPJ", "CPF"},

	// det
	"det": {"prod", "imposto", "obsItem"},

	// prod
	"prod": {
		"cProd", "cEAN", "cBarra", "xProd", "NCM", "NVE", "CEST",
		"indEscala", "CNPJFab", "cBenef", "gCred", "tpCredPresIBSZFM", "EXTIPI",
		"CFOP", "uCom", "qCom", "vUnCom", "vProd",
		"cEANTrib", "cBarraTrib", "uTrib", "qTrib", "vUnTrib",
		"vFrete", "vSeg", "vDesc", "vOutro", "indTot", "indBemMovelUsado",
		"DI", "detExport", "xPed", "nItemPed", "nFCI", "rastro",
		"infProdNFF", "infProdEmb", "veicProd", "med", "arma", "comb", "nRECOPI",
	},

	// avulsa
	"avulsa": {"CNPJ", "xOrgao", "matr", "xAgente", "fone", "UF", "nDAR", "dEmi", "vDAR", "repEmi", "dPag"},

	// NFref
	"NFref":  {"refNFe", "refNFeSig", "refNF", "refNFP", "refCTe", "refECF"},
	"refNF":  {"cUF", "AAMM", "CNPJ", "mod", "serie", "nNF"},
	"refNFP": {"cUF", "AAMM", "CNPJ", "CPF", "IE", "mod", "serie", "nNF"},
	"refECF": {"mod", "nECF", "nCOO"},

	// gCred — crédito presumido no item
	"gCred": {"cCredPresumido", "pCredPresumido", "vCredPresumido"},

	// DI / adi
	"DI": {
		"nDI", "dDI", "xLocDesemb", "UFDesemb", "dDesemb", "tpViaTransp",
		"vAFRMM", "tpIntermedio", "CNPJ", "CPF", "UFTerceiro", "cExportador", "adi",
	},
	"adi": {"nAdicao", "nSeqAdic", "cFabricante", "vDescDI", "nDraw"},

	// detExport / exportInd
	"detExport": {"nDraw", "exportInd"},
	"exportInd": {"nRE", "chNFe", "qExport"},

	// rastro
	"rastro": {"nLote", "qLote", "dFab", "dVal", "cAgreg"},

	// infProdNFF / infProdEmb
	"infProdNFF": {"cProdFisco", "cOperNFF"},
	"infProdEmb": {"xEmb", "qVolEmb", "uEmb"},

	// veicProd
	"veicProd": {
		"tpOp", "chassi", "cCor", "xCor", "pot", "cilin", "pesoL", "pesoB",
		"nSerie", "tpComb", "nMotor", "CMT", "dist", "anoMod", "anoFab",
		"tpPint", "tpVeic", "espVeic", "VIN", "condVeic", "cMod",
		"cCorDENATRAN", "lota", "tpRest",
	},

	// med
	"med": {"cProdANVISA", "xMotivoIsencao", "vPMC"},

	// arma
	"arma": {"tpArma", "nSerie", "nCano", "descr"},

	// comb
	"comb": {
		"cProdANP", "descANP", "pGLP", "pGNn", "pGNi", "vPart",
		"CODIF", "qTemp", "UFCons", "CIDE", "encerrante", "pBio", "origComb",
	},
	"CIDE":       {"qBCProd", "vAliqProd", "vCIDE"},
	"encerrante": {"nBico", "nBomba", "nTanque", "vEncIni", "vEncFin"},
	"origComb":   {"indImport", "cUFOrig", "pOrig"},

	// ── Impostos do item ──────────────────────────────────────────────────────
	// Sequência no imposto do item (NT 2024.001)
	"imposto": {
		"vTotTrib",
		"ICMS", "IPI", "II", "ISSQNcalc",
		"PIS", "PISST", "COFINS", "COFINSST",
		"ICMSUFDest",
		"IS",     // Imposto Seletivo (separado do IBSCBS)
		"IBSCBS", // IBS + CBS — type TTribNFe
	},

	// IBSCBS — type TTribNFe (CST + cClassTrib + choice de valores)
	"IBSCBS": {
		"CST", "cClassTrib", "indDoacao",
		"gIBSCBS", "gIBSCBSMono", "gTransfCred", "gAjusteCompet",
		"gEstornoCred", "gCredPresOper", "gCredPresIBSZFM",
	},

	// gIBSCBS — type TCIBS (valores de IBS Estadual, Municipal e CBS)
	// parent_tag = IBSCBS para distinguir do lookup genérico
	"IBSCBS:gIBSCBS": {
		"vBC",
		"gIBSUF",
		"gIBSMun",
		"vIBS",
		"gCBS",
	},

	// Subgrupos IBS estadual / municipal / CBS
	"gIBSUF":  {"pIBSUF", "gDif", "gDevTrib", "gRed", "vIBSUF"},
	"gIBSMun": {"pIBSMun", "gDif", "gDevTrib", "gRed", "vIBSMun"},
	"gCBS":    {"pCBS", "gDif", "gDevTrib", "gRed", "vCBS"},

	// IS — Imposto Seletivo (PL_010e_v1.02)
	"IS": {
		"CSTIS", "cClassTribIS", "vBCIS", "pIS",
		"adRemIS", "uTrib", "qTrib", "vIS",
	},

	// Grupos ICMS
	"ICMS00":     {"orig", "CST", "modBC", "vBC", "pICMS", "vICMS", "pFCP", "vFCP"},
	"ICMS02":     {"orig", "CST", "qBCMono", "adRemICMS", "vICMSMono"},
	"ICMS10":     {"orig", "CST", "modBC", "vBC", "pICMS", "vICMS", "vBCFCP", "pFCP", "vFCP", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "vICMSSTDeson", "motDesICMSST"},
	"ICMS15":     {"orig", "CST", "qBCMono", "adRemICMS", "vICMSMono", "qBCMonoReten", "adRemICMSReten", "vICMSMonoReten", "pRedAdRem", "motRedAdRem"},
	"ICMS20":     {"orig", "CST", "modBC", "pRedBC", "vBC", "pICMS", "vICMS", "vBCFCP", "pFCP", "vFCP", "vICMSDeson", "motDesICMS", "indDeduzDeson"},
	"ICMS30":     {"orig", "CST", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "vICMSDeson", "motDesICMS", "indDeduzDeson"},
	"ICMS40":     {"orig", "CST", "vICMSDeson", "motDesICMS", "indDeduzDeson"},
	"ICMS51":     {"orig", "CST", "modBC", "pRedBC", "cBenefRBC", "vBC", "pICMS", "vICMSOp", "pDif", "vICMSDif", "vICMS", "vBCFCP", "pFCP", "vFCP", "pFCPDif", "vFCPDif", "vFCPEfet"},
	"ICMS53":     {"orig", "CST", "qBCMono", "adRemICMS", "vICMSMonoOp", "pDif", "vICMSMonoDif", "vICMSMono", "qBCMonoDif", "adRemICMSDif"},
	"ICMS60":     {"orig", "CST", "vBCSTRet", "pST", "vICMSSubstituto", "vICMSSTRet", "vBCFCPSTRet", "pFCPSTRet", "vFCPSTRet", "pRedBCEfet", "vBCEfet", "pICMSEfet", "vICMSEfet"},
	"ICMS61":     {"orig", "CST", "qBCMonoRet", "adRemICMSRet", "vICMSMonoRet"},
	"ICMS70":     {"orig", "CST", "modBC", "pRedBC", "vBC", "pICMS", "vICMS", "vBCFCP", "pFCP", "vFCP", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "vICMSDeson", "motDesICMS", "indDeduzDeson", "vICMSSTDeson", "motDesICMSST"},
	"ICMS90":     {"orig", "CST", "modBC", "vBC", "pRedBC", "cBenefRBC", "pICMS", "vICMSOp", "pDif", "vICMSDif", "vICMS", "vBCFCP", "pFCP", "vFCP", "pFCPDif", "vFCPDif", "vFCPEfet", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "vICMSDeson", "motDesICMS", "indDeduzDeson", "vICMSSTDeson", "motDesICMSST"},
	"ICMSPart":   {"orig", "CST", "modBC", "vBC", "pRedBC", "pICMS", "vICMS", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "pBCOp", "UFST", "vICMSDeson", "motDesICMS", "indDeduzDeson"},
	"ICMSST":     {"orig", "CST", "vBCSTRet", "pST", "vICMSSubstituto", "vICMSSTRet", "vBCFCPSTRet", "pFCPSTRet", "vFCPSTRet", "vBCSTDest", "vICMSSTDest", "pRedBCEfet", "vBCEfet", "pICMSEfet", "vICMSEfet"},
	"ICMSSN101":  {"orig", "CSOSN", "pCredSN", "vCredICMSSN"},
	"ICMSSN102":  {"orig", "CSOSN"},
	"ICMSSN201":  {"orig", "CSOSN", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "pCredSN", "vCredICMSSN"},
	"ICMSSN202":  {"orig", "CSOSN", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST"},
	"ICMSSN500":  {"orig", "CSOSN", "vBCSTRet", "pST", "vICMSSubstituto", "vICMSSTRet", "vBCFCPSTRet", "pFCPSTRet", "vFCPSTRet", "pRedBCEfet", "vBCEfet", "pICMSEfet", "vICMSEfet"},
	"ICMSSN900":  {"orig", "CSOSN", "modBC", "vBC", "pRedBC", "pICMS", "vICMS", "modBCST", "pMVAST", "pRedBCST", "vBCST", "pICMSST", "vICMSST", "vBCFCPST", "pFCPST", "vFCPST", "pCredSN", "vCredICMSSN"},
	"ICMSUFDest": {"vBCUFDest", "vBCFCPUFDest", "pFCPUFDest", "pICMSUFDest", "pICMSInter", "pICMSInterPart", "vFCPUFDest", "vICMSUFDest", "vICMSUFRemet"},

	// IPI
	"IPI":     {"CNPJProd", "cSelo", "qSelo", "cEnq", "IPITrib", "IPINT"},
	"IPITrib": {"CST", "vBC", "pIPI", "qUnid", "vUnid", "vIPI"},
	"IPINT":   {"CST"},

	// PIS / COFINS
	"PISAliq":    {"CST", "vBC", "pPIS", "vPIS"},
	"PISQtde":    {"CST", "qBCProd", "vAliqProd", "vPIS"},
	"PISNT":      {"CST"},
	"PISOutr":    {"CST", "vBC", "pPIS", "qBCProd", "vAliqProd", "vPIS"},
	"PISST":      {"vBC", "pPIS", "qBCProd", "vAliqProd", "vPIS", "indSomaPISST"},
	"COFINSAliq": {"CST", "vBC", "pCOFINS", "vCOFINS"},
	"COFINSQtde": {"CST", "qBCProd", "vAliqProd", "vCOFINS"},
	"COFINSNT":   {"CST"},
	"COFINSOutr": {"CST", "vBC", "pCOFINS", "qBCProd", "vAliqProd", "vCOFINS"},
	"COFINSST":   {"vBC", "pCOFINS", "qBCProd", "vAliqProd", "vCOFINS", "indSomaCOFINSST"},

	// total — NF-e (com IBS/CBS/IS — NT 2024.001)
	// Ordem conforme leiauteNFe_v4.00.xsd: ISTot precede IBSCBSTot
	"total":    {"ICMSTot", "ISSQNtot", "retTrib", "ISTot", "IBSCBSTot", "vNFTot"},
	"ISSQN":    {"vBC", "vAliq", "vISSQN", "cMunFG", "cListServ", "vDeducao", "vOutro", "vDescIncond", "vDescCond", "vISSRet", "indISS", "cServico", "cMun", "cPais", "nProcesso", "indIncentivo"},
	"ISSQNtot": {"vServ", "vBC", "vISS", "vPIS", "vCOFINS", "dCompet", "vDeducao", "vOutro", "vDescIncond", "vDescCond", "vISSRet", "cRegTrib"},
	"obsItem":  {"obsCont", "obsFisco"},
	"retTrib":  {"vRetPIS", "vRetCOFINS", "vRetCSLL", "vBCIRRF", "vIRRF", "vBCRetPrev", "vRetPrev"},

	// IBSCBSTot — totais IBS/CBS por nota (type TIBSCBSMonoTot)
	"IBSCBSTot": {"vBCIBSCBS", "gIBS", "gCBS", "gMono", "gEstornoCred"},

	// gIBS dentro de IBSCBSTot
	"IBSCBSTot:gIBS": {"gIBSUF", "gIBSMun", "vIBS", "vCredPres", "vCredPresCondSus"},
	"gIBS:gIBSUF":    {"vDif", "vDevTrib", "vIBSUF"},
	"gIBS:gIBSMun":   {"vDif", "vDevTrib", "vIBSMun"},

	// gCBS dentro de IBSCBSTot
	"IBSCBSTot:gCBS": {"vDif", "vDevTrib", "vCBS", "vCredPres", "vCredPresCondSus"},

	"ICMSTot": {
		"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet",
		"vProd", "vFrete", "vSeg", "vDesc", "vII", "vIPI", "vIPIDevol",
		"vPIS", "vCOFINS", "vOutro", "vNF", "vTotTrib",
	},

	"ISTot": {"vIS"},

	// transp
	"transp": {
		"modFrete", "transporta", "retTransp",
		"veicTransp", "reboque", "vagao", "balsa", "vol",
	},
	"transporta": {"CNPJ", "CPF", "xNome", "IE", "xEnder", "xMun", "UF"},
	"retTransp":  {"vServ", "vBCRet", "pICMSRet", "vICMSRet", "CFOP", "cMunFG"},
	"veicTransp": {"placa", "UF", "RNTC"},
	"reboque":    {"placa", "UF", "RNTC"},
	"vol":        {"qVol", "esp", "marca", "nVol", "pesoL", "pesoB", "lacres"},

	// cobr
	"cobr": {"fat", "dup"},
	"fat":  {"nFat", "vOrig", "vDesc", "vLiq"},
	"dup":  {"nDup", "dVenc", "vDup"},

	// pag — new XSD structure for detPag
	"pag":    {"detPag", "vTroco"},
	"detPag": {"indPag", "tPag", "xPag", "vPag", "dPag", "CNPJPag", "UFPag", "card"},
	"card":   {"tpIntegra", "CNPJ", "tBand", "cAut", "CNPJReceb", "idTermPag"},

	// infAdic — NF-e (infCpl, obsCont, obsFisco, procRef are NF-e specific;
	//           MDF-e uses only infAdFisco+infCpl — see "infMDFe:infAdic")
	"infAdic":  {"infAdFisco", "infCpl", "obsCont", "obsFisco", "procRef"},
	"infObs":   {"xCampo", "xTexto"},
	"obsCont":  {"xTexto"},
	"obsFisco": {"xTexto"},
	"procRef":  {"nProc", "indProc", "tpAto"},

	// exporta
	"exporta": {"UFSaidaPais", "xLocExporta", "xLocDespacho"},

	// compra
	"compra": {"xNEmp", "xPed", "xCont"},

	// cana
	"cana":   {"safra", "ref", "forDia", "qTotMes", "qTotAnt", "qTotGer", "deduc", "vFor", "vTotDed", "vLiqFor"},
	"forDia": {"qtde"},
	"deduc":  {"xDed", "vDed"},

	// agropecuario
	"agropecuario": {"defensivo", "guiaTransito"},
	"defensivo":    {"nReceituario", "CPFRespTec"},
	"guiaTransito": {"tpGuia", "UFGuia", "serieGuia", "nGuia"},

	// infIntermed
	"infIntermed": {"CNPJ", "idCadIntTran"},

	// ── NFC-e suplemento (QR Code) ────────────────────────────────────────────
	"infNFeSupl": {"qrCode", "urlChave"},
}

// ─────────────────────────────────────────────────────────────────────────────
// NF-e — Eventos
// ─────────────────────────────────────────────────────────────────────────────
var envEventoNfe = map[string][]string{

	"envEvento": {"idLote", "evento"},
	"evento":    {"infEvento", "Signature"},

	// infEvento — NF-e (chave de acesso = chNFe)
	"evento:infEvento": {
		"cOrgao", "tpAmb", "CNPJ", "CPF",
		"chNFe", "dhEvento", "tpEvento", "nSeqEvento", "verEvento",
		"detEvento",
	},

	// detEvento — superset de todos os tipos de evento NF-e:
	//   110110 CC-e:          descEvento, xCorrecao, xCondUso
	//   110111 Canc:          descEvento, nProt, xJust
	//   110112 CancSubst:     descEvento, cOrgaoAutor, tpAutor, verAplic, nProt, xJust, chNFeRef
	//   110140 EPEC:          descEvento, cOrgaoAutor, tpAutor, verAplic, dhEmi, tpNF, IE, dest, vNF, vICMS
	//   111500/1 Prorrog:     descEvento, nProt, itemPedido
	//   111502/3 CancProrrog: descEvento, idPedidoCancelado, nProt
	//   110001 CancEvento:    descEvento, cOrgaoAutor, verAplic, tpEventoAut, nProtEvento
	//   210200/10 Manifest:   descEvento
	//   210220 Desconhec:     descEvento, xJust
	//   210240 Op.n.Real:     descEvento, xJust
	"detEvento:descEvento": {}, // leaf — sem filhos próprios
	"detEvento": {
		"descEvento",
		"xCorrecao", "xCondUso", // CC-e (110110)
		"cOrgaoAutor", "tpAutor", "verAplic", // cancel-subst (110112), EPEC (110140), cancel-evento (110001)
		"tpEventoAut", "nProtEvento", // cancel-evento (110001)
		"dhEmi", "tpNF", "IE", "dest", // EPEC (110140)
		"vNF", "vICMS", // EPEC (110140) — filhos diretos de detEvento
		"idPedidoCancelado", // cancel-prorrogação (111502/111503)
		"nProt",             // cancel, prorrogação, cancel-prorrogação
		"xJust",             // cancel, desconhecimento, op. não realizada
		"chNFeRef",          // cancel-subst (110112)
		"itemPedido",        // prorrogação (111500/111501)
	},

	// EPEC (110140) — dest dentro de detEvento
	"detEvento:dest": {"UF", "CNPJ", "CPF", "idEstrangeiro", "IE", "vNF", "vICMS", "vST"},

	// Prorrogação (111500/111501) — item do pedido
	"itemPedido": {"qtdeItem"},
}

// ─────────────────────────────────────────────────────────────────────────────
// NF-e — Serviços: status, consulta cadastro, consulta situação, inutilização,
//
//	distribuição DFe
//
// ─────────────────────────────────────────────────────────────────────────────
var servicosNfe = map[string][]string{

	// STATUS (NF-e/NFC-e)
	"consStatServ": {"tpAmb", "cUF", "xServ"},

	// Consulta cadastro (NF-e)
	"ConsCad":         {"infCons"},
	"ConsCad:infCons": {"xServ", "UF", "CNPJ", "CPF", "IE"},

	// Consulta situação NF-e
	"consSitNFe": {"tpAmb", "xServ", "chNFe"},

	// Inutilização
	"inutNFe": {"infInut", "Signature"},
	"inutNFe:infInut": {
		"tpAmb", "xServ", "cUF", "ano",
		"CNPJ", "mod", "serie", "nNFIni", "nNFFin", "xJust",
	},

	// Distribuição DFe (NF-e)
	"distDFeInt": {"tpAmb", "cUFAutor", "CNPJ", "CPF", "distNSU", "consNSU", "consChNFe"},
	"distNSU":    {"ultNSU"},
	"consNSU":    {"NSU"},
	"consChNFe":  {"chNFe"},
}

// ─────────────────────────────────────────────────────────────────────────────
// CT-e  (mod 57)
// ─────────────────────────────────────────────────────────────────────────────
var envicte = map[string][]string{

	"enviCTe": {"idLote", "CTe"},
	"CTe":     {"infCte", "infCteSupl"},

	"infCte": {
		"ide", "compl", "emit", "rem", "exped", "receb", "dest", "vPrest",
		"imp", "infCteNorm", "infCteSub", "infCteComp", "autXML",
		"infRespTec", "infSolicNFF",
	},

	// ide — CT-e
	"infCte:ide": {
		"cUF", "cCT", "CFOP", "natOp", "mod", "serie", "nCT",
		"dhEmi", "tpImp", "tpEmis", "cDV", "tpAmb", "tpCTe",
		"procEmi", "verProc", "indGlobalizado",
		"cMunEnv", "xMunEnv", "UFEnv",
		"modal", "tpServ", "dhIniViagem", "indIEToma",
		"indCarga", "toma3", "toma4",
		"dhCont", "xJust",
		"UFIni", "cMunIni", "xMunIni", "UFFim", "cMunFim", "xMunFim",
	},

	// emit — CT-e: IE vem antes de xNome (diferente da NF-e)
	"infCte:emit": {"CNPJ", "CPF", "IE", "IEST", "xNome", "xFant", "enderEmit", "CRT"},

	// rem / exped / receb / dest — CT-e (ordem comum)
	"rem":         {"CNPJ", "CPF", "IE", "xNome", "xFant", "fone", "enderReme"},
	"exped":       {"CNPJ", "CPF", "IE", "xNome", "xFant", "fone", "enderExped"},
	"receb":       {"CNPJ", "CPF", "IE", "xNome", "xFant", "fone", "enderReceb"},
	"infCte:dest": {"CNPJ", "CPF", "IE", "xNome", "xFant", "fone", "enderDest"},

	// compl
	"compl":    {"xCaracAd", "xCaracSer", "xEmi", "xObs", "ObsCont", "ObsFisco"},
	"ObsCont":  {"xTexto"},
	"ObsFisco": {"xTexto"},

	// vPrest
	"vPrest": {"vTPrest", "vRec", "Comp"},
	"Comp":   {"xNome", "vComp"},

	// imp — CT-e
	"imp": {"ICMS", "ICMSUFFim", "ICMSUFIni", "vTotTrib", "infAdFisco", "ICMSGSub", "IBS", "CBS", "IS"},
}

// ─────────────────────────────────────────────────────────────────────────────
// CT-e — Serviços: status, consulta situação, eventos
// ─────────────────────────────────────────────────────────────────────────────
var servicosCte = map[string][]string{

	// Status CT-e
	"consStatServCTe": {"tpAmb", "cUF", "xServ"},

	// Consulta CT-e
	"consSitCTe": {"tpAmb", "xServ", "chCTe"},

	// Eventos CT-e
	"eventoCTe": {"infEvento", "Signature"},
	"eventoCTe:infEvento": {
		"cOrgao", "tpAmb", "CNPJ", "CPF",
		"chCTe", "dhEvento", "tpEvento", "nSeqEvento",
		"detEvento",
	},

	// Cancelamento CT-e (evCancCTe)
	"evCancCTe": {"descEvento", "nProt", "xJust"},
}

// ─────────────────────────────────────────────────────────────────────────────
// MDF-e  (mod 58)
// ─────────────────────────────────────────────────────────────────────────────
var envimdfe = map[string][]string{

	"enviMDFe": {"idLote", "MDFe"},
	"MDFe":     {"infMDFe", "infMDFeSupl"},

	"infMDFe": {
		"ide", "emit", "infModal", "infDoc",
		"seg", "prodPred", "tot", "lacres", "autXML",
		"infAdic", "infRespTec", "infSolicNFF", "infPAA",
	},

	// ide — MDF-e (infMunCarrega e infPercurso inseridos entre UFFim e dhIniViagem)
	"infMDFe:ide": {
		"cUF", "tpAmb", "tpEmit", "tpTransp", "mod", "serie", "nMDF",
		"cMDF", "cDV", "modal", "dhEmi", "tpEmis",
		"procEmi", "verProc", "UFIni", "UFFim",
		"infMunCarrega", "infPercurso",
		"dhIniViagem", "indCanalVerde", "indCarregaPosterior",
	},

	// emit — MDF-e
	"infMDFe:emit":           {"CNPJ", "CPF", "IE", "xNome", "xFant", "enderEmit"},
	"infMDFe:emit:enderEmit": {"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "CEP", "UF", "fone", "email"},

	// infDoc — infMunCarrega foi para ide; infDoc agora contém apenas infMunDescarga
	"infDoc":         {"infMunDescarga"},
	"infMunCarrega":  {"cMunCarrega", "xMunCarrega"},
	"infMunDescarga": {"cMunDescarga", "xMunDescarga", "infCTe", "infNFe", "infMDFeTransp"},
	"infPercurso":    {"UFPer"},

	// infMunDescarga items com contexto específico
	"infMunDescarga:infCTe": {
		"chCTe", "SegCodBarra", "indReentrega", "infUnidTransp", "peri",
		"infEntregaParcial", "indPrestacaoParcial", "infNFePrestParcial",
	},
	"infEntregaParcial":     {"qtdTotal", "qtdParcial"},
	"infNFePrestParcial":    {"chNFe"},
	"infMunDescarga:infNFe": {"chNFe", "SegCodBarra", "indReentrega", "infUnidTransp", "peri"},
	"infMDFeTransp":         {"chMDFe", "indReentrega", "infUnidTransp", "peri"},
	"infUnidTransp":         {"tpUnidTransp", "idUnidTransp", "lacUnidTransp", "infUnidCarga", "qtdRat"},
	"infUnidCarga":          {"tpUnidCarga", "idUnidCarga", "lacUnidCarga", "qtdRat"},
	"lacUnidTransp":         {"nLacre"},
	"lacUnidCarga":          {"nLacre"},

	// peri
	"peri": {"nONU", "xNomeAE", "xClaRisco", "grEmb", "qTotProd", "qVolTipo"},

	// seg
	"seg":         {"infResp", "infSeg", "nApol", "nAver"},
	"seg:infResp": {"respSeg", "CNPJ", "CPF"},
	"infSeg":      {"xSeg", "CNPJ"},

	// prodPred
	"prodPred":           {"tpCarga", "xProd", "cEAN", "NCM", "infLotacao"},
	"infLotacao":         {"infLocalCarrega", "infLocalDescarrega"},
	"infLocalCarrega":    {"CEP", "latitude", "longitude"},
	"infLocalDescarrega": {"CEP", "latitude", "longitude"},

	// tot
	"infMDFe:tot": {"qCTe", "qNFe", "qMDFe", "vCarga", "cUnid", "qCarga"},

	// infPAA
	"infPAA":       {"CNPJPAA", "PAASignature"},
	"PAASignature": {"SignatureValue", "RSAKeyValue"},

	// infAdic — MDF-e usa apenas infAdFisco + infCpl
	// (NF-e usa "infAdic" genérico com infAdFisco, infCpl, obsCont, obsFisco, procRef)
	"infMDFe:infAdic": {"infAdFisco", "infCpl"},

	// ── Modal rodoviário (mdfeModalRodoviario_v3.00) ──────────────────────────
	"infModal":       {"rodo", "aereo", "aquav", "ferrov"},
	"rodo":           {"infANTT", "veicTracao", "veicReboque", "codAgPorto", "lacRodo"},
	"infANTT":        {"RNTRC", "infCIOT", "valePed", "infContratante", "infPag"},
	"infCIOT":        {"CIOT", "CPF", "CNPJ"},
	"valePed":        {"disp", "categCombVeic"},
	"disp":           {"CNPJForn", "CNPJPg", "CPFPg", "nCompra", "vValePed", "tpValePed"},
	"infContratante": {"xNome", "CPF", "CNPJ", "idEstrangeiro", "infContrato"},
	"infContrato":    {"NroContrato", "vContratoGlobal"},
	"veicTracao":     {"cInt", "placa", "RENAVAM", "tara", "capKG", "capM3", "prop", "condutor", "tpRod", "tpCar", "UF"},
	"veicReboque":    {"cInt", "placa", "RENAVAM", "tara", "capKG", "capM3", "prop", "tpCar", "UF"},
	"prop":           {"CPF", "CNPJ", "RNTRC", "xNome", "IE", "UF", "tpProp"},
	"lacRodo":        {"nLacre"},

	// ── Modal aéreo (mdfeModalAereo_v3.00) ────────────────────────────────────
	"aereo": {"nac", "matr", "nVoo", "cAerEmb", "cAerDes", "dVoo"},

	// ── Modal aquaviário (mdfeModalAquaviario_v3.00) ──────────────────────────
	"aquav": {
		"irin", "tpEmb", "cEmbar", "xEmbar", "nViag", "cPrtEmb", "cPrtDest",
		"prtTrans", "tpNav", "infTermCarreg", "infTermDescarreg", "infEmbComb",
		"infUnidCargaVazia", "infUnidTranspVazia", "MMSI",
	},
	"infTermCarreg":      {"cTermCarreg", "xTermCarreg"},
	"infTermDescarreg":   {"cTermDescarreg", "xTermDescarreg"},
	"infEmbComb":         {"cEmbComb", "xBalsa"},
	"infUnidCargaVazia":  {"idUnidCargaVazia", "tpUnidCargaVazia"},
	"infUnidTranspVazia": {"idUnidTranspVazia", "tpUnidTranspVazia"},

	// ── Modal ferroviário (mdfeModalFerroviario_v3.00) ────────────────────────
	"ferrov": {"trem", "vag"},
	"trem":   {"xPref", "dhTrem", "xOri", "xDest", "qVag"},
	"vag":    {"pesoBC", "pesoR", "tpVag", "serie", "nVag", "nSeq", "TU"},
}

// ─────────────────────────────────────────────────────────────────────────────
// MDF-e — Serviços: status, consulta situação, não-encerrados, eventos
// ─────────────────────────────────────────────────────────────────────────────
var servicosMdfe = map[string][]string{

	// Status MDF-e
	"consStatServMDFe": {"tpAmb", "xServ"},

	// Consulta MDF-e
	"consSitMDFe":    {"tpAmb", "xServ", "chMDFe"},
	"consNaoEncMDFe": {"tpAmb", "xServ", "CNPJ", "CPF"},

	// Eventos MDF-e
	"eventoMDFe": {"infEvento", "Signature"},
	"eventoMDFe:infEvento": {
		"cOrgao", "tpAmb", "CNPJ", "CPF",
		"chMDFe", "dhEvento", "tpEvento", "nSeqEvento",
		"detEvento", "infSolicNFF", "infPAA",
	},

	// Encerramento MDF-e (evEncMDFe — 110112)
	"evEncMDFe": {"descEvento", "nProt", "dtEnc", "cUF", "cMun", "indEncPorTerceiro"},

	// Cancelamento MDF-e (evCancMDFe — 110111)
	"evCancMDFe": {"descEvento", "nProt", "xJust"},

	// Inclusão de condutor MDF-e (evIncCondutorMDFe — 110114)
	"evIncCondutorMDFe": {"descEvento", "condutor"},
	"condutor":          {"xNome", "CPF"},

	// Confirmação de serviço de transporte MDF-e (evConfirmaServMDFe — 110117)
	"evConfirmaServMDFe": {"descEvento", "nProt"},

	// Inclusão de DF-e MDF-e (evIncDFeMDFe — 110115)
	"evIncDFeMDFe":        {"descEvento", "nProt", "cMunCarrega", "xMunCarrega", "infDoc"},
	"evIncDFeMDFe:infDoc": {"cMunDescarga", "xMunDescarga", "chNFe"},

	// Pagamento da operação de transporte MDF-e (evPagtoOperMDFe — 110116)
	"evPagtoOperMDFe": {"descEvento", "nProt", "infViagens", "infPag"},
	"infViagens":      {"qtdViagens", "nroViagem"},

	// Alteração de pagamento de serviço MDF-e (evAlteracaoPagtoServMDFe — 110118)
	"evAlteracaoPagtoServMDFe": {"descEvento", "nProt", "infPag"},

	// infPag — superset modal rodoviário + eventos de pagamento MDF-e
	// (indAltoDesemp presente apenas no modal rodoviário, ignorado nas ausências)
	"infPag": {
		"xNome", "CPF", "CNPJ", "idEstrangeiro",
		"Comp",
		"vContrato", "indAltoDesemp",
		"indPag", "vAdiant", "indAntecipaAdiant",
		"infPrazo", "tpAntecip", "infBanc",
	},
	"infPag:Comp": {"tpComp", "vComp", "xComp"},
	"infPrazo":    {"nParcela", "dVenc", "vParcela"},
	"infBanc":     {"codBanco", "codAgencia", "CNPJIPEF", "PIX"},
}

// ─────────────────────────────────────────────────────────────────────────────
// Compartilhado — elementos com ordenação idêntica em todos os doc types
// ─────────────────────────────────────────────────────────────────────────────
var compartilhado = map[string][]string{

	// Endereços (ordem idêntica em todos os doc types)
	"enderEmit": {"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "UF", "CEP", "cPais", "xPais", "fone"},
	"enderDest": {"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "UF", "CEP", "cPais", "xPais", "fone"},

	// infRespTec — compartilhado NF-e / CT-e / MDF-e
	"infRespTec": {"CNPJ", "xContato", "email", "fone", "idCSRT", "hashCSRT"},

	// infSolicNFF
	"infSolicNFF": {"xSolic"},

	// infNFeSupl (NFC-e QR Code)
	"infNFeSupl": {"qrCode", "urlChave"},
}

// =============================================================================
// Table — merge of all private maps above.
// Later maps win on key collision (none expected, but explicit order gives
// precedence to more-specific sections), mirroring the Python
// `{**a, **b, **c}` merge order in xsd_order.py.
// =============================================================================
var Table = buildTable()

func buildTable() map[string][]string {
	t := make(map[string][]string)
	for _, src := range []map[string][]string{
		compartilhado,
		servicosNfe,
		envEventoNfe,
		envInfe,
		envicte,
		servicosCte,
		envimdfe,
		servicosMdfe,
	} {
		for k, v := range src {
			t[k] = v
		}
	}
	return t
}

// Lookup returns the XSD-mandated child element order registered directly
// under key (either a plain tag, e.g. "ide", or an ancestor-scoped key, e.g.
// "infNFe:ide"). ok is false when key has no entry — mirrors Python's
// XSD_ORDER.get(key).
func Lookup(key string) (order []string, ok bool) {
	order, ok = Table[key]
	return order, ok
}

// Resolve returns the XSD-mandated child element order for tag given its
// ancestor path (colon-joined, outermost first, e.g. "infMDFe:emit" when
// resolving the children of infMDFe/emit). It mirrors py-dfe's
// xmlops/builder.py _build_element lookup exactly: try the most-specific
// ancestor-scoped key first ("<parentTag>:tag"), narrowing the path one
// ancestor at a time, then fall back to the plain "tag". Pass "" for
// parentTag when tag has no ancestor (i.e. it is the document root).
func Resolve(parentTag string, tag string) (order []string, ok bool) {
	if parentTag != "" {
		parts := splitPath(parentTag)
		for i := range parts {
			key := joinPath(parts[i:]) + ":" + tag
			if order, ok = Table[key]; ok {
				return order, ok
			}
		}
	}
	order, ok = Table[tag]
	return order, ok
}

func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == ':' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ":" + p
	}
	return out
}
