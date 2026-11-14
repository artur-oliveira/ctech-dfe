package validate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Regras de negócio da NF-e (55) e NFC-e (65) do MOC "Regras de Validação" que se provam só com o documento
// (sem banco, sem tabela externa, sem relógio). O ID é o da regra no MOC e o CStat a rejeição. O modo padrão é
// Warn; Block só para o que é determinístico e que a SEFAZ recusa com certeza (dígitos verificadores e somatórios
// exatos). As mensagens não repetem CPF/CNPJ/chaves (podem ser PII).

func init() { catalog = append(catalog, nfeRules...) }

var nfeRules = buildNFeRules()

func nfeRule(id string, cstat int, mode Mode, models string, fn func(c *nfeCtx) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: "nfe", Check: func(doc map[string]any) []string {
		c := newNFeCtx(doc)
		if c.inf == nil || c.mod == "" || (models != "55/65" && c.mod != models) { // sem ide/mod o schema já recusa
			return nil
		}
		return fn(c)
	}}
}

func nfeWhen(id string, cstat int, mode Mode, models, msg string, when func(c *nfeCtx) bool) Rule {
	return nfeRule(id, cstat, mode, models, func(c *nfeCtx) []string {
		if when(c) {
			return []string{msg}
		}
		return nil
	})
}

// nfeEach aplica fn a cada item; fn devolve a mensagem (sem o prefixo do item) ou "".
func nfeEach(id string, cstat int, mode Mode, models string, fn func(c *nfeCtx, it nfeItem) string) Rule {
	return nfeRule(id, cstat, mode, models, func(c *nfeCtx) []string {
		var out []string
		for _, it := range c.items {
			if m := fn(c, it); m != "" {
				out = append(out, it.msg("%s", m))
			}
		}
		return out
	})
}

// nfeTotalRule compara total/<path> com o somatório dos itens. Total ausente conta como 0 (grupo presente).
func nfeTotalRule(id string, cstat int, mode Mode, label string, path []string, sum func(c *nfeCtx) float64) Rule {
	return nfeRule(id, cstat, mode, "55/65", func(c *nfeCtx) []string {
		total := sub(c.inf, append([]string{"total"}, path[:len(path)-1]...)...)
		if total == nil {
			return nil
		}
		got, want := nfeF(total, path[len(path)-1]), sum(c)
		if nfeDiff(got, want) {
			return []string{fmt.Sprintf("total %s %.2f difere do somatório dos itens %.2f", label, got, want)}
		}
		return nil
	})
}

func dp(path ...string) func(it nfeItem) float64 {
	return func(it nfeItem) float64 { return it.f(path...) }
}
func ic(tag string) func(it nfeItem) float64 { return func(it nfeItem) float64 { return it.ic(tag) } }

func itemSum(f func(it nfeItem) float64) func(c *nfeCtx) float64 {
	return func(c *nfeCtx) float64 { return c.sum(f) }
}

func buildNFeRules() []Rule {
	var r []Rule
	add := func(x ...Rule) { r = append(r, x...) }
	add(nfeIdentRules()...)
	add(nfeNFCeRules()...)
	add(nfeParticipantRules()...)
	add(nfeItemRules()...)
	add(nfeICMSRules()...)
	add(nfeTotalRules()...)
	add(nfePaymentRules()...)
	add(nfeRefKeyRules()...)
	add(nfeMiscRules()...)
	return r
}

// --- identificação, chave de acesso e emissão ---

func nfeIdentRules() []Rule {
	forbiddenCNF := []string{"00000000", "11111111", "22222222", "33333333", "44444444", "55555555", "66666666",
		"77777777", "88888888", "99999999", "12345678", "23456789", "34567890", "45678901", "56789012", "67890123",
		"78901234", "89012345", "90123456", "01234567"}
	return []Rule{
		nfeRule("A03-10", 502, ModeBlock, "55/65", func(c *nfeCtx) []string {
			id := strings.TrimPrefix(nfeS(c.inf, "@Id"), "NFe")
			serie := nfeS(c.ide, "serie")
			if !nfeDigits(id, 44) || nfeSerieIn(serie, [2]int{890, 899}) { // NFA usa o CNPJ da SEFAZ
				return nil
			}
			doc := nfeS(c.emit, "CNPJ")
			if doc == "" {
				doc = "000" + nfeS(c.emit, "CPF")
			}
			dh := nfeS(c.ide, "dhEmi")
			if len(dh) < 7 || len(doc) != 14 || len(serie) > 3 || len(nfeS(c.ide, "nNF")) > 9 {
				return nil
			}
			want := nfeS(c.ide, "cUF") + dh[2:4] + dh[5:7] + doc + nfeS(c.ide, "mod") +
				fmt.Sprintf("%03s%09s", serie, nfeS(c.ide, "nNF")) + nfeS(c.ide, "tpEmis") + nfeS(c.ide, "cNF")
			want = strings.ReplaceAll(want, " ", "0")
			if len(want) == 43 && id[:43] != want {
				return []string{"chave de acesso do Id difere da concatenação de cUF, AAMM, CNPJ/CPF, mod, serie, nNF, tpEmis e cNF"}
			}
			return nil
		}),
		nfeRule("B23-10", 253, ModeBlock, "55/65", func(c *nfeCtx) []string {
			id := strings.TrimPrefix(nfeS(c.inf, "@Id"), "NFe")
			if nfeDigits(id, 44) && !nfeKeyOK(id) {
				return []string{"dígito verificador da chave de acesso inválido"}
			}
			return nil
		}),
		nfeWhen("B03-10", 897, ModeWarn, "55/65", "cNF inválido: sequência proibida ou igual ao número da nota", func(c *nfeCtx) bool {
			cnf := nfeS(c.ide, "cNF")
			if cnf == "" {
				return false
			}
			n, ok1 := nfeFok(c.ide, "nNF")
			k, ok2 := nfeFok(c.ide, "cNF")
			return nfeIn(cnf, forbiddenCNF...) || (ok1 && ok2 && n == k)
		}),
		nfeWhen("B26-10", 244, ModeWarn, "55/65", "procEmi 0 ou 3 com série fora das faixas 0-889 e 920-969", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "procEmi"), "0", "3") && !nfeSerieIn(nfeS(c.ide, "serie"), [2]int{0, 889}, [2]int{920, 969})
		}),
		nfeWhen("B26-20", 451, ModeWarn, "55/65", "procEmi 1 ou 2 com série fora da faixa 890-919", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "procEmi"), "1", "2") && !nfeSerieIn(nfeS(c.ide, "serie"), [2]int{890, 919})
		}),
		nfeWhen("B26-24", 667, ModeWarn, "55", "procEmi 4 (PAA) com série fora da faixa 970-989", func(c *nfeCtx) bool {
			return nfeS(c.ide, "procEmi") == "4" && !nfeSerieIn(nfeS(c.ide, "serie"), [2]int{970, 989})
		}),
		nfeWhen("B26-26", 867, ModeWarn, "55", "procEmi 4 (PAA) sem o grupo infPAA", func(c *nfeCtx) bool {
			return nfeS(c.ide, "procEmi") == "4" && !nfeHas(c.inf, "infPAA")
		}),
		nfeWhen("B26-28", 893, ModeWarn, "55", "grupo infPAA informado com procEmi diferente de 4", func(c *nfeCtx) bool {
			p := nfeS(c.ide, "procEmi")
			return p != "" && p != "4" && nfeHas(c.inf, "infPAA")
		}),
		nfeWhen("B26-30", 370, ModeWarn, "55/65", "procEmi 1 ou 2 com tpEmis diferente de 1, 6 e 7", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "procEmi"), "1", "2") && !nfeIn(nfeS(c.ide, "tpEmis"), "1", "6", "7")
		}),
		nfeWhen("B26-50", 957, ModeWarn, "65", "NFC-e com tpEmis diferente de 3 e procEmi diferente de 0", func(c *nfeCtx) bool {
			p := nfeS(c.ide, "procEmi")
			return nfeS(c.ide, "tpEmis") != "3" && p != "" && p != "0" && !nfeIn(p, "1", "2")
		}),
		nfeWhen("B28-10", 556, ModeWarn, "55/65", "emissão normal com dhCont ou xJust informados", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpEmis") == "1" && (nfeHas(c.ide, "dhCont") || nfeHas(c.ide, "xJust"))
		}),
		nfeWhen("B28-20", 557, ModeWarn, "55/65", "contingência sem dhCont e xJust", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "tpEmis"), "2", "4", "5", "9") && (!nfeHas(c.ide, "dhCont") || !nfeHas(c.ide, "xJust"))
		}),
		nfeWhen("B22-10", 711, ModeWarn, "55", "contingência off-line (tpEmis 9) com tpImp diferente de 6", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpEmis") == "9" && c.tpImp != "" && c.tpImp != "6"
		}),
		nfeWhen("B21-20", 710, ModeWarn, "55", "NF-e com tipo de impressão 4 ou 5", func(c *nfeCtx) bool {
			return nfeIn(c.tpImp, "4", "5")
		}),
		nfeWhen("B21-10", 709, ModeWarn, "65", "NFC-e com tipo de impressão diferente de 4 e 5", func(c *nfeCtx) bool {
			return c.tpImp != "" && !nfeIn(c.tpImp, "4", "5")
		}),
		nfeWhen("B22-34", 714, ModeWarn, "65", "NFC-e com opção de contingência inválida (tpEmis 2, 4 ou 5)", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "tpEmis"), "2", "4", "5")
		}),
		nfeWhen("D01-10", 403, ModeWarn, "55/65", "grupo avulsa informado com procEmi diferente de 1 e 2", func(c *nfeCtx) bool {
			p := nfeS(c.ide, "procEmi")
			return nfeHas(c.inf, "avulsa") && p != "" && !nfeIn(p, "1", "2")
		}),
		nfeWhen("D01-20", 369, ModeWarn, "55/65", "procEmi 1 ou 2 sem o grupo avulsa", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.ide, "procEmi"), "1", "2") && !nfeHas(c.inf, "avulsa")
		}),
		nfeWhen("B12-20", 271, ModeWarn, "55/65", "código do município do fato gerador não começa com o código da UF do emitente", func(c *nfeCtx) bool {
			m, u := nfeS(c.ide, "cMunFG"), nfeS(c.ide, "cUF")
			return len(m) >= 2 && u != "" && m[:2] != u
		}),
		nfeWhen("C10-20", 273, ModeWarn, "55/65", "código do município do emitente não começa com o código da UF do emitente", func(c *nfeCtx) bool {
			m, u := nfeS(c.emit, "enderEmit", "cMun"), nfeUFs[nfeS(c.emit, "enderEmit", "UF")]
			return len(m) >= 2 && u != "" && m[:2] != u
		}),
		nfeWhen("B10-40", 506, ModeWarn, "55", "data de saída anterior à data de emissão em NF-e de saída", func(c *nfeCtx) bool {
			s, ok1 := nfeDateTime(nfeS(c.ide, "dhSaiEnt"))
			e, ok2 := nfeDateTime(nfeS(c.ide, "dhEmi"))
			return nfeS(c.ide, "tpNF") == "1" && ok1 && ok2 && s.Before(e)
		}),
		nfeRule("B10a-30", 1155, ModeWarn, "55", func(c *nfeCtx) []string {
			p, ok1 := nfeDate(nfeS(c.ide, "dPrevEntrega"))
			ref, ok2 := nfeRefDate(c)
			if ok1 && ok2 && p.Before(ref) {
				return []string{"data de previsão de entrega anterior à data de saída (ou de emissão)"}
			}
			return nil
		}),
		nfeRule("B10a-20", 1154, ModeWarn, "55", func(c *nfeCtx) []string {
			p, ok1 := nfeDate(nfeS(c.ide, "dPrevEntrega"))
			ref, ok2 := nfeRefDate(c)
			if ok1 && ok2 && p.After(ref.AddDate(0, 3, 0)) {
				return []string{"data de previsão de entrega superior a 3 meses da data de saída (ou de emissão)"}
			}
			return nil
		}),
		nfeWhen("B10a-40", 1156, ModeWarn, "55", "previsão de entrega com finNFe diferente de 1 e 4", func(c *nfeCtx) bool {
			return nfeHas(c.ide, "dPrevEntrega") && c.finNFe != "" && !nfeIn(c.finNFe, "1", "4")
		}),
		nfeWhen("B10a-50", 1157, ModeWarn, "55", "previsão de entrega com modFrete 1, 4 ou 9", func(c *nfeCtx) bool {
			return nfeHas(c.ide, "dPrevEntrega") && nfeIn(nfeS(c.inf, "transp", "modFrete"), "1", "4", "9")
		}),
		nfeWhen("B25-30", 254, ModeWarn, "55", "NF-e complementar ou de crédito (1, 3, 4) sem NF referenciada", func(c *nfeCtx) bool {
			return nfeNeedsOneRef(c) && len(list(c.ide, "NFref")) == 0
		}),
		nfeWhen("B25-40", 255, ModeWarn, "55", "NF-e complementar, devolução ou crédito (1, 3, 4) com mais de uma NF referenciada", func(c *nfeCtx) bool {
			return (nfeNeedsOneRef(c) || c.finNFe == "4") && len(list(c.ide, "NFref")) > 1
		}),
		nfeWhen("B25-65", 1027, ModeWarn, "55", "NF-e de crédito tipo 2 com NF referenciada", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpNFCredito") == "02" && len(list(c.ide, "NFref")) > 0
		}),
		nfeWhen("B25-110", 1161, ModeWarn, "55", "NF-e de crédito (finNFe 5) com tipo de operação diferente de entrada", func(c *nfeCtx) bool {
			return c.finNFe == "5" && nfeS(c.ide, "tpNF") == "1"
		}),
		nfeWhen("B25-120", 1162, ModeWarn, "55", "NF-e de débito (finNFe 6) com tipo de operação diferente de saída", func(c *nfeCtx) bool {
			return c.finNFe == "6" && nfeS(c.ide, "tpNF") == "0"
		}),
		nfeWhen("B25.1-10", 1139, ModeWarn, "55", "tpNFDebito informado em nota que não é de débito (finNFe diferente de 6)", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpNFDebito") != "" && c.finNFe != "6"
		}),
		nfeWhen("B25.1-20", 1009, ModeWarn, "55", "nota de débito (finNFe 6) sem tpNFDebito", func(c *nfeCtx) bool {
			return c.finNFe == "6" && nfeS(c.ide, "tpNFDebito") == ""
		}),
		nfeWhen("B25.2-10", 1163, ModeWarn, "55", "tpNFCredito informado em nota que não é de crédito (finNFe diferente de 5)", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpNFCredito") != "" && c.finNFe != "5"
		}),
		nfeWhen("B25.2-20", 1164, ModeWarn, "55", "nota de crédito (finNFe 5) sem tpNFCredito", func(c *nfeCtx) bool {
			return c.finNFe == "5" && nfeS(c.ide, "tpNFCredito") == ""
		}),
		nfeWhen("B25.2-30", 1145, ModeWarn, "55", "tpNFCredito 02 só pode ser usado a partir de janeiro de 2029", func(c *nfeCtx) bool {
			e, ok := nfeDate(nfeS(c.ide, "dhEmi"))
			return ok && nfeS(c.ide, "tpNFCredito") == "02" && e.Year() < 2029
		}),
		nfeWhen("B25.2-40", 1152, ModeWarn, "55", "nota de crédito de retorno (tpNFCredito 03) com tipo de operação diferente de entrada", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpNFCredito") == "03" && nfeS(c.ide, "tpNF") != "0"
		}),
		nfeWhen("B25-100", 1003, ModeWarn, "55/65", "nota de crédito (finNFe 5) referencia documento que não é NF-e modelo 55", func(c *nfeCtx) bool {
			if c.finNFe != "5" {
				return false
			}
			allowed := []string{"55"}
			if nfeS(c.ide, "tpNFCredito") == "03" {
				allowed = append(allowed, "65")
			}
			for _, n := range list(c.ide, "NFref") {
				k, ok := nfeParseKey(nfeS(n, "refNFe"))
				if !ok || !nfeIn(k.mod, allowed...) {
					return true
				}
			}
			return false
		}),
		nfeWhen("B25-80", 1001, ModeWarn, "55/65", "finNFe 5 ou 6 (ou tpOperGov 2) com grupos de imposto informados", func(c *nfeCtx) bool {
			if !nfeIn(c.finNFe, "5", "6") && nfeS(c.ide, "gCompraGov", "tpOperGov") != "2" {
				return false
			}
			allowed := nfeCreditDebitAllowed(c)
			if allowed == nil { // exceção 1: todos os grupos são aceitos
				return false
			}
			return c.anyItem(func(it nfeItem) bool {
				for _, g := range NFeCreditDebitForbidden {
					if nfeHas(it.imp, g) && !allowed[g] {
						return true
					}
				}
				return nfeHas(it.det, "impostoDevol")
			})
		}),
		nfeWhen("B25-90", 1002, ModeWarn, "55/65", "item sem grupo ICMS e sem grupo ISSQN", func(c *nfeCtx) bool {
			if nfeIn(c.finNFe, "5", "6") || nfeS(c.ide, "gCompraGov", "tpOperGov") == "2" || (c.mod == "55" && !nfeHas(c.emit, "IE")) {
				return false
			}
			return c.anyItem(func(it nfeItem) bool { return !nfeHas(it.imp, "ICMS") && !it.hasISSQN() })
		}),
		nfeWhen("B34-10", 1140, ModeWarn, "55", "tpOperGov 2 sem NF-e referenciada (refNFe)", func(c *nfeCtx) bool {
			if nfeS(c.ide, "gCompraGov", "tpOperGov") != "2" {
				return false
			}
			return !c.anyRef()
		}),
		nfeWhen("B25b-40", 864, ModeWarn, "55", "indPres 5 sem refNFe e sem refNF", func(c *nfeCtx) bool {
			if nfeS(c.ide, "indPres") != "5" {
				return false
			}
			for _, n := range list(c.ide, "NFref") {
				if nfeHas(n, "refNFe") || nfeHas(n, "refNF") {
					return false
				}
			}
			return true
		}),
		nfeWhen("B25c-10", 434, ModeWarn, "55/65", "indIntermed obrigatório em venda normal de saída com indPres 1, 2, 3, 4 ou 9", func(c *nfeCtx) bool {
			if !nfeIn(nfeS(c.ide, "indPres"), "1", "2", "3", "4", "9") || nfeS(c.ide, "tpNF") != "1" || c.finNFe != "1" || nfeHas(c.ide, "indIntermed") {
				return false
			}
			return !c.anyItem(func(it nfeItem) bool {
				return nfeIn(it.cfop(), "5205", "5206", "5207", "5251", "5252", "5253", "5254", "5255", "5256", "5257", "5258", "5301", "5302", "5303", "5304", "5305", "5306", "5307")
			})
		}),
		nfeWhen("B25c-20", 435, ModeWarn, "55/65", "indIntermed informado com indPres fora de 1, 2, 3, 4 e 9", func(c *nfeCtx) bool {
			p := nfeS(c.ide, "indPres")
			return p != "" && !nfeIn(p, "1", "2", "3", "4", "9") && nfeHas(c.ide, "indIntermed")
		}),
		nfeWhen("YB01-10", 438, ModeWarn, "55/65", "indIntermed 1 sem o grupo infIntermed", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indIntermed") == "1" && !nfeHas(c.inf, "infIntermed")
		}),
		nfeWhen("YB01-20", 439, ModeWarn, "55/65", "grupo infIntermed informado com indIntermed diferente de 1", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indIntermed") != "1" && nfeHas(c.inf, "infIntermed")
		}),
		nfeWhen("YB02-10", 440, ModeBlock, "55/65", "CNPJ do intermediador inválido (zeros ou dígito verificador)", func(c *nfeCtx) bool {
			v := nfeS(c.inf, "infIntermed", "CNPJ")
			return v != "" && !nfeCNPJ(v)
		}),
		nfeWhen("ZE01-10", 819, ModeWarn, "55/65", "infSolicNFF informado com tpEmis diferente de 3 (NFF)", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpEmis") != "3" && nfeHas(c.inf, "infSolicNFF")
		}),
		nfeWhen("ZE01-20", 834, ModeWarn, "55/65", "tpEmis 3 (NFF) sem infSolicNFF", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpEmis") == "3" && !nfeHas(c.inf, "infSolicNFF")
		}),
		nfeWhen("ZD02-10", 973, ModeBlock, "55/65", "CNPJ do responsável técnico inválido (zeros ou dígito verificador)", func(c *nfeCtx) bool {
			v := nfeS(c.inf, "infRespTec", "CNPJ")
			return v != "" && !nfeCNPJ(v)
		}),
		nfeWhen("ZA01-20", 356, ModeWarn, "55", "grupo exporta informado em operação que não é de exportação (tpNF 0 ou idDest diferente de 3)", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "exporta") && (nfeS(c.ide, "tpNF") == "0" || (nfeS(c.ide, "idDest") != "" && nfeS(c.ide, "idDest") != "3"))
		}),
		nfeWhen("H02-10", 927, ModeWarn, "55/65", "nItem fora de ordem incremental a partir de 1", func(c *nfeCtx) bool {
			for _, it := range c.items {
				if it.det["@nItem"] != nil && nfeS(it.det, "@nItem") != fmt.Sprint(it.n) {
					return true
				}
			}
			return false
		}),
	}
}

func nfeRefDate(c *nfeCtx) (t time.Time, ok bool) {
	if t, ok = nfeDate(nfeS(c.ide, "dhSaiEnt")); ok {
		return
	}
	return nfeDate(nfeS(c.ide, "dhEmi"))
}

// nfeNeedsOneRef: finNFe 2 (complementar) ou NF-e de crédito tipos 1, 3 e 4.
func nfeNeedsOneRef(c *nfeCtx) bool {
	return c.finNFe == "2" || nfeIn(nfeS(c.ide, "tpNFCredito"), "01", "03", "04")
}

func (c *nfeCtx) anyRef() bool {
	for _, n := range list(c.ide, "NFref") {
		if nfeHas(n, "refNFe") {
			return true
		}
	}
	return false
}

// --- NFC-e e NF-e com DANFE Simplificado Tipo 2 ---

func nfeNFCeRules() []Rule {
	n6 := func(id string, cstat int, msg string, when func(c *nfeCtx) bool) Rule {
		return nfeRule(id, cstat, ModeWarn, "55/65", func(c *nfeCtx) []string {
			if c.nfeNFCe6() && when(c) {
				return []string{msg}
			}
			return nil
		})
	}
	r := []Rule{
		n6("B10-10", 705, "NFC-e/DANFE Tipo 2 com data de entrada/saída informada", func(c *nfeCtx) bool { return nfeHas(c.ide, "dhSaiEnt") }),
		n6("B11-10", 706, "NFC-e/DANFE Tipo 2 com operação de entrada (tpNF 0)", func(c *nfeCtx) bool { return nfeS(c.ide, "tpNF") == "0" }),
		n6("B11a-10", 707, "NFC-e/DANFE Tipo 2 com operação interestadual ou com exterior (idDest diferente de 1)", func(c *nfeCtx) bool {
			return nfeS(c.ide, "idDest") != "" && nfeS(c.ide, "idDest") != "1"
		}),
		n6("B25-20", 715, "NFC-e/DANFE Tipo 2 com finalidade diferente de 1", func(c *nfeCtx) bool { return c.finNFe != "" && c.finNFe != "1" }),
		n6("B25a-10", 716, "NFC-e/DANFE Tipo 2 com indFinal 0", func(c *nfeCtx) bool { return nfeS(c.ide, "indFinal") == "0" }),
		n6("B25b-20", 717, "NFC-e/DANFE Tipo 2 com indPres diferente de 1, 4 e 5", func(c *nfeCtx) bool {
			p := nfeS(c.ide, "indPres")
			return p != "" && !nfeIn(p, "1", "4", "5")
		}),
		n6("BA01-10", 708, "NFC-e/DANFE Tipo 2 não pode referenciar documentos (NFref)", func(c *nfeCtx) bool { return len(list(c.ide, "NFref")) > 0 }),
		n6("E01-20", 787, "NFC-e/DANFE Tipo 2 com indPres 4 sem identificação do destinatário", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indPres") == "4" && c.dest == nil
		}),
		n6("X02-10", 753, "NFC-e/DANFE Tipo 2 com frete que não é entrega a domicílio (modFrete diferente de 9 e indPres diferente de 4)", func(c *nfeCtx) bool {
			m := nfeS(c.inf, "transp", "modFrete")
			return m != "" && m != "9" && nfeS(c.ide, "indPres") != "4"
		}),
		n6("I17b-10", 774, "NFC-e/DANFE Tipo 2 com item fora do total (indTot 0)", func(c *nfeCtx) bool {
			return c.anyItem(func(it nfeItem) bool { return it.at("prod", "indTot") == "0" })
		}),
		n6("C18-10", 718, "NFC-e/DANFE Tipo 2 com IE de substituto tributário (IEST)", func(c *nfeCtx) bool { return nfeHas(c.emit, "IEST") }),
		n6("W16-30", 780, "NFC-e/DANFE Tipo 2 com total acima do limite padrão de R$ 200.000,00", func(c *nfeCtx) bool {
			return nfeF(c.inf, "total", "ICMSTot", "vNF") > 200000
		}),
		n6("W16-60", 752, "NFC-e/DANFE Tipo 2 acima de R$ 10.000,00 sem endereço do destinatário", func(c *nfeCtx) bool {
			return nfeF(c.inf, "total", "ICMSTot", "vNF") > 10000 && !nfeHas(c.dest, "enderDest")
		}),
	}
	// grupos proibidos em NFC-e / DANFE Tipo 2 (id, rejeição, caminho a partir do infNFe, descrição)
	for _, g := range []struct {
		id    string
		cstat int
		path  []string
		name  string
	}{
		{"F01-10", 669, []string{"retirada"}, "local de retirada"},
		{"G01-10", 670, []string{"entrega"}, "local de entrega"},
		{"X11-10", 755, []string{"transp", "retTransp"}, "retenção do ICMS no transporte"},
		{"X18-10", 756, []string{"transp", "veicTransp"}, "veículo de transporte"},
		{"X22-10", 757, []string{"transp", "reboque"}, "reboque"},
		{"X25a-10", 758, []string{"transp", "vagao"}, "vagão"},
		{"X25b-10", 759, []string{"transp", "balsa"}, "balsa"},
		{"Y01-10", 760, []string{"cobr"}, "cobrança"},
		{"ZB01-10", 762, []string{"compra"}, "compras"},
		{"ZC01-10", 763, []string{"cana"}, "aquisição de cana"},
	} {
		g := g
		r = append(r, n6(g.id, g.cstat, "NFC-e/DANFE Tipo 2 com grupo de "+g.name, func(c *nfeCtx) bool { return nfeHas(c.inf, g.path...) }))
	}
	// grupos de item proibidos em NFC-e / DANFE Tipo 2
	for _, g := range []struct {
		id    string
		cstat int
		path  []string
		name  string
	}{
		{"J01-10", 736, []string{"prod", "veicProd"}, "veículo novo"},
		{"L01-10", 738, []string{"prod", "arma"}, "armamento"},
		{"O01-10", 742, []string{"imposto", "IPI"}, "IPI"},
		{"P01-10", 743, []string{"imposto", "II"}, "II"},
		{"R01-10", 746, []string{"imposto", "PISST"}, "PIS-ST"},
		{"T01-10", 749, []string{"imposto", "COFINSST"}, "COFINS-ST"},
		{"UA01-20", 390, []string{"impostoDevol"}, "devolução de tributos"},
		{"NA01-10", 807, []string{"imposto", "ICMSUFDest"}, "ICMS UF de destino"},
	} {
		g := g
		r = append(r, n6(g.id, g.cstat, "NFC-e/DANFE Tipo 2 com grupo de "+g.name, func(c *nfeCtx) bool {
			return c.anyItem(func(it nfeItem) bool { return nfeHas(it.det, g.path...) })
		}))
	}
	r = append(r,
		nfeWhen("N12-50", 741, ModeWarn, "55/65", "NFC-e/DANFE Tipo 2 com partilha de ICMS (ICMSPart)", func(c *nfeCtx) bool {
			return c.nfeNFCe6() && c.anyItem(func(it nfeItem) bool { return it.icmsGrp == "ICMSPart" })
		}),
		nfeWhen("N12-60", 740, ModeWarn, "55/65", "NFC-e/DANFE Tipo 2 com repasse de ICMS-ST (ICMSST)", func(c *nfeCtx) bool {
			return c.nfeNFCe6() && c.anyItem(func(it nfeItem) bool { return it.icmsGrp == "ICMSST" })
		}),
		nfeWhen("B10a-10", 1153, ModeWarn, "65", "NFC-e com data de previsão de entrega", func(c *nfeCtx) bool { return nfeHas(c.ide, "dPrevEntrega") }),
		nfeWhen("B25d-10", 1099, ModeWarn, "65", "NFC-e com indicador de local da operação (cIndOp)", func(c *nfeCtx) bool { return nfeHas(c.ide, "cIndOp") }),
		nfeWhen("B25b-60", 1000, ModeWarn, "65", "NFC-e com cMunFGIBS fora de operação presencial fora do estabelecimento (indPres diferente de 5)", func(c *nfeCtx) bool {
			return nfeHas(c.ide, "cMunFGIBS") && nfeS(c.ide, "indPres") != "5"
		}),
		nfeWhen("B25b-70", 1005, ModeWarn, "65", "NFC-e com indPres 5 sem cMunFGIBS", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indPres") == "5" && !nfeHas(c.ide, "cMunFGIBS")
		}),
		nfeWhen("C17-42", 156, ModeWarn, "65", "NFC-e sem IE do emitente", func(c *nfeCtx) bool {
			y := nfeS(c.ide, "dhEmi")
			return !nfeHas(c.emit, "IE") && (len(y) < 4 || y[:4] < "2033")
		}),
		nfeWhen("E16a-10", 789, ModeWarn, "65", "NFC-e com indIEDest diferente de 9", func(c *nfeCtx) bool {
			v := nfeS(c.dest, "indIEDest")
			return v != "" && v != "9"
		}),
		nfeWhen("E17-10", 729, ModeWarn, "65", "NFC-e em operação com exterior com IE do destinatário", func(c *nfeCtx) bool {
			return nfeS(c.ide, "idDest") == "3" && nfeHas(c.dest, "IE")
		}),
		nfeWhen("E18-10", 730, ModeWarn, "65", "NFC-e com inscrição SUFRAMA do destinatário", func(c *nfeCtx) bool { return nfeHas(c.dest, "ISUF") }),
		nfeWhen("E02-20", 220, ModeWarn, "65", "NFC-e com CNPJ do destinatário igual ao do emitente", func(c *nfeCtx) bool {
			d := nfeS(c.dest, "CNPJ")
			return d != "" && d == nfeS(c.emit, "CNPJ")
		}),
		nfeWhen("E05-20", 788, ModeWarn, "65", "NFC-e com entrega a domicílio (indPres 4) sem endereço do destinatário", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indPres") == "4" && !nfeHas(c.dest, "enderDest")
		}),
		nfeWhen("X03-10", 754, ModeWarn, "65", "NFC-e com transportador que não é entrega a domicílio", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "transp", "transporta") && nfeS(c.ide, "indPres") != "4"
		}),
		nfeWhen("X03-20", 786, ModeWarn, "65", "NFC-e com entrega a domicílio sem transportador", func(c *nfeCtx) bool {
			return nfeS(c.ide, "indPres") == "4" && !nfeHas(c.inf, "transp", "transporta")
		}),
		nfeWhen("ZA01-30", 814, ModeWarn, "65", "NFC-e com grupo de exportação", func(c *nfeCtx) bool { return nfeHas(c.inf, "exporta") }),
		nfeWhen("BB01-10", 1006, ModeWarn, "65", "NFC-e com grupo gCompraGov", func(c *nfeCtx) bool { return nfeHas(c.ide, "gCompraGov") }),
		nfeWhen("C02a-04", 337, ModeWarn, "65", "NFC-e com CPF do emitente", func(c *nfeCtx) bool {
			return nfeHas(c.emit, "CPF") && nfeS(c.ide, "tpEmis") != "3"
		}),
		nfeWhen("W16-40", 750, ModeWarn, "65", "NFC-e acima de R$ 10.000,00 sem identificação do destinatário", func(c *nfeCtx) bool {
			return nfeF(c.inf, "total", "ICMSTot", "vNF") > 10000 && !nfeHas(c.dest, "CNPJ") && !nfeHas(c.dest, "CPF") && !nfeHas(c.dest, "idEstrangeiro")
		}),
		nfeWhen("YA09-20", 965, ModeWarn, "55/65", "valor do troco acima de R$ 300.000,00", func(c *nfeCtx) bool {
			return nfeF(c.inf, "pag", "vTroco") > 300000
		}),
		nfeEach("I08-150", 725, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			cf := it.cfop()
			if cf == "" || nfeIn(cf, "5101", "5102", "5103", "5104", "5115", "5405", "5656", "5667", "5910", "5933") {
				return ""
			}
			cst, uf := it.cst(), nfeS(c.emit, "enderEmit", "UF")
			if cf == "5949" && ((nfeIn(uf, "RS", "ES") && nfeIn(cst, "900", "90")) || (nfeIn(uf, "SP", "AM") && nfeIn(cst, "900", "40"))) {
				return ""
			}
			return "CFOP inválido para NFC-e"
		}),
		nfeEach("N12-30", 766, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if !c.nfeNFCe6() || it.icms == nil || it.icms["CST"] == nil || nfeIn(nfeS(it.icms, "CST"), "00", "20", "40", "41", "60", "61", "90") {
				return "" // 90 aceito: exceção 1 do MOC, a critério da UF
			}
			return "CST de ICMS não permitido em NFC-e/DANFE Tipo 2"
		}),
		nfeEach("N12a-20", 383, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if !c.nfeNFCe6() || it.icms == nil || it.icms["CSOSN"] == nil || nfeIn(nfeS(it.icms, "CSOSN"), "102", "103", "300", "400", "500", "900") {
				return ""
			}
			return "CSOSN não permitido em NFC-e/DANFE Tipo 2"
		}),
		nfeEach("N12-34", 381, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if c.nfeNFCe6() && nfeS(it.icms, "CST") == "90" && nfeHas(it.icms, "modBCST") {
				return "CST 90 com dados de ICMS-ST em NFC-e/DANFE Tipo 2"
			}
			return ""
		}),
		nfeEach("N12a-34", 385, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if c.nfeNFCe6() && nfeS(it.icms, "CSOSN") == "900" && nfeHas(it.icms, "modBCST") {
				return "CSOSN 900 com dados de ICMS-ST em NFC-e/DANFE Tipo 2"
			}
			return ""
		}),
		nfeEach("N12-40", 382, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(nfeS(it.icms, "CST"), "00", "20", "40", "41", "90") && !nfeIn(it.cfop(), "5101", "5102", "5103", "5104", "5115", "5910") {
				return "CST 00, 20, 40, 41 ou 90 com CFOP não permitido em NFC-e"
			}
			return ""
		}),
		nfeEach("N12-44", 382, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			if nfeS(it.icms, "CST") == "60" && !nfeIn(it.cfop(), "5405", "5656", "5667", "5910") {
				return "CST 60 com CFOP não permitido em NFC-e"
			}
			return ""
		}),
		nfeEach("N12a-40", 386, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(nfeS(it.icms, "CSOSN"), "102", "103", "300", "400", "900") && !nfeIn(it.cfop(), "5101", "5102", "5103", "5104", "5115", "5910") {
				return "CSOSN 102, 103, 300, 400 ou 900 com CFOP não permitido em NFC-e"
			}
			return ""
		}),
		nfeEach("N12a-44", 386, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			if nfeS(it.icms, "CSOSN") == "500" && !nfeIn(it.cfop(), "5405", "5656", "5667", "5910") {
				return "CSOSN 500 com CFOP não permitido em NFC-e"
			}
			return ""
		}),
		nfeEach("N12a-81", 782, ModeWarn, "65", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.emit, "CRT") == "4" && nfeS(c.ide, "idDest") != "3" && it.icms != nil && it.icms["CSOSN"] != nil && !nfeIn(nfeS(it.icms, "CSOSN"), "102", "300") {
				return "MEI em NFC-e aceita só CSOSN 102 e 300"
			}
			return ""
		}),
	)
	return r
}

// --- participantes ---

func nfeParticipantRules() []Rule {
	cnpj := func(id string, cstat int, who string, get func(c *nfeCtx) string) Rule {
		return nfeWhen(id, cstat, ModeBlock, "55/65", "CNPJ "+who+" inválido (zeros ou dígito verificador)", func(c *nfeCtx) bool {
			v := get(c)
			return v != "" && !nfeCNPJ(v)
		})
	}
	cpf := func(id string, cstat int, who string, get func(c *nfeCtx) string) Rule {
		return nfeWhen(id, cstat, ModeBlock, "55/65", "CPF "+who+" inválido (zeros, repetido ou dígito verificador)", func(c *nfeCtx) bool {
			v := get(c)
			return v != "" && !nfeCPF(v)
		})
	}
	at := func(path ...string) func(c *nfeCtx) string {
		return func(c *nfeCtx) string { return nfeS(c.inf, path...) }
	}
	uf := func(c *nfeCtx) bool { return nfeS(c.dest, "enderDest", "UF") == "EX" }
	comb := func(c *nfeCtx) bool {
		return c.anyItem(func(it nfeItem) bool { return nfeHas(it.prod, "comb", "UFCons") })
	}
	idDest := func(c *nfeCtx) string { return nfeS(c.ide, "idDest") }
	return []Rule{
		cnpj("C02-10", 207, "do emitente", at("emit", "CNPJ")),
		cpf("C02a-20", 401, "do emitente", at("emit", "CPF")),
		cnpj("E02-10", 208, "do destinatário", at("dest", "CNPJ")),
		cpf("E03-10", 237, "do destinatário", at("dest", "CPF")),
		cnpj("F02-10", 512, "do local de retirada", at("retirada", "CNPJ")),
		cpf("F02a-10", 540, "do local de retirada", at("retirada", "CPF")),
		cnpj("G02-10", 514, "do local de entrega", at("entrega", "CNPJ")),
		cpf("G02a-10", 541, "do local de entrega", at("entrega", "CPF")),
		cpf("X05-10", 543, "do transportador", at("transp", "transporta", "CPF")),
		nfeRule("YA05-20", 437, ModeBlock, "55/65", func(c *nfeCtx) []string {
			for _, p := range list(sub(c.inf, "pag"), "detPag") {
				if v := nfeS(p, "card", "CNPJ"); v != "" && !nfeCNPJ(v) {
					return []string{"CNPJ da instituição de pagamento inválido"}
				}
			}
			return nil
		}),
		nfeRule("YA03c-10", 961, ModeBlock, "55/65", func(c *nfeCtx) []string {
			for _, p := range list(sub(c.inf, "pag"), "detPag") {
				if v := nfeS(p, "CNPJPag"); v != "" && !nfeCNPJ(v) {
					return []string{"CNPJ transacional do pagamento inválido"}
				}
			}
			return nil
		}),
		nfeWhen("E01-10", 719, ModeWarn, "55", "NF-e sem a identificação do destinatário", func(c *nfeCtx) bool { return c.dest == nil }),
		nfeWhen("E03a-10", 720, ModeWarn, "55", "operação com exterior sem idEstrangeiro", func(c *nfeCtx) bool {
			return idDest(c) == "3" && !nfeHas(c.dest, "idEstrangeiro") &&
				!c.anyItem(func(it nfeItem) bool { return nfeIn(it.cfop(), "7552", "7501", "7667") })
		}),
		nfeWhen("E03a-20", 721, ModeWarn, "55", "idEstrangeiro informado fora de operação com exterior e sem consumidor final", func(c *nfeCtx) bool {
			return idDest(c) != "" && idDest(c) != "3" && nfeHas(c.dest, "idEstrangeiro") && nfeS(c.ide, "indFinal") != "1"
		}),
		nfeWhen("E03a-30", 925, ModeWarn, "55/65", "idEstrangeiro com IE do destinatário", func(c *nfeCtx) bool {
			return nfeS(c.dest, "idEstrangeiro") != "" && nfeHas(c.dest, "IE")
		}),
		nfeWhen("E03a-60", 372, ModeWarn, "55/65", "idEstrangeiro com caracteres fora do conjunto permitido", func(c *nfeCtx) bool {
			return !nfeForeignID.MatchString(nfeS(c.dest, "idEstrangeiro"))
		}),
		nfeWhen("E04-20", 598, ModeWarn, "55/65", "xNome do destinatário diferente do texto exigido em homologação", func(c *nfeCtx) bool {
			return nfeS(c.ide, "tpAmb") == "2" && c.dest != nil && nfeS(c.ide, "tpEmis") != "3" &&
				nfeS(c.dest, "xNome") != "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
		}),
		nfeWhen("E10-20", 275, ModeWarn, "55/65", "código do município do destinatário não começa com o código da UF do destinatário", func(c *nfeCtx) bool {
			m, u := nfeS(c.dest, "enderDest", "cMun"), nfeUFs[nfeS(c.dest, "enderDest", "UF")]
			return len(m) >= 2 && u != "" && m[:2] != u
		}),
		nfeWhen("E10-30", 509, ModeWarn, "55", "destinatário no exterior com município diferente de 9999999", func(c *nfeCtx) bool {
			return uf(c) && nfeS(c.dest, "enderDest", "cMun") != "9999999"
		}),
		nfeWhen("F07-10", 513, ModeWarn, "55/65", "retirada no exterior com município diferente de 9999999", func(c *nfeCtx) bool {
			return nfeS(c.inf, "retirada", "UF") == "EX" && nfeS(c.inf, "retirada", "cMun") != "9999999"
		}),
		nfeWhen("G07-10", 515, ModeWarn, "55/65", "entrega no exterior com município diferente de 9999999", func(c *nfeCtx) bool {
			return nfeS(c.inf, "entrega", "UF") == "EX" && nfeS(c.inf, "entrega", "cMun") != "9999999"
		}),
		nfeWhen("F07-30", 277, ModeWarn, "55/65", "município da retirada não começa com o código da UF da retirada", func(c *nfeCtx) bool {
			m, u := nfeS(c.inf, "retirada", "cMun"), nfeUFs[nfeS(c.inf, "retirada", "UF")]
			return len(m) >= 2 && u != "" && m[:2] != u
		}),
		nfeWhen("G07-30", 279, ModeWarn, "55/65", "município da entrega não começa com o código da UF da entrega", func(c *nfeCtx) bool {
			m, u := nfeS(c.inf, "entrega", "cMun"), nfeUFs[nfeS(c.inf, "entrega", "UF")]
			return len(m) >= 2 && u != "" && m[:2] != u
		}),
		nfeWhen("E12-10", 727, ModeWarn, "55", "operação com exterior com UF do destinatário diferente de EX", func(c *nfeCtx) bool {
			u := nfeS(c.dest, "enderDest", "UF")
			return idDest(c) == "3" && u != "" && u != "EX" && !comb(c)
		}),
		nfeWhen("E14-10", 510, ModeWarn, "55", "operação com exterior com país do destinatário Brasil ou ausente", func(c *nfeCtx) bool {
			p := strings.TrimLeft(nfeS(c.dest, "enderDest", "cPais"), "0")
			return idDest(c) == "3" && c.dest != nil && (p == "" || p == "1058") && !comb(c)
		}),
		nfeWhen("E14-20", 511, ModeWarn, "55/65", "país do destinatário diferente de Brasil fora de operação com exterior", func(c *nfeCtx) bool {
			p := strings.TrimLeft(nfeS(c.dest, "enderDest", "cPais"), "0")
			return idDest(c) != "" && idDest(c) != "3" && p != "" && p != "1058" && nfeS(c.dest, "idEstrangeiro") == ""
		}),
		nfeWhen("E14-30", 926, ModeWarn, "55/65", "destinatário com UF EX e país Brasil", func(c *nfeCtx) bool {
			return uf(c) && strings.TrimLeft(nfeS(c.dest, "enderDest", "cPais"), "0") == "1058"
		}),
		nfeRule("E12-30", 772, ModeWarn, "55", func(c *nfeCtx) []string { return nfeUFRule(c, "1", "2", true) }),
		nfeRule("E12-50", 772, ModeWarn, "55", func(c *nfeCtx) []string { return nfeUFRule(c, "0", "2", true) }),
		nfeRule("E12-40", 773, ModeWarn, "55", func(c *nfeCtx) []string { return nfeUFRule(c, "1", "1", false) }),
		nfeRule("E12-60", 773, ModeWarn, "55", func(c *nfeCtx) []string { return nfeUFRule(c, "0", "1", false) }),
		nfeWhen("E16a-20", 790, ModeWarn, "55", "operação com exterior com indIEDest diferente de 9", func(c *nfeCtx) bool {
			v := nfeS(c.dest, "indIEDest")
			return idDest(c) == "3" && v != "" && v != "9" && !comb(c)
		}),
		nfeWhen("E16a-40", 696, ModeWarn, "55/65", "indIEDest 9 sem consumidor final em saída que não é com exterior", func(c *nfeCtx) bool {
			return nfeS(c.dest, "indIEDest") == "9" && nfeS(c.ide, "indFinal") == "0" && nfeS(c.ide, "tpNF") == "1" && idDest(c) != "3" && idDest(c) != ""
		}),
		nfeWhen("E17-20", 728, ModeWarn, "55", "destinatário contribuinte (indIEDest 1) sem IE", func(c *nfeCtx) bool {
			return nfeS(c.dest, "indIEDest") == "1" && !nfeHas(c.dest, "IE")
		}),
		nfeWhen("E17-30", 791, ModeWarn, "55", "destinatário isento (indIEDest 2) com IE", func(c *nfeCtx) bool {
			return nfeS(c.dest, "indIEDest") == "2" && nfeS(c.dest, "IE") != ""
		}),
		nfeWhen("E16a-30", 805, ModeWarn, "55", "destinatário isento de IE (indIEDest 2) em UF que não admite contribuinte isento", func(c *nfeCtx) bool {
			if nfeS(c.dest, "indIEDest") != "2" || !nfeIn(idDest(c), "1", "2") || !nfeIn(nfeS(c.dest, "enderDest", "UF"), nfeNoIsentoUFs...) {
				return false
			}
			// exceções 1 e 2: destaque de ICMS-ST ou ST retida anteriormente em algum item
			if c.anyItem(func(it nfeItem) bool { return nfeF(it.icms, "vICMSST") > 0 || nfeHas(it.icms, "vICMSSTRet") }) {
				return false
			}
			// exceção 4: operação isenta, imune ou não tributada (todo item com ICMS em CST 40/41 ou CSOSN 103/300/400)
			return c.anyItem(func(it nfeItem) bool {
				return it.icms != nil && !nfeIn(nfeS(it.icms, "CST"), "40", "41") && !nfeIn(nfeS(it.icms, "CSOSN"), "103", "300", "400")
			})
		}),
		nfeWhen("E17-40", 792, ModeWarn, "55", "IE do destinatário com endereço no exterior", func(c *nfeCtx) bool { return uf(c) && nfeS(c.dest, "IE") != "" }),
		nfeWhen("C17-30", 554, ModeWarn, "55/65", "IE ISENTO no emitente com modelo 65 ou série fora de 890-919", func(c *nfeCtx) bool {
			return strings.EqualFold(nfeS(c.emit, "IE"), "ISENTO") && (c.mod == "65" || !nfeSerieIn(nfeS(c.ide, "serie"), [2]int{890, 919}))
		}),
		nfeWhen("C02-30", 503, ModeWarn, "55/65", "emitente CNPJ com série fora das faixas 0-909 e 980-989", func(c *nfeCtx) bool {
			s := nfeS(c.ide, "serie")
			return nfeHas(c.emit, "CNPJ") && s != "" && !nfeSerieIn(s, [2]int{0, 909}, [2]int{980, 989})
		}),
		nfeWhen("C02a-10", 495, ModeWarn, "55/65", "emitente CPF com série fora das faixas 890-899 e 910-979", func(c *nfeCtx) bool {
			s := nfeS(c.ide, "serie")
			return nfeHas(c.emit, "CPF") && nfeS(c.ide, "tpEmis") != "3" && s != "" && !nfeSerieIn(s, [2]int{890, 899}, [2]int{910, 979})
		}),
		nfeWhen("C18-40", 363, ModeWarn, "55", "IEST idêntica à IE do emitente ou do destinatário", func(c *nfeCtx) bool {
			s := nfeS(c.emit, "IEST")
			return s != "" && (s == nfeS(c.emit, "IE") || s == nfeS(c.dest, "IE"))
		}),
		nfeWhen("C18-50", 158, ModeWarn, "55", "IEST informada sem IE do emitente", func(c *nfeCtx) bool {
			return !nfeHas(c.emit, "IE") && nfeHas(c.emit, "IEST")
		}),
		nfeWhen("N12-20", 590, ModeWarn, "55/65", "CST de ICMS informado para CRT 1 ou 4 (use CSOSN)", func(c *nfeCtx) bool {
			return nfeIn(nfeS(c.emit, "CRT"), "1", "4") && c.anyItem(func(it nfeItem) bool {
				return it.icms != nil && it.icms["CST"] != nil && !nfeIn(nfeS(it.icms, "CST"), "02", "15", "53", "61")
			})
		}),
		nfeWhen("N12a-10", 591, ModeWarn, "55/65", "CSOSN informado para CRT diferente de 1 e 4", func(c *nfeCtx) bool {
			crt := nfeS(c.emit, "CRT")
			return crt != "" && !nfeIn(crt, "1", "4") && c.anyItem(func(it nfeItem) bool { return it.icms != nil && it.icms["CSOSN"] != nil })
		}),
		nfeEach("N12a-80", 782, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.emit, "CRT") == "4" && nfeS(c.ide, "idDest") != "3" && it.icms != nil && it.icms["CSOSN"] != nil && !nfeIn(nfeS(it.icms, "CSOSN"), "102", "300", "400", "900") {
				return "MEI em NF-e aceita só CSOSN 102, 300, 400 e 900"
			}
			return ""
		}),
		nfeWhen("U01-10", 530, ModeWarn, "55/65", "ISSQN informado sem a inscrição municipal do emitente", func(c *nfeCtx) bool {
			return c.anyItem(func(it nfeItem) bool { return it.hasISSQN() }) && !nfeHas(c.emit, "IM")
		}),
	}
}

var nfeForeignID = regexp.MustCompile(`^[A-Za-z0-9:.+\-/()]*$`)

// nfeUFRule: E12-30/40/50/60 (UF do destinatário contra a do emitente, por tpNF e idDest).
func nfeUFRule(c *nfeCtx, tpNF, idDest string, mustDiffer bool) []string {
	du, eu := nfeS(c.dest, "enderDest", "UF"), nfeS(c.emit, "enderEmit", "UF")
	if nfeS(c.ide, "tpNF") != tpNF || nfeS(c.ide, "idDest") != idDest || du == "" || eu == "" {
		return nil
	}
	if c.anyItem(func(it nfeItem) bool { return nfeHas(it.prod, "comb", "UFCons") }) {
		return nil
	}
	if mustDiffer { // interestadual: UF igual com CNPJ diferente
		if du == eu && nfeS(c.emit, "CNPJ") != nfeS(c.dest, "CNPJ") {
			return []string{"operação interestadual com UF do destinatário igual à do emitente"}
		}
		return nil
	}
	if nfeS(c.ide, "indFinal") != "1" && du != eu { // interna sem consumidor final
		return []string{"operação interna com UF do destinatário diferente da do emitente"}
	}
	return nil
}

// --- itens ---

func nfeItemRules() []Rule {
	cfop := func(it nfeItem) string {
		if len(it.cfop()) > 0 {
			return it.cfop()[:1]
		}
		return ""
	}
	noUFCons := func(it nfeItem) bool { return !nfeHas(it.prod, "comb", "UFCons") }
	importExc := []string{"3201", "3202", "3211", "3503", "3553"}
	return []Rule{
		nfeEach("I08-10", 518, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.ide, "tpNF") == "1" && nfeIn(cfop(it), "1", "2", "3") {
				return "CFOP de entrada em NF-e de saída"
			}
			return ""
		}),
		nfeEach("I08-20", 519, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.ide, "tpNF") == "0" && nfeIn(cfop(it), "5", "6", "7") {
				return "CFOP de saída em NF-e de entrada"
			}
			return ""
		}),
		nfeEach("I08-30", 731, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			d := nfeS(c.ide, "idDest")
			if nfeIn(cfop(it), "3", "7") && d != "" && d != "3" && noUFCons(it) {
				return "CFOP de operação com exterior e idDest diferente de 3"
			}
			return ""
		}),
		nfeEach("I08-40", 732, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			d := nfeS(c.ide, "idDest")
			if nfeIn(cfop(it), "2", "6") && d != "" && d != "2" {
				return "CFOP interestadual e idDest diferente de 2"
			}
			return ""
		}),
		nfeEach("I08-50", 733, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			d := nfeS(c.ide, "idDest")
			if nfeIn(cfop(it), "1", "5") && d != "" && d != "1" {
				return "CFOP interno e idDest diferente de 1"
			}
			return ""
		}),
		nfeEach("I08-60", 520, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			u := nfeS(c.dest, "enderDest", "UF")
			if nfeIn(cfop(it), "3", "7") && u != "" && u != "EX" && noUFCons(it) {
				return "CFOP de operação com exterior e UF do destinatário diferente de EX"
			}
			return ""
		}),
		nfeEach("I08-110", 525, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if cfop(it) == "3" && !nfeIn(it.cfop(), "3201", "3202", "3503", "3553") && !nfeHas(it.prod, "DI") {
				return "CFOP de importação sem a declaração de importação (DI)"
			}
			return ""
		}),
		nfeEach("I08-120", 597, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if cfop(it) == "3" && !nfeIn(it.cfop(), importExc...) && !nfeHas(it.imp, "IPI") {
				return "CFOP de importação sem o grupo de IPI"
			}
			return ""
		}),
		nfeEach("I08-130", 599, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if cfop(it) == "3" && !nfeIn(it.cfop(), importExc...) && !nfeHas(it.imp, "II") {
				return "CFOP de importação sem o grupo de II"
			}
			return ""
		}),
		nfeEach("I08-160", 374, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(it.cfop(), "1933", "2933", "5933", "6933") && !it.hasISSQN() {
				return "CFOP de serviço (x933) sem o grupo ISSQN"
			}
			return ""
		}),
		nfeEach("I08-170", 374, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.cfop() != "" && !nfeIn(it.cfop(), "1933", "2933", "5933", "6933") && it.hasISSQN() {
				return "grupo ISSQN com CFOP que não é de serviço (x933)"
			}
			return ""
		}),
		nfeEach("I08-184", 701, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(it.cfop(), "5929", "6929") && len(list(c.ide, "NFref")) == 0 {
				return "CFOP 5929/6929 sem documento referenciado"
			}
			return ""
		}),
		nfeEach("I08-190", 701, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(it.cfop(), "3503", "7501") && len(list(c.ide, "NFref")) == 0 && c.finNFe != "2" {
				return "CFOP de exportação indireta sem NF referenciada"
			}
			return ""
		}),
		nfeEach("I08-186", 953, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if !nfeIn(it.cfop(), "5929", "6929") {
				return ""
			}
			for _, n := range list(c.ide, "NFref") {
				if nfeHas(n, "refECF") {
					return "CFOP 5929/6929 com ECF referenciado"
				}
			}
			return ""
		}),
		nfeEach("I08-180", 375, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if !nfeIn(it.cfop(), "5929", "6929") {
				return ""
			}
			for _, n := range list(c.ide, "NFref") {
				if k := nfeS(n, "refNFe"); len(k) == 44 && k[20:22] == "65" {
					return "CFOP 5929/6929 com NFC-e referenciada"
				}
			}
			return ""
		}),
		nfeEach("I11-10", 629, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			q, ok1 := nfeFok(it.prod, "qCom")
			u, ok2 := nfeFok(it.prod, "vUnCom")
			v, ok3 := nfeFok(it.prod, "vProd")
			if c.finNFe == "1" && ok1 && ok2 && ok3 && nfeDiff(nfeRound2(q*u), v) {
				return fmt.Sprintf("vProd %.2f difere de qCom x vUnCom (%.2f)", v, nfeRound2(q*u))
			}
			return ""
		}),
		nfeEach("I11-20", 630, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			q, ok1 := nfeFok(it.prod, "qTrib")
			u, ok2 := nfeFok(it.prod, "vUnTrib")
			v, ok3 := nfeFok(it.prod, "vProd")
			if c.finNFe == "1" && ok1 && ok2 && ok3 && nfeDiff(nfeRound2(q*u), v) {
				return fmt.Sprintf("vProd %.2f difere de qTrib x vUnTrib (%.2f)", v, nfeRound2(q*u))
			}
			return ""
		}),
		nfeEach("I17-10", 483, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.f("prod", "vDesc") > it.f("prod", "vProd")+0.0001 {
				return "vDesc maior que vProd"
			}
			return ""
		}),
		nfeEach("I03-10", 611, ModeBlock, "55/65", func(c *nfeCtx, it nfeItem) string {
			g := it.at("prod", "cEAN")
			if g != "" && g != "SEM GTIN" && !nfeGTIN(g) {
				return "cEAN com dígito de controle inválido"
			}
			return ""
		}),
		nfeEach("I12-10", 612, ModeBlock, "55/65", func(c *nfeCtx, it nfeItem) string {
			g := it.at("prod", "cEANTrib")
			if g != "" && g != "SEM GTIN" && !nfeGTIN(g) {
				return "cEANTrib com dígito de controle inválido"
			}
			return ""
		}),
		nfeEach("I12-30", 885, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			a, b := it.at("prod", "cEAN"), it.at("prod", "cEANTrib")
			if a != "" && a != "SEM GTIN" && (b == "" || b == "SEM GTIN") {
				return "GTIN específico em cEAN e SEM GTIN em cEANTrib"
			}
			return ""
		}),
		nfeEach("I12-40", 886, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			a, b := it.at("prod", "cEAN"), it.at("prod", "cEANTrib")
			if b != "" && b != "SEM GTIN" && (a == "" || a == "SEM GTIN") {
				return "GTIN específico em cEANTrib e SEM GTIN em cEAN"
			}
			return ""
		}),
		nfeEach("I05-24", 471, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.at("prod", "NCM") == "00" && c.finNFe != "3" && !it.hasISSQN() {
				return "NCM 00 em item que não é serviço nem NF-e de ajuste"
			}
			return ""
		}),
		nfeEach("I05e-10", 879, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.at("prod", "indEscala") == "N" && it.at("prod", "CNPJFab") == "" {
				return "indEscala N sem CNPJ do fabricante"
			}
			return ""
		}),
		nfeEach("I05e-20", 489, ModeBlock, "55/65", func(c *nfeCtx, it nfeItem) string {
			if v := it.at("prod", "CNPJFab"); v != "" && !nfeCNPJ(v) {
				return "CNPJFab inválido (zeros ou dígito verificador)"
			}
			return ""
		}),
		nfeEach("I84-10", 870, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			for _, ra := range list(it.prod, "rastro") {
				f, v := nfeS(ra, "dFab"), nfeS(ra, "dVal")
				if f != "" && v != "" && v < f {
					return "data de validade anterior à data de fabricação"
				}
			}
			return ""
		}),
		nfeEach("I86-10", 820, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.ide, "tpEmis") != "3" && !nfeIn(nfeS(c.ide, "procEmi"), "1", "2") && nfeHas(it.prod, "infProdNFF") {
				return "infProdNFF informado fora de NFF"
			}
			return ""
		}),
		nfeEach("I87-10", 833, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if (nfeS(c.ide, "tpEmis") != "3" || nfeIn(nfeS(c.ide, "procEmi"), "1", "2")) && nfeHas(it.prod, "infProdEmb") {
				return "infProdEmb informado fora de NFF"
			}
			return ""
		}),
		nfeEach("I50-10", 336, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			d := nfeS(c.ide, "idDest")
			if nfeHas(it.prod, "detExport") && d != "" && d != "3" {
				return "grupo detExport em operação que não é com exterior"
			}
			return ""
		}),
		nfeEach("I23b-10", 330, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			for _, di := range list(it.prod, "DI") {
				if nfeS(di, "tpViaTransp") == "1" && !nfeHas(di, "vAFRMM") {
					return "importação marítima sem vAFRMM"
				}
			}
			return ""
		}),
		nfeEach("I23d-10", 331, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			for _, di := range list(it.prod, "DI") {
				if nfeIn(nfeS(di, "tpIntermedio"), "2", "3") && !nfeHas(di, "CNPJ") && !nfeHas(di, "CPF") {
					return "importação por conta e ordem/encomenda sem CNPJ ou CPF do adquirente"
				}
			}
			return ""
		}),
		nfeEach("I23e-10", 333, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			for _, di := range list(it.prod, "DI") {
				if nfeIn(nfeS(di, "tpIntermedio"), "2", "3") && !nfeHas(di, "UFTerceiro") {
					return "importação por conta e ordem/encomenda sem UF do adquirente"
				}
			}
			return ""
		}),
		nfeEach("I23d-20", 332, ModeBlock, "55", func(c *nfeCtx, it nfeItem) string {
			for _, di := range list(it.prod, "DI") {
				if v := nfeS(di, "CNPJ"); v != "" && !nfeCNPJ(v) {
					return "CNPJ do adquirente/encomendante inválido"
				}
			}
			return ""
		}),
		nfeEach("I23d1-10", 332, ModeBlock, "55", func(c *nfeCtx, it nfeItem) string {
			for _, di := range list(it.prod, "DI") {
				if v := nfeS(di, "CPF"); v != "" && !nfeCPF(v) {
					return "CPF do adquirente/encomendante inválido"
				}
			}
			return ""
		}),
		nfeEach("Q01-20", 745, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if !nfeHas(it.imp, "PIS") && !nfeIn(c.finNFe, "5", "6") && nfeS(c.ide, "gCompraGov", "tpOperGov") != "2" {
				return "item sem grupo de PIS"
			}
			return ""
		}),
		nfeEach("S01-20", 748, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if !nfeHas(it.imp, "COFINS") && !nfeIn(c.finNFe, "5", "6") && nfeS(c.ide, "gCompraGov", "tpOperGov") != "2" {
				return "item sem grupo de COFINS"
			}
			return ""
		}),
		nfeEach("O09-10", 388, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			cst, enq := it.at("imposto", "IPI", "IPINT", "CST"), it.at("imposto", "IPI", "cEnq")
			n, err := strconv.Atoi(enq)
			if cst == "" || err != nil {
				return ""
			}
			bad := (nfeIn(cst, "02", "52") && (n < 301 || n > 399)) ||
				(nfeIn(cst, "04", "54") && n > 99) ||
				(nfeIn(cst, "05", "55") && (n < 101 || n > 199))
			if bad {
				return "CST do IPI incompatível com o código de enquadramento legal (cEnq)"
			}
			return ""
		}),
		nfeEach("LA03c-10", 461, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.at("prod", "comb", "cProdANP") != "210203001" && (nfeHas(it.prod, "comb", "pGLP") || nfeHas(it.prod, "comb", "pGNn") || nfeHas(it.prod, "comb", "pGNi")) {
				return "percentuais de GLP/GN informados para produto diferente de GLP"
			}
			return ""
		}),
		nfeEach("LA03c-20", 855, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.at("prod", "comb", "cProdANP") == "210203001" {
				if s := it.f("prod", "comb", "pGLP") + it.f("prod", "comb", "pGNn") + it.f("prod", "comb", "pGNi"); nfeDiff(s, 100) {
					return fmt.Sprintf("pGLP + pGNn + pGNi = %.4f, deveria ser 100", s)
				}
			}
			return ""
		}),
		nfeEach("LA03d-10", 856, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if it.at("prod", "comb", "cProdANP") == "210203001" && !nfeHas(it.prod, "comb", "vPart") {
				return "GLP sem vPart"
			}
			return ""
		}),
		nfeEach("LA16-10", 380, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			i, ok1 := nfeFok(it.prod, "comb", "encerrante", "vEncIni")
			f, ok2 := nfeFok(it.prod, "comb", "encerrante", "vEncFin")
			if ok1 && ok2 && f <= i {
				return "encerrante final não é maior que o inicial"
			}
			return ""
		}),
		nfeEach("LA11-20", 379, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.prod, "comb", "encerrante") && !nfeIn(it.cfop(), "5656", "5667") {
				return "encerrante informado com CFOP que não é de venda de combustível a consumidor final"
			}
			return ""
		}),
	}
}

// --- ICMS por item ---

func nfeICMSRules() []Rule {
	calc := func(id string, cstat int, groups []string, label string, val, a, b string, perc bool) Rule {
		return nfeEach(id, cstat, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if c.finNFe != "1" || it.icms == nil || !nfeIn(it.cst(), groups...) {
				return ""
			}
			v, ok0 := nfeFok(it.icms, val)
			x, ok1 := nfeFok(it.icms, a)
			y, ok2 := nfeFok(it.icms, b)
			if !ok0 || !ok1 || !ok2 {
				return ""
			}
			want := x * y
			if perc {
				want /= 100
			}
			if nfeDiff(nfeRound2(want), v) {
				return fmt.Sprintf("%s %.2f difere do calculado %.2f", label, v, nfeRound2(want))
			}
			return ""
		})
	}
	return []Rule{
		calc("N17-20", 528, []string{"00", "10", "20", "70"}, "vICMS", "vICMS", "vBC", "pICMS", true),
		calc("N16a-10", 351, []string{"51"}, "vICMSOp", "vICMSOp", "vBC", "pICMS", true),
		calc("N16c-10", 352, []string{"51"}, "vICMSDif", "vICMSDif", "vICMSOp", "pDif", true),
		nfeEach("N17-10", 353, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if it.cst() != "51" || !nfeHas(it.icms, "vICMSOp") || !nfeHas(it.icms, "vICMS") {
				return ""
			}
			if nfeDiff(it.ic("vICMS"), it.ic("vICMSOp")-it.ic("vICMSDif")) {
				return "vICMS difere de vICMSOp menos vICMSDif (CST 51)"
			}
			return ""
		}),
		calc("N39-10", 962, []string{"02", "15"}, "vICMSMono", "vICMSMono", "qBCMono", "adRemICMS", false),
		calc("N45-10", 966, []string{"61"}, "vICMSMonoRet", "vICMSMonoRet", "qBCMonoRet", "adRemICMSRet", false),
		calc("N27d-10", 860, []string{"60", "500"}, "vFCPSTRet", "vFCPSTRet", "vBCFCPSTRet", "pFCPSTRet", true),
		nfeEach("N17c-10", 860, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			v, ok := nfeFok(it.icms, "vFCP")
			if c.finNFe != "1" || !ok {
				return ""
			}
			base := "vBCFCP"
			if it.cst() == "00" {
				base = "vBC"
			}
			b, okb := nfeFok(it.icms, base)
			p, okp := nfeFok(it.icms, "pFCP")
			if okb && okp && nfeDiff(nfeRound2(b*p/100), v) {
				return "vFCP difere da base x pFCP"
			}
			return ""
		}),
		nfeEach("N17b-10", 880, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.icms, "pFCP") && it.ic("pFCP") == 0 {
				return "pFCP igual a zero; não informe FCP para produto sem incidência"
			}
			return ""
		}),
		nfeEach("N23b-10", 881, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.icms, "pFCPST") && it.ic("pFCPST") == 0 {
				return "pFCPST igual a zero"
			}
			return ""
		}),
		nfeEach("N27b-10", 881, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.icms, "pFCPSTRet") && it.ic("pFCPSTRet") == 0 {
				return "pFCPSTRet igual a zero"
			}
			return ""
		}),
		nfeEach("N28-30", 627, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.icms, "motDesICMS") && it.ic("vICMSDeson") <= 0 {
				return "motDesICMS informado com vICMSDeson ausente ou zero"
			}
			return ""
		}),
		nfeEach("N28-12", 652, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(it.icms, "motDesICMS") == "7" && nfeS(it.icms, "indDeduzDeson") != "1" {
				return "desoneração SUFRAMA (motDesICMS 7) com indDeduzDeson diferente de 1"
			}
			return ""
		}),
		nfeEach("N28-10", 625, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if nfeS(it.icms, "motDesICMS") == "7" && !nfeHas(c.dest, "ISUF") && !nfeIn(cfopFirst(it), "1", "2") {
				return "desoneração SUFRAMA sem inscrição SUFRAMA do destinatário"
			}
			return ""
		}),
		nfeEach("N18-10", 932, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(it.icms, "modBCST") == "4" && !nfeHas(it.icms, "pMVAST") {
				return "modBCST 4 sem pMVAST"
			}
			return ""
		}),
		nfeEach("N18-20", 933, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			m := nfeS(it.icms, "modBCST")
			if m != "" && m != "4" && nfeHas(it.icms, "pMVAST") {
				return "pMVAST informado com modBCST diferente de 4"
			}
			return ""
		}),
		nfeEach("N37a-10", 767, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeIn(it.cst(), "02", "15", "53") && it.icmsGrp != "" && !nfeHas(it.icms, "qBCMono") {
				return "CST monofásico sem quantidade tributada (qBCMono)"
			}
			return ""
		}),
		nfeEach("N39a-10", 768, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if it.cst() == "15" && !nfeHas(it.icms, "qBCMonoReten") {
				return "CST 15 sem quantidade sujeita a retenção (qBCMonoReten)"
			}
			return ""
		}),
		nfeEach("N43a-10", 769, ModeWarn, "55/65", func(c *nfeCtx, it nfeItem) string {
			if it.cst() == "61" && !nfeHas(it.icms, "qBCMonoRet") {
				return "CST 61 sem quantidade retida anteriormente (qBCMonoRet)"
			}
			return ""
		}),
		nfeEach("N23-10", 806, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			cst := it.cst()
			need := nfeIn(cst, "10", "30", "60", "70", "201", "202", "203", "500") ||
				(nfeIn(cst, "90", "900") && it.ic("vICMSST") != 0)
			if need && it.icmsGrp != "ICMSPart" && !nfeHas(it.prod, "CEST") {
				return "CST/CSOSN com substituição tributária sem CEST"
			}
			return ""
		}),
		nfeEach("NA13-10", 793, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			g := sub(it.imp, "ICMSUFDest")
			v, ok := nfeFok(g, "vFCPUFDest")
			b, okb := nfeFok(g, "vBCFCPUFDest")
			p, okp := nfeFok(g, "pFCPUFDest")
			if ok && okb && okp && nfeDiff(nfeRound2(b*p/100), v) {
				return "vFCPUFDest difere de vBCFCPUFDest x pFCPUFDest"
			}
			return ""
		}),
		nfeEach("NA01-30", 695, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeHas(it.imp, "ICMSUFDest") && (nfeS(c.ide, "idDest") != "2" || nfeS(c.ide, "indFinal") != "1" || nfeS(c.dest, "indIEDest") != "9") {
				return "ICMSUFDest informado fora de operação interestadual a consumidor final não contribuinte"
			}
			return ""
		}),
		nfeEach("NA01-20", 694, ModeWarn, "55", func(c *nfeCtx, it nfeItem) string {
			if nfeS(c.ide, "idDest") == "2" && nfeS(c.ide, "indFinal") == "1" && nfeS(c.dest, "indIEDest") == "9" &&
				nfeS(c.ide, "tpNF") == "1" && c.finNFe != "4" && it.icms != nil && it.icmsGrp != "ICMSPart" &&
				!it.hasISSQN() && !nfeHas(it.prod, "comb") && !nfeHas(it.imp, "ICMSUFDest") &&
				nfeS(c.inf, "entrega", "UF") != nfeS(c.emit, "enderEmit", "UF") {
				return "falta o grupo ICMSUFDest em operação interestadual a consumidor final não contribuinte"
			}
			return ""
		}),
	}
}

func cfopFirst(it nfeItem) string {
	if s := it.cfop(); s != "" {
		return s[:1]
	}
	return ""
}

// --- totais ---

func nfeTotalRules() []Rule {
	icmsTot := func(id string, cstat int, tag string, f func(it nfeItem) float64) Rule {
		return nfeTotalRule(id, cstat, ModeWarn, tag, []string{"ICMSTot", tag}, itemSum(f))
	}
	blockTot := func(id string, cstat int, tag string, f func(it nfeItem) float64) Rule {
		r := icmsTot(id, cstat, tag, f)
		r.Mode = ModeWarn // soma com tolerância e exceções: aviso, não bloqueio
		return r
	}
	issqn := func(id string, cstat int, tag string, f func(it nfeItem) float64) Rule {
		return nfeTotalRule(id, cstat, ModeWarn, tag, []string{"ISSQNtot", tag}, itemSum(func(it nfeItem) float64 {
			if !it.hasISSQN() {
				return 0
			}
			return f(it)
		}))
	}
	iss := func(tag string) func(it nfeItem) float64 { return dp("imposto", "ISSQN", tag) }
	pisOf := func(tag, field string) func(it nfeItem) float64 {
		return func(it nfeItem) float64 { return nfeF(it.child(tag), field) }
	}
	ibs := func(id string, cstat int, label string, totPath []string, itemPath ...string) Rule {
		return nfeTotalRule(id, cstat, ModeWarn, label, append([]string{"IBSCBSTot"}, totPath...), itemSum(dp(append([]string{"imposto", "IBSCBS"}, itemPath...)...)))
	}
	g := func(s ...string) []string { return s }
	r := []Rule{
		nfeTotalRule("W07-10", 564, ModeWarn, "vProd", []string{"ICMSTot", "vProd"}, itemSum(func(it nfeItem) float64 {
			if it.hasISSQN() || it.at("prod", "indTot") == "0" {
				return 0
			}
			return it.f("prod", "vProd")
		})),
		blockTot("W08-10", 535, "vFrete", dp("prod", "vFrete")),
		blockTot("W09-10", 536, "vSeg", dp("prod", "vSeg")),
		blockTot("W10-10", 537, "vDesc", dp("prod", "vDesc")),
		blockTot("W11-10", 601, "vII", dp("imposto", "II", "vII")),
		blockTot("W12-10", 538, "vIPI", dp("imposto", "IPI", "IPITrib", "vIPI")),
		blockTot("W12a-10", 863, "vIPIDevol", dp("impostoDevol", "IPI", "vIPIDevol")),
		blockTot("W15-10", 604, "vOutro", dp("prod", "vOutro")),
		icmsTot("W03-10", 531, "vBC", ic("vBC")),
		icmsTot("W04-10", 532, "vICMS", ic("vICMS")),
		icmsTot("W04a-10", 795, "vICMSDeson", ic("vICMSDeson")),
		icmsTot("W04b-10", 861, "vFCP", ic("vFCP")),
		icmsTot("W05-10", 533, "vBCST", ic("vBCST")),
		icmsTot("W06-10", 534, "vST", ic("vICMSST")),
		icmsTot("W06a-10", 862, "vFCPST", ic("vFCPST")),
		icmsTot("W06b-10", 859, "vFCPSTRet", ic("vFCPSTRet")),
		icmsTot("W06c-10", 967, "vICMSMono", ic("vICMSMono")),
		icmsTot("W06d-10", 968, "vICMSMonoReten", ic("vICMSMonoReten")),
		icmsTot("W06e-10", 969, "vICMSMonoRet", ic("vICMSMonoRet")),
		icmsTot("W04c-10", 798, "vFCPUFDest", dp("imposto", "ICMSUFDest", "vFCPUFDest")),
		icmsTot("W04e-10", 799, "vICMSUFDest", dp("imposto", "ICMSUFDest", "vICMSUFDest")),
		icmsTot("W04g-10", 800, "vICMSUFRemet", dp("imposto", "ICMSUFDest", "vICMSUFRemet")),
		nfeTotalRule("W13-10", 602, ModeWarn, "vPIS", []string{"ICMSTot", "vPIS"}, itemSum(func(it nfeItem) float64 {
			if it.icms == nil {
				return 0
			}
			return pisOf("PIS", "vPIS")(it)
		})),
		nfeTotalRule("W14-10", 603, ModeWarn, "vCOFINS", []string{"ICMSTot", "vCOFINS"}, itemSum(func(it nfeItem) float64 {
			if it.icms == nil {
				return 0
			}
			return pisOf("COFINS", "vCOFINS")(it)
		})),
		issqn("W18-10", 605, "vServ", dp("prod", "vProd")),
		issqn("W19-10", 606, "vBC", iss("vBC")),
		issqn("W20-10", 607, "vISS", iss("vISSQN")),
		issqn("W21-10", 608, "vPIS", pisOf("PIS", "vPIS")),
		issqn("W22-10", 609, "vCOFINS", pisOf("COFINS", "vCOFINS")),
		issqn("W22b-10", 364, "vDeducao", iss("vDeducao")),
		issqn("W22c-10", 365, "vOutro", iss("vOutro")),
		issqn("W22d-10", 366, "vDescIncond", iss("vDescIncond")),
		issqn("W22e-10", 367, "vDescCond", iss("vDescCond")),
		issqn("W22f-10", 368, "vISSRet", iss("vISSRet")),
		nfeTotalRule("W33-10", 1075, ModeWarn, "vIS", []string{"ISTot", "vIS"}, itemSum(dp("imposto", "IS", "vIS"))),
		nfeWhen("W31-10", 1073, ModeWarn, "55/65", "ISTot informado sem Imposto Seletivo nos itens", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "total", "ISTot") && !c.anyItem(func(it nfeItem) bool { return nfeHas(it.imp, "IS") })
		}),
		nfeWhen("W31-20", 1117, ModeWarn, "55/65", "Imposto Seletivo nos itens sem o grupo ISTot", func(c *nfeCtx) bool {
			return !nfeHas(c.inf, "total", "ISTot") && c.anyItem(func(it nfeItem) bool { return nfeHas(it.imp, "IS") })
		}),
		nfeWhen("W34-10", 1118, ModeWarn, "55/65", "IBSCBSTot informado sem IBS/CBS nos itens", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "total", "IBSCBSTot") && !c.anyItem(func(it nfeItem) bool { return nfeHas(it.imp, "IBSCBS") })
		}),
		nfeWhen("W34-20", 1119, ModeWarn, "55/65", "IBS/CBS nos itens sem o grupo IBSCBSTot", func(c *nfeCtx) bool {
			return !nfeHas(c.inf, "total", "IBSCBSTot") && c.anyItem(func(it nfeItem) bool { return nfeHas(it.imp, "IBSCBS") })
		}),
		ibs("W35-10", 1076, "vBCIBSCBS", g("vBCIBSCBS"), "gIBSCBS", "vBC"),
		ibs("W38-10", 1077, "gIBSUF/vDif", g("gIBS", "gIBSUF", "vDif"), "gIBSCBS", "gIBSUF", "gDif", "vDif"),
		ibs("W39-10", 1078, "gIBSUF/vDevTrib", g("gIBS", "gIBSUF", "vDevTrib"), "gIBSCBS", "gIBSUF", "gDevTrib", "vDevTrib"),
		ibs("W41-10", 1080, "vIBSUF", g("gIBS", "gIBSUF", "vIBSUF"), "gIBSCBS", "gIBSUF", "vIBSUF"),
		ibs("W43-10", 1081, "gIBSMun/vDif", g("gIBS", "gIBSMun", "vDif"), "gIBSCBS", "gIBSMun", "gDif", "vDif"),
		ibs("W44-10", 1082, "gIBSMun/vDevTrib", g("gIBS", "gIBSMun", "vDevTrib"), "gIBSCBS", "gIBSMun", "gDevTrib", "vDevTrib"),
		ibs("W46-10", 1084, "vIBSMun", g("gIBS", "gIBSMun", "vIBSMun"), "gIBSCBS", "gIBSMun", "vIBSMun"),
		ibs("W47-10", 1085, "vIBS", g("gIBS", "vIBS"), "gIBSCBS", "vIBS"),
		ibs("W48-10", 1086, "vCredPres IBS", g("gIBS", "vCredPres"), "gCredPresOper", "gIBSCredPres", "vCredPres"),
		ibs("W53-10", 1088, "gCBS/vDif", g("gCBS", "vDif"), "gIBSCBS", "gCBS", "gDif", "vDif"),
		ibs("W54-10", 1089, "gCBS/vDevTrib", g("gCBS", "vDevTrib"), "gIBSCBS", "gCBS", "gDevTrib", "vDevTrib"),
		ibs("W56-10", 1091, "vCBS", g("gCBS", "vCBS"), "gIBSCBS", "gCBS", "vCBS"),
		ibs("W56a-10", 1087, "vCredPres CBS", g("gCBS", "vCredPres"), "gCredPresOper", "gCBSCredPres", "vCredPres"),
		ibs("W58-10", 1092, "vIBSMono", g("gMono", "vIBSMono"), "gIBSCBSMono", "gMonoPadrao", "vIBSMono"),
		ibs("W59-10", 1093, "vCBSMono", g("gMono", "vCBSMono"), "gIBSCBSMono", "gMonoPadrao", "vCBSMono"),
		ibs("W59a-10", 1120, "vIBSMonoReten", g("gMono", "vIBSMonoReten"), "gIBSCBSMono", "gMonoReten", "vIBSMonoReten"),
		ibs("W59b-10", 1121, "vCBSMonoReten", g("gMono", "vCBSMonoReten"), "gIBSCBSMono", "gMonoReten", "vCBSMonoReten"),
		ibs("W59c-10", 1122, "vIBSMonoRet", g("gMono", "vIBSMonoRet"), "gIBSCBSMono", "gMonoRet", "vIBSMonoRet"),
		ibs("W59d-10", 1123, "vCBSMonoRet", g("gMono", "vCBSMonoRet"), "gIBSCBSMono", "gMonoRet", "vCBSMonoRet"),
		ibs("W59f-10", 1176, "vIBSEstCred", g("gEstornoCred", "vIBSEstCred"), "gEstornoCred", "vIBSEstCred"),
		ibs("W59g-10", 1177, "vCBSEstCred", g("gEstornoCred", "vCBSEstCred"), "gEstornoCred", "vCBSEstCred"),
		nfeRule("W16-10", 610, ModeWarn, "55/65", nfeVNF),
	}
	return r
}

// nfeVNF: vNF contra a soma do MOC (W16-10), com as exceções de faturamento direto (tpOp 2), importação
// (CFOP 3) e do vICMSDeson (só abatido nos itens com indDeduzDeson=1; aceita as duas leituras).
func nfeVNF(c *nfeCtx) []string {
	t := sub(c.inf, "total", "ICMSTot")
	vnf, ok := nfeFok(t, "vNF")
	if !ok || c.anyItem(func(it nfeItem) bool { return cfopFirst(it) == "3" }) {
		return nil
	}
	g := func(k string) float64 { return nfeF(t, k) }
	base := g("vProd") - g("vDesc") + g("vICMSMonoReten") + g("vFrete") + g("vSeg") + g("vOutro") + g("vII") + g("vIPI") + nfeF(c.inf, "total", "ISSQNtot", "vServ")
	if !c.anyItem(func(it nfeItem) bool { return it.at("prod", "veicProd", "tpOp") == "2" }) {
		base += g("vST") + g("vFCPST") + g("vIPIDevol")
	}
	base += c.sum(func(it nfeItem) float64 {
		var s float64
		if it.at("imposto", "PISST", "indSomaPISST") == "1" {
			s += it.f("imposto", "PISST", "vPIS")
		}
		if it.at("imposto", "COFINSST", "indSomaCOFINSST") == "1" {
			s += it.f("imposto", "COFINSST", "vCOFINS")
		}
		return s
	})
	deson := c.sum(func(it nfeItem) float64 {
		if nfeS(it.icms, "indDeduzDeson") == "1" {
			return it.ic("vICMSDeson")
		}
		return 0
	})
	for _, d := range []float64{g("vICMSDeson"), deson, 0} {
		if !nfeDiff(base-d, vnf) {
			return nil
		}
	}
	return []string{fmt.Sprintf("vNF %.2f difere do somatório do MOC %.2f", vnf, base-deson)}
}

// --- pagamento e cobrança ---

func nfePaymentRules() []Rule {
	pags := func(c *nfeCtx) []map[string]any { return list(sub(c.inf, "pag"), "detPag") }
	vNF := func(c *nfeCtx) float64 { return nfeF(c.inf, "total", "ICMSTot", "vNF") }
	sumPag := func(c *nfeCtx) float64 {
		var s float64
		for _, p := range pags(c) {
			s += nfeF(p, "vPag")
		}
		return s
	}
	anyTPag := func(c *nfeCtx, f func(t string) bool) bool {
		for _, p := range pags(c) {
			if f(nfeS(p, "tPag")) {
				return true
			}
		}
		return false
	}
	dups := func(c *nfeCtx) []map[string]any { return list(sub(c.inf, "cobr"), "dup") }
	return []Rule{
		nfeWhen("YA03-10", 865, ModeWarn, "55/65", "soma dos pagamentos menor que o total da nota", func(c *nfeCtx) bool {
			if len(pags(c)) == 0 || nfeIn(c.finNFe, "3", "4") || anyTPag(c, func(t string) bool { return t == "90" || t == "91" }) {
				return false
			}
			return sumPag(c) < vNF(c)-nfeTol
		}),
		nfeWhen("YA03-20", 866, ModeWarn, "55/65", "soma dos pagamentos maior que o total da nota e sem vTroco", func(c *nfeCtx) bool {
			return len(pags(c)) > 0 && !nfeHas(c.inf, "pag", "vTroco") && sumPag(c) > vNF(c)+nfeTol
		}),
		nfeWhen("YA09-10", 869, ModeWarn, "55/65", "vTroco difere de soma dos vPag menos vNF", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "pag", "vTroco") && nfeDiff(nfeF(c.inf, "pag", "vTroco"), sumPag(c)-vNF(c))
		}),
		nfeWhen("YA03-30", 904, ModeWarn, "55/65", "tPag 90 ou 91 com vPag diferente de zero", func(c *nfeCtx) bool {
			for _, p := range pags(c) {
				if nfeIn(nfeS(p, "tPag"), "90", "91") && nfeF(p, "vPag") != 0 {
					return true
				}
			}
			return false
		}),
		nfeWhen("YA02-04", 871, ModeWarn, "55", "finNFe 3 ou 4 com meio de pagamento diferente de 90", func(c *nfeCtx) bool {
			return nfeIn(c.finNFe, "3", "4") && anyTPag(c, func(t string) bool { return t != "" && t != "90" })
		}),
		nfeWhen("YA02-10", 857, ModeWarn, "65", "NFC-e com tPag 14", func(c *nfeCtx) bool { return anyTPag(c, func(t string) bool { return t == "14" }) }),
		nfeWhen("YA02-40", 899, ModeWarn, "65", "NFC-e com tPag 90 (sem pagamento)", func(c *nfeCtx) bool { return anyTPag(c, func(t string) bool { return t == "90" }) }),
		nfeWhen("YA02a-10", 441, ModeWarn, "55/65", "tPag 99 sem xPag", func(c *nfeCtx) bool {
			for _, p := range pags(c) {
				if nfeS(p, "tPag") == "99" && nfeS(p, "xPag") == "" {
					return true
				}
			}
			return false
		}),
		nfeWhen("YA02a-20", 442, ModeWarn, "55/65", "xPag informado com tPag diferente de 99", func(c *nfeCtx) bool {
			for _, p := range pags(c) {
				if t := nfeS(p, "tPag"); t != "" && t != "99" && nfeHas(p, "xPag") {
					return true
				}
			}
			return false
		}),
		nfeWhen("YA04-20", 963, ModeWarn, "55/65", "grupo card com meio de pagamento que não é cartão/PIX", func(c *nfeCtx) bool {
			for _, p := range pags(c) {
				if t := nfeS(p, "tPag"); t != "" && !nfeIn(t, "03", "04", "10", "11", "12", "13", "15", "17", "18") && nfeHas(p, "card") {
					return true
				}
			}
			return false
		}),
		nfeWhen("YA05-10", 392, ModeWarn, "55/65", "cartão integrado (tpIntegra 1) sem CNPJ da credenciadora e cAut", func(c *nfeCtx) bool {
			for _, p := range pags(c) {
				if nfeS(p, "card", "tpIntegra") == "1" && (nfeS(p, "card", "CNPJ") == "" || nfeS(p, "card", "cAut") == "") {
					return true
				}
			}
			return false
		}),
		nfeWhen("Y05-10", 901, ModeWarn, "55", "desconto da fatura maior que o valor original", func(c *nfeCtx) bool {
			return nfeF(c.inf, "cobr", "fat", "vDesc") > nfeF(c.inf, "cobr", "fat", "vOrig")+0.0001
		}),
		nfeWhen("Y06-10", 902, ModeWarn, "55", "vLiq da fatura difere de vOrig menos vDesc", func(c *nfeCtx) bool {
			return nfeHas(c.inf, "cobr", "fat", "vLiq") && nfeHas(c.inf, "cobr", "fat", "vOrig") &&
				nfeDiff(nfeF(c.inf, "cobr", "fat", "vLiq"), nfeF(c.inf, "cobr", "fat", "vOrig")-nfeF(c.inf, "cobr", "fat", "vDesc"))
		}),
		nfeWhen("Y08-10", 852, ModeWarn, "55", "nDup fora da sequência 001, 002, 003...", func(c *nfeCtx) bool {
			for i, d := range dups(c) {
				if nfeS(d, "nDup") != fmt.Sprintf("%03d", i+1) {
					return true
				}
			}
			return false
		}),
		nfeWhen("Y09-20", 900, ModeWarn, "55", "vencimento da parcela anterior à data de emissão", func(c *nfeCtx) bool {
			e, ok := nfeDate(nfeS(c.ide, "dhEmi"))
			for _, d := range dups(c) {
				if v, ok2 := nfeDate(nfeS(d, "dVenc")); ok && ok2 && v.Before(e) {
					return true
				}
			}
			return false
		}),
		nfeWhen("Y09-30", 850, ModeWarn, "55", "vencimento da parcela anterior ao da parcela anterior", func(c *nfeCtx) bool {
			prev := ""
			for _, d := range dups(c) {
				v := nfeS(d, "dVenc")
				if prev != "" && v != "" && v < prev {
					return true
				}
				prev = v
			}
			return false
		}),
		nfeWhen("Y09-40", 853, ModeWarn, "55", "parcela única com vencimento igual à data de emissão", func(c *nfeCtx) bool {
			d := dups(c)
			e := nfeS(c.ide, "dhEmi")
			return len(d) == 1 && len(e) >= 10 && nfeS(d[0], "dVenc") == e[:10]
		}),
		nfeWhen("Y10-10", 851, ModeWarn, "55", "soma das parcelas difere do vLiq da fatura", func(c *nfeCtx) bool {
			if len(dups(c)) == 0 || !nfeHas(c.inf, "cobr", "fat", "vLiq") {
				return false
			}
			var s float64
			for _, d := range dups(c) {
				s += nfeF(d, "vDup")
			}
			return nfeDiff(s, nfeF(c.inf, "cobr", "fat", "vLiq"))
		}),
	}
}

// --- chaves referenciadas (refNFe, refCTe, refDFeAnt) ---

func nfeRefKeyRules() []Rule {
	type spec struct {
		name   string
		get    func(c *nfeCtx) []string
		okMod  func(m string) bool
		ids    [9]string // dv, uf, mes, doc, modelo, numero, duplicada, propria, tpEmis
		cstats [9]int
		modMsg string
	}
	nfref := func(tag string) func(c *nfeCtx) []string {
		return func(c *nfeCtx) []string {
			var out []string
			for _, n := range list(c.ide, "NFref") {
				if v := nfeS(n, tag); v != "" {
					out = append(out, v)
				}
			}
			return out
		}
	}
	specs := []spec{
		{"refNFe", nfref("refNFe"), func(m string) bool { return nfeIn(m, "55", "65", "59") },
			[9]string{"BA02-10", "BA02-14", "BA02-24", "BA02-30", "BA02-34", "BA02-40", "BA02-44", "BA02-50", "BA02-60"},
			[9]int{547, 522, 524, 552, 679, 683, 680, 316, 953}, "55, 65 ou 59"},
		{"refCTe", nfref("refCTe"), func(m string) bool { return m == "57" },
			[9]string{"BA19-10", "BA19-14", "BA19-24", "BA19-30", "BA19-34", "BA19-40", "BA19-44", "", ""},
			[9]int{547, 522, 524, 552, 679, 683, 680, 0, 0}, "57"},
		{"refDFeAnt", func(c *nfeCtx) []string {
			var out []string
			for _, v := range items(sub(c.ide, "gCompraGov")["refDFeAnt"]) {
				if s := strings.TrimSpace(scalar(v)); s != "" && v != nil {
					out = append(out, s)
				}
			}
			return out
		}, func(m string) bool { return m == "55" },
			[9]string{"BB05-70", "BB05-80", "BB05-100", "BB05-110", "BB05-120", "BB05-130", "BB05-140", "BB05-150", "BB05-160"},
			[9]int{1204, 1205, 1207, 1208, 1209, 1210, 1211, 1212, 1213}, "55"},
	}
	var rules []Rule
	for _, sp := range specs {
		sp := sp
		each := func(i int, mode Mode, msg string, bad func(c *nfeCtx, k nfeKey, raw string) bool) {
			if sp.ids[i] == "" {
				return
			}
			rules = append(rules, nfeRule(sp.ids[i], sp.cstats[i], mode, "55", func(c *nfeCtx) []string {
				for _, raw := range sp.get(c) {
					if k, ok := nfeParseKey(raw); ok && bad(c, k, raw) {
						return []string{sp.name + ": " + msg}
					}
				}
				return nil
			}))
		}
		each(0, ModeBlock, "dígito verificador da chave inválido", func(c *nfeCtx, k nfeKey, raw string) bool { return !nfeKeyOK(raw) })
		each(1, ModeWarn, "UF da chave inválida", func(c *nfeCtx, k nfeKey, raw string) bool { return !nfeValidUFCode(k.cUF) })
		each(2, ModeWarn, "mês de emissão da chave inválido", func(c *nfeCtx, k nfeKey, raw string) bool {
			m, _ := strconv.Atoi(k.aamm[2:])
			return m < 1 || m > 12
		})
		each(3, ModeWarn, "CNPJ/CPF do emitente da chave zerado ou com dígito inválido", func(c *nfeCtx, k nfeKey, raw string) bool {
			if k.tpEmis == "3" {
				return false
			}
			if sp.name == "refCTe" || nfeSerieIn(k.serie, [2]int{0, 909}, [2]int{980, 989}) {
				return !nfeCNPJ(k.doc)
			}
			return !nfeCPF(k.doc[3:])
		})
		each(4, ModeWarn, "modelo da chave diferente de "+sp.modMsg, func(c *nfeCtx, k nfeKey, raw string) bool { return !sp.okMod(k.mod) })
		each(5, ModeWarn, "número da nota na chave zerado", func(c *nfeCtx, k nfeKey, raw string) bool { return k.nNF == "000000000" })
		each(6, ModeWarn, "chave referenciada em duplicidade", func(c *nfeCtx, k nfeKey, raw string) bool {
			n := 0
			for _, o := range sp.get(c) {
				if o == raw {
					n++
				}
			}
			return n > 1
		})
		each(7, ModeWarn, "chave referenciada igual à da própria nota", func(c *nfeCtx, k nfeKey, raw string) bool {
			return raw == strings.TrimPrefix(nfeS(c.inf, "@Id"), "NFe")
		})
		each(8, ModeWarn, "tipo de emissão da chave referenciada inválido", func(c *nfeCtx, k nfeKey, raw string) bool {
			return (k.mod == "55" || k.mod == "65") && k.aamm > "1104" && !nfeIn(k.tpEmis, "1", "2", "3", "4", "5", "6", "7", "9")
		})
	}
	return rules
}

// --- demais regras ---

func nfeMiscRules() []Rule {
	base := func(id string, cstat int, tpNF, modFrete string, eq bool, withDest bool, negate bool) Rule {
		return nfeRule(id, cstat, ModeWarn, "55", func(c *nfeCtx) []string {
			tr := nfeIDBase(sub(c.inf, "transp", "transporta"))
			mf := nfeS(c.inf, "transp", "modFrete")
			if tr == "" || nfeS(c.ide, "tpNF") != tpNF || mf == "" {
				return nil
			}
			applies := mf == modFrete
			if negate {
				applies = mf != modFrete
			}
			if !applies {
				return nil
			}
			ref := nfeIDBase(c.emit)
			if withDest {
				ref = nfeIDBase(c.dest)
			}
			if ref == "" || c.anyItem(func(it nfeItem) bool { return nfeHas(it.prod, "comb") }) && !eq {
				return nil
			}
			if (tr == ref) != eq {
				return []string{"CNPJ base/CPF do transportador incompatível com o modFrete e o tipo de operação"}
			}
			return nil
		})
	}
	return []Rule{
		// X04: o "remetente" é o emitente na saída (tpNF 1) e o destinatário na entrada (tpNF 0).
		base("X04-30", 846, "1", "3", true, false, false),
		base("X04-40", 846, "0", "3", true, true, false),
		base("X04-50", 847, "1", "3", false, false, true),
		base("X04-60", 847, "0", "3", false, true, true),
		base("X04-70", 848, "1", "4", true, true, false),
		base("X04-80", 848, "0", "4", true, false, false),
		base("X04-90", 849, "1", "4", false, true, true),
		base("X04-100", 849, "0", "4", false, false, true),
		nfeWhen("X03-30", 845, ModeWarn, "55", "modFrete 9 com grupo transporta", func(c *nfeCtx) bool {
			return nfeS(c.inf, "transp", "modFrete") == "9" && nfeHas(c.inf, "transp", "transporta")
		}),
		nfeWhen("X02-20", 868, ModeWarn, "55", "operação interestadual com veículo ou reboque de transporte", func(c *nfeCtx) bool {
			return nfeS(c.ide, "idDest") == "2" && (nfeHas(c.inf, "transp", "veicTransp") || nfeHas(c.inf, "transp", "reboque"))
		}),
		nfeWhen("BA10-10", 682, ModeWarn, "55", "NF de produtor referenciada (refNFP) é proibida", func(c *nfeCtx) bool {
			for _, n := range list(c.ide, "NFref") {
				if nfeHas(n, "refNFP") {
					return true
				}
			}
			return false
		}),
		nfeWhen("BA06-10", 548, ModeBlock, "55", "CNPJ da NF modelo 1 referenciada inválido", func(c *nfeCtx) bool {
			for _, n := range list(c.ide, "NFref") {
				if v := nfeS(n, "refNF", "CNPJ"); v != "" && !nfeCNPJ(v) {
					return true
				}
			}
			return false
		}),
		nfeWhen("BA13-10", 549, ModeBlock, "55", "CNPJ da NF de produtor referenciada inválido", func(c *nfeCtx) bool {
			for _, n := range list(c.ide, "NFref") {
				if v := nfeS(n, "refNFP", "CNPJ"); v != "" && !nfeCNPJ(v) {
					return true
				}
			}
			return false
		}),
		nfeWhen("BA14-10", 550, ModeBlock, "55", "CPF da NF de produtor referenciada inválido", func(c *nfeCtx) bool {
			for _, n := range list(c.ide, "NFref") {
				if v := nfeS(n, "refNFP", "CPF"); v != "" && !nfeCPF(v) {
					return true
				}
			}
			return false
		}),
		nfeWhen("BA20-10", 684, ModeWarn, "55", "cupom fiscal referenciado em duplicidade", func(c *nfeCtx) bool {
			seen := map[string]bool{}
			for _, n := range list(c.ide, "NFref") {
				if e := sub(n, "refECF"); e != nil {
					k := nfeS(e, "mod") + "|" + nfeS(e, "nECF") + "|" + nfeS(e, "nCOO")
					if seen[k] {
						return true
					}
					seen[k] = true
				}
			}
			return false
		}),
		nfeWhen("BA20-20", 923, ModeWarn, "55", "cupom fiscal ou NF modelo 1/2 referenciada em operação que não é interna", func(c *nfeCtx) bool {
			d := nfeS(c.ide, "idDest")
			if d == "" || d == "1" {
				return false
			}
			for _, n := range list(c.ide, "NFref") {
				if nfeHas(n, "refECF") || nfeHas(n, "refNF") {
					return true
				}
			}
			return false
		}),
		nfeWhen("ZX02-10", 394, ModeWarn, "65", "NFC-e sem qrCode", func(c *nfeCtx) bool {
			y := nfeS(c.ide, "dhEmi")
			return nfeS(c.doc, "infNFeSupl", "qrCode") == "" && !(len(y) >= 10 && y[:10] < "2016-04-01")
		}),
		nfeWhen("ZX02-220", 672, ModeWarn, "55", "qrCode de NF-e com versão diferente de 3", func(c *nfeCtx) bool {
			p := nfeQRParams(nfeS(c.doc, "infNFeSupl", "qrCode"))
			return len(p) >= 2 && p[1] != "3"
		}),
		nfeWhen("ZX02-222", 444, ModeWarn, "65", "NFC-e com qrCode versão 2 e emitente pessoa física", func(c *nfeCtx) bool {
			p := nfeQRParams(nfeS(c.doc, "infNFeSupl", "qrCode"))
			return len(p) >= 2 && p[1] == "2" && nfeHas(c.emit, "CPF")
		}),
		nfeWhen("ZX02-228", 397, ModeWarn, "55/65", "chave de acesso do qrCode difere da chave da nota", func(c *nfeCtx) bool {
			p := nfeQRParams(nfeS(c.doc, "infNFeSupl", "qrCode"))
			return len(p) >= 1 && p[0] != strings.TrimPrefix(nfeS(c.inf, "@Id"), "NFe")
		}),
		nfeWhen("ZX02-236", 398, ModeWarn, "55/65", "versão do qrCode diferente de 2 e 3", func(c *nfeCtx) bool {
			p := nfeQRParams(nfeS(c.doc, "infNFeSupl", "qrCode"))
			return len(p) >= 2 && p[1] != "2" && p[1] != "3"
		}),
		nfeWhen("ZX02-244", 397, ModeWarn, "55/65", "tipo de ambiente do qrCode difere do tpAmb da nota", func(c *nfeCtx) bool {
			p := nfeQRParams(nfeS(c.doc, "infNFeSupl", "qrCode"))
			return len(p) >= 3 && p[2] != nfeS(c.ide, "tpAmb")
		}),
	}
}

// nfeQRParams devolve os campos do parâmetro p= do qrCode (chave|versão|tpAmb|...).
func nfeQRParams(qr string) []string {
	i := strings.Index(qr, "p=")
	if i < 0 {
		return nil
	}
	p := qr[i+2:]
	if j := strings.IndexByte(p, '&'); j >= 0 {
		p = p[:j]
	}
	return strings.Split(p, "|")
}
