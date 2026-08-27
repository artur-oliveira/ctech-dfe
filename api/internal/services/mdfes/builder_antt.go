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
