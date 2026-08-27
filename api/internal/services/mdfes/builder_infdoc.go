package mdfes

// builder_infdoc.go — grupo infDoc (municípios de descarga e documentos
// transportados). Extraído de builder.go.

func (p buildParams) buildInfDoc() map[string]any {
	munDescarga := make([]map[string]any, 0, len(p.cargo.descarga))
	for _, g := range p.cargo.descarga {
		node := map[string]any{
			"cMunDescarga": g.mun.IBGECode,
			"xMunDescarga": g.mun.City,
		}
		if len(g.nfeKeys) > 0 {
			node["infNFe"] = p.docNodes("chNFe", g.nfeKeys)
		}
		if len(g.cteKeys) > 0 {
			node["infCTe"] = p.docNodes("chCTe", g.cteKeys)
		}
		munDescarga = append(munDescarga, node)
	}
	return map[string]any{"infMunDescarga": munDescarga}
}

// docNodes monta os nós de documento transportado. NF-e e CT-e têm a mesma
// forma no leiaute (chave, SegCodBarra, indReentrega), então é uma função só.
func (p buildParams) docNodes(keyField string, keys []string) []map[string]any {
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		// SegCodBarra é o código de barras do documento — que é a própria
		// chave. Perguntá-lo ao operador seria pedir de volta o dado que ele
		// acabou de referenciar.
		node := map[string]any{keyField: k, "SegCodBarra": k}
		if p.redelivery[k] {
			node["indReentrega"] = indReentregaSim
		}
		// peri vem do cadastro do produto que a nota referenciada declara — o
		// operador classifica a ONU uma vez, não a cada viagem.
		if peri := p.peri[k]; len(peri) > 0 {
			node["peri"] = peri
		}
		out = append(out, node)
	}
	return out
}
