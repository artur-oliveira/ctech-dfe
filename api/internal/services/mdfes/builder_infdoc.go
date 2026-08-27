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
			nfes := make([]map[string]any, 0, len(g.nfeKeys))
			for _, k := range g.nfeKeys {
				nfes = append(nfes, map[string]any{"chNFe": k})
			}
			node["infNFe"] = nfes
		}
		if len(g.cteKeys) > 0 {
			ctes := make([]map[string]any, 0, len(g.cteKeys))
			for _, k := range g.cteKeys {
				ctes = append(ctes, map[string]any{"chCTe": k})
			}
			node["infCTe"] = ctes
		}
		munDescarga = append(munDescarga, node)
	}
	return map[string]any{"infMunDescarga": munDescarga}
}
