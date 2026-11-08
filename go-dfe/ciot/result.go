package ciot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Result is the parsed ANTT response. Codigo and Mensagem arrive as a scalar or a list; the first item wins.
// Body keeps the whole decoded payload for services whose data fields the spec does not name.
type Result struct {
	Codigo             int            `json:"codigo"`
	Mensagem           string         `json:"mensagem"`
	Protocolo          string         `json:"protocolo,omitempty"`
	IdOperacao         string         `json:"id_operacao,omitempty"`
	CodigoVerificador  string         `json:"codigo_verificador,omitempty"`
	CIOT               string         `json:"ciot,omitempty"` // 12 + verifier, when both are present
	AvisoTransportador string         `json:"aviso_transportador,omitempty"`
	Body               map[string]any `json:"body,omitempty"`
}

// Success reports codes 110 and 111.
func (r Result) Success() bool { return r.Codigo == CodeInserted || r.Codigo == CodeQueried }

// Rejection is a business rejection by the ANTT (never retried). It carries the stable internal code and keeps
// the ANTT code and message as reference.
type Rejection struct {
	Result
	Internal string
	Group    string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("ciot: ANTT rejection %d (%s): %s", r.Codigo, r.Internal, r.Mensagem)
}

// ParseResult decodes a response body. It accepts IdOperacaoTransporte or CodigoIdentificacaoOperacao.
func ParseResult(data []byte) (Result, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return Result{}, fmt.Errorf("ciot: decode response: %w", err)
	}
	r := Result{Body: m}
	code := firstScalar(m["Codigo"])
	n, err := strconv.Atoi(strings.TrimSpace(code))
	if err != nil {
		return Result{}, fmt.Errorf("ciot: response without a numeric Codigo")
	}
	r.Codigo = n
	r.Mensagem = firstScalar(m["Mensagem"])
	r.Protocolo = firstScalar(m["Protocolo"])
	r.IdOperacao = firstScalar(m["IdOperacaoTransporte"])
	if r.IdOperacao == "" {
		r.IdOperacao = firstScalar(m["CodigoIdentificacaoOperacao"])
	}
	r.CodigoVerificador = firstScalar(m["CodigoVerificador"])
	r.AvisoTransportador = firstScalar(m["AvisoTransportador"])
	if len(r.IdOperacao) == IDLen && r.CodigoVerificador != "" {
		r.CIOT = r.IdOperacao + r.CodigoVerificador
	}
	return r, nil
}

// firstScalar renders a JSON scalar, or the first item of a list, as text.
func firstScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []any:
		if len(x) == 0 {
			return ""
		}
		return firstScalar(x[0])
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}
