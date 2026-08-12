// Package validation provides a single shared go-playground/validator instance
// configured with the custom rules used across the API (CPF/CNPJ check digits,
// fiscal field formats, Brazilian UF codes) and a translator that converts
// validation failures into RFC 7807 field-level errors.
//
// Request bodies are validated at the route boundary (see internal/api/v1).
// This package never touches HTTP or DynamoDB — it only validates Go structs.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"gopkg.aoctech.app/dfe/api/internal/problem"

	"github.com/go-playground/validator/v10"
)

var (
	instance *validator.Validate
	once     sync.Once
)

// get lazily builds and returns the shared validator instance.
func get() *validator.Validate {
	once.Do(func() {
		v := validator.New(validator.WithRequiredStructEnabled())

		// Report field paths using JSON tag names so error paths match the
		// payload the client sent (and the frontend Zod schema).
		v.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			if field.Anonymous && name == "" {
				// An embedded struct with no json tag is inlined in the payload,
				// so it must be inlined in the error path too — otherwise the
				// client gets "cfop_config[0].TaxFieldsBody.ibs_cbs_cst" for a
				// field it sent as "cfop_config[0].ibs_cbs_cst".
				return embeddedSegment
			}
			if name == "-" || name == "" {
				return field.Name
			}
			return name
		})

		// Regex-backed format validators (mirror the frontend Zod regexes).
		for tag, re := range regexValidators {
			re := re
			_ = v.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
				return re.MatchString(fl.Field().String())
			})
		}

		// Brazilian document / UF / timezone validators.
		_ = v.RegisterValidation("uf", ufValidator)
		_ = v.RegisterValidation("timezone", timezoneValidator)
		_ = v.RegisterValidation("cpf", cpfValidator)
		_ = v.RegisterValidation("cnpj", cnpjValidator)
		_ = v.RegisterValidation("cpfcnpj", cpfCnpjValidator)
		_ = v.RegisterValidation("isodate", isoDateValidator)
		_ = v.RegisterValidation("dfe_access_key", accessKeyValidator)

		// Validadores de tabela oficial NFS-e (Anexos B e C).
		_ = v.RegisterValidation("tribnac", tribNacionalValidator)
		_ = v.RegisterValidation("nbs", nbsValidator)
		_ = v.RegisterValidation("indop", indOpValidator)

		instance = v
	})
	return instance
}

// RegisterStructRule registers a struct-level validation on the shared
// instance. Callers are the packages that define the DTOs (internal/api/v1) —
// this package cannot import them back (import cycle), so DTO packages call
// this from their own init() instead.
func RegisterStructRule(fn validator.StructLevelFunc, types ...any) {
	get().RegisterStructValidation(fn, types...)
}

// Struct validates v. It returns nil when valid, or an RFC 7807 *problem.Problem
// (HTTP 422) carrying one FieldError per failed rule.
func Struct(v any) *problem.Problem {
	err := get().Struct(v)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if asInvalid(err, &invalid) {
		return problem.InternalServer("validation misconfigured: " + err.Error())
	}
	var ve validator.ValidationErrors
	ok := errors.As(err, &ve)
	if !ok {
		return problem.InternalServer("validation failed: " + err.Error())
	}
	errs := make([]problem.FieldError, 0, len(ve))
	for _, fe := range ve {
		errs = append(errs, problem.FieldError{
			Field:   fieldPath(fe.Namespace()),
			Message: message(fe),
			Tag:     fe.Tag(),
		})
	}
	return problem.Validation(errs)
}

// asInvalid reports whether err is an *InvalidValidationError (programmer error,
// e.g. validating a non-struct). Kept tiny to avoid importing errors twice.
func asInvalid(err error, target **validator.InvalidValidationError) bool {
	if e, ok := errors.AsType[*validator.InvalidValidationError](err); ok {
		*target = e
		return true
	}
	return false
}

// embeddedSegment marks a namespace segment produced by an anonymous struct
// field, which fieldPath drops so the reported path matches the JSON payload.
const embeddedSegment = "\x00embedded"

// fieldPath strips the root struct name from a validator namespace, yielding a
// client-facing JSON path, e.g. "ProductBody.cfop_config[0].cfop" -> "cfop_config[0].cfop".
func fieldPath(namespace string) string {
	if i := strings.IndexByte(namespace, '.'); i >= 0 {
		namespace = namespace[i+1:]
	}
	if !strings.Contains(namespace, embeddedSegment) {
		return namespace
	}
	segments := strings.Split(namespace, ".")
	kept := segments[:0]
	for _, seg := range segments {
		if seg != embeddedSegment {
			kept = append(kept, seg)
		}
	}
	return strings.Join(kept, ".")
}

// message renders a Portuguese human-readable message for a single failure.
func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "campo obrigatório"
	case "required_if", "required_with", "required_without":
		return "campo obrigatório neste contexto"
	case "required_with_group":
		return "obrigatório quando outro campo do grupo IBS/CBS está preenchido"
	case "min":
		return fmt.Sprintf("mínimo de %s", fe.Param())
	case "max":
		return fmt.Sprintf("máximo de %s", fe.Param())
	case "len":
		return fmt.Sprintf("deve ter exatamente %s caracteres", fe.Param())
	case "oneof":
		return fmt.Sprintf("valor inválido (esperado um de: %s)", fe.Param())
	case "email":
		return "e-mail inválido"
	case "numeric":
		return "deve conter apenas dígitos"
	case "gt":
		return fmt.Sprintf("deve ser maior que %s", fe.Param())
	case "gte":
		return fmt.Sprintf("deve ser maior ou igual a %s", fe.Param())
	case "dive", "unique":
		return "lista inválida"
	case "uf":
		return "UF inválida"
	case "timezone":
		return "fuso horário inválido"
	case "money", "money2":
		return "valor monetário inválido"
	case "cpf":
		return "CPF inválido"
	case "cnpj":
		return "CNPJ inválido"
	case "cpfcnpj":
		return "CPF/CNPJ inválido"
	case "dfe_access_key":
		return "chave de acesso inválida"
	case "cfop":
		return "CFOP deve ter 4 dígitos"
	case "ncm":
		return "NCM deve ter 8 dígitos"
	case "cest":
		return "CEST deve ter 7 dígitos"
	case "ibge":
		return "código IBGE deve ter 7 dígitos"
	case "cep":
		return "CEP deve ter 8 dígitos"
	case "phonebr":
		return "telefone inválido (10 ou 11 dígitos)"
	case "placa":
		return "placa Mercosul inválida (ex: ABC1D23)"
	case "rntrc":
		return "RNTRC deve ter 8 a 12 dígitos"
	case "renavam":
		return "RENAVAM deve ter 9 a 11 dígitos"
	case "unit":
		return "unidade inválida (1 a 6 letras A–Z)"
	case "decimalv":
		return "valor numérico inválido"
	case "percent":
		return "percentual inválido"
	case "serie":
		return "série deve ter 1 a 3 dígitos"
	case "isodate":
		return "data inválida (formato AAAA-MM-DD)"
	case "inscmun":
		return "inscrição municipal deve ter de 1 a 15 dígitos"
	case "caepf":
		return "CAEPF deve ter 14 dígitos"
	case "nif":
		return "NIF inválido (até 40 caracteres alfanuméricos)"
	case "cnae":
		return "CNAE deve ter 7 dígitos"
	case "tribnac":
		return "código de tributação nacional inexistente na lista de serviços"
	case "nbs":
		return "código NBS inexistente (informe sem pontos)"
	case "indop":
		return "código indOp inexistente na tabela do Anexo C"
	default:
		return "valor inválido para a regra '" + fe.Tag() + "'"
	}
}
