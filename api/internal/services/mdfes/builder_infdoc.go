package mdfes

// builder_infdoc.go — grupo infDoc (municípios de descarga e documentos
// transportados). Extraído de builder.go.

// partialDelivery é a entrega parcial de um CT-e já resolvida para o builder.
type partialDelivery struct {
	QtdTotal   string
	QtdParcial string
	NFeKeys    []string
}

func (p buildParams) buildInfDoc() map[string]any {
	// Os MDF-e transportados entram no município de descarga que o request
	// informa; um município que ainda não existe vira um grupo novo.
	transpByMun := map[string][]string{}
	munOrder := make([]MdfeMun, 0, len(p.transportedMdfes))
	for _, t := range p.transportedMdfes {
		if _, seen := transpByMun[t.Unloading.IBGECode]; !seen {
			munOrder = append(munOrder, t.Unloading)
		}
		transpByMun[t.Unloading.IBGECode] = append(transpByMun[t.Unloading.IBGECode], t.AccessKey)
	}

	munDescarga := make([]map[string]any, 0, len(p.cargo.descarga)+len(munOrder))
	seenMun := map[string]bool{}
	for _, g := range p.cargo.descarga {
		seenMun[g.mun.IBGECode] = true
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
		if keys := transpByMun[g.mun.IBGECode]; len(keys) > 0 {
			node["infMDFeTransp"] = p.docNodes("chMDFe", keys)
		}
		munDescarga = append(munDescarga, node)
	}
	// Município que só recebe MDF-e transportado ainda precisa do seu grupo.
	for _, mun := range munOrder {
		if seenMun[mun.IBGECode] {
			continue
		}
		munDescarga = append(munDescarga, map[string]any{
			"cMunDescarga":  mun.IBGECode,
			"xMunDescarga":  mun.City,
			"infMDFeTransp": p.docNodes("chMDFe", transpByMun[mun.IBGECode]),
		})
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
		if unids := p.unidTransp[k]; len(unids) > 0 {
			node["infUnidTransp"] = unids
		}
		// peri vem do cadastro do produto que a nota referenciada declara — o
		// operador classifica a ONU uma vez, não a cada viagem.
		if peri := p.peri[k]; len(peri) > 0 {
			node["peri"] = peri
		}
		// A entrega parcial (corte de voo) e a prestação parcial só existem no
		// CT-e transportado; o XSD não as prevê em infNFe nem em infMDFeTransp.
		if keyField == "chCTe" {
			if part, ok := p.partial[k]; ok {
				node["infEntregaParcial"] = map[string]any{
					"qtdTotal": part.QtdTotal, "qtdParcial": part.QtdParcial,
				}
				if len(part.NFeKeys) > 0 {
					node["indPrestacaoParcial"] = indPrestacaoParcialSim
					nfes := make([]map[string]any, 0, len(part.NFeKeys))
					for _, ch := range part.NFeKeys {
						nfes = append(nfes, map[string]any{"chNFe": ch})
					}
					node["infNFePrestParcial"] = nfes
				}
			}
		}
		out = append(out, node)
	}
	return out
}
