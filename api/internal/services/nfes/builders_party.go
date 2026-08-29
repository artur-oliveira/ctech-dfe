package nfes

// builders_party.go — nós de identificação de partes (endereços, locais de
// retirada/entrega e autXML). Extraído de builders_doc.go.

import (
	"fmt"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// getIEForUF returns the IE for the given UF from state_registrations list.
func getIEForUF(person map[string]any, uf string) string {
	if regs, ok := person["state_registrations"].([]any); ok && len(regs) > 0 {
		for _, r := range regs {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if rm["uf"] == uf {
				if v, ok := rm["state_registration"].(string); ok && v != "" {
					return v
				}
				if v, ok := rm["ie"].(string); ok && v != "" {
					return v
				}
			}
		}
		// fallback: first entry
		first := regs[0].(map[string]any)
		if v, ok := first["state_registration"].(string); ok && v != "" {
			return v
		}
		if v, ok := first["ie"].(string); ok && v != "" {
			return v
		}
	}
	if v, ok := person["state_registration"].(string); ok {
		return v
	}
	return ""
}

func buildEnder(person map[string]any) map[string]string {
	address := services.FirstAddress(person)
	uf := anyStr(address, "state_federation", "")
	ender := map[string]string{
		"xLgr":    anyStr(address, "street", ""),
		"nro":     strOrDefault(anyStr(address, "number", ""), "S/N"),
		"xBairro": anyStr(address, "neighborhood", ""),
		"cMun":    strOrDefault(anyStr(address, "city_ibge_code", ""), "0000000"),
		"xMun":    anyStr(address, "city", ""),
		"UF":      uf,
		"CEP":     strings.ReplaceAll(anyStr(address, "postal_code", ""), "-", ""),
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if phone := services.FirstPhone(person); phone != "" {
		ender["fone"] = phone
	}
	return ender
}

// buildAutXML builds the autXML list (CPF/CNPJ authorized to view this
// organization's NF-e XML) from the organization's authorized_xml_viewers
// attribute. Returns nil (key omitted) when the organization has none.
func buildAutXML(org map[string]any) []map[string]any {
	raw, _ := org["authorized_xml_viewers"].([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		doc := anyStr(vm, "cpf_cnpj", "")
		if doc == "" {
			continue
		}
		entry := map[string]any{}
		if len(doc) == 14 {
			entry["CNPJ"] = doc
		} else {
			entry["CPF"] = doc
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildLocal builds a TLocal-shaped map (local de retirada/entrega) — same
// field set for both, per xsd_order.py's "retirada"/"entrega" ordering.
// Unlike buildEnder (TEndereco), TLocal has no CEP.
func buildLocal(l *NfeLocalBody) map[string]any {
	if l == nil {
		return nil
	}
	m := map[string]any{
		"xLgr":    l.XLgr,
		"nro":     l.Nro,
		"xBairro": l.XBairro,
		"cMun":    l.CMun,
		"xMun":    l.XMun,
		"UF":      l.UF,
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if l.CNPJ != nil && *l.CNPJ != "" {
		m["CNPJ"] = *l.CNPJ
	}
	if l.CPF != nil && *l.CPF != "" {
		m["CPF"] = *l.CPF
	}
	if l.XNome != nil && *l.XNome != "" {
		m["xNome"] = *l.XNome
	}
	if l.XCpl != nil && *l.XCpl != "" {
		m["xCpl"] = *l.XCpl
	}
	if l.Fone != nil && *l.Fone != "" {
		m["fone"] = *l.Fone
	}
	if l.Email != nil && *l.Email != "" {
		m["email"] = *l.Email
	}
	return m
}

// cnpjPrefix é o prefixo do SK de pessoa jurídica no DynamoDB — destinatário,
// transportadora e intermediador decidem CNPJ vs CPF por ele.
//
// Não vale para o emitente: desde a ADR 0022 a partition key da organização é
// um company id e não carrega documento nenhum. Use services.IssuerDoc.
const cnpjPrefix = "CNPJ_"

func getPersonMap(entity map[string]any) map[string]any {
	if p, ok := entity["person"].(map[string]any); ok {
		return p
	}
	return map[string]any{}
}

// buildEmit monta o nó emit (emitente) a partir da organização.
func buildEmit(org, orgPerson map[string]any, orgPK, emitUF, destUF string, orgCRT int) map[string]any {
	// The issuer's document comes off the record, never off the key: since ADR
	// 0022 the key is a company id and carries none.
	emitDoc, isEmitPJ := services.IssuerDocMap(org, orgPK)
	emitKey := services.TagCPF
	if isEmitPJ {
		emitKey = services.TagCNPJ
	}
	emit := map[string]any{
		emitKey:     emitDoc,
		"xNome":     anyStr(org, "name", ""),
		"xFant":     anyStr(orgPerson, "fantasy_name", ""),
		"enderEmit": buildEnder(orgPerson),
		"IE":        getIEForUF(orgPerson, emitUF),
		"CRT":       fmt.Sprintf("%d", orgCRT),
	}
	// IEST só é informado na operação interestadual em que o emitente é
	// substituto tributário na UF de destino.
	if iest := getIESTForUF(orgPerson, destUF); iest != "" {
		emit["IEST"] = iest
	}
	// IM já existe no cadastro (person.nfse.im) por causa da NFS-e — a NF-e
	// mista só precisa lê-lo. CNAE é obrigatório quando IM está presente.
	if nfse, ok := orgPerson["nfse"].(map[string]any); ok {
		if im := anyStr(nfse, "im", ""); im != "" {
			emit["IM"] = im
			if cnae := anyStr(orgPerson, "cnae", ""); cnae != "" {
				emit["CNAE"] = cnae
			}
		}
	}
	if suframa := anyStr(orgPerson, "isuf_emit", ""); suframa != "" {
		emit["ISUFEmit"] = suframa
	}
	return emit
}

// getIESTForUF devolve a inscrição de substituto tributário na UF de destino.
// Mora na mesma lista de state_registrations que a IE: é a mesma inscrição, no
// mesmo cadastro, com outro papel — criar uma segunda lista duplicaria a UF.
func getIESTForUF(person map[string]any, uf string) string {
	regs, ok := person["state_registrations"].([]any)
	if !ok {
		return ""
	}
	for _, r := range regs {
		rm, ok := r.(map[string]any)
		if !ok || rm["uf"] != uf {
			continue
		}
		if v, ok := rm["ie_st"].(string); ok {
			return v
		}
	}
	return ""
}

// buildDest monta o nó dest. O documento do destinatário é um choice do XSD —
// CPF, CNPJ ou idEstrangeiro, nunca dois. Na NFC-e o consumidor é identificado
// só pelo documento, sem endereço, e pode ser omitido por inteiro.
func buildDest(receiver, destPerson map[string]any, receiverSK, destUF string, isNFCe bool, environment int, destIE string) map[string]any {
	if len(receiver) == 0 {
		return nil
	}
	dest := map[string]any{}
	switch {
	case strings.HasPrefix(receiverSK, repositories.SKPrefixForeign):
		// choice do XSD: idEstrangeiro exclui CPF e CNPJ.
		dest["idEstrangeiro"] = strings.TrimPrefix(receiverSK, repositories.SKPrefixForeign)
	case strings.HasPrefix(receiverSK, cnpjPrefix):
		dest["CNPJ"] = services.StripPKPrefix(receiverSK)
	default:
		dest["CPF"] = services.StripPKPrefix(receiverSK)
	}

	if isNFCe {
		dest["indIEDest"] = indIEDestNaoContrib
		return dest
	}

	receiverName := anyStr(receiver, "name", "")
	if environment != 1 {
		receiverName = homNameReceiver
	}
	dest["xNome"] = receiverName
	dest["enderDest"] = buildEnder(destPerson)
	if destIE != "" && destUF != ufExterior {
		dest["IE"] = destIE
		dest["indIEDest"] = indIEDestContrib
	} else {
		dest["indIEDest"] = indIEDestNaoContrib
	}
	if email := services.FirstEmail(destPerson); email != "" {
		dest["email"] = email
	}
	return dest
}
