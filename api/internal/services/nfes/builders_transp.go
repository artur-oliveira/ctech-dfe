package nfes

// builders_transp.go — nó transp da NF-e (modalidade de frete, transportadora,
// veículo e volumes). Extraído de builders_doc.go.

import (
	"github.com/shopspring/decimal"
)

// buildTransp builds the transp XML node.
//
// For own-transport freight modes the transportador is the issuer or the
// recipient itself, so its transporta data is taken from emitTransporta /
// destTransporta instead of the request-supplied transporta_* fields:
//   - modFrete "3" (próprio por conta do remetente)    → emitTransporta
//   - modFrete "4" (próprio por conta do destinatário)  → destTransporta
func buildTransp(hasPesoL, hasPesoB bool, totalPesoL, totalPesoB decimal.Decimal, transport, emitTransporta, destTransporta map[string]any) map[string]any {
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

	if transport != nil {
		veiculo := map[string]any{}
		if v := anyStr(transport, "veiculo_placa", ""); v != "" {
			veiculo["placa"] = v
		}
		if v := anyStr(transport, "veiculo_uf", ""); v != "" {
			veiculo["UF"] = v
		}
		if v := anyStr(transport, "veiculo_rntrc", ""); v != "" {
			veiculo["RNTRC"] = v
		}
		if len(veiculo) > 0 {
			transp["veicTransp"] = veiculo
		}
	}

	if hasPesoL || hasPesoB {
		vol := map[string]any{"qVol": qVolPadrao}
		if hasPesoL {
			vol["pesoL"] = totalPesoL.StringFixed(3)
		}
		if hasPesoB {
			vol["pesoB"] = totalPesoB.StringFixed(3)
		}
		transp["vol"] = vol
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
