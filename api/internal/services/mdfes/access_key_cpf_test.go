package mdfes

import (
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Emitente CPF: posições 7 a 20 da chave são "000" + CPF (zeros à esquerda).
func TestGenerateAccessKeyCPFComZerosAEsquerda(t *testing.T) {
	key := services.GenerateAccessKey("SP", "12345678909", services.ModelMDFe, 1, 1, time.Now(), services.TpEmisNormal)
	if len(key) != 44 {
		t.Fatalf("chave com %d dígitos", len(key))
	}
	if got := key[6:20]; got != "00012345678909" {
		t.Fatalf("CPF na chave = %q, want 00012345678909", got)
	}
}
