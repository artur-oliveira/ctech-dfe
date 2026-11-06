package validate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Helpers das regras do CT-e (rules_cte.go). Todos prefixados com cte para não colidir com os de NF-e/MDF-e.

// cteTol é a tolerância de R$ 0,01 que o MOC do CT-e dá às identidades de valor.
const cteTol = 0.0101

// cteHomolXNome é a razão social obrigatória dos atores em homologação (G002 a G005).
const cteHomolXNome = "CTE EMITIDO EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"

// cteUF mapeia o código IBGE da UF (2 primeiras posições do cMun e da chave) para a sigla.
var cteUF = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL", "28": "SE", "29": "BA",
	"31": "MG", "32": "ES", "33": "RJ", "35": "SP", "41": "PR", "42": "SC", "43": "RS",
	"50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// cteEnder liga o ator ao nó de endereço (o leiaute usa um nome por ator).
var cteEnder = map[string]string{
	"emit": "enderEmit", "rem": "enderReme", "exped": "enderExped", "receb": "enderReceb", "dest": "enderDest", "toma4": "enderToma",
}

var (
	cteKeyRe  = regexp.MustCompile(`^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$`)
	cteCNPJRe = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)
	cteCPFRe  = regexp.MustCompile(`^[0-9]{11}$`)
)

// cteS devolve o escalar como string ("" se ausente).
func cteS(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	switch t := m[k].(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	}
	return ""
}

func cteMsg(format string, a ...any) []string { return []string{fmt.Sprintf(format, a...)} }

func cteIn(s string, set ...string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

// cteNormal: tpCTe 0 (normal) ou 3 (substituição), os tipos que levam infCTeNorm.
func cteNormal(inf map[string]any) bool { return cteIn(cteS(sub(inf, "ide"), "tpCTe"), "0", "3") }

// cteNFF: forma de emissão Regime Especial da NFF (tpEmis 3), exceção de várias regras.
func cteNFF(inf map[string]any) bool { return cteS(sub(inf, "ide"), "tpEmis") == "3" }

// cteActor devolve o nó do ator; toma4 vive dentro de ide.
func cteActor(inf map[string]any, tag string) map[string]any {
	if tag == "toma4" {
		return sub(inf, "ide", "toma4")
	}
	return sub(inf, tag)
}

// cteActorUF devolve a UF do endereço do ator.
func cteActorUF(inf map[string]any, tag string) string {
	return cteS(sub(cteActor(inf, tag), cteEnder[tag]), "UF")
}

// cteDoc devolve o CNPJ ou CPF do ator.
func cteDoc(a map[string]any) string {
	if v := cteS(a, "CNPJ"); v != "" {
		return v
	}
	return cteS(a, "CPF")
}

// cteTomador devolve a tag do ator tomador (rem, exped, receb, dest ou toma4) e se o grupo toma3/toma4 existe.
func cteTomador(inf map[string]any) (tag string, found bool) {
	ide := sub(inf, "ide")
	if t3 := sub(ide, "toma3"); t3 != nil {
		switch cteS(t3, "toma") {
		case "0":
			return "rem", true
		case "1":
			return "exped", true
		case "2":
			return "receb", true
		case "3":
			return "dest", true
		}
		return "", false
	}
	if sub(ide, "toma4") != nil {
		return "toma4", true
	}
	return "", false
}

// cteDV calcula o dígito verificador (módulo 11, pesos 2 a 9 da direita para a esquerda) dos 43 primeiros
// caracteres da chave. O valor do caractere é ord-48, o que cobre a chave alfanumérica.
func cteDV(k43 string) int {
	w, s := 2, 0
	for i := len(k43) - 1; i >= 0; i-- {
		s += int(k43[i]-'0') * w
		if w++; w > 9 {
			w = 2
		}
	}
	if r := s % 11; r >= 2 {
		return 11 - r
	}
	return 0
}

func cteAllSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return len(s) > 0
}

// cteCNPJOK valida CNPJ numérico ou alfanumérico: formato, zeros e os dois dígitos de controle.
func cteCNPJOK(s string) bool {
	if !cteCNPJRe.MatchString(s) || strings.Trim(s, "0") == "" {
		return false
	}
	dv := func(n int, w []int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-'0') * w[i]
		}
		if r := sum % 11; r >= 2 {
			return 11 - r
		}
		return 0
	}
	d1 := dv(12, []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	d2 := dv(13, []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	return int(s[12]-'0') == d1 && int(s[13]-'0') == d2
}

// cteCPFOK valida CPF: 11 dígitos, não repetidos, dois dígitos de controle.
func cteCPFOK(s string) bool {
	if !cteCPFRe.MatchString(s) || cteAllSame(s) {
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
	return int(s[9]-'0') == dv(9) && int(s[10]-'0') == dv(10)
}

// cteDocOK valida um identificador que pode ser CNPJ ou CPF conforme o comprimento.
func cteDocOK(s string) bool {
	if len(s) == 11 {
		return cteCPFOK(s)
	}
	return cteCNPJOK(s)
}

// cteKeyReason valida uma chave de acesso como a SEFAZ (formato, UF, mês, ano, CNPJ/CPF, modelo, número,
// tipo de emissão e DV) e devolve o motivo da recusa, ou "" se estiver válida. mod "" não confere o modelo.
func cteKeyReason(k, mod string, minYear int) string {
	if !cteKeyRe.MatchString(k) {
		return "formato inválido"
	}
	if _, ok := cteUF[k[:2]]; !ok {
		return "UF inválida"
	}
	yy, _ := strconv.Atoi(k[2:4])
	mm, _ := strconv.Atoi(k[4:6])
	if mm < 1 || mm > 12 {
		return "mês inválido"
	}
	if y := 2000 + yy; y < minYear || y > time.Now().Year() {
		return "ano inválido"
	}
	id := k[6:20]
	if !cteCNPJOK(id) && !(id[:3] == "000" && cteCPFOK(id[3:])) {
		return "CNPJ/CPF zerado ou inválido"
	}
	if mod != "" && k[20:22] != mod {
		return "modelo diferente de " + mod
	}
	if strings.Trim(k[25:34], "0") == "" {
		return "número zerado"
	}
	if k[34] == '0' {
		return "tipo de emissão inválido"
	}
	if int(k[43]-'0') != cteDV(k[:43]) {
		return "DV inválido"
	}
	return ""
}

func cteLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

// cteKeyBody compõe os 43 primeiros caracteres da chave a partir de ide e emit ("" se faltar componente).
func cteKeyBody(inf map[string]any) string {
	ide, emit := sub(inf, "ide"), sub(inf, "emit")
	dh := cteS(ide, "dhEmi")
	if len(dh) < 7 {
		return ""
	}
	id := cteS(emit, "CNPJ")
	if id == "" {
		id = "000" + cteS(emit, "CPF")
	}
	body := cteS(ide, "cUF") + dh[2:4] + dh[5:7] + id + cteS(ide, "mod") +
		cteLeft(cteS(ide, "serie"), 3) + cteLeft(cteS(ide, "nCT"), 9) + cteS(ide, "tpEmis") + cteS(ide, "cCT")
	if len(body) != 43 {
		return ""
	}
	return body
}

// cteNFeKeys devolve as chaves de infDoc/infNFe.
func cteNFeKeys(inf map[string]any) []string {
	var out []string
	for _, n := range list(sub(inf, "infCTeNorm", "infDoc"), "infNFe") {
		out = append(out, cteS(n, "chave"))
	}
	return out
}

// cteDocAntKeys devolve os chCTe de docAnt/emiDocAnt/idDocAnt/idDocAntEle.
func cteDocAntKeys(inf map[string]any) []string {
	var out []string
	for _, e := range list(sub(inf, "infCTeNorm", "docAnt"), "emiDocAnt") {
		for _, i := range list(e, "idDocAnt") {
			for _, el := range list(i, "idDocAntEle") {
				out = append(out, cteS(el, "chCTe"))
			}
		}
	}
	return out
}

// cteDup devolve a primeira chave repetida.
func cteDup(keys []string) string {
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			return k
		}
		seen[k] = true
	}
	return ""
}

// cteEmiYear devolve o ano de dhEmi (0 se ilegível).
func cteEmiYear(inf map[string]any) int {
	t, err := time.Parse(time.RFC3339, cteS(sub(inf, "ide"), "dhEmi"))
	if err != nil {
		return 0
	}
	return t.Year()
}

// cteDocBase devolve o CNPJ base (8) ou o CPF inteiro.
func cteDocBase(d string) string {
	if len(d) == 14 {
		return d[:8]
	}
	return d
}
