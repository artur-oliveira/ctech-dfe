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

	vCarga  decimal.Decimal
	proPred string
	weight  decimal.Decimal
	volumes int64
	nfeKeys []string

	rntrc   string
	addInfo string

	tech         TechData
	csrtID, csrt string
}

// BuildCTe monta o dicionário do CT-e para o CTeRecepcaoSinc. Na autorização síncrona do CT-e 4.00 o
// <CTe> é a raiz (sem enviCTe), como no MDF-e; a ordem dos filhos vem da tabela xsdorder do go-dfe.
func BuildCTe(p buildParams) map[string]any {
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
		"imp":        map[string]any{"ICMS": p.icms},
		"infCTeNorm": p.buildInfCTeNorm(),
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
	if p.tech.CNPJ != "" {
		inf["infRespTec"] = services.BuildRespTec(p.tech.CNPJ, p.tech.Name, p.tech.Email, onlyDigits(p.tech.Phone), p.csrtID, p.csrt, p.accessKey)
	}

	return services.PruneEmpty(map[string]any{
		"CTe": map[string]any{
			"@xmlns":     cteXMLNS,
			"infCte":     inf,
			"infCTeSupl": map[string]any{"qrCodCTe": qrCodeURL(p.emit.Addr.UF, p.accessKey, p.environment)},
		},
	}).(map[string]any)
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
		"tpCTe":   tpCTeNormal,
		"procEmi": procEmiOwn,
		"verProc": verProc(p.tech),
		"cMunEnv": p.emit.Addr.CityIBGE,
		"xMunEnv": p.emit.Addr.City,
		"UFEnv":   p.emit.Addr.UF,
		"modal":   modalRodoviario,
		"tpServ":  tpServNormal,
		"cMunIni": p.origin.CityIBGE,
		"xMunIni": p.origin.City,
		"UFIni":   p.origin.UF,
		"cMunFim": p.destination.CityIBGE,
		"xMunFim": p.destination.City,
		"UFFim":   p.destination.UF,
		"retira":  retiraNao,
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
	return map[string]any{
		"infCarga": map[string]any{"vCarga": q2(p.vCarga), "proPred": p.proPred, "infQ": infQ},
		"infDoc":   map[string]any{"infNFe": nfes},
		"infModal": map[string]any{"@versaoModal": cteVersao, "rodo": map[string]any{"RNTRC": p.rntrc}},
	}
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

func qrCodeURL(uf, accessKey string, environment int) string {
	base, ok := qrCodeBaseByUF[uf]
	if !ok {
		base = qrCodeBaseSVRS
	}
	if hom, ok := qrCodeBaseHomByUF[uf]; ok && environment == 2 {
		base = hom
	}
	return fmt.Sprintf("%s?chCTe=%s&tpAmb=%d", base, accessKey, environment)
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
