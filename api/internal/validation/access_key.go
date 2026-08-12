package validation

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	accessKeyLen    = 44
	accessKeyModNFe = "55" // this feature is NF-e-only (mod 65 = NFC-e, out of scope)
)

// accessKeyValidTpEmis holds the tpEmis codes valid for chave-de-acesso
// manual entry — every code except 9, which SEFAZ reserves for NFC-e offline
// contingency and never appears on an NF-e.
var accessKeyValidTpEmis = map[byte]struct{}{
	'1': {}, '2': {}, '3': {}, '4': {}, '5': {}, '6': {}, '7': {},
}

// accessKeyUFCodes is the set of valid IBGE cUF codes, derived once from
// services.UFCode (api/internal/services/shared.go) — never redeclare a UF
// map (project DRY rule).
var accessKeyUFCodes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(services.UFCode))
	for _, code := range services.UFCode {
		m[code] = struct{}{}
	}
	return m
}()

// accessKeyValidator wires ValidAccessKey into the shared go-playground
// validator instance under the "dfe_access_key" tag.
func accessKeyValidator(fl validator.FieldLevel) bool {
	return ValidAccessKey(fl.Field().String())
}

// ValidAccessKey validates an NF-e access key (chave de acesso) beyond its
// 44-character length: cUF, AAMM, CNPJ-xor-CPF (with check digit), mod=55,
// tpEmis, and the final cDV check digit. Exported for direct use on the
// manifestation route's `:access_key` path param, which has no request body
// struct to attach a `validate` tag to. Mirrors
// ui/src/lib/utils/access-key.ts — keep both in lock-step (see
// docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
func ValidAccessKey(s string) bool {
	if len(s) != accessKeyLen {
		return false
	}
	// Every position is a digit except the 14-char CNPJ/CPF segment [6:20),
	// which may contain uppercase letters (alphanumeric CNPJ, IN RFB 2229/2024).
	for i := 0; i < accessKeyLen; i++ {
		if i >= 6 && i < 20 {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if _, ok := accessKeyUFCodes[s[0:2]]; !ok {
		return false
	}
	mm := int(s[4]-'0')*10 + int(s[5]-'0')
	if mm < 1 || mm > 12 {
		return false
	}
	if !validAccessKeyDoc(s[6:20]) {
		return false
	}
	if s[20:22] != accessKeyModNFe {
		return false
	}
	if _, ok := accessKeyValidTpEmis[s[34]]; !ok {
		return false
	}
	dv := int(s[43] - '0')
	return dv == calcAccessKeyDV(s[:43])
}

// validAccessKeyDoc validates the 14-char document segment: either a CPF
// (SEFAZ convention — "000" prefix + 11-digit CPF, both check-digit
// validated) or an alphanumeric CNPJ (IN RFB 2229/2024, check-digit
// validated) — never both, never neither.
func validAccessKeyDoc(doc string) bool {
	if strings.HasPrefix(doc, "000") {
		return ValidCPF(doc[3:])
	}
	return ValidCNPJ(doc)
}

// calcAccessKeyDV computes the NF-e access key's own check digit (cDV) over
// its first 43 characters: weights 2-9 cycling right-to-left, mod-11. NT
// 2023.002 defines an alphanumeric character's value here as (ASCII code −
// 48) — a DIFFERENT algorithm from the CNPJ field's own internal check
// digits, which use the IN RFB 2229/2024 A=10..Z=35 mapping (see ValidCNPJ,
// api/internal/validation/validators.go:152). Two distinct algorithms for two
// distinct check digits, both real, both required.
func calcAccessKeyDV(key43 string) int {
	weights := [8]int{2, 3, 4, 5, 6, 7, 8, 9}
	sum := 0
	for i, wi := len(key43)-1, 0; i >= 0; i, wi = i-1, wi+1 {
		sum += (int(key43[i]) - 48) * weights[wi%8]
	}
	rem := sum % 11
	if rem < 2 {
		return 0
	}
	return 11 - rem
}
