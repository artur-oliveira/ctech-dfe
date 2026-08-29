package nfes

// comb.go monta prod/comb — o grupo do combustível. Três coisas não são
// digitadas: o vCIDE (é qBCProd × vAliqProd), o vEncIni (é o vEncFin da venda
// anterior da mesma bomba) e o próprio qBCProd (é a quantidade vendida).

import (
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Casas decimais do grupo comb no leiaute: encerrante em 3, CIDE em 4.
const (
	encerrantePlaces = 3
	cidePlaces       = 4
)

// combFieldsFromProduct são os campos do comb que vêm direto do cadastro do
// produto, sem cálculo: chave do item → tag do XML.
var combFieldsFromProduct = map[string]string{
	"comb_p_glp": "pGLP", "comb_p_gnn": "pGNn", "comb_p_gni": "pGNi",
	"comb_v_part": "vPart", "comb_codif": "CODIF", "comb_p_bio": "pBio",
}

// buildComb monta o grupo comb na ordem do XSD. pump é a bomba do cadastro
// (nil quando a venda não passa por bico) e qty é a quantidade vendida, que
// serve de base do CIDE.
func buildComb(item map[string]any, pump map[string]any, qty decimal.Decimal) map[string]any {
	combProd := anyStr(item, "comb_c_prod_anp", "")
	if combProd == "" {
		return nil
	}
	comb := map[string]any{
		"cProdANP": combProd,
		"descANP":  strOrDefault(anyStr(item, "comb_desc_anp", ""), ""),
		"UFCons":   strOrDefault(anyStr(item, "comb_uf_cons", ""), ""),
	}
	for field, xml := range combFieldsFromProduct {
		if v := anyStr(item, field, ""); v != "" {
			comb[xml] = v
		}
	}
	// qTemp é a quantidade a 20 °C: muda a cada venda, não é do cadastro.
	if v := anyStr(item, "comb_q_temp", ""); v != "" {
		comb["qTemp"] = v
	}
	if cide := buildCIDE(item, qty); cide != nil {
		comb["CIDE"] = cide
	}
	if enc := buildEncerrante(item, pump); enc != nil {
		comb["encerrante"] = enc
	}
	if orig := buildOrigComb(item); len(orig) > 0 {
		comb["origComb"] = orig
	}
	return comb
}

// buildCIDE calcula a CIDE do item: a base é a quantidade vendida e o valor é
// base × alíquota. Só a alíquota é cadastrada — o resto seria redigitação
// sujeita a divergir do próprio item.
func buildCIDE(item map[string]any, qty decimal.Decimal) map[string]any {
	aliq := anyStr(item, "comb_cide_v_aliq_prod", "")
	if aliq == "" {
		return nil
	}
	rate := d(aliq)
	return map[string]any{
		"qBCProd":   qty.StringFixed(cidePlaces),
		"vAliqProd": rate.StringFixed(cidePlaces),
		"vCIDE":     qty.Mul(rate).RoundBank(2).StringFixed(2),
	}
}

// buildEncerrante monta o grupo do encerrante da bomba. O vEncIni é a leitura
// final da venda anterior, guardada no cadastro da bomba: o operador informa
// só onde o marcador parou agora.
func buildEncerrante(item map[string]any, pump map[string]any) map[string]any {
	if pump == nil {
		return nil
	}
	vEncFin := anyStr(item, "comb_v_enc_fin", "")
	if vEncFin == "" {
		return nil
	}
	enc := map[string]any{
		"nBico":   anyStr(pump, "n_bico", ""),
		"vEncIni": d(anyStr(pump, "last_v_enc_fin", "0")).StringFixed(encerrantePlaces),
		"vEncFin": d(vEncFin).StringFixed(encerrantePlaces),
	}
	if v := anyStr(pump, "n_bomba", ""); v != "" {
		enc["nBomba"] = v
	}
	if v := anyStr(pump, "n_tanque", ""); v != "" {
		enc["nTanque"] = v
	}
	return enc
}

// buildOrigComb monta a origem do combustível (até 30 entradas), cadastrada no
// produto: de onde veio e em que proporção.
func buildOrigComb(item map[string]any) []map[string]any {
	raw, ok := item["comb_orig"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"indImport": anyStr(m, "ind_import", ""),
			"cUFOrig":   anyStr(m, "c_uf_orig", ""),
			"pOrig":     anyStr(m, "p_orig", ""),
		})
	}
	return out
}

// validateEncerrante recusa a leitura que anda para trás: o encerrante da bomba
// é um totalizador mecânico, e um vEncFin menor que a leitura anterior é erro
// de digitação — não um volume negativo a ser emitido.
func validateEncerrante(pump map[string]any, vEncFin string) error {
	if vEncFin == "" {
		return nil
	}
	prev := d(anyStr(pump, "last_v_enc_fin", "0"))
	if d(vEncFin).LessThan(prev) {
		return problem.BadRequest(
			"leitura final do encerrante (" + vEncFin + ") é menor que a leitura anterior da bomba (" +
				prev.StringFixed(encerrantePlaces) + ")")
	}
	return nil
}
