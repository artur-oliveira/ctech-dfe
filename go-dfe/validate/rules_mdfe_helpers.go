package validate

import (
	"fmt"
	"strings"
	"time"
)

// Auxiliares das regras do MDF-e (rules_mdfe.go). Prefixo mdfe para não colidir com os de NF-e e CT-e.

// mdfeNow é o relógio das regras de data (F79, F80 e ano da chave); os testes o fixam.
var mdfeNow = time.Now

// Códigos de modal (ide/modal).
const (
	mdfeModalRodo  = "1"
	mdfeModalAereo = "2"
	mdfeModalAquav = "3"
)

// Tipos de emitente (ide/tpEmit) e de transportador (ide/tpTransp).
const (
	mdfeTpEmitPrestador = "1"
	mdfeTpEmitPropria   = "2"
	mdfeTpEmitGlobal    = "3"
	mdfeTpTranspETC     = "1"
	mdfeTpTranspTAC     = "2"
	mdfeTpTranspCTC     = "3"
	mdfeTpEmisNormal    = "1"
	mdfeTpEmisNFF       = "3" // Regime Especial da Nota Fiscal Fácil
	mdfeTpEmisFSDA      = "5" // FS-DA do documento referenciado (posição 35 da chave)
	mdfeUFExterior      = "EX"
)

// mdfeUFCode é o código IBGE da UF por sigla.
var mdfeUFCode = map[string]string{
	"RO": "11", "AC": "12", "AM": "13", "RR": "14", "PA": "15", "AP": "16", "TO": "17",
	"MA": "21", "PI": "22", "CE": "23", "RN": "24", "PB": "25", "PE": "26", "AL": "27", "SE": "28", "BA": "29",
	"MG": "31", "ES": "32", "RJ": "33", "SP": "35", "PR": "41", "SC": "42", "RS": "43",
	"MS": "50", "MT": "51", "GO": "52", "DF": "53",
}

// mdfeUFBorders são as divisas entre UFs (uma aresta por par; o espelho é montado em mdfeAdjacent).
var mdfeUFBorders = map[string]string{
	"AC": "AM RO", "AL": "PE SE BA", "AM": "RR PA MT RO", "AP": "PA", "BA": "SE PE PI TO GO MG ES",
	"CE": "RN PB PE PI", "DF": "GO MG", "ES": "MG RJ", "GO": "TO MG MS MT", "MA": "PI TO PA",
	"MG": "SP RJ MS", "MS": "MT SP PR", "MT": "RO PA TO", "PA": "RR TO", "PB": "RN PE", "PE": "PI",
	"PI": "TO", "PR": "SP SC", "RJ": "SP", "RS": "SC",
}

var mdfeAdjacent = func() map[string]map[string]bool {
	m := map[string]map[string]bool{}
	add := func(a, b string) {
		if m[a] == nil {
			m[a] = map[string]bool{}
		}
		m[a][b] = true
	}
	for a, bs := range mdfeUFBorders {
		for _, b := range strings.Fields(bs) {
			add(a, b)
			add(b, a)
		}
	}
	return m
}()

// mdfeStr devolve o texto escalar de m[key] sem espaços nas pontas.
func mdfeStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

func mdfeDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mdfeAllSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

// mdfeValidCNPJ confere CNPJ numérico ou alfanumérico (IN RFB 2229/2024; NT conjunta 2025.001): formato, não
// zerado e dígitos verificadores pelo valor ord-48 do caractere. Mesma regra do CT-e (cteCNPJOK).
func mdfeValidCNPJ(s string) bool { return cteCNPJOK(s) }

// mdfeValidCPF confere 11 dígitos, não repetido e dígitos verificadores.
func mdfeValidCPF(s string) bool {
	if len(s) != 11 || !mdfeDigits(s) || mdfeAllSame(s) {
		return false
	}
	dv := func(n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-'0') * (n + 1 - i)
		}
		if r := (sum * 10) % 11; r < 10 {
			return r
		}
		return 0
	}
	return dv(9) == int(s[9]-'0') && dv(10) == int(s[10]-'0')
}

// mdfeValidDoc valida um CPF (11) ou CNPJ (14) pelo tamanho.
func mdfeValidDoc(s string) bool {
	if len(s) == 11 {
		return mdfeValidCPF(s)
	}
	return mdfeValidCNPJ(s)
}

// mdfeKeyDV calcula o dígito verificador (módulo 11) dos 43 primeiros caracteres de uma chave de acesso.
func mdfeKeyDV(k43 string) int {
	sum, w := 0, 2
	for i := len(k43) - 1; i >= 0; i-- {
		sum += int(k43[i]-'0') * w
		if w++; w > 9 {
			w = 2
		}
	}
	if r := sum % 11; r >= 2 {
		return 11 - r
	}
	return 0
}

// mdfeKeyOK diz se a chave tem o formato do XSD: 44 posições, numéricas fora do CNPJ (posições 7 a 18 aceitam letra,
// CNPJ alfanumérico).
func mdfeKeyOK(k string) bool { return cteKeyRe.MatchString(k) }

// mdfeKeyDVOK diz se a chave tem o formato do XSD e o DV confere (o caractere vale ord-48, o que cobre a letra).
func mdfeKeyDVOK(k string) bool {
	return mdfeKeyOK(k) && mdfeKeyDV(k[:43]) == int(k[43]-'0')
}

// mdfeKeyReason devolve o motivo de a chave de um documento referenciado ser inválida ("" se válida), na lista do MOC:
// CNPJ/CPF zerado ou inválido, ano fora da faixa, mês inválido, modelo, número zerado, tipo de emissão, UF e DV.
func mdfeKeyReason(k, mod string, minYear int, tpEmisOK string) string {
	if !mdfeKeyOK(k) {
		return "formato"
	}
	if !mdfeKeyDVOK(k) {
		return "DV invalido"
	}
	valid := false
	for _, c := range mdfeUFCode {
		if c == k[:2] {
			valid = true
		}
	}
	if !valid {
		return "UF invalida"
	}
	year := 2000 + int(k[2]-'0')*10 + int(k[3]-'0')
	if year < minYear || year > mdfeNow().Year() {
		return "ano invalido"
	}
	if m := int(k[4]-'0')*10 + int(k[5]-'0'); m < 1 || m > 12 {
		return "mes invalido"
	}
	doc := k[6:20]
	if strings.HasPrefix(doc, "000") && mdfeValidCPF(doc[3:]) { // CPF vai com três zeros à esquerda
		doc = doc[3:]
	}
	if !mdfeValidDoc(doc) {
		return "CNPJ/CPF invalido"
	}
	if k[20:22] != mod {
		return "modelo invalido"
	}
	if strings.Trim(k[25:34], "0") == "" {
		return "numero zerado"
	}
	if !strings.Contains(tpEmisOK, k[34:35]) {
		return "tipo de emissao invalido"
	}
	return ""
}

// mdfeKeyMonths devolve ano*12+mês da chave (AAMM).
func mdfeKeyMonths(k string) int {
	if !mdfeKeyOK(k) {
		return 0
	}
	return (2000+int(k[2]-'0')*10+int(k[3]-'0'))*12 + int(k[4]-'0')*10 + int(k[5]-'0')
}

// mdfeEmi lê ide/dhEmi.
func mdfeEmi(ide map[string]any) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, mdfeStr(ide, "dhEmi"))
	return t, err == nil
}

// mdfeEmitDoc devolve o CNPJ ou CPF do emitente, só dígitos.
func mdfeEmitDoc(inf map[string]any) string {
	emit := sub(inf, "emit")
	if d := mdfeStr(emit, "CNPJ"); d != "" {
		return d
	}
	return mdfeStr(emit, "CPF")
}

// mdfeKeyOf devolve a chave de acesso do @Id sem o literal "MDFe".
func mdfeKeyOf(inf map[string]any) string {
	return strings.TrimPrefix(mdfeStr(inf, "@Id"), "MDFe")
}

// mdfeMuns devolve os infMunDescarga.
func mdfeMuns(inf map[string]any) []map[string]any {
	return list(sub(inf, "infDoc"), "infMunDescarga")
}

// mdfeKeys junta as chaves (ch) do grupo group (infCTe, infNFe, infMDFeTransp) de todos os municípios.
func mdfeKeys(inf map[string]any, group, ch string) []string {
	var out []string
	for _, mun := range mdfeMuns(inf) {
		for _, d := range list(mun, group) {
			out = append(out, mdfeStr(d, ch))
		}
	}
	return out
}

// mdfeDocCount é o total de DF-e transportados (CT-e, NF-e e MDF-e).
func mdfeDocCount(inf map[string]any) int {
	return len(mdfeKeys(inf, "infCTe", "chCTe")) + len(mdfeKeys(inf, "infNFe", "chNFe")) + len(mdfeKeys(inf, "infMDFeTransp", "chMDFe"))
}

func mdfeRodo(inf map[string]any) map[string]any { return sub(inf, "infModal", "rodo") }

func mdfeIsRodo(inf map[string]any) bool { return mdfeStr(sub(inf, "ide"), "modal") == mdfeModalRodo }

// mdfePagGate é a condição comum das regras F52 a F63: modal rodoviário e emitente prestador (1), CT-e globalizado (3)
// ou carga própria (2) que informou tpTransp.
func mdfePagGate(inf map[string]any) bool {
	if !mdfeIsRodo(inf) {
		return false
	}
	ide := sub(inf, "ide")
	switch mdfeStr(ide, "tpEmit") {
	case mdfeTpEmitPrestador, mdfeTpEmitGlobal:
		return true
	case mdfeTpEmitPropria:
		return mdfeStr(ide, "tpTransp") != ""
	}
	return false
}

// mdfeInfPag devolve os infANTT/infPag.
func mdfeInfPag(inf map[string]any) []map[string]any {
	return list(sub(mdfeRodo(inf), "infANTT"), "infPag")
}

// mdfePartyDoc devolve CPF ou CNPJ de um nó (o choice), e se havia algum informado.
func mdfePartyDoc(n map[string]any) (string, bool) {
	if v := mdfeStr(n, "CNPJ"); v != "" {
		return v, true
	}
	if v := mdfeStr(n, "CPF"); v != "" {
		return v, true
	}
	return "", false
}

// mdfeBadDocs avalia o CPF/CNPJ de cada nó e devolve um achado por documento inválido.
func mdfeBadDocs(nodes []map[string]any, what string) []string {
	var out []string
	for i, n := range nodes {
		if d, ok := mdfePartyDoc(n); ok && !mdfeValidDoc(d) {
			out = append(out, fmt.Sprintf("%s %d: CNPJ/CPF invalido", what, i+1))
		}
	}
	return out
}

// mdfeDup devolve o índice (1 baseado) de cada valor repetido.
func mdfeDup(vals []string) []int {
	seen := map[string]bool{}
	var out []int
	for i, v := range vals {
		if v == "" {
			continue
		}
		if seen[v] {
			out = append(out, i+1)
		}
		seen[v] = true
	}
	return out
}

// mdfeRule monta a regra sobre o conteúdo de infMDFe; sem infMDFe, nada a avaliar.
func mdfeRule(id string, cstat int, mode Mode, check func(inf map[string]any) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: "mdfe", Check: func(doc map[string]any) []string {
		inf := sub(doc, "infMDFe")
		if inf == nil {
			return nil
		}
		return check(inf)
	}}
}

// mdfeOne devolve msg quando cond vale.
func mdfeOne(cond bool, msg string) []string {
	if cond {
		return []string{msg}
	}
	return nil
}
