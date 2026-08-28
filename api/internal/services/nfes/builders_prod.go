package nfes

// builders_prod.go — nó prod de cada item da NF-e, incluindo os grupos
// específicos de combustível, medicamento, veículo novo e arma. Extraído de
// builders_doc.go.

import (
	"strconv"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// prodParams carrega os valores já calculados no laço de itens que o nó prod
// consome. Struct em vez de parâmetros posicionais: prod cresce por tag.
type prodParams struct {
	Description                string
	Unit, TaxableUnit          string
	QTrib, VUnTrib, VProd      string
	Disc, VFrete, VSeg, VOutro decimal.Decimal
}

// buildProd monta o nó prod de um item.
func buildProd(item map[string]any, p prodParams) map[string]any {
	prod := map[string]any{
		"cProd":    anyStr(item, "product_code", ""),
		"cEAN":     strOrDefault(anyStr(item, "cean", ""), "SEM GTIN"),
		"xProd":    p.Description,
		"NCM":      anyStr(item, "ncm", ""),
		"CFOP":     anyStr(item, "cfop", ""),
		"uCom":     p.Unit,
		"qCom":     anyStr(item, "quantity", "0"),
		"vUnCom":   anyStr(item, "unit_value", "0"),
		"vProd":    p.VProd,
		"cEANTrib": strOrDefault(anyStr(item, "cean", ""), "SEM GTIN"),
		"uTrib":    p.TaxableUnit,
		"qTrib":    p.QTrib,
		"vUnTrib":  p.VUnTrib,
		"indTot":   strOrDefault(anyStr(item, "ind_tot", ""), indTotCompoe),
	}
	if d(anyStr(item, "discount", "0")).GreaterThan(decimal.Zero) {
		prod["vDesc"] = q2(p.Disc.RoundBank(2))
	}
	if p.VFrete.GreaterThan(decimal.Zero) {
		prod["vFrete"] = q2(p.VFrete.RoundBank(2))
	}
	if p.VSeg.GreaterThan(decimal.Zero) {
		prod["vSeg"] = q2(p.VSeg.RoundBank(2))
	}
	if p.VOutro.GreaterThan(decimal.Zero) {
		prod["vOutro"] = q2(p.VOutro.RoundBank(2))
	}
	if cest := anyStr(item, "cest", ""); cest != "" {
		prod["CEST"] = cest
		if v := anyStr(item, "ind_escala", ""); v != "" {
			prod["indEscala"] = v
		}
		if v := anyStr(item, "cnpj_fab", ""); v != "" {
			prod["CNPJFab"] = v
		}
	}
	if v := anyStr(item, "c_benef", ""); v != "" {
		prod["cBenef"] = v
	}
	if v := anyStr(item, "ext_ipi", ""); v != "" {
		prod["EXTIPI"] = v
	}
	// NVE, nFCI e os códigos de barra próprios são do cadastro do produto.
	if nve := anyStrList(item, "nve"); len(nve) > 0 {
		prod["NVE"] = nve
	}
	if v := anyStr(item, "n_fci", ""); v != "" {
		prod["nFCI"] = v
	}
	if v := anyStr(item, "c_barra", ""); v != "" {
		prod["cBarra"] = v
	}
	if v := anyStr(item, "c_barra_trib", ""); v != "" {
		prod["cBarraTrib"] = v
	}
	// Reforma tributária no produto: crédito presumido da UF, classificação da
	// subapuração do IBS na ZFM e o indicador de bem móvel usado.
	if creds := buildGCred(anyList(item, "gcred"), d(p.VProd)); len(creds) > 0 {
		prod["gCred"] = creds
	}
	if v := anyStr(item, "tp_cred_pres_ibs_zfm", ""); v != "" {
		prod["tpCredPresIBSZFM"] = v
	}
	// O XSD enumera um valor só: 1 = bem móvel usado.
	if anyStr(item, "ind_bem_movel_usado", "") == indBemMovelUsadoSim {
		prod["indBemMovelUsado"] = indBemMovelUsadoSim
	}
	// Pedido de compra do cliente: controle B2B do emissor, informado por item
	// na emissão. nItemPed sem xPed não identifica nada, então anda junto.
	if v := anyStr(item, "x_ped", ""); v != "" {
		prod["xPed"] = v
		if n := anyStr(item, "n_item_ped", ""); n != "" {
			prod["nItemPed"] = n
		}
	}
	// DI: o item aponta a declaração e a adição; nAdicao/nSeqAdic saem daí.
	if dis, ok := item["import_declarations"].([]map[string]any); ok && len(dis) > 0 {
		prod["DI"] = dis
	}
	// rastro: o lote vem do cadastro e a quantidade sai do rateio (rastro.go).
	if lots, ok := item["lots"].([]map[string]any); ok && len(lots) > 0 {
		prod["rastro"] = lots
	}
	if exports, ok := item["exports"].([]map[string]any); ok {
		if node := buildDetExport(exports); node != nil {
			prod["detExport"] = node
		}
	}

	// comb: CIDE, encerrante e origComb entram em comb.go — a bomba é resolvida
	// na emissão e chega em item["fuel_pump"].
	pump, _ := item["fuel_pump"].(map[string]any)
	if comb := buildComb(item, pump, d(anyStr(item, "quantity", "0"))); comb != nil {
		prod["comb"] = comb
	}

	if med, err := buildMed(item); err == nil && med != nil {
		prod["med"] = med
	}

	if veic, err := buildVeicProd(item); err == nil && veic != nil {
		prod["veicProd"] = veic
	}

	if armas, ok := item["armas"].([]any); ok && len(armas) > 0 {
		armaList := make([]map[string]any, 0, len(armas))
		for _, a := range armas {
			if am, ok := a.(map[string]any); ok {
				armaItem := map[string]any{
					"tpArma": strOrDefault(anyStr(item, "arma_tp_arma", ""), "0"),
					"nSerie": anyStr(am, "n_serie", ""),
					"nCano":  anyStr(am, "n_cano", ""),
					"descr":  firstNonEmpty(anyStr(am, "descr", ""), anyStr(item, "arma_descr", "")),
				}
				armaList = append(armaList, armaItem)
			}
		}
		if len(armaList) > 0 {
			prod["arma"] = armaList
		}
	}

	// nRECOPI (papel imune) é o último ramo do choice do XSD: comb, med,
	// veicProd e arma o excluem. Emitir os dois é rejeição, então o grupo já
	// presente vence — o RECOPI de um item de combustível é erro de cadastro.
	if v := anyStr(item, "n_recopi", ""); v != "" && !hasProdChoiceGroup(prod) {
		prod["nRECOPI"] = v
	}
	return prod
}

// prodChoiceGroups são os ramos do choice de prod que excluem nRECOPI.
var prodChoiceGroups = []string{"veicProd", "med", "arma", "comb"}

// hasProdChoiceGroup diz se o item já ocupou o choice de prod.
func hasProdChoiceGroup(prod map[string]any) bool {
	for _, g := range prodChoiceGroups {
		if _, ok := prod[g]; ok {
			return true
		}
	}
	return false
}

// anyStrList lê uma lista de strings do item — o cadastro devolve []any depois
// do unmarshal do DynamoDB, e o request devolve []string.
func anyStrList(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// buildDI monta prod/DI com uma única adição — a que representa este item.
// Ordem XSD: nDI, dDI, xLocDesemb, UFDesemb, dDesemb, tpViaTransp, vAFRMM,
// tpIntermedio, CNPJ, CPF, UFTerceiro, cExportador, adi; e dentro de adi:
// nAdicao, nSeqAdic, cFabricante, vDescDI, nDraw.
//
// additionIndex é a posição (base 1) da adição no cadastro da DI e nSeqAdic é a
// ordem do item dentro dela: os dois são derivados do vínculo, nunca digitados.
func buildDI(di map[string]any, additionIndex, seq int, nDraw string) map[string]any {
	node := map[string]any{
		"nDI":          anyStr(di, "n_di", ""),
		"dDI":          anyStr(di, "d_di", ""),
		"xLocDesemb":   anyStr(di, "x_loc_desemb", ""),
		"UFDesemb":     anyStr(di, "uf_desemb", ""),
		"dDesemb":      anyStr(di, "d_desemb", ""),
		"tpViaTransp":  anyStr(di, "tp_via_transp", ""),
		"tpIntermedio": anyStr(di, "tp_intermedio", ""),
		"cExportador":  anyStr(di, "c_exportador", ""),
	}
	if v := anyStr(di, "v_afrmm", ""); v != "" {
		node["vAFRMM"] = v
	}
	if v := anyStr(di, "cnpj", ""); v != "" {
		node["CNPJ"] = v
	}
	if v := anyStr(di, "uf_terceiro", ""); v != "" {
		node["UFTerceiro"] = v
	}

	additions, _ := di["additions"].([]any)
	idx := additionIndex - 1
	if idx < 0 || idx >= len(additions) {
		return node
	}
	add, _ := additions[idx].(map[string]any)
	if add == nil {
		return node
	}
	adi := map[string]any{
		"nAdicao":     anyStr(add, "n_adicao", ""),
		"nSeqAdic":    strconv.Itoa(seq),
		"cFabricante": anyStr(add, "c_fabricante", ""),
	}
	if v := anyStr(add, "v_desc_di", ""); v != "" {
		adi["vDescDI"] = v
	}
	// nDraw da emissão vence o do cadastro: o drawback é do embarque, não da DI.
	if nDraw == "" {
		nDraw = anyStr(add, "n_draw", "")
	}
	if nDraw != "" {
		adi["nDraw"] = nDraw
	}
	node["adi"] = []map[string]any{adi}
	return node
}

// buildDetExport monta prod/detExport — a exportação indireta do item.
// Ordem XSD: nDraw, exportInd{nRE, chNFe, qExport}.
func buildDetExport(exports []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(exports))
	for _, e := range exports {
		node := map[string]any{}
		if v := anyStr(e, "n_draw", ""); v != "" {
			node["nDraw"] = v
		}
		// exportInd é tudo-ou-nada: os três campos ou nenhum.
		nRE, chNFe, qExport := anyStr(e, "n_re", ""), anyStr(e, "ch_nfe", ""), anyStr(e, "q_export", "")
		if nRE != "" && chNFe != "" && qExport != "" {
			node["exportInd"] = map[string]any{"nRE": nRE, "chNFe": chNFe, "qExport": qExport}
		}
		if len(node) > 0 {
			out = append(out, node)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildII monta imposto/II — o imposto de importação do item. Sem despesa
// aduaneira nem imposto declarados, não há grupo: o item importado que não
// recolheu II não declara II.
func buildII(item map[string]any, vProd decimal.Decimal) map[string]any {
	vII := anyStr(item, "ii_v_ii", "")
	vDespAdu := anyStr(item, "ii_v_desp_adu", "")
	if vII == "" && vDespAdu == "" {
		return nil
	}
	return map[string]any{
		"vBC":      q2(vProd.RoundBank(2)),
		"vDespAdu": strOrDefault(vDespAdu, "0.00"),
		"vII":      strOrDefault(vII, "0.00"),
		"vIOF":     strOrDefault(anyStr(item, "ii_v_iof", ""), "0.00"),
	}
}

// ── veicProd e med ───────────────────────────────────────────────────────────
//
// As 24 tags de veicProd são todas obrigatórias no XSD, e as três de med também
// (xMotivoIsencao é condicional). Nenhuma delas ganha default: um "000001" de
// cMod ou um "06" de tpVeic inventado aqui é rejeição adiada para a SEFAZ, com
// a diferença de que lá o erro não diz qual campo falta no cadastro.

// medIsento é o literal aceito em cProdANVISA no lugar do número do registro.
const medIsento = "ISENTO"

// veicProdTagOrder é a ordem do XSD (leiauteNFe_v4.00, grupo veicProd). Serve
// de fonte única do conjunto de tags obrigatórias.
var veicProdTagOrder = []string{
	"tpOp", "chassi", "cCor", "xCor", "pot", "cilin", "pesoL", "pesoB", "nSerie",
	"tpComb", "nMotor", "CMT", "dist", "anoMod", "anoFab", "tpPint", "tpVeic",
	"espVeic", "VIN", "condVeic", "cMod", "cCorDENATRAN", "lota", "tpRest",
}

// veicProdFields mapeia a chave do item (cadastro do produto ou emissão) para a
// tag do XML. A cor tem override de emissão e é tratada fora do mapa.
var veicProdFields = map[string]string{
	"veic_tp_op":          "tpOp",
	"veic_chassi":         "chassi",
	"veic_pot":            "pot",
	"veic_cilin":          "cilin",
	"net_weight":          "pesoL",
	"gross_weight":        "pesoB",
	"veic_n_serie":        "nSerie",
	"veic_tp_comb":        "tpComb",
	"veic_n_motor":        "nMotor",
	"veic_cmt":            "CMT",
	"veic_dist":           "dist",
	"veic_ano_mod":        "anoMod",
	"veic_ano_fab":        "anoFab",
	"veic_tp_pint":        "tpPint",
	"veic_tp_veic":        "tpVeic",
	"veic_esp_veic":       "espVeic",
	"veic_vin":            "VIN",
	"veic_cond_veic":      "condVeic",
	"veic_c_mod":          "cMod",
	"veic_c_cor_denatran": "cCorDENATRAN",
	"veic_lota":           "lota",
	"veic_tp_rest":        "tpRest",
}

// buildVeicProd monta prod/veicProd. Devolve (nil, nil) quando o item não é
// veículo novo — a ausência do chassi é o que define isso — e erro nomeando a
// tag faltante quando é, mas está incompleto.
func buildVeicProd(item map[string]any) (map[string]any, error) {
	if anyStr(item, "veic_chassi", "") == "" {
		return nil, nil
	}
	node := make(map[string]any, len(veicProdTagOrder))
	// cCor e xCor aceitam override na emissão: a cor é do veículo vendido, não
	// do modelo cadastrado.
	node["cCor"] = firstNonEmpty(anyStr(item, "veic_c_cor_override", ""), anyStr(item, "veic_c_cor", ""))
	node["xCor"] = firstNonEmpty(anyStr(item, "veic_x_cor_override", ""), anyStr(item, "veic_x_cor", ""))
	for key, tag := range veicProdFields {
		node[tag] = anyStr(item, key, "")
	}
	for _, tag := range veicProdTagOrder {
		if node[tag] == "" {
			return nil, problem.BadRequest(
				"veículo novo sem " + tag + ": complete o cadastro do produto ou informe o campo na emissão")
		}
	}
	return node, nil
}

// buildMed monta prod/med. Devolve (nil, nil) quando o item não é medicamento.
// cProdANVISA = ISENTO exige xMotivoIsencao; registro numérico o proíbe.
func buildMed(item map[string]any) (map[string]any, error) {
	reg := anyStr(item, "med_c_prod_anvisa", "")
	if reg == "" {
		return nil, nil
	}
	pmc := anyStr(item, "med_v_pmc", "")
	if pmc == "" {
		return nil, problem.BadRequest("medicamento sem vPMC: informe o preço máximo ao consumidor no cadastro do produto")
	}
	motivo := anyStr(item, "med_x_motivo_isencao", "")
	if reg == medIsento && motivo == "" {
		return nil, problem.BadRequest("medicamento isento de registro exige xMotivoIsencao (número da RDC que o isenta)")
	}
	if reg != medIsento && motivo != "" {
		return nil, problem.BadRequest("xMotivoIsencao só é aceito quando cProdANVISA é " + medIsento)
	}
	node := map[string]any{"cProdANVISA": reg, "vPMC": pmc}
	if motivo != "" {
		node["xMotivoIsencao"] = motivo
	}
	return node, nil
}

// ── gCred, tpCredPresIBSZFM e indBemMovelUsado ───────────────────────────────

// maxGCred é o maxOccurs do grupo gCred no XSD.
const maxGCred = 4

// indBemMovelUsadoSim é o único valor que o XSD enumera para o indicador.
const indBemMovelUsadoSim = "1"

// buildGCred monta prod/gCred — os créditos presumidos da UF aplicados ao item.
// Ordem XSD: cCredPresumido, pCredPresumido, vCredPresumido.
//
// O vCredPresumido é **derivado** do percentual sobre o valor do item: código e
// percentual são do cadastro, e o valor é aritmética. Digitar os três seria
// pedir que o operador feche uma conta que o sistema já sabe fazer.
func buildGCred(creds []any, vProd decimal.Decimal) []map[string]any {
	if len(creds) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, min(len(creds), maxGCred))
	for _, raw := range creds {
		if len(out) == maxGCred {
			break
		}
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		code := anyStr(m, "c_cred_presumido", "")
		pct := anyStr(m, "p_cred_presumido", "")
		if code == "" || pct == "" {
			continue
		}
		out = append(out, map[string]any{
			"cCredPresumido": code,
			"pCredPresumido": pct,
			"vCredPresumido": calcTaxValue(vProd, pct),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// anyList lê uma lista de mapas de um item, aceitando o []any que vem do
// unmarshal do DynamoDB e o []map[string]any que o request monta.
func anyList(m map[string]any, key string) []any {
	switch v := m[key].(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, e := range v {
			out = append(out, e)
		}
		return out
	}
	return nil
}
