package nfes

// builders_ide.go — nó ide da NF-e (identificação do documento). Extraído de
// builders_doc.go: ide é o nó que mais cresce ao longo da cobertura de tags.

import (
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// ideParams agrupa o que o nó ide precisa. É struct e não 15 parâmetros
// posicionais porque ide é o nó que mais cresce neste plano.
type ideParams struct {
	CUF, CNF, NatOp, Model, AccessKey string
	Serie, Number, Environment        int
	DhEmi                             string
	TpNF, IdDest, CMunFG              string
	FinNFe, IndFinal, IndPres         string
	Mode                              EmissionMode
	VerProc                           string
	// DhSaiEnt e DPrevEntrega são a saída da mercadoria e a previsão de
	// entrega. Já resolvidas (offset da operação ou valor da nota) — o builder
	// só emite o que recebe.
	DhSaiEnt, DPrevEntrega string
	// IndIntermed marca a venda em plataforma de terceiros (marketplace).
	IndIntermed string
	// Campos da reforma tributária no ide. CIndOp é o código do local da
	// operação de fornecimento e CMunFGIBS o município do fato gerador do
	// IBS/CBS; TpNFDebito e TpNFCredito marcam nota de débito e de crédito.
	CIndOp, CMunFGIBS, TpNFDebito, TpNFCredito string
	// CompraGov é o grupo de compras governamentais, já validado.
	CompraGov map[string]any
	// PagAntecipado são as chaves das NF-e de antecipação de pagamento a abater.
	PagAntecipado []string
	// NFref são os documentos referenciados já resolvidos (ide/NFref).
	NFref []map[string]any
}

// buildIde monta o nó ide, incluindo o grupo de contingência quando a emissão
// não é normal.
func buildIde(p ideParams) map[string]any {
	ide := map[string]any{
		"cUF":      p.CUF,
		"cNF":      p.CNF,
		"natOp":    p.NatOp,
		"mod":      p.Model,
		"serie":    fmt.Sprintf("%d", p.Serie),
		"nNF":      fmt.Sprintf("%d", p.Number),
		"dhEmi":    p.DhEmi,
		"tpNF":     p.TpNF,
		"idDest":   p.IdDest,
		"cMunFG":   p.CMunFG,
		"tpImp":    p.Mode.TpImp,
		"tpEmis":   p.Mode.TpEmis,
		"cDV":      string(p.AccessKey[len(p.AccessKey)-1]),
		"tpAmb":    fmt.Sprintf("%d", p.Environment),
		"finNFe":   p.FinNFe,
		"indFinal": p.IndFinal,
		"indPres":  p.IndPres,
		"procEmi":  procEmiApp,
		"verProc":  p.VerProc,
	}
	// Ordem XSD: dhSaiEnt e dPrevEntrega vêm logo depois de dhEmi; indIntermed
	// depois de indPres. A tabela xsdorder ordena — aqui só a presença importa.
	if p.DhSaiEnt != "" {
		ide["dhSaiEnt"] = p.DhSaiEnt
	}
	if p.DPrevEntrega != "" {
		ide["dPrevEntrega"] = p.DPrevEntrega
	}
	if p.IndIntermed != "" {
		ide["indIntermed"] = p.IndIntermed
	}
	for tag, v := range map[string]string{
		"cIndOp":      p.CIndOp,
		"cMunFGIBS":   p.CMunFGIBS,
		"tpNFDebito":  p.TpNFDebito,
		"tpNFCredito": p.TpNFCredito,
	} {
		if v != "" {
			ide[tag] = v
		}
	}
	if len(p.CompraGov) > 0 {
		ide["gCompraGov"] = p.CompraGov
	}
	if len(p.PagAntecipado) > 0 {
		ide["gPagAntecipado"] = map[string]any{"refNFe": p.PagAntecipado}
	}
	// dhCont + xJust são exigidos "apenas para tpEmis diferente de 1" (XSD).
	if p.Mode.IsContingency() {
		ide["dhCont"] = fmtDhEmi(p.Mode.ContingencyAt)
		ide["xJust"] = p.Mode.Justification
	}
	if len(p.NFref) > 0 {
		ide["NFref"] = p.NFref
	}
	return ide
}

// ── dhSaiEnt, dPrevEntrega e infIntermed ─────────────────────────────────────

// opFieldDhSaiEntOffsetDays é o prazo padrão de saída da mercadoria, em dias
// corridos a partir da emissão. Quem despacha sempre no dia seguinte cadastra 1
// e nunca mais digita a data.
const opFieldDhSaiEntOffsetDays = "dh_sai_ent_offset_days"

// personIntermediaryIDField é o identificador do emitente no cadastro do
// intermediador (infIntermed/idCadIntTran) — o "seller id" do marketplace. É
// invariante do par emitente↔plataforma, então mora na pessoa.
const personIntermediaryIDField = "intermediary_id"

// resolveDhSaiEnt devolve a data-hora de saída da mercadoria: o valor explícito
// da nota vence o offset em dias cadastrado na operação. Sem os dois, devolve
// vazio — a maioria das notas não declara saída, e uma data inventada aqui
// mentiria sobre quando a mercadoria circulou.
func resolveDhSaiEnt(op map[string]any, explicit *string, now time.Time) string {
	if explicit != nil && *explicit != "" {
		return *explicit
	}
	days, ok := anyInt(op, opFieldDhSaiEntOffsetDays)
	if !ok {
		return ""
	}
	return fmtDhEmi(now.AddDate(0, 0, days))
}

// buildInfIntermed monta infNFe/infIntermed a partir da pessoa cadastrada como
// intermediador. Os dois campos são obrigatórios no XSD e o CNPJ é TCnpj — um
// intermediador pessoa física não existe no leiaute, então o grupo é omitido em
// vez de emitir algo que a SEFAZ rejeitaria.
func buildInfIntermed(person map[string]any) map[string]any {
	if person == nil {
		return nil
	}
	sk := anyStr(person, "sk", "")
	if !strings.HasPrefix(sk, cnpjPrefix) {
		return nil
	}
	cnpj := services.StripPKPrefix(sk)
	idCad := anyStr(person, personIntermediaryIDField, "")
	if cnpj == "" || idCad == "" {
		return nil
	}
	return map[string]any{"CNPJ": cnpj, "idCadIntTran": idCad}
}

// ── gCompraGov ───────────────────────────────────────────────────────────────

// Campos das compras governamentais no cadastro da operação. Quem vende para
// órgão público vende sempre para o mesmo tipo de ente, com o mesmo redutor e
// o mesmo tipo de operação — as chaves referenciadas são o que muda por nota.
const (
	opFieldCompraGovTpEnte   = "compra_gov_tp_ente"
	opFieldCompraGovPRedutor = "compra_gov_p_redutor"
	opFieldCompraGovTpOper   = "compra_gov_tp_oper"
)

// Tipos de operação com ente governamental (TOperCompraGov):
// 1 fornecimento com pagamento posterior, 2 recebimento do pagamento com
// fornecimento já realizado, 3 fornecimento com pagamento já realizado,
// 4 recebimento do pagamento com fornecimento posterior.
const (
	tpOperGovRecebimentoPosFornecimento = "2"
	tpOperGovFornecimentoPosPagamento   = "3"
)

// buildCompraGov monta ide/gCompraGov. Ordem XSD: tpEnteGov, pRedutor,
// tpOperGov, refDFeAnt.
//
// A regra do refDFeAnt é do próprio leiaute e é o tipo de erro que só aparece
// como rejeição: obrigatório em tpOperGov 2 e 3, vedado em 1 e 4, e no tipo 2
// aceita **uma** chave só.
func buildCompraGov(op map[string]any, refs []string) (map[string]any, error) {
	tpEnte := anyStr(op, opFieldCompraGovTpEnte, "")
	if tpEnte == "" {
		return nil, nil
	}
	tpOper := anyStr(op, opFieldCompraGovTpOper, "")
	if tpOper == "" {
		return nil, problem.BadRequest(
			"compra governamental sem tipo de operação: cadastre tpOperGov na natureza de operação")
	}
	needsRef := tpOper == tpOperGovRecebimentoPosFornecimento || tpOper == tpOperGovFornecimentoPosPagamento
	switch {
	case needsRef && len(refs) == 0:
		return nil, problem.BadRequest("compra governamental com tpOperGov " + tpOper +
			" exige a chave do documento fiscal anterior")
	case !needsRef && len(refs) > 0:
		return nil, problem.BadRequest("compra governamental com tpOperGov " + tpOper +
			" não aceita documento fiscal anterior")
	case tpOper == tpOperGovRecebimentoPosFornecimento && len(refs) > 1:
		return nil, problem.BadRequest("compra governamental com tpOperGov " +
			tpOperGovRecebimentoPosFornecimento + " aceita uma chave referenciada só")
	}
	node := map[string]any{
		"tpEnteGov": tpEnte,
		"pRedutor":  anyStr(op, opFieldCompraGovPRedutor, "0.0000"),
		"tpOperGov": tpOper,
	}
	if len(refs) > 0 {
		node["refDFeAnt"] = refs
	}
	return node, nil
}
