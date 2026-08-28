package nfes

// builders_prod.go — nó prod de cada item da NF-e, incluindo os grupos
// específicos de combustível, medicamento, veículo novo e arma. Extraído de
// builders_doc.go.

import (
	"strconv"

	"github.com/shopspring/decimal"
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

	if medProd := anyStr(item, "med_c_prod_anvisa", ""); medProd != "" {
		medNode := map[string]any{
			"cProdANVISA": medProd,
			"vPMC":        strOrDefault(anyStr(item, "med_v_pmc", ""), "0.00"),
		}
		if v := anyStr(item, "med_x_motivo_isencao", ""); v != "" {
			medNode["xMotivoIsencao"] = v
		}
		prod["med"] = medNode
	}

	if veicChassi := anyStr(item, "veic_chassi", ""); veicChassi != "" {
		veicNode := map[string]any{
			"tpOp":         strOrDefault(anyStr(item, "veic_tp_op", ""), "0"),
			"chassi":       veicChassi,
			"cCor":         strOrDefault(firstNonEmpty(anyStr(item, "veic_c_cor_override", ""), anyStr(item, "veic_c_cor", "")), ""),
			"xCor":         strOrDefault(firstNonEmpty(anyStr(item, "veic_x_cor_override", ""), anyStr(item, "veic_x_cor", "")), ""),
			"pot":          strOrDefault(anyStr(item, "veic_pot", ""), "0"),
			"cilin":        strOrDefault(anyStr(item, "veic_cilin", ""), "0"),
			"pesoL":        strOrDefault(anyStr(item, "net_weight", ""), "0"),
			"pesoB":        strOrDefault(anyStr(item, "gross_weight", ""), "0"),
			"nSerie":       strOrDefault(anyStr(item, "veic_n_serie", ""), ""),
			"tpComb":       strOrDefault(anyStr(item, "veic_tp_comb", ""), "02"),
			"nMotor":       strOrDefault(anyStr(item, "veic_n_motor", ""), ""),
			"CMT":          strOrDefault(anyStr(item, "veic_cmt", ""), "0"),
			"dist":         strOrDefault(anyStr(item, "veic_dist", ""), "0"),
			"anoMod":       strOrDefault(anyStr(item, "veic_ano_mod", ""), ""),
			"anoFab":       strOrDefault(anyStr(item, "veic_ano_fab", ""), ""),
			"tpPint":       strOrDefault(anyStr(item, "veic_tp_pint", ""), "S"),
			"tpVeic":       strOrDefault(anyStr(item, "veic_tp_veic", ""), "06"),
			"espVeic":      strOrDefault(anyStr(item, "veic_esp_veic", ""), "1"),
			"VIN":          strOrDefault(anyStr(item, "veic_vin", ""), "N"),
			"condVeic":     strOrDefault(anyStr(item, "veic_cond_veic", ""), "1"),
			"cMod":         strOrDefault(anyStr(item, "veic_c_mod", ""), "000001"),
			"cCorDENATRAN": strOrDefault(anyStr(item, "veic_c_cor_denatran", ""), "01"),
			"lota":         strOrDefault(anyStr(item, "veic_lota", ""), "5"),
			"tpRest":       strOrDefault(anyStr(item, "veic_tp_rest", ""), "0"),
		}
		prod["veicProd"] = veicNode
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
	return prod
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
