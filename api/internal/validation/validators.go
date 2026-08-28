package validation

import (
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// UFSet holds the 27 Brazilian federation units (26 states + DF). Defined once
// here and reused by the "uf" validator — never redeclared per file.
var UFSet = map[string]struct{}{
	"AC": {}, "AL": {}, "AM": {}, "AP": {}, "BA": {}, "CE": {}, "DF": {},
	"ES": {}, "GO": {}, "MA": {}, "MG": {}, "MS": {}, "MT": {}, "PA": {},
	"PB": {}, "PE": {}, "PI": {}, "PR": {}, "RJ": {}, "RN": {}, "RO": {},
	"RR": {}, "RS": {}, "SC": {}, "SE": {}, "SP": {}, "TO": {},
}

// regexValidators maps a validation tag name to the regular expression the field
// value must fully match. These mirror the frontend Zod schemas so backend and
// frontend validation stay in lock-step.
var regexValidators = map[string]*regexp.Regexp{
	// Identity / address
	"cfop":    regexp.MustCompile(`^\d{4}$`),
	"ncm":     regexp.MustCompile(`^\d{8}$`),
	"cest":    regexp.MustCompile(`^\d{7}$`),
	"ibge":    regexp.MustCompile(`^\d{7}$`),
	"cep":     regexp.MustCompile(`^\d{8}$`),
	"phonebr": regexp.MustCompile(`^\d{10,11}$`),
	"placa":   regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`),
	"rntrc":   regexp.MustCompile(`^\d{8,12}$`),
	"renavam": regexp.MustCompile(`^\d{9,11}$`),
	"inscmun": regexp.MustCompile(`^\d{1,15}$`),
	"caepf":   regexp.MustCompile(`^\d{14}$`),
	"nif":     regexp.MustCompile(`^[A-Za-z0-9]{1,40}$`),
	"cnae":    regexp.MustCompile(`^\d{7}$`),
	// Generic numeric / units
	"unit":     regexp.MustCompile(`^[A-Z]{1,6}$`),
	"decimalv": regexp.MustCompile(`^\d+(\.\d+)?$`),
	"percent":  regexp.MustCompile(`^\d{1,3}(\.\d{1,4})?$`),
	"serie":    regexp.MustCompile(`^\d{1,3}$`),
	"money":    regexp.MustCompile(`^\d+(\.\d{1,4})?$`),
	"money2":   regexp.MustCompile(`^\d+(\.\d{1,2})?$`),
	"weight3":  regexp.MustCompile(`^\d+(\.\d{1,3})?$`),
	// Product identity / fiscal codes (mirror frontend products.ts)
	"prodcode": regexp.MustCompile(`^[A-Z0-9._\-]+$`),
	"cean":     regexp.MustCompile(`^(\d{8,14}|SEM GTIN)$`),
	"cbenef":   regexp.MustCompile(`^([!-ÿ]{8}|[!-ÿ]{10}|SEM CBENEF)$`),
	// cCredPresumido (prod/gCred) tem o mesmo formato do cBenef, mas sem o
	// literal de ausência: sem código não há crédito presumido nenhum.
	"ccredpres": regexp.MustCompile(`^([!-ÿ]{8}|[!-ÿ]{10})$`),
	"digits2":   regexp.MustCompile(`^\d{2}$`),
	"extipi":    regexp.MustCompile(`^\d{2,3}$`),
	"digits9":   regexp.MustCompile(`^\d{9}$`),
	"digits14":  regexp.MustCompile(`^\d{14}$`),
	"class6":    regexp.MustCompile(`^\d{6}$`),
	"letters2":  regexp.MustCompile(`^[A-Z]{2}$`),
	"ibscst":    regexp.MustCompile(`^(000|010|011|200|220|221|222|400|410|510|515|550|620|800|810|811|820|830)$`),
	// cana (infNFe/cana): mês de referência MM/AAAA e dia do mês sem zero à
	// esquerda — os dois padrões vêm do próprio XSD.
	"canaref": regexp.MustCompile(`^(0[1-9]|1[0-2])/2\d{3}$`),
	"canadia": regexp.MustCompile(`^([1-9]|[12]\d|3[01])$`),
	// Vehicle-product (veicProd) micro-formats
	"d1":  regexp.MustCompile(`^\d$`),
	"d12": regexp.MustCompile(`^\d{1,2}$`),
	"d4":  regexp.MustCompile(`^\d{4}$`),
	"d16": regexp.MustCompile(`^\d{1,6}$`),
}

// isoDateValidator valida uma data civil ISO 8601 completa, incluindo os
// limites reais de mês/dia; regex aceitaria valores como 2026-02-31.
func isoDateValidator(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	parsed, err := time.Parse(time.DateOnly, value)
	return err == nil && parsed.Format(time.DateOnly) == value
}

// TimezoneSet holds the IANA timezones the fiscal configs accept (mirrors the
// frontend BRAZIL_TIMEZONES list).
var TimezoneSet = map[string]struct{}{
	"America/Sao_Paulo": {}, "America/Belem": {}, "America/Fortaleza": {},
	"America/Recife": {}, "America/Maceio": {}, "America/Bahia": {},
	"America/Manaus": {}, "America/Cuiaba": {}, "America/Porto_Velho": {},
	"America/Boa_Vista": {}, "America/Rio_Branco": {}, "America/Noronha": {},
}

// ufValidator reports whether the field is a valid Brazilian UF code.
func ufValidator(fl validator.FieldLevel) bool {
	_, ok := UFSet[fl.Field().String()]
	return ok
}

// timezoneValidator reports whether the field is an accepted Brazilian timezone.
func timezoneValidator(fl validator.FieldLevel) bool {
	_, ok := TimezoneSet[fl.Field().String()]
	return ok
}

// cpfValidator validates a Brazilian CPF (11 digits + 2 check digits).
// Punctuation is stripped before validation. Ported from the frontend
// validateCPF so both layers agree.
func cpfValidator(fl validator.FieldLevel) bool {
	return ValidCPF(fl.Field().String())
}

// cnpjValidator validates a Brazilian CNPJ, including the alphanumeric format
// (IN RFB 2229/2024). Ported from the frontend validateCNPJ.
func cnpjValidator(fl validator.FieldLevel) bool {
	return ValidCNPJ(fl.Field().String())
}

// cpfCnpjValidator accepts a value that is a valid CPF OR a valid CNPJ.
func cpfCnpjValidator(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	return ValidCPF(v) || ValidCNPJ(v)
}

var (
	nonDigit = regexp.MustCompile(`\D`)
	nonAlnum = regexp.MustCompile(`[^A-Z0-9]`)
)

// ValidCPF reports whether s is a structurally valid CPF (check digits included).
func ValidCPF(s string) bool {
	clean := nonDigit.ReplaceAllString(s, "")
	if len(clean) != 11 {
		return false
	}
	if allSameByte(clean) {
		return false
	}
	sum := 0
	for i := 1; i <= 9; i++ {
		sum += int(clean[i-1]-'0') * (11 - i)
	}
	rem := (sum * 10) % 11
	if rem == 10 || rem == 11 {
		rem = 0
	}
	if rem != int(clean[9]-'0') {
		return false
	}
	sum = 0
	for i := 1; i <= 10; i++ {
		sum += int(clean[i-1]-'0') * (12 - i)
	}
	rem = (sum * 10) % 11
	if rem == 10 || rem == 11 {
		rem = 0
	}
	return rem == int(clean[10]-'0')
}

// ValidCNPJ reports whether s is a structurally valid CNPJ. Supports the
// alphanumeric format: A–Z map to 10–35, digits to their face value; the two
// check digits (positions 13–14) must be numeric.
func ValidCNPJ(s string) bool {
	clean := nonAlnum.ReplaceAllString(strings.ToUpper(s), "")
	if len(clean) != 14 {
		return false
	}
	if allSameByte(clean) {
		return false
	}
	if clean[12] < '0' || clean[12] > '9' || clean[13] < '0' || clean[13] > '9' {
		return false
	}
	val := func(c byte) int {
		if c >= '0' && c <= '9' {
			return int(c - '0')
		}
		return int(c - 'A' + 10)
	}
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	s1 := 0
	for i := range 12 {
		s1 += val(clean[i]) * w1[i]
	}
	r1 := s1 % 11
	d1 := 0
	if r1 >= 2 {
		d1 = 11 - r1
	}
	if val(clean[12]) != d1 {
		return false
	}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	s2 := 0
	for i := range 13 {
		s2 += val(clean[i]) * w2[i]
	}
	r2 := s2 % 11
	d2 := 0
	if r2 >= 2 {
		d2 = 11 - r2
	}
	return val(clean[13]) == d2
}

// allSameByte reports whether every byte in s is identical (e.g. "00000000000").
func allSameByte(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return len(s) > 0
}

// Validadores de tabela NFS-e. Consultam as tabelas de referência geradas dos
// Anexos B e C, versionadas em go-dfe/nfse/tables — a mesma fonte que a go-dfe
// usa para montar o XML da DPS, para que api e go-dfe nunca divirjam.
func tribNacionalValidator(fl validator.FieldLevel) bool {
	return tables.IsValidTribNacional(fl.Field().String())
}

func nbsValidator(fl validator.FieldLevel) bool {
	return tables.IsValidNBS(fl.Field().String())
}

func indOpValidator(fl validator.FieldLevel) bool {
	return tables.IsValidIndOp(fl.Field().String())
}
