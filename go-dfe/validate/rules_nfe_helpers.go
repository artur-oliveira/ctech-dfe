package validate

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Helpers das regras de NF-e/NFC-e (rules_nfe.go). Tudo prefixado com nfe para não colidir com os outros modais.

// nfeTol é a tolerância de R$ 0,01 do MOC (nota (*4)), com folga para o erro de ponto flutuante.
const nfeTol = 0.011

// nfeCtx reúne o que quase toda regra precisa: o infNFe, a ide, os itens e o modelo.
type nfeCtx struct {
	doc, inf, ide, emit, dest map[string]any
	mod, tpImp, finNFe        string
	items                     []nfeItem
}

// nfeItem é um det com atalhos para o grupo de ICMS (o único filho de imposto/ICMS).
type nfeItem struct {
	n         int
	det, prod map[string]any
	imp       map[string]any
	icmsGrp   string
	icms      map[string]any
}

func newNFeCtx(doc map[string]any) *nfeCtx {
	inf := sub(doc, "infNFe")
	if inf == nil {
		return &nfeCtx{doc: doc}
	}
	c := &nfeCtx{doc: doc, inf: inf, ide: sub(inf, "ide"), emit: sub(inf, "emit"), dest: sub(inf, "dest")}
	c.mod = nfeS(c.ide, "mod")
	c.tpImp = nfeS(c.ide, "tpImp")
	c.finNFe = nfeS(c.ide, "finNFe")
	for i, det := range list(inf, "det") {
		it := nfeItem{n: i + 1, det: det, prod: sub(det, "prod"), imp: sub(det, "imposto")}
		if ic := sub(it.imp, "ICMS"); ic != nil {
			it.icmsGrp, it.icms = nfeChildGroup(ic)
		}
		c.items = append(c.items, it)
	}
	return c
}

// nfeChildGroup devolve o primeiro filho mapa (ignorando atributos) com o seu nome.
func nfeChildGroup(m map[string]any) (string, map[string]any) {
	for k, v := range m {
		if strings.HasPrefix(k, "@") {
			continue
		}
		if mm, ok := v.(map[string]any); ok {
			return k, mm
		}
	}
	return "", nil
}

// nfeS devolve o texto (sem espaços nas pontas) de m[path...]; "" se não existir.
func nfeS(m map[string]any, path ...string) string {
	if m == nil || len(path) == 0 {
		return ""
	}
	if len(path) > 1 {
		if m = sub(m, path[:len(path)-1]...); m == nil {
			return ""
		}
	}
	return strings.TrimSpace(scalar(m[path[len(path)-1]]))
}

// nfeHas diz se o nó (grupo ou tag) existe.
func nfeHas(m map[string]any, path ...string) bool {
	if m == nil || len(path) == 0 {
		return false
	}
	if len(path) > 1 {
		if m = sub(m, path[:len(path)-1]...); m == nil {
			return false
		}
	}
	v, ok := m[path[len(path)-1]]
	return ok && v != nil
}

// nfeF devolve o número em m[path...]; 0 se ausente ou inválido.
func nfeF(m map[string]any, path ...string) float64 {
	f, _ := nfeFok(m, path...)
	return f
}

func nfeFok(m map[string]any, path ...string) (float64, bool) {
	s := nfeS(m, path...)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func nfeRound2(f float64) float64 { return math.Round(f*100) / 100 }

func nfeDiff(a, b float64) bool { return math.Abs(a-b) > nfeTol }

func nfeIn(s string, set ...string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

// nfeSerieIn diz se a série (texto numérico) cai em algum dos intervalos [lo, hi].
func nfeSerieIn(serie string, ranges ...[2]int) bool {
	n, err := strconv.Atoi(serie)
	if err != nil {
		return false
	}
	for _, r := range ranges {
		if n >= r[0] && n <= r[1] {
			return true
		}
	}
	return false
}

// nfeNFCe6 é o escopo de "NFC-e ou NF-e com DANFE Simplificado Tipo 2 (tpImp=6)".
func (c *nfeCtx) nfeNFCe6() bool { return c.mod == "65" || c.tpImp == "6" }

func (c *nfeCtx) anyItem(f func(it nfeItem) bool) bool {
	for _, it := range c.items {
		if f(it) {
			return true
		}
	}
	return false
}

func (c *nfeCtx) sum(f func(it nfeItem) float64) float64 {
	var s float64
	for _, it := range c.items {
		s += f(it)
	}
	return s
}

func (it nfeItem) cfop() string   { return nfeS(it.prod, "CFOP") }
func (it nfeItem) hasISSQN() bool { return nfeHas(it.imp, "ISSQN") }
func (it nfeItem) cst() string {
	if s := nfeS(it.icms, "CST"); s != "" {
		return s
	}
	return nfeS(it.icms, "CSOSN")
}
func (it nfeItem) at(path ...string) string { return nfeS(it.det, path...) }
func (it nfeItem) f(path ...string) float64 { return nfeF(it.det, path...) }
func (it nfeItem) ic(tag string) float64    { return nfeF(it.icms, tag) }
func (it nfeItem) msg(f string, a ...any) string {
	return fmt.Sprintf("item %d: ", it.n) + fmt.Sprintf(f, a...)
}

// nfeChildOf devolve o grupo filho único de imposto/<tag> (PISAliq, COFINSOutr, ...).
func (it nfeItem) child(tag string) map[string]any {
	g := sub(it.imp, tag)
	if g == nil {
		return nil
	}
	_, m := nfeChildGroup(g)
	return m
}

// --- documentos ---

func nfeAllEqual(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func nfeDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// nfeCNPJ valida tamanho, zeros e os dois dígitos verificadores (módulo 11).
func nfeCNPJ(s string) bool {
	if !nfeDigits(s, 14) || nfeAllEqual(s) {
		return false
	}
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	dv := func(w []int) int {
		sum := 0
		for i, x := range w {
			sum += int(s[i]-'0') * x
		}
		if r := sum % 11; r >= 2 {
			return 11 - r
		}
		return 0
	}
	return dv(w1) == int(s[12]-'0') && dv(w2) == int(s[13]-'0')
}

// nfeCPF valida tamanho, sequências repetidas (111..., 222...) e os dois dígitos verificadores.
func nfeCPF(s string) bool {
	if !nfeDigits(s, 11) || nfeAllEqual(s) {
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

// nfeKeyDV calcula o dígito verificador (módulo 11, pesos 2 a 9) dos 43 primeiros dígitos da chave.
func nfeKeyDV(k43 string) int {
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

func nfeKeyOK(k string) bool {
	return nfeDigits(k, 44) && nfeKeyDV(k[:43]) == int(k[43]-'0')
}

// nfeGTIN valida o dígito de controle GS1 (GTIN-8, 12, 13 ou 14).
func nfeGTIN(s string) bool {
	if len(s) != 8 && len(s) != 12 && len(s) != 13 && len(s) != 14 {
		return false
	}
	if !nfeDigits(s, len(s)) {
		return false
	}
	sum, w := 0, 3
	for i := len(s) - 2; i >= 0; i-- {
		sum += int(s[i]-'0') * w
		w = 4 - w
	}
	return (10-sum%10)%10 == int(s[len(s)-1]-'0')
}

// nfeKey são os campos da chave de acesso de 44 dígitos.
type nfeKey struct{ cUF, aamm, doc, mod, serie, nNF, tpEmis, cNF string }

func nfeParseKey(k string) (nfeKey, bool) {
	if !nfeDigits(k, 44) {
		return nfeKey{}, false
	}
	return nfeKey{k[0:2], k[2:6], k[6:20], k[20:22], k[22:25], k[25:34], k[34:35], k[35:43]}, true
}

// nfeUFs mapeia a sigla ao código IBGE da UF.
var nfeUFs = map[string]string{
	"RO": "11", "AC": "12", "AM": "13", "RR": "14", "PA": "15", "AP": "16", "TO": "17",
	"MA": "21", "PI": "22", "CE": "23", "RN": "24", "PB": "25", "PE": "26", "AL": "27", "SE": "28", "BA": "29",
	"MG": "31", "ES": "32", "RJ": "33", "SP": "35", "PR": "41", "SC": "42", "RS": "43",
	"MS": "50", "MT": "51", "GO": "52", "DF": "53",
}

func nfeValidUFCode(code string) bool {
	for _, c := range nfeUFs {
		if c == code {
			return true
		}
	}
	return false
}

// nfeDate lê xs:date ou xs:dateTime e devolve a data (sem hora, no fuso do documento).
func nfeDate(s string) (time.Time, bool) {
	if len(s) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s[:10])
	return t, err == nil
}

func nfeDateTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

// nfeIDBase devolve o CNPJ base (8 dígitos) ou o CPF do grupo (emit, dest, transporta); "" se nenhum.
func nfeIDBase(m map[string]any) string {
	if v := nfeS(m, "CNPJ"); len(v) >= 8 {
		return v[:8]
	}
	return nfeS(m, "CPF")
}

// NFeCreditDebitForbidden são os grupos de imposto do item que a nota de crédito ou de débito (finNFe 5 ou 6) não
// leva: só IBS/CBS (e IS). Regra B25-80 (rejeição 1001). Exportado para que o builder da api omita exatamente estes.
var NFeCreditDebitForbidden = []string{"ICMS", "ISSQN", "IPI", "II", "PIS", "PISST", "COFINS", "COFINSST", "ICMSUFDest"}

// NFeCreditDebitAllowed devolve os grupos de NFeCreditDebitForbidden que as exceções da B25-80 liberam; nil quando
// a exceção 1 libera todos (e também o impostoDevol): tpNFCredito 03, 04 ou 06 e tpNFDebito 07. tpNFDebito 06
// (pagamento antecipado) libera o IPI (exceção 3) e, em nota emitida em 2026, PIS, PISST, COFINS e COFINSST
// (exceção 2). O ICMS da exceção 4 é só o ICMSPrevistoPagtoAntecip, que não é emitido.
func NFeCreditDebitAllowed(tpNFCredito, tpNFDebito string, year int) map[string]bool {
	if nfeIn(tpNFCredito, "03", "04", "06") || tpNFDebito == "07" {
		return nil
	}
	out := map[string]bool{}
	if tpNFDebito == "06" {
		out["IPI"] = true
		if year == 2026 {
			for _, g := range []string{"PIS", "PISST", "COFINS", "COFINSST"} {
				out[g] = true
			}
		}
	}
	return out
}

func nfeCreditDebitAllowed(c *nfeCtx) map[string]bool {
	year := 0
	if e, ok := nfeDate(nfeS(c.ide, "dhEmi")); ok {
		year = e.Year()
	}
	return NFeCreditDebitAllowed(nfeS(c.ide, "tpNFCredito"), nfeS(c.ide, "tpNFDebito"), year)
}

// nfeNoIsentoUFs são as UFs que não admitem destinatário contribuinte isento de IE (regra E16a-30, rejeição 805).
var nfeNoIsentoUFs = []string{"AL", "AM", "BA", "CE", "DF", "ES", "GO", "MG", "MS", "MT", "PB", "PE", "RJ", "RN", "RS", "SE", "SP"}
