package nfse

import (
	"fmt"
	"strings"
)

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
// Status é o HTTP devolvido pela API nacional. Body é o corpo cru da resposta,
// mantido porque o fisco frequentemente devolve descrição vazia e o detalhe
// real só existe em complemento ou em campos fora do envelope conhecido.
type FiscalError struct {
	Status   int
	Messages []Message
	Body     string
}

func (e *FiscalError) Error() string {
	if len(e.Messages) == 0 {
		return fmt.Sprintf("nfse: fisco retornou HTTP %d sem mensagens: %s", e.Status, e.Body)
	}
	parts := make([]string, 0, len(e.Messages))
	for _, m := range e.Messages {
		part := fmt.Sprintf("%s - %s", m.Codigo, m.Descricao)
		if m.Complemento != "" {
			part += " (" + m.Complemento + ")"
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf("nfse: HTTP %d: %s", e.Status, strings.Join(parts, "; "))
}
