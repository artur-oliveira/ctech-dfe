package nfes

// builders_transp.go — nó transp da NF-e (modalidade de frete, transportadora,
// veículo e volumes). Extraído de builders_doc.go.

import (
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// buildTransp builds the transp XML node.
//
// For own-transport freight modes the transportador is the issuer or the
// recipient itself, so its transporta data is taken from emitTransporta /
// destTransporta instead of the request-supplied transporta_* fields:
//   - modFrete "3" (próprio por conta do remetente)    → emitTransporta
//   - modFrete "4" (próprio por conta do destinatário)  → destTransporta
func buildTransp(
	hasPesoL, hasPesoB bool, totalPesoL, totalPesoB decimal.Decimal,
	transport, emitTransporta, destTransporta map[string]any,
	vols []map[string]any, reboques []map[string]any,
) map[string]any {
	modFrete := modFreteSemFrete
	if transport != nil {
		if v := anyStr(transport, "mod_frete", ""); v != "" {
			modFrete = v
		}
	}
	transp := map[string]any{"modFrete": modFrete}

	var transporta map[string]any
	switch modFrete {
	case modFreteProprioRemetente:
		transporta = emitTransporta
	case modFreteProprioDestinatario:
		transporta = destTransporta
	default:
		transporta = transportaFromRequest(transport)
	}
	if len(transporta) > 0 {
		transp["transporta"] = transporta
	}
	if ret := buildRetTransp(transport); ret != nil {
		transp["retTransp"] = ret
	}

	if transport != nil {
		veiculo := map[string]any{}
		if v := anyStr(transport, "veiculo_placa", ""); v != "" {
			veiculo["placa"] = v
		}
		if v := anyStr(transport, "veiculo_uf", ""); v != "" {
			veiculo["UF"] = v
		}
		// A tag da NF-e é RNTC; RNTRC é do MDF-e/CT-e.
		if v := anyStr(transport, "veiculo_rntrc", ""); v != "" {
			veiculo["RNTC"] = v
		}
		if len(veiculo) > 0 {
			transp["veicTransp"] = veiculo
		}
	}

	// vol é lista no XSD (0..N). Volume explícito vence; sem nenhum, o
	// comportamento antigo é preservado — um volume sintético com os pesos
	// somados dos itens, que é o que a maioria das notas precisa.
	switch {
	case len(vols) > 0:
		transp["vol"] = vols
	case hasPesoL || hasPesoB:
		vol := map[string]any{"qVol": qVolPadrao}
		if hasPesoL {
			vol["pesoL"] = totalPesoL.StringFixed(3)
		}
		if hasPesoB {
			vol["pesoB"] = totalPesoB.StringFixed(3)
		}
		transp["vol"] = []map[string]any{vol}
	}
	if len(reboques) > 0 {
		transp["reboque"] = reboques
	}
	return transp
}

// transportaFromRequest builds the transporta node from the request-supplied
// transporta_* fields (used for non-own-transport freight modes).
func transportaFromRequest(transport map[string]any) map[string]any {
	if transport == nil {
		return nil
	}
	transporta := map[string]any{}
	if v := anyStr(transport, "transporta_cnpj", ""); v != "" {
		transporta["CNPJ"] = v
	} else if v := anyStr(transport, "transporta_cpf", ""); v != "" {
		transporta["CPF"] = v
	}
	if v := anyStr(transport, "transporta_nome", ""); v != "" {
		transporta["xNome"] = v
	}
	if v := anyStr(transport, "transporta_ie", ""); v != "" {
		transporta["IE"] = v
	}
	if v := anyStr(transport, "transporta_ender", ""); v != "" {
		transporta["xEnder"] = v
	}
	if v := anyStr(transport, "transporta_mun", ""); v != "" {
		transporta["xMun"] = v
	}
	if v := anyStr(transport, "transporta_uf", ""); v != "" {
		transporta["UF"] = v
	}
	return transporta
}

// buildPartyTransporta builds a transporta node from a party (emitente or
// destinatário) for own-transport freight modes (modFrete 3/4). Address fields
// are sourced from the party's first address.
func buildPartyTransporta(doc string, isPJ bool, name, ie string, address map[string]any) map[string]any {
	if doc == "" {
		return nil
	}
	transporta := map[string]any{}
	if isPJ {
		transporta["CNPJ"] = doc
	} else {
		transporta["CPF"] = doc
	}
	if name != "" {
		transporta["xNome"] = name
	}
	if ie != "" {
		transporta["IE"] = ie
	}
	if v := anyStr(address, "street", ""); v != "" {
		transporta["xEnder"] = v
	}
	if v := anyStr(address, "city", ""); v != "" {
		transporta["xMun"] = v
	}
	if v := anyStr(address, "state_federation", ""); v != "" {
		transporta["UF"] = v
	}
	return transporta
}

// buildVols traduz os volumes do request para os nós transp/vol. `esp` e
// `marca` sem valor caem para o default da operação — a espécie do volume é
// característica da operação, não da nota.
func buildVols(vols []NfeVolBody, defEsp, defMarca string) []map[string]any {
	out := make([]map[string]any, 0, len(vols))
	for _, v := range vols {
		node := map[string]any{}
		for key, val := range map[string]string{
			"qVol":  ptrStr(v.QVol),
			"esp":   firstNonEmpty(ptrStr(v.Esp), defEsp),
			"marca": firstNonEmpty(ptrStr(v.Marca), defMarca),
			"nVol":  ptrStr(v.NVol),
			"pesoL": ptrStr(v.PesoL),
			"pesoB": ptrStr(v.PesoB),
		} {
			if val != "" {
				node[key] = val
			}
		}
		if len(v.Lacres) > 0 {
			node["lacres"] = services.SealNodes(v.Lacres)
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

// buildReboques traduz os reboques do request para os nós transp/reboque.
func buildReboques(reboques []NfeReboqueBody) []map[string]any {
	out := make([]map[string]any, 0, len(reboques))
	for _, r := range reboques {
		node := map[string]any{"placa": r.Placa, "UF": r.UF}
		if r.RNTC != nil && *r.RNTC != "" {
			node["RNTC"] = *r.RNTC
		}
		out = append(out, node)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildRetTransp monta transp/retTransp — o ICMS retido pelo remetente sobre o
// serviço de transporte. O perfil vive no cadastro da transportadora; o valor
// retido é calculado. Ordem XSD: vServ, vBCRet, pICMSRet, vICMSRet, CFOP, cMunFG.
func buildRetTransp(transport map[string]any) map[string]any {
	ret, _ := transport["freight_retention"].(map[string]any)
	if len(ret) == 0 {
		return nil
	}
	vServ := anyStr(ret, "v_serv", "")
	vBCRet := anyStr(ret, "v_bc_ret", "")
	pICMSRet := anyStr(ret, "p_icms_ret", "")
	if vServ == "" || vBCRet == "" || pICMSRet == "" {
		// O grupo é tudo-ou-nada no leiaute: meio preenchido seria recusado.
		return nil
	}
	return map[string]any{
		"vServ":    q2(d(vServ).RoundBank(2)),
		"vBCRet":   q2(d(vBCRet).RoundBank(2)),
		"pICMSRet": pICMSRet,
		"vICMSRet": q2(d(vBCRet).Mul(d(pICMSRet)).Div(decimal.NewFromInt(100)).RoundBank(2)),
		"CFOP":     anyStr(ret, "cfop", ""),
		"cMunFG":   anyStr(ret, "c_mun_fg", ""),
	}
}

// buildObsItem monta det/obsItem — a observação fiscal do item. O par
// campo/texto pode vir da tributação (nível 3) ou do próprio produto (nível 4);
// a tributação vence, porque é a mais específica do cenário.
func buildObsItem(cfg, item map[string]any) map[string]any {
	campo := anyStr(cfg, "obs_item_x_campo", anyStr(item, "obs_item_x_campo", ""))
	texto := anyStr(cfg, "obs_item_x_texto", anyStr(item, "obs_item_x_texto", ""))
	if campo == "" || texto == "" {
		return nil
	}
	return map[string]any{"obsCont": map[string]any{"@xCampo": campo, "xTexto": texto}}
}
