package mdfes

// builder_antt.go — grupo infANTT do modal rodoviário (RNTRC, CIOT, vale-pedágio,
// contratante e pagamento). Extraído de builder.go.

// buildInfANTT assembles the ANTT regulatory group (RNTRC + optional CIOT).
func (p buildParams) buildInfANTT() map[string]any {
	infANTT := map[string]any{}
	if rntrc := p.resolveRNTRC(); rntrc != "" {
		infANTT["RNTRC"] = rntrc
	}
	if p.ciot != nil && *p.ciot != "" {
		infANTT["infCIOT"] = map[string]any{"CIOT": *p.ciot, "CPF": onlyDigits(p.firstCondutorCPF())}
	}
	if vp := p.buildValePed(); vp != nil {
		infANTT["valePed"] = vp
	}
	if ct := p.buildInfContratante(); ct != nil {
		infANTT["infContratante"] = ct
	}
	if len(p.infPag) > 0 {
		infANTT["infPag"] = p.infPag
	}
	return infANTT
}

// categCombVeic (valePed/categCombVeic) classifica a combinação veicular pelo
// número de eixos, que aqui é derivado da composição: o trator sozinho é
// caminhão simples; cada reboque acrescenta uma categoria. Perguntar isso ao
// operador seria perguntar algo que o próprio manifesto já diz.
func categCombVeic(trailers int) string {
	switch trailers {
	case 0:
		return categCombCaminhao
	case 1:
		return categCombCaminhaoReboque
	case 2:
		return categCombCaminhaoDoisReboques
	default:
		return categCombCaminhaoTresReboques
	}
}

// buildValePed monta infANTT/valePed. Ordem XSD: disp (0..N), categCombVeic.
// O fornecedor e o pagador vêm do cadastro de fornecedoras de vale-pedágio; da
// viagem saem só número da compra e valor.
func (p buildParams) buildValePed() map[string]any {
	if len(p.tolls) == 0 {
		return nil
	}
	disp := make([]map[string]any, 0, len(p.tolls))
	for _, t := range p.tolls {
		item := map[string]any{"CNPJForn": t.CNPJForn}
		// CNPJPg e CPFPg são um choice no XSD: no máximo um.
		if t.CNPJPg != "" {
			item["CNPJPg"] = t.CNPJPg
		} else if t.CPFPg != "" {
			item["CPFPg"] = t.CPFPg
		}
		item["nCompra"] = t.NCompra
		item["vValePed"] = t.VValePed
		if t.TpValePed != "" {
			item["tpValePed"] = t.TpValePed
		}
		disp = append(disp, item)
	}
	return map[string]any{"disp": disp, "categCombVeic": categCombVeic(len(p.trailers))}
}

// buildInfContratante monta infANTT/infContratante (0..10). Ordem XSD:
// xNome, choice{CPF|CNPJ|idEstrangeiro}, infContrato. Identidade e nome vêm do
// cadastro de pessoas; o contrato é da viagem.
func (p buildParams) buildInfContratante() []map[string]any {
	if len(p.contractors) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(p.contractors))
	for _, c := range p.contractors {
		node := map[string]any{"xNome": c.Name}
		switch {
		case c.CNPJ != "":
			node["CNPJ"] = c.CNPJ
		case c.CPF != "":
			node["CPF"] = c.CPF
		default:
			node["idEstrangeiro"] = c.Foreign
		}
		// NroContrato é obrigatório dentro de infContrato: sem ele o grupo
		// inteiro fica de fora, em vez de sair vazio e ser recusado.
		if c.ContractNumber != "" {
			node["infContrato"] = map[string]any{
				"NroContrato": c.ContractNumber, "vContratoGlobal": c.ContractValue,
			}
		}
		out = append(out, node)
	}
	return out
}
