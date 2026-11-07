package nfes

import (
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.aoctech.app/api-commons/problem"
)

// decimalError é o pânico de d()/dField() diante de um número malformado. Os builders são funções puras que
// devolvem mapas, com mais de cem leituras de decimal; propagar erro em cada uma reescreveria todos eles. O
// pânico tipado (o mesmo recurso que encoding/json usa por dentro) sobe até recoverDecimal, na entrada da
// emissão, e vira um 422 que nomeia o campo. Nenhum outro pânico é capturado.
type decimalError struct {
	field, value string // value vai só para o log, nunca para a resposta
}

// CodeInvalidDecimal é o code do 422 de número malformado (cadastro antigo ou dado gravado fora da api).
const CodeInvalidDecimal = "invalid_decimal"

// d lê um decimal. Vazio é zero (campo ausente); malformado entra em pânico com decimalError, nunca vira 0.00
// calado no XML (achado 1.1 item 4 da auditoria do MOC).
func d(s string) decimal.Decimal { return parseDecimalStrict("", s) }

// dField lê m[key] como decimal, nomeando a chave no erro. Ausente ou vazio é zero.
func dField(m map[string]any, key string) decimal.Decimal {
	return parseDecimalStrict(key, anyStr(m, key, ""))
}

func parseDecimalStrict(field, s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(decimalError{field: field, value: s})
	}
	return v
}

// recoverDecimal converte o pânico de decimal malformado em 422 no erro de retorno da emissão. Use com defer
// numa função de retorno nomeado: `defer recoverDecimal(&err)`. Qualquer outro pânico segue adiante.
func recoverDecimal(err *error) {
	r := recover()
	if r == nil {
		return
	}
	de, ok := r.(decimalError)
	if !ok {
		panic(r)
	}
	field := de.field
	slog.Warn("NF-e decimal malformado", "field", field, "value", de.value)
	if field == "" {
		field = "valor"
	}
	*err = problem.UnprocessableEntityFields(
		"valor numérico inválido em "+field+": corrija o cadastro ou o pedido antes de emitir",
		problem.Field(field, CodeInvalidDecimal, "número malformado"),
	).WithCode(CodeInvalidDecimal)
}

func q2(v decimal.Decimal) string { return v.StringFixed(2) }

func q4(v decimal.Decimal) string { return v.StringFixed(4) }

func dn(s string, n int32) decimal.Decimal {
	parts := strings.Split(s, ".")
	if len(parts) == 2 && len(parts[1]) <= int(n) {
		return d(s)
	}
	return d(s).Round(n)
}
