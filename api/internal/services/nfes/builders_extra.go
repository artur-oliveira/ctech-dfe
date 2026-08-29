package nfes

// builders_extra.go — helpers de saneamento de campos livres da NF-e e os
// grupos de nicho do fim de infNFe (compra, cana, agropecuario).
// Extraído de builders_doc.go.

import (
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// natOpMaxLen is the SEFAZ ide.natOp limit (xNatOp: 1-60 chars).
const natOpMaxLen = 60

// Campos do CSRT na configuração fiscal (organization_*_configs).
const (
	csrtIDField = "csrt_id"
	csrtField   = "csrt"
)

// truncateNatOp enforces the SEFAZ ide.natOp 60-char limit. The frontend sends a
// summarized CFOP description; this is a rune-safe safety net that truncates with
// an ellipsis suffix when the value exceeds natOpMaxLen.
func truncateNatOp(s string) string {
	r := []rune(s)
	if len(r) <= natOpMaxLen {
		return s
	}
	return string(r[:natOpMaxLen-3]) + "..."
}

// docExtras carrega os grupos opcionais da NF-e que não cabem nos parâmetros
// posicionais de BuildEnviNFe. Cada tarefa de cobertura de tags acrescenta um
// campo aqui em vez de mais um parâmetro na assinatura.
type docExtras struct {
	// Exporta é o grupo infNFe/exporta, já resolvido da operação + locais de
	// retirada salvos na organização.
	Exporta map[string]any
	// NFref são os documentos referenciados já resolvidos (ide/NFref).
	NFRefs []map[string]any
	// Vols e Reboques são os volumes e reboques de transp, já resolvidos.
	Vols     []map[string]any
	Reboques []map[string]any
	// InfAdFisco, ObsCont, ObsFisco e ProcRef completam infAdic; infCpl continua
	// vindo do parâmetro additionalInfo de BuildEnviNFe.
	InfAdFisco string
	ObsCont    []map[string]any
	ObsFisco   []map[string]any
	ProcRef    []map[string]any
	// PaymentTerminals são os terminais de captura citados pelos pagamentos,
	// indexados pelo SK do cadastro.
	PaymentTerminals map[string]map[string]any
	// RetTrib é o perfil de retenções federais da operação (total/retTrib).
	RetTrib map[string]any
	// FinNFe4 diz se a nota é de devolução — o único caso em que impostoDevol
	// pode existir.
	FinNFe4 bool
	// CsrtID/Csrt são o Código de Segurança do Responsável Técnico. O segredo
	// não é gravado no documento: só o hash entra no XML.
	CsrtID string
	Csrt   string
	// Compra, Cana e Agropecuario são os grupos de nicho do fim de infNFe, já
	// resolvidos da operação + organização + request.
	Compra       map[string]any
	Cana         map[string]any
	Agropecuario map[string]any
	// DhSaiEnt e DPrevEntrega são a saída e a previsão de entrega, já resolvidas
	// (offset da operação ou valor explícito da nota).
	DhSaiEnt, DPrevEntrega string
	// IndIntermed e InfIntermed são o canal de venda: indicador em ide e o
	// grupo do intermediador em infNFe, resolvidos da operação.
	IndIntermed string
	InfIntermed map[string]any
	// Reforma tributária no ide: os quatro campos da operação, o grupo de
	// compras governamentais já validado e as chaves de antecipação a abater.
	CIndOp, CMunFGIBS, TpNFDebito, TpNFCredito string
	CompraGov                                  map[string]any
	PagAntecipado                              []string
}

// buildInfAdic monta infAdic. Ordem XSD: infAdFisco, infCpl, obsCont, obsFisco,
// procRef. Devolve nil quando não há nada — nó vazio é rejeição.
func buildInfAdic(infAdFisco, infCpl string, obsCont, obsFisco, procRef []map[string]any) map[string]any {
	node := map[string]any{}
	if infAdFisco != "" {
		node["infAdFisco"] = infAdFisco
	}
	if infCpl != "" {
		node["infCpl"] = infCpl
	}
	if len(obsCont) > 0 {
		node["obsCont"] = obsCont
	}
	if len(obsFisco) > 0 {
		node["obsFisco"] = obsFisco
	}
	if len(procRef) > 0 {
		node["procRef"] = procRef
	}
	if len(node) == 0 {
		return nil
	}
	return node
}

// buildProcRef traduz os processos referenciados do request (infAdic/procRef).
func buildProcRef(refs []NfeProcRefBody) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		node := map[string]any{"nProc": r.NProc, "indProc": r.IndProc}
		if r.TpAto != nil && *r.TpAto != "" {
			node["tpAto"] = *r.TpAto
		}
		out = append(out, node)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildExporta monta infNFe/exporta — UF de saída do país e local de despacho.
// A UF vem da operação de exportação (nível 1) e o local reusa os
// pickup_locations já salvos na organização, em vez de redigitar o endereço.
// Ordem XSD: UFSaidaPais, xLocExporta, xLocDespacho.
func buildExporta(op map[string]any, pickups []any) map[string]any {
	uf := anyStr(op, "export_uf_saida_pais", "")
	if uf == "" {
		return nil
	}
	node := map[string]any{"UFSaidaPais": uf}
	idx, ok := op["export_loc_despacho_index"].(float64)
	if !ok {
		if n, isInt := op["export_loc_despacho_index"].(int); isInt {
			idx, ok = float64(n), true
		}
	}
	if !ok || int(idx) < 0 || int(idx) >= len(pickups) {
		return node
	}
	loc, _ := pickups[int(idx)].(map[string]any)
	if loc == nil {
		return node
	}
	// xLocExporta é o município onde a mercadoria sai; xLocDespacho, o
	// logradouro do recinto — os dois saem do mesmo local salvo.
	if v := anyStr(loc, "x_mun", ""); v != "" {
		node["xLocExporta"] = v
	}
	if v := anyStr(loc, "x_lgr", ""); v != "" {
		node["xLocDespacho"] = v
	}
	return node
}

// ── compra, cana e agropecuario ──────────────────────────────────────────────
//
// Três grupos de nicho no fim de infNFe. A alocação segue a régua: o que é
// invariante do cenário fica na operação (nota de empenho, safra), o que é
// invariante do emitente fica na organização (CPF do responsável técnico
// agronômico) e só o que muda a cada nota entra no request.

// Casas decimais dos grupos: qtde da cana em TDec_1110v (emitimos 4, como as
// demais quantidades) e os valores em TDec_1302.
const canaQtdePlaces = 4

// techManagerCPFField é o CPF do responsável técnico agronômico do emitente
// (agropecuario/defensivo/CPFRespTec) — invariante da organização.
const techManagerCPFField = "technical_manager_cpf"

// buildCompra monta infNFe/compra. A nota de empenho é da operação (quem vende
// para órgão público vende sempre por empenho); pedido e contrato são da nota.
// Ordem XSD: xNEmp, xPed, xCont.
func buildCompra(op map[string]any, xPed, xCont string) map[string]any {
	node := map[string]any{}
	if v := anyStr(op, opFieldCompraXNEmp, ""); v != "" {
		node["xNEmp"] = v
	}
	if xPed != "" {
		node["xPed"] = xPed
	}
	if xCont != "" {
		node["xCont"] = xCont
	}
	if len(node) == 0 {
		return nil
	}
	return node
}

// buildCana monta infNFe/cana. Ordem XSD: safra, ref, forDia, qTotMes, qTotAnt,
// qTotGer, deduc, vFor, vTotDed, vLiqFor.
//
// Quatro valores são derivados e nunca digitados: qTotMes é a soma dos
// lançamentos diários, qTotGer é ela mais o acumulado anterior, vTotDed é a
// soma das deduções e vLiqFor é o fornecimento menos elas. vFor é o valor do
// fornecimento do mês, que é a base da própria nota.
func buildCana(op map[string]any, req *NfeCanaBody, vFor decimal.Decimal) (map[string]any, error) {
	if req == nil {
		return nil, nil
	}
	safra := anyStr(op, opFieldCanaSafra, "")
	if safra == "" {
		return nil, problem.BadRequest("aquisição de cana sem safra: cadastre a safra na natureza de operação")
	}
	dias := make([]map[string]any, 0, len(req.Deliveries))
	qTotMes := decimal.Zero
	seen := make(map[string]struct{}, len(req.Deliveries))
	for _, dl := range req.Deliveries {
		if _, dup := seen[dl.Dia]; dup {
			return nil, problem.BadRequest("dois lançamentos de cana no dia " + dl.Dia +
				": o leiaute aceita um por dia")
		}
		seen[dl.Dia] = struct{}{}
		qty := d(dl.Qtde)
		qTotMes = qTotMes.Add(qty)
		dias = append(dias, map[string]any{"@dia": dl.Dia, "qtde": qty.StringFixed(canaQtdePlaces)})
	}
	qTotAnt := d(ptrStr(req.QTotAnt))

	deducoes := make([]map[string]any, 0, len(req.Deducoes))
	vTotDed := decimal.Zero
	for _, dd := range req.Deducoes {
		v := d(dd.VDed)
		vTotDed = vTotDed.Add(v)
		deducoes = append(deducoes, map[string]any{"xDed": dd.XDed, "vDed": q2(v.RoundBank(2))})
	}

	node := map[string]any{
		"safra":   safra,
		"ref":     req.Ref,
		"forDia":  dias,
		"qTotMes": qTotMes.StringFixed(canaQtdePlaces),
		"qTotAnt": qTotAnt.StringFixed(canaQtdePlaces),
		"qTotGer": qTotMes.Add(qTotAnt).StringFixed(canaQtdePlaces),
		"vFor":    q2(vFor.RoundBank(2)),
		"vTotDed": q2(vTotDed.RoundBank(2)),
		"vLiqFor": q2(vFor.Sub(vTotDed).RoundBank(2)),
	}
	if len(deducoes) > 0 {
		node["deduc"] = deducoes
	}
	return node, nil
}

// buildAgropecuario monta infNFe/agropecuario. O XSD é um **choice**: ou até 20
// receituários de defensivo, ou uma guia de trânsito — nunca os dois.
//
// O CPF do responsável técnico acompanha cada receituário, mas é o mesmo
// agrônomo da organização: fica no cadastro do emitente, não na nota.
func buildAgropecuario(org map[string]any, req *NfeAgroBody) (map[string]any, error) {
	if req == nil {
		return nil, nil
	}
	hasRecs := len(req.Receituarios) > 0
	hasGuia := req.Guia != nil
	switch {
	case hasRecs && hasGuia:
		return nil, problem.BadRequest(
			"receituário de defensivo e guia de trânsito são alternativos no grupo agropecuario: informe um só")
	case hasRecs:
		cpf := anyStr(org, techManagerCPFField, "")
		if cpf == "" {
			return nil, problem.BadRequest("receituário de defensivo exige CPFRespTec: cadastre o CPF do " +
				"responsável técnico agronômico na organização")
		}
		defensivos := make([]map[string]any, 0, len(req.Receituarios))
		for _, rec := range req.Receituarios {
			defensivos = append(defensivos, map[string]any{"nReceituario": rec, "CPFRespTec": cpf})
		}
		return map[string]any{"defensivo": defensivos}, nil
	case hasGuia:
		guia := map[string]any{
			"tpGuia": req.Guia.TpGuia,
			"UFGuia": req.Guia.UFGuia,
			"nGuia":  req.Guia.NGuia,
		}
		if v := ptrStr(req.Guia.SerieGuia); v != "" {
			guia["serieGuia"] = v
		}
		return map[string]any{"guiaTransito": guia}, nil
	}
	return nil, nil
}
