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
