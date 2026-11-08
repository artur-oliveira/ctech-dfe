package ctes

import (
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/shopspring/decimal"
)

const (
	cteVersao = "4.00"
	cteXMLNS  = "http://www.portalfiscal.inf.br/cte"

	// Valores fixos do escopo da spec (CT-e normal, serviço normal, rodoviário).
	tpCTeNormal     = "0"
	tpCTeComplement = "1"
	tpCTeSubstitute = "3"
	tpServNormal    = "0"
	modalRodoviario = "01"
	tpImpRetrato    = "1"
	procEmiOwn      = "0" // aplicativo do contribuinte
	// retiraNao: o recebedor não retira no terminal (ide/retira: 0 sim, 1 não).
	retiraNao = "1"
	// natOpTransporte é a descrição da natureza da operação (texto livre do leiaute; o fiscal é o CFOP).
	natOpTransporte = "PRESTACAO DE SERVICO DE TRANSPORTE"

	// ide/indIEToma: 1 contribuinte, 2 isento de inscrição, 9 não contribuinte.
	indIETomaContrib  = "1"
	indIETomaIsento   = "2"
	indIETomaNaoContr = "9"
	ieIsento          = "ISENTO"

	// toma3/toma: 0 remetente, 1 expedidor, 2 recebedor, 3 destinatário; toma4/toma: 4 outros.
	toma3Remetente    = "0"
	toma3Expedidor    = "1"
	toma3Recebedor    = "2"
	toma3Destinatario = "3"
	toma4Outros       = "4"

	// infCteSub/indAlteraToma: única opção do leiaute (1 = troca de tomador).
	indAlteraTomaSim = "1"

	// compFreteValor é o nome do componente principal do frete (vPrest/Comp/xNome).
	compFreteValor = "FRETE VALOR"

	// infCarga/infQ: cUnid 01 KG, 03 UNIDADE.
	cUnidKG      = "01"
	cUnidUnidade = "03"
	tpMedPeso    = "PESO BRUTO"
	tpMedVolumes = "VOLUMES"

	// xNome obrigatório de remetente, expedidor, recebedor e destinatário em homologação (MOC G002 a G005).
	homologXNome = "CTE EMITIDO EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"

	// cnpjLen distingue CNPJ de CPF.
	cnpjLen = 14
)

// qrCodeBaseByUF e qrCodeBaseHomByUF: URL de consulta por QR Code do CT-e por autorizador, conferida na relação
// de serviços do Portal SVRS do CT-e (https://dfe-portal.svrs.rs.gov.br/CTe/Servicos, linha "QR Code") em 2026-11-06.
// As UFs que não aparecem aqui autorizam na SVRS (mesma divisão de go-dfe/internal/endpoints cteUFAuth).
// Quando a homologação não tem endereço próprio, vale o de produção.
var qrCodeBaseByUF = map[string]string{
	"MG": "https://portalcte.fazenda.mg.gov.br/portalcte/sistema/qrcode.xhtml",
	"MS": "http://www.dfe.ms.gov.br/cte/qrcode",
	"MT": "https://www.sefaz.mt.gov.br/cte/qrcode",
	"PR": "http://www.fazenda.pr.gov.br/cte/qrcode",
	"SP": "https://nfe.fazenda.sp.gov.br/CTeConsulta/qrCode",
}

var qrCodeBaseHomByUF = map[string]string{
	"MT": "https://homologacao.sefaz.mt.gov.br/cte/qrcode",
	"SP": "https://homologacao.nfe.fazenda.sp.gov.br/CTeConsulta/qrCode",
}

const qrCodeBaseSVRS = "https://dfe-portal.svrs.rs.gov.br/cte/qrCode"

// actor é um participante do CT-e (emitente, remetente, destinatário, expedidor, recebedor ou tomador).
type actor struct {
	CNPJ, CPF, IE, Name, Fantasy, Phone, Email string
	Addr                                       nfexml.Address
}

func (a actor) doc() string {
	if a.CNPJ != "" {
		return a.CNPJ
	}
	return a.CPF
}

// buildParams carrega tudo o que BuildCTe precisa, já resolvido e validado.
type buildParams struct {
	accessKey   string
	serie       int
	number      int
	environment int
	now         time.Time
	tpEmis      string
	cfop        string
	// dhCont e xJust só no off-line (tpEmis 2); na SVC (7/8) o MOC proíbe (G008, 586).
	contingencyAt time.Time
	justification string
	// qrSign é o parâmetro sign do QR Code (off-line; G215): RSA SHA-1 Base64 da chave, assinada pelo egress.
	qrSign string

	emit actor
	crt  int

	rem, dest    actor
	exped, receb *actor
	// takerRole é um dos Role*; toma4 só para RoleOutro.
	takerRole string
	toma4     *actor

	origin, destination Place

	// vPrest já resolvido: total e Comp (o primeiro é FRETE VALOR).
	vTPrest decimal.Decimal
	comps   []map[string]any
	// icms é o grupo imp/ICMS já calculado (resolveICMS).
	icms map[string]any
	// ibscbs é o grupo imp/IBSCBS (nil: não informado); vTotDFe sai junto (regra IBSCBS-360).
	ibscbs *ibscbsResult
	// autXML são os autorizados a baixar o XML, já validados.
	autXML []map[string]any

	vCarga  decimal.Decimal
	proPred string
	weight  decimal.Decimal
	volumes int64
	nfeKeys []string

	rntrc   string
	addInfo string

	// tpCTe vazio é o CT-e normal. Complemento (1): complementKeys leva os infCteComp e não há infCTeNorm.
	// Substituto (3): sub leva o infCteSub dentro do infCTeNorm.
	tpCTe          string
	complementKeys []string
	sub            *subGroup
	// serviceType é o ide/tpServ; vazio é o serviço normal. Complemento e substituto herdam o do original (G176, G095).
	serviceType string
	// os não é nil no CT-e OS (modelo 67): BuildCTeOS, osRecord e o serviço CTeRecepcaoOS no lugar dos do CT-e 57.
	os *osData

	tech         TechData
	csrtID, csrt string
}

// subGroup é o infCteSub: chave do CT-e substituído e, na troca de tomador, indAlteraToma.
type subGroup struct {
	chCte          string
	alterTakerFlag bool
}

// BuildCTe monta o dicionário do CT-e para o CTeRecepcaoSinc. Na autorização síncrona do CT-e 4.00 o
// <CTe> é a raiz (sem enviCTe), como no MDF-e; a ordem dos filhos vem da tabela xsdorder do go-dfe.
func BuildCTe(p buildParams) map[string]any {
	if p.os != nil {
		return BuildCTeOS(p)
	}
	cUF, cCT, cDV := p.accessKey[0:2], p.accessKey[35:43], string(p.accessKey[43])
	homolog := p.environment != 1

	inf := map[string]any{
		"@versao": cteVersao,
		"@Id":     "CTe" + p.accessKey,
		"ide":     p.buildIde(cUF, cCT, cDV),
		"emit":    p.buildEmit(),
		"rem":     actorNode(p.rem, "enderReme", homolog),
		"dest":    actorNode(p.dest, "enderDest", homolog),
		"vPrest": map[string]any{
			"vTPrest": q2(p.vTPrest),
			"vRec":    q2(p.vTPrest),
			"Comp":    p.comps,
		},
		"imp": p.buildImp(),
	}
	if p.tpCTe == tpCTeComplement {
		comp := make([]map[string]any, 0, len(p.complementKeys))
		for _, k := range p.complementKeys {
			comp = append(comp, map[string]any{"chCTe": k})
		}
		inf["infCteComp"] = comp
	} else {
		inf["infCTeNorm"] = p.buildInfCTeNorm()
	}
	if p.exped != nil {
		inf["exped"] = actorNode(*p.exped, "enderExped", homolog)
	}
	if p.receb != nil {
		inf["receb"] = actorNode(*p.receb, "enderReceb", homolog)
	}
	if p.addInfo != "" {
		inf["compl"] = map[string]any{"xObs": p.addInfo}
	}
	if len(p.autXML) > 0 {
		inf["autXML"] = p.autXML
	}
	if p.tech.CNPJ != "" {
		inf["infRespTec"] = services.BuildRespTec(p.tech.CNPJ, p.tech.Name, p.tech.Email, onlyDigits(p.tech.Phone), p.csrtID, p.csrt, p.accessKey)
	}

	return services.PruneEmpty(map[string]any{
		"CTe": map[string]any{
			"@xmlns":     cteXMLNS,
			"infCte":     inf,
			"infCTeSupl": map[string]any{"qrCodCTe": qrCodeURL(p.emit.Addr.UF, p.accessKey, p.environment, p.qrSign)},
		},
	}).(map[string]any)
}

// buildImp é o grupo imp: ICMS e, quando informado, IBSCBS com o vTotDFe.
func (p buildParams) buildImp() map[string]any {
	imp := map[string]any{"ICMS": p.icms}
	if p.ibscbs != nil {
		imp["IBSCBS"] = p.ibscbs.node
		imp["vTotDFe"] = p.ibscbs.totalDFe(p.vTPrest, p.now.Year())
	}
	return imp
}

func (p buildParams) buildIde(cUF, cCT, cDV string) map[string]any {
	ide := map[string]any{
		"cUF":     cUF,
		"cCT":     cCT,
		"CFOP":    p.cfop,
		"natOp":   natOpTransporte,
		"mod":     services.ModelCTe,
		"serie":   fmt.Sprintf("%d", p.serie),
		"nCT":     fmt.Sprintf("%d", p.number),
		"dhEmi":   p.now.Format(layoutDateTimeTZ),
		"tpImp":   tpImpRetrato,
		"tpEmis":  p.tpEmis,
		"cDV":     cDV,
		"tpAmb":   fmt.Sprintf("%d", p.environment),
		"tpCTe":   strOr(p.tpCTe, tpCTeNormal),
		"procEmi": procEmiOwn,
		"verProc": verProc(p.tech),
		"cMunEnv": p.emit.Addr.CityIBGE,
		"xMunEnv": p.emit.Addr.City,
		"UFEnv":   p.emit.Addr.UF,
		"modal":   modalRodoviario,
		"tpServ":  strOr(p.serviceType, tpServNormal),
		"cMunIni": p.origin.CityIBGE,
		"xMunIni": p.origin.City,
		"UFIni":   p.origin.UF,
		"cMunFim": p.destination.CityIBGE,
		"xMunFim": p.destination.City,
		"UFFim":   p.destination.UF,
		"retira":  retiraNao,
	}
	if p.tpEmis == tpEmisOffline {
		ide["dhCont"] = p.contingencyAt.In(p.now.Location()).Format(layoutDateTimeTZ)
		ide["xJust"] = p.justification
	}
	taker := p.takerActor()
	ide["indIEToma"] = indIEToma(taker.IE)
	if p.takerRole == RoleOutro {
		t := map[string]any{"toma": toma4Outros, "IE": ieForXML(taker.IE), "xNome": taker.Name, "fone": onlyDigits(taker.Phone),
			"enderToma": enderNode(taker.Addr, false), "email": taker.Email}
		setDoc(t, taker)
		ide["toma4"] = t
	} else {
		ide["toma3"] = map[string]any{"toma": toma3Code[p.takerRole]}
	}
	return ide
}

var toma3Code = map[string]string{
	RoleRemetente: toma3Remetente, RoleExpedidor: toma3Expedidor,
	RoleRecebedor: toma3Recebedor, RoleDestinatario: toma3Destinatario,
}

// takerActor devolve o ator que paga o frete.
func (p buildParams) takerActor() actor {
	switch p.takerRole {
	case RoleDestinatario:
		return p.dest
	case RoleExpedidor:
		return *p.exped
	case RoleRecebedor:
		return *p.receb
	case RoleOutro:
		return *p.toma4
	}
	return p.rem
}

func (p buildParams) buildEmit() map[string]any {
	ender := enderNode(p.emit.Addr, false)
	ender["fone"] = onlyDigits(p.emit.Phone)
	e := map[string]any{"IE": onlyDigits(p.emit.IE), "xNome": p.emit.Name, "xFant": p.emit.Fantasy, "enderEmit": ender, "CRT": fmt.Sprintf("%d", p.crt)}
	setDoc(e, p.emit)
	return e
}

func (p buildParams) buildInfCTeNorm() map[string]any {
	infQ := []map[string]any{{"cUnid": cUnidKG, "tpMed": tpMedPeso, "qCarga": p.weight.StringFixed(4)}}
	if p.volumes > 0 {
		infQ = append(infQ, map[string]any{"cUnid": cUnidUnidade, "tpMed": tpMedVolumes, "qCarga": decimal.NewFromInt(p.volumes).StringFixed(4)})
	}
	nfes := make([]map[string]any, 0, len(p.nfeKeys))
	for _, k := range p.nfeKeys {
		nfes = append(nfes, map[string]any{"chave": k})
	}
	norm := map[string]any{
		"infCarga": map[string]any{"vCarga": q2(p.vCarga), "proPred": p.proPred, "infQ": infQ},
		"infDoc":   map[string]any{"infNFe": nfes},
		"infModal": map[string]any{"@versaoModal": cteVersao, "rodo": map[string]any{"RNTRC": p.rntrc}},
	}
	if p.sub != nil {
		g := map[string]any{"chCte": p.sub.chCte}
		if p.sub.alterTakerFlag {
			g["indAlteraToma"] = indAlteraTomaSim
		}
		norm["infCteSub"] = g
	}
	return norm
}

// actorNode monta rem/exped/receb/dest (choice CNPJ|CPF, IE, xNome, fone, ender*, email).
func actorNode(a actor, enderTag string, homolog bool) map[string]any {
	name := a.Name
	if homolog {
		name = homologXNome
	}
	n := map[string]any{"IE": ieForXML(a.IE), "xNome": name, "fone": onlyDigits(a.Phone), enderTag: enderNode(a.Addr, true), "email": a.Email}
	setDoc(n, a)
	return n
}

// enderNode monta TEndereco (ou TEndeEmi sem país). Logradouro sem número vira "S/N", como na NF-e.
func enderNode(a nfexml.Address, withCountry bool) map[string]any {
	e := map[string]any{
		"xLgr": a.Street, "nro": strOr(a.Number, "S/N"), "xCpl": a.Complement, "xBairro": a.District,
		"cMun": a.CityIBGE, "xMun": a.City, "CEP": onlyDigits(a.CEP), "UF": a.UF,
	}
	if withCountry {
		e["cPais"], e["xPais"] = a.CountryCode, a.Country
	}
	return e
}

func setDoc(m map[string]any, a actor) {
	if a.CNPJ != "" {
		m["CNPJ"] = a.CNPJ
	} else {
		m["CPF"] = a.CPF
	}
}

// ieForXML: o leiaute aceita só dígitos ou a literal ISENTO.
func ieForXML(ie string) string {
	if strings.EqualFold(strings.TrimSpace(ie), ieIsento) {
		return ieIsento
	}
	return onlyDigits(ie)
}

func indIEToma(ie string) string {
	switch ieForXML(ie) {
	case "":
		return indIETomaNaoContr
	case ieIsento:
		return indIETomaIsento
	}
	return indIETomaContrib
}

// qrCodeURL builds URL?chCTe=...&tpAmb=... and, in off-line contingency, &sign=... (MOC CT-e, QR Code 2.3.2).
func qrCodeURL(uf, accessKey string, environment int, sign string) string {
	base, ok := qrCodeBaseByUF[uf]
	if !ok {
		base = qrCodeBaseSVRS
	}
	if hom, ok := qrCodeBaseHomByUF[uf]; ok && environment == 2 {
		base = hom
	}
	u := fmt.Sprintf("%s?chCTe=%s&tpAmb=%d", base, accessKey, environment)
	if sign != "" {
		u += "&sign=" + sign
	}
	return u
}

func verProc(t TechData) string {
	if t.Version != "" {
		return t.Version
	}
	return "ctech-dfe"
}

// q2 formata valor monetário com 2 casas (arredondamento bancário, como as NF-e).
func q2(v decimal.Decimal) string { return v.RoundBank(2).StringFixed(2) }

func strOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
