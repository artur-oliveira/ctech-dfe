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

// buildEnder monta TEndereco (enderEmit/enderDest). Endereço no exterior (UF "EX") sai com cMun 9999999, sem CEP,
// e com o país do cadastro (código BACEN e nome); no Brasil, cPais 1058. Tag vazia não sai: a emissão já recusou
// o cadastro sem município (requireAddress), nunca há código de município inventado.
func buildEnder(person map[string]any) map[string]any {
	address := services.FirstAddress(person)
	uf := anyStr(address, "state_federation", "")
	ender := map[string]any{
		"xLgr":    anyStr(address, "street", ""),
		"nro":     strOrDefault(anyStr(address, "number", ""), "S/N"),
		"xCpl":    anyStr(address, "complement", ""),
		"xBairro": anyStr(address, "neighborhood", ""),
		"cMun":    anyStr(address, "city_ibge_code", ""),
		"xMun":    anyStr(address, "city", ""),
		"UF":      uf,
		"CEP":     strings.ReplaceAll(anyStr(address, "postal_code", ""), "-", ""),
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if uf == ufExterior {
		ender["cMun"] = cMunExterior
		ender["CEP"] = ""
		ender["cPais"] = anyStr(address, addrCountryCode, "")
		ender["xPais"] = anyStr(address, addrCountryName, "")
	}
	if phone := services.FirstPhone(person); phone != "" {
		ender["fone"] = phone
	}
	for k, v := range ender {
		if v == "" {
			delete(ender, k)
		}
	}
	return ender
}

// buildAutXML builds the autXML list (CPF/CNPJ authorized to view this
// organization's NF-e XML) from the organization's authorized_xml_viewers
// attribute. Returns nil (key omitted) when the organization has none.
func buildAutXML(org map[string]any) []map[string]any {
	raw, _ := org["authorized_xml_viewers"].([]any)
	docs := make([]string, 0, len(raw))
	for _, v := range raw {
		if vm, ok := v.(map[string]any); ok {
			docs = append(docs, anyStr(vm, "cpf_cnpj", ""))
		}
	}
	return services.AutXMLNodes(docs)
}

// buildLocal monta TLocal (local de retirada/entrega). Local no exterior (UF "EX") sai com cMun 9999999 e o país
// informado; no Brasil, cPais 1058. CEP e IE são opcionais no leiaute e saem quando informados.
func buildLocal(l *NfeLocalBody) map[string]any {
	if l == nil {
		return nil
	}
	m := map[string]any{
		"CNPJ":    ptrStr(l.CNPJ),
		"CPF":     ptrStr(l.CPF),
		"xNome":   ptrStr(l.XNome),
		"xLgr":    l.XLgr,
		"nro":     l.Nro,
		"xCpl":    ptrStr(l.XCpl),
		"xBairro": l.XBairro,
		"cMun":    l.CMun,
		"xMun":    l.XMun,
		"UF":      l.UF,
		"CEP":     ptrStr(l.CEP),
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
		"fone":    ptrStr(l.Fone),
		"email":   ptrStr(l.Email),
		"IE":      ptrStr(l.IE),
	}
	if l.UF == ufExterior {
		m["cMun"] = cMunExterior
		m["CEP"] = ""
		m["cPais"] = ptrStr(l.CPais)
		m["xPais"] = ptrStr(l.XPais)
	}
	for k, v := range m {
		if v == "" {
			delete(m, k)
		}
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
	// C21a tpRegimeIBSCBS (NT 2026.005): só quem optou pelo regime regular do IBS e da
	// CBS no Simples (1) ou está com a opção pelo Simples pendente (2 a 4). Não vale
	// para o MEI nem para o Regime Normal comum.
	if tp := getAnyInt(orgPerson, "tp_regime_ibs_cbs", 0); validTpRegimeIBSCBS(orgCRT, tp) {
		emit["tpRegimeIBSCBS"] = fmt.Sprintf("%d", tp)
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
	dest["indIEDest"] = indIEDestNaoContrib
	switch {
	case destUF == ufExterior:
		// E16a-20 (790) e E17-40 (792): no exterior é sempre não contribuinte, sem IE.
	case strings.EqualFold(strings.TrimSpace(destIE), ieIsento):
		// E17-30 (791): o isento de inscrição sai com indIEDest 2 e sem a tag IE.
		dest["indIEDest"] = indIEDestIsento
	case destIE != "":
		dest["IE"] = destIE
		dest["indIEDest"] = indIEDestContrib
	}
	if email := services.FirstEmail(destPerson); email != "" {
		dest["email"] = email
	}
	return dest
}

// validTpRegimeIBSCBS: 1 para CRT 1 ou 2 com opção pelo regime regular; 2, 3 ou 4 para
// CRT 3 com opção pendente pelo Simples (NT 2026.005, seção 2.1).
func validTpRegimeIBSCBS(crt, tp int) bool {
	switch crt {
	case 1, 2:
		return tp == 1
	case 3:
		return tp >= 2 && tp <= 4
	}
	return false
}
