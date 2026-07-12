// Package problem implements RFC 7807 Problem Details, mirroring the Python
// ProblemException hierarchy in api/app/core/exceptions.py.
package problem

import (
	"encoding/json"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

const ContentType = "application/problem+json"

// Problem type URIs (RFC 7807 "type" member). Defined as constants so they are
// never scattered as raw string literals across the codebase.
const (
	TypeBadRequest      = "/problems/bad-request"
	TypeNoCertificate   = "/problems/no-certificate"
	TypeSefazRejection  = "/problems/sefaz-rejection"
	TypeUnauthorized    = "/problems/unauthorized"
	TypeForbidden       = "/problems/forbidden"
	TypeNotFound        = "/problems/not-found"
	TypeConflict        = "/problems/conflict"
	TypeValidation      = "/problems/validation-error"
	TypeTooManyRequests = "/problems/too-many-requests"
	TypeInternalServer  = "/problems/internal-server-error"
)

// FieldError is a single field-level validation failure. It mirrors the shape
// the frontend Zod layer produces so the UI can map each error back to its input.
type FieldError struct {
	Field   string `json:"field"`         // dotted JSON path, e.g. "person.addresses[0].postal_code"
	Message string `json:"message"`       // human-readable message
	Tag     string `json:"tag,omitempty"` // validation rule that failed, e.g. "required", "cnpj"
}

// Problem is the RFC 7807 response body. Errors carries field-level validation
// failures (only populated for validation problems; omitted otherwise).
type Problem struct {
	Type   string       `json:"type"`
	Title  string       `json:"title"`
	Status int          `json:"status"`
	Detail string       `json:"detail,omitempty"`
	Errors []FieldError `json:"errors,omitempty"`
}

// Error implements the error interface so problems can be returned as errors.
func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}
func (p *Problem) Send(c fiber.Ctx) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	c.Status(p.Status)
	c.Set(fiber.HeaderContentType, ContentType)
	return c.Send(b)
}
func New(status int, typ, title, detail string) *Problem {
	return &Problem{Type: typ, Title: title, Status: status, Detail: detail}
}

func BadRequest(detail string) *Problem {
	return New(http.StatusBadRequest, TypeBadRequest, "Bad Request", detail)
}

func NoCertificate(detail string) *Problem {
	return New(http.StatusBadRequest, TypeNoCertificate, "No Certificate Found", detail)
}

func SefazRejection(detail string) *Problem {
	return New(http.StatusBadRequest, TypeSefazRejection, "Sefaz Rejection", detail)
}

func Unauthorized(detail string) *Problem {
	return New(http.StatusUnauthorized, TypeUnauthorized, "Unauthorized", detail)
}

func Forbidden(detail string) *Problem {
	return New(http.StatusForbidden, TypeForbidden, "Forbidden", detail)
}

func NotFound(detail string) *Problem {
	return New(http.StatusNotFound, TypeNotFound, "Not Found", detail)
}

// Validation returns a 422 problem carrying field-level validation failures.
// Used by the request-binding layer when a request body fails struct validation.
func Validation(errs []FieldError) *Problem {
	p := New(http.StatusUnprocessableEntity, TypeValidation, "Validation Error",
		"the request body failed validation")
	p.Errors = errs
	return p
}

func FromFiber(err *fiber.Error) *Problem {
	switch err.Code {
	case http.StatusNotFound:
		return NotFound(err.Error())
	default:
		return InternalServer(err.Error())
	}
}

func Conflict(detail string) *Problem {
	return New(http.StatusConflict, TypeConflict, "Conflict", detail)
}

func TooManyRequests(detail string) *Problem {
	return New(http.StatusTooManyRequests, TypeTooManyRequests, "Too Many Requests", detail)
}

func InternalServer(detail string) *Problem {
	return New(http.StatusInternalServerError, TypeInternalServer, "Internal Server Error", detail)
}
