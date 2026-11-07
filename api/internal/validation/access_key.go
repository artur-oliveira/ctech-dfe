package validation

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const accessKeyLen = 44

// Modelos de documento fiscal da chave de acesso (posições 21 e 22).
const (
	AccessKeyModNFe   = "55"
	AccessKeyModCTe   = "57"
	AccessKeyModMDFe  = "58"
	AccessKeyModCTeOS = "67"
)

// accessKeyTpEmis holds, per model, the tpEmis codes (position 35) a key of
// that model can carry. NF-e: every code except 9, which SEFAZ reserves for
// NFC-e offline contingency. CT-e and CT-e OS: 1 normal, 4 EPEC, 5 FS-DA,
// 7 SVC-RS, 8 SVC-SP. MDF-e: 1 normal, 2 contingência, 3 NFF.
var accessKeyTpEmis = map[string]string{
	AccessKeyModNFe:   "1234567",
	AccessKeyModCTe:   "14578",
	AccessKeyModCTeOS: "14578",
	AccessKeyModMDFe:  "123",
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

// accessKeyValidator wires AccessKeyReason into the shared go-playground
// validator instance under the "dfe_access_key" tag. The optional param lists
// the accepted models separated by spaces (`dfe_access_key=55 57`); without a
// param the key must be an NF-e (55), the tag's original meaning.
func accessKeyValidator(fl validator.FieldLevel) bool {
	return AccessKeyReason(fl.Field().String(), accessKeyModels(fl.Param())...) == ""
}

func accessKeyModels(param string) []string {
	if param == "" {
		return []string{AccessKeyModNFe}
	}
	return strings.Fields(param)
}

// ValidAccessKey validates an NF-e access key (model 55). Exported for direct
// use on the manifestation route's `:access_key` path param, which has no
// request body struct to attach a `validate` tag to. Mirrors
// ui/src/lib/utils/access-key.ts — keep both in lock-step (see
// docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
func ValidAccessKey(s string) bool { return AccessKeyReason(s, AccessKeyModNFe) == "" }

// AccessKeyReason validates an access key (chave de acesso) of one of the
// given models beyond its 44-character length: cUF, AAMM, CNPJ-xor-CPF (with
// check digit, alphanumeric CNPJ included), model, tpEmis of that model and
// the final cDV. It returns "" for a valid key, or the reason in Portuguese
// (the MOC's own list: formato, UF, mês, CNPJ/CPF, modelo, tipo de emissão, DV).
func AccessKeyReason(s string, models ...string) string {
	if len(s) != accessKeyLen {
		return "deve ter 44 caracteres"
	}
	// Every position is a digit except the 14-char CNPJ/CPF segment [6:20),
	// which may contain uppercase letters (alphanumeric CNPJ, IN RFB 2229/2024).
	for i := 0; i < accessKeyLen; i++ {
		if i >= 6 && i < 20 {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return "só o CNPJ (posições 7 a 20) aceita letras"
		}
	}
	if _, ok := accessKeyUFCodes[s[0:2]]; !ok {
		return "UF inválida"
	}
	if mm := int(s[4]-'0')*10 + int(s[5]-'0'); mm < 1 || mm > 12 {
		return "mês inválido"
	}
	if !validAccessKeyDoc(s[6:20]) {
		return "CNPJ/CPF inválido"
	}
	mod := s[20:22]
	ok := false
	for _, m := range models {
		ok = ok || m == mod
	}
	if !ok {
		return "modelo " + mod + " não aceito aqui (esperado " + strings.Join(models, " ou ") + ")"
	}
	if !strings.ContainsRune(accessKeyTpEmis[mod], rune(s[34])) {
		return "tipo de emissão inválido"
	}
	if int(s[43]-'0') != calcAccessKeyDV(s[:43]) {
		return "dígito verificador inválido"
	}
	return ""
}

// validAccessKeyDoc validates the 14-char document segment: a CNPJ (numeric
// or alphanumeric, IN RFB 2229/2024, check-digit validated) or a CPF (SEFAZ
// convention: "000" prefix + 11-digit CPF, check-digit validated). A CNPJ may
// itself start with "000" (00.000.000/0001-91), so the CNPJ is tried first.
func validAccessKeyDoc(doc string) bool {
	return ValidCNPJ(doc) || (strings.HasPrefix(doc, "000") && ValidCPF(doc[3:]))
}

// calcAccessKeyDV computes the access key's own check digit (cDV) over its
// first 43 characters: weights 2-9 cycling right-to-left, mod-11. NT 2023.002
// defines an alphanumeric character's value here as (ASCII code − 48), the
// same value the alphanumeric CNPJ's own check digits use (see ValidCNPJ).
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
