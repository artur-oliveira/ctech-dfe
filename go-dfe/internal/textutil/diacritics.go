// Package textutil reúne transformações textuais compartilhadas pelos
// adapters fiscais do go-dfe.
package textutil

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// RemoveDiacritics substitui letras acentuadas por suas formas base sem
// alterar o case. A função espera Unicode válido; ela não tenta reparar texto
// UTF-8 previamente interpretado como Latin-1 (mojibake).
func RemoveDiacritics(value string) string {
	decomposed := norm.NFD.String(value)
	var result strings.Builder
	result.Grow(len(decomposed))
	for _, char := range decomposed {
		if unicode.Is(unicode.Mn, char) {
			continue
		}
		result.WriteRune(char)
	}
	return result.String()
}
