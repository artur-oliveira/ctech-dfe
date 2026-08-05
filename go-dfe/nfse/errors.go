package nfse

import "fmt"

// FieldNotSupportedError é a falha explícita exigida pela spec §4.3: um
// campo presente no modelo neutro que o provider de destino não representa
// NUNCA é descartado em silêncio.
type FieldNotSupportedError struct {
	Provider string
	Field    string
}

func (e *FieldNotSupportedError) Error() string {
	return fmt.Sprintf("nfse: provider %q não suporta o campo %q", e.Provider, e.Field)
}

// FiscalError carrega a rejeição do fisco com código e descrição preservados.
// Status é o HTTP devolvido pela API nacional.
type FiscalError struct {
	Status   int
	Messages []Message
}

func (e *FiscalError) Error() string {
	if len(e.Messages) == 0 {
		return fmt.Sprintf("nfse: fisco retornou HTTP %d sem mensagens", e.Status)
	}
	return fmt.Sprintf("nfse: %s - %s", e.Messages[0].Codigo, e.Messages[0].Descricao)
}
