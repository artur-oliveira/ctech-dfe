package nfse

import "testing"

// Regressão: o fisco devolveu {"codigo":"L0017","descricao":""} e a mensagem
// virava "nfse: L0017 - " — complemento, mensagens seguintes, status HTTP e
// corpo cru eram descartados.
func TestFiscalErrorIncludesAllDetails(t *testing.T) {
	err := &FiscalError{
		Status: 400,
		Messages: []Message{
			{Codigo: "L0017", Descricao: "", Complemento: "cLocIncid divergente"},
			{Codigo: "L0002", Descricao: "DPS duplicada"},
		},
		Body: `{"erros":[...]}`,
	}
	want := "nfse: HTTP 400: L0017 -  (cLocIncid divergente); L0002 - DPS duplicada"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, esperado %q", got, want)
	}
}

func TestFiscalErrorWithoutMessagesKeepsRawBody(t *testing.T) {
	err := &FiscalError{Status: 502, Body: "<html>gateway</html>"}
	want := "nfse: fisco retornou HTTP 502 sem mensagens: <html>gateway</html>"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, esperado %q", got, want)
	}
}
