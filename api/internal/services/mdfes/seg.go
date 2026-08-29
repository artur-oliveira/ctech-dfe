package mdfes

// resolvedPolicy é uma apólice já cruzada com o cadastro: responsável,
// seguradora e número da apólice vêm de organization_insurance_policies; só as
// averbações (nAver) mudam a cada viagem.
type resolvedPolicy struct {
	RespSeg string
	CNPJ    string
	CPF     string
	XSeg    string
	CNPJSeg string
	NApol   string
	NAver   []string
}

// buildSeg monta infMDFe/seg. Ordem XSD: infResp{respSeg, CNPJ|CPF},
// infSeg{xSeg, CNPJ}, nApol, nAver. O documento do responsável só é informado
// quando ele não é o emitente; a seguradora é opcional no XSD, mas nome e CNPJ
// andam sempre juntos (o cadastro já recusa a metade).
func buildSeg(policies []resolvedPolicy) []map[string]any {
	if len(policies) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(policies))
	for _, p := range policies {
		infResp := map[string]any{"respSeg": p.RespSeg}
		if p.CNPJ != "" {
			infResp["CNPJ"] = onlyDigits(p.CNPJ)
		} else if p.CPF != "" {
			infResp["CPF"] = onlyDigits(p.CPF)
		}
		seg := map[string]any{"infResp": infResp}
		if p.XSeg != "" {
			seg["infSeg"] = map[string]any{"xSeg": p.XSeg, "CNPJ": onlyDigits(p.CNPJSeg)}
		}
		if p.NApol != "" {
			seg["nApol"] = p.NApol
		}
		if len(p.NAver) > 0 {
			seg["nAver"] = p.NAver
		}
		out = append(out, seg)
	}
	return out
}
